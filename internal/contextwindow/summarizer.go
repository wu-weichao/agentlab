package contextwindow

import (
	"context"
	"fmt"
	"log"
	"strings"

	"agentlab/internal/llm"
	"agentlab/internal/requestctx"
)

var summarySections = []string{
	"用户目标",
	"已确认约束",
	"已做决策",
	"关键事实",
	"未解决问题",
}

const (
	summarySystemInstruction = `你是一个对话上下文压缩器。你的任务是把旧摘要和被裁剪的对话历史整理成稳定、简洁、结构化的短期记忆。

输出要求：
1. 只输出以下固定分段，且顺序不能改变：
[用户目标]
[已确认约束]
[已做决策]
[关键事实]
[未解决问题]
2. 每个分段使用 bullet 列表；没有内容时写 "- 暂无"。
3. 只保留事实、约束、决策和未完成事项，不要复述寒暄，不要扩写，不要猜测。
4. 如果同一信息重复出现，合并成一条更高层的归纳。
5. 输出必须是纯文本，不要使用代码块。`

	compressionSystemInstruction = `你是一个对话摘要压缩器。你会收到一份已经结构化的摘要，请在不改变固定分段顺序的前提下继续压缩它。

压缩要求：
1. 仍然只输出固定的五个分段和 bullet 列表。
2. 优先删除重复和低价值措辞，保留目标、约束、决策、事实和未决问题。
3. 保持原有事实语义，不要编造新内容。
4. 输出必须是纯文本，不要使用代码块。`
)

// Summarizer 使用 LLM 把“旧摘要 + 被驱逐消息”压缩成新的结构化滚动摘要。
type Summarizer struct {
	client llm.Client
}

// NewSummarizer 创建摘要更新器。
func NewSummarizer(client llm.Client) *Summarizer {
	return &Summarizer{client: client}
}

// Update 基于旧摘要和被驱逐消息生成新的结构化摘要。
// 当首次摘要结果仍超过预算时，会再做一次 summary-of-summary 压缩。
func (s *Summarizer) Update(ctx context.Context, oldSummary string, evicted []llm.Message, maxChars int) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("summarizer client is nil")
	}

	log.Printf(
		"[summary] request_id=%s action=start old_summary_chars=%d evicted_messages=%d evicted_chars=%d max_chars=%d",
		requestctx.FromContext(ctx),
		len(strings.TrimSpace(oldSummary)),
		len(evicted),
		EstimateMessagesChars(evicted),
		maxChars,
	)

	summary, err := s.generate(ctx, summarySystemInstruction, buildSummaryInput(oldSummary, evicted, maxChars))
	if err != nil {
		return "", err
	}
	log.Printf("[summary] request_id=%s action=first_pass_complete chars=%d within_budget=%t", requestctx.FromContext(ctx), len(summary), len(summary) <= maxChars)
	if len(summary) <= maxChars {
		return summary, nil
	}

	log.Printf("[summary] request_id=%s action=second_pass_start chars=%d max_chars=%d", requestctx.FromContext(ctx), len(summary), maxChars)
	compressed, err := s.generate(ctx, compressionSystemInstruction, buildCompressionInput(summary, maxChars))
	if err != nil {
		return "", err
	}
	log.Printf("[summary] request_id=%s action=second_pass_complete chars=%d within_budget=%t", requestctx.FromContext(ctx), len(compressed), len(compressed) <= maxChars)
	if len(compressed) <= maxChars {
		return compressed, nil
	}

	// 最终兜底只做长度裁切，避免因为模型偶发不遵守长度要求而让整轮请求失败。
	fallback := compactText(compressed, maxChars)
	log.Printf("[summary] request_id=%s action=fallback_truncate original_chars=%d final_chars=%d", requestctx.FromContext(ctx), len(compressed), len(fallback))
	return fallback, nil
}

// Compress 仅针对已有 summary 做再次压缩，不引入新的对话原文。
func (s *Summarizer) Compress(ctx context.Context, summary string, maxChars int) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("summarizer client is nil")
	}
	if strings.TrimSpace(summary) == "" {
		return "", nil
	}

	log.Printf("[summary] request_id=%s action=compress_start chars=%d max_chars=%d", requestctx.FromContext(ctx), len(summary), maxChars)
	compressed, err := s.generate(ctx, compressionSystemInstruction, buildCompressionInput(summary, maxChars))
	if err != nil {
		return "", err
	}
	log.Printf("[summary] request_id=%s action=compress_complete chars=%d within_budget=%t", requestctx.FromContext(ctx), len(compressed), len(compressed) <= maxChars)
	if len(compressed) <= maxChars {
		return compressed, nil
	}

	fallback := compactText(compressed, maxChars)
	log.Printf("[summary] request_id=%s action=compress_fallback_truncate original_chars=%d final_chars=%d", requestctx.FromContext(ctx), len(compressed), len(fallback))
	return fallback, nil
}

func (s *Summarizer) generate(ctx context.Context, systemInstruction string, userInput string) (string, error) {
	resp, err := s.client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    llm.RoleSystem,
				Content: systemInstruction,
			},
			{
				Role:    llm.RoleUser,
				Content: userInput,
			},
		},
		ToolCallingMode: llm.ToolCallingModeNative,
	})
	if err != nil {
		return "", fmt.Errorf("generate rolling summary: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

func buildSummaryInput(oldSummary string, evicted []llm.Message, maxChars int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("目标：请把以下内容总结成不超过 %d 个字符的结构化摘要。\n\n", maxChars))
	b.WriteString("旧摘要：\n")
	if strings.TrimSpace(oldSummary) == "" {
		b.WriteString("[用户目标]\n- 暂无\n\n[已确认约束]\n- 暂无\n\n[已做决策]\n- 暂无\n\n[关键事实]\n- 暂无\n\n[未解决问题]\n- 暂无\n")
	} else {
		b.WriteString(oldSummary)
		b.WriteString("\n")
	}

	b.WriteString("\n本次被移出的历史消息：\n")
	b.WriteString(renderTranscript(evicted))
	return b.String()
}

func buildCompressionInput(summary string, maxChars int) string {
	return fmt.Sprintf("请把下面这份结构化摘要继续压缩到不超过 %d 个字符，同时保留最重要的目标、约束、决策、事实和未决问题。\n\n%s", maxChars, summary)
}

// renderTranscript 把被驱逐消息渲染成稳定文本，便于模型做真正的归纳总结。
func renderTranscript(messages []llm.Message) string {
	var b strings.Builder
	for _, message := range messages {
		b.WriteString(string(message.Role))
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(message.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// compactText 统一折叠空白并限制最终兜底文本长度。
func compactText(text string, max int) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len(trimmed) <= max {
		return trimmed
	}
	if max <= 3 {
		return trimmed[:max]
	}
	return trimmed[:max-3] + "..."
}
