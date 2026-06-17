package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"agentlab/internal/config"
	"agentlab/internal/contextwindow"
	"agentlab/internal/llm"
	"agentlab/internal/requestctx"
	"agentlab/internal/session"
	"agentlab/internal/tools"
)

const DefaultMaxToolLoopSteps = 3

var ErrToolLoopExceeded = errors.New("tool loop exceeded max steps")

type ToolLoopOptions struct {
	MaxSteps int
}

type ChatBotOptions struct {
	Client          llm.Client
	Session         *session.Session
	ContextConfig   config.ContextConfig
	Executor        *tools.Executor
	ToolLoopOptions ToolLoopOptions
}

// ChatBot 负责把 CLI 输入、会话上下文和模型调用串成一次完整对话。
type ChatBot struct {
	client          llm.Client
	session         *session.Session
	builder         *contextwindow.Builder
	summarizer      *contextwindow.Summarizer
	contextCfg      config.ContextConfig
	executor        *tools.Executor
	toolLoopOptions ToolLoopOptions
}

// NewChatBot 创建一个可执行多轮对话的 ChatBot 实例。
// executor 为 nil 时使用当前工作区的默认工具集合；toolLoopOptions 为空值时使用默认工具循环配置。
func NewChatBot(options ChatBotOptions) *ChatBot {
	if options.Executor == nil {
		options.Executor = tools.NewDefaultExecutorForCurrentWorkspace(nil)
	}
	options.Session.AppendSystemPromptSection(buildToolInstructions(options.Executor))
	return &ChatBot{
		client:          options.Client,
		session:         options.Session,
		builder:         contextwindow.NewBuilder(),
		summarizer:      contextwindow.NewSummarizer(options.Client),
		contextCfg:      options.ContextConfig,
		executor:        options.Executor,
		toolLoopOptions: normalizeToolLoopOptions(options.ToolLoopOptions),
	}
}

func normalizeToolLoopOptions(options ToolLoopOptions) ToolLoopOptions {
	if options.MaxSteps <= 0 {
		options.MaxSteps = DefaultMaxToolLoopSteps
	}
	return options
}

// Send 执行一轮对话：记录用户输入，调用模型和受控工具循环，再把最终回复写回会话。
func (b *ChatBot) Send(ctx context.Context, input string) (string, error) {
	ctx, requestID := requestctx.WithNewRequestID(ctx)
	trimmed := strings.TrimSpace(input)
	log.Printf("[chatbot] request_id=%s 收到用户输入，长度=%d", requestID, len(trimmed))

	// 先写入 user 消息，再基于最新会话状态做预算控制和摘要更新。
	b.session.AddUserMessage(trimmed)

	buildResult, err := b.prepareRequest(ctx)
	if err != nil {
		log.Printf("[chatbot] request_id=%s 上下文构建失败: %v", requestID, err)
		return "", err
	}

	runCache := map[tools.ToolCallKey]tools.ToolResult{}
	resp, err := b.runToolLoop(ctx, buildResult.Messages, runCache)
	if err != nil {
		log.Printf("[chatbot] request_id=%s 工具循环失败: %v", requestID, err)
		return "", err
	}
	b.session.AddAssistantMessage(resp.Content)
	log.Printf("[chatbot] request_id=%s 本轮对话完成，回复长度=%d", requestID, len(resp.Content))
	return resp.Content, nil
}

func buildToolInstructions(executor *tools.Executor) string {
	if executor == nil {
		return ""
	}
	specs := executor.Specs()
	if len(specs) == 0 {
		return ""
	}
	return renderToolInstructions(specs)
}

func renderToolInstructions(specs []tools.ToolSpec) string {
	var builder strings.Builder
	builder.WriteString("[工具能力]\n")
	builder.WriteString("即使模型内部进行 thinking/reasoning，最终给用户看的回答也必须输出在 assistant message 的 content 中，不能只输出在 reasoning_content 或 thinking 字段中。\n")
	builder.WriteString("你可以在确实需要外部能力时调用工具。若不需要工具，请直接正常回答。\n")
	builder.WriteString("如果需要调用工具，你的本次回复必须只输出一个 JSON 对象，不要包含解释、Markdown 或其他文本。\n")
	builder.WriteString("JSON 格式如下：\n")
	builder.WriteString(`{"tool_name":"工具名","arguments":{"参数名":"参数值"},"reason":"调用原因"}`)
	builder.WriteString("\n\n可用工具：\n")

	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	for _, spec := range specs {
		builder.WriteString("- ")
		builder.WriteString(spec.Name)
		builder.WriteString(": ")
		builder.WriteString(spec.Description)
		if len(spec.Parameters) == 0 {
			builder.WriteString("。参数：无")
			builder.WriteString("\n")
			continue
		}
		builder.WriteString("。参数：")
		for i, param := range spec.Parameters {
			if i > 0 {
				builder.WriteString("；")
			}
			builder.WriteString(param.Name)
			builder.WriteString("(")
			builder.WriteString(param.Type)
			if param.Required {
				builder.WriteString(", required")
			} else {
				builder.WriteString(", optional")
			}
			builder.WriteString(")")
			if strings.TrimSpace(param.Description) != "" {
				builder.WriteString(": ")
				builder.WriteString(param.Description)
			}
		}
		builder.WriteString("\n")
	}

	builder.WriteString("\n约束：每次回复最多请求一个工具；不要请求未列出的工具；不要重复请求相同工具和相同参数；如果已有工具结果足以回答用户问题，必须直接给出最终回答；只有当已有工具结果仍不足以完成回答时，才继续请求另一个工具。")
	return builder.String()
}

// runToolLoop 在同一次用户请求内执行受 MaxSteps 限制的顺序工具循环。
// 工具轨迹只保存在本轮工作消息中，不写入 Session。
func (b *ChatBot) runToolLoop(
	ctx context.Context,
	requestMessages []llm.Message,
	runCache map[tools.ToolCallKey]tools.ToolResult,
) (*llm.ChatResponse, error) {
	messages := append([]llm.Message(nil), requestMessages...)
	maxSteps := normalizeToolLoopOptions(b.toolLoopOptions).MaxSteps

	for step := 1; step <= maxSteps; step++ {
		resp, err := b.client.Chat(ctx, messages)
		if err != nil {
			log.Printf("[chatbot] request_id=%s 模型调用失败 step=%d: %v", requestctx.FromContext(ctx), step, err)
			return nil, err
		}

		call, err := tools.ParseToolCall(resp.Content)
		if errors.Is(err, tools.ErrToolCallNotPresent) {
			return resp, nil
		}
		if errors.Is(err, tools.ErrMultipleToolCalls) {
			return nil, tools.ErrMultipleToolCalls
		}
		if err != nil {
			return nil, err
		}

		log.Printf(
			"[chatbot] request_id=%s 检测到工具调用 step=%d/%d tool=%s reason=%q",
			requestctx.FromContext(ctx),
			step,
			maxSteps,
			call.ToolName,
			call.Reason,
		)
		result, err := executeToolWithRunCache(ctx, b.executor, *call, runCache)
		if err != nil {
			return nil, err
		}
		resultJSON := formatToolResultLogJSON(result)
		log.Printf(
			"[chatbot] request_id=%s 工具执行完成 step=%d/%d tool=%s success=%t error=%q tool_result_json=%s",
			requestctx.FromContext(ctx),
			step,
			maxSteps,
			call.ToolName,
			result.Success,
			result.Error,
			resultJSON,
		)
		messages = appendToolFeedback(messages, *call, result)
	}

	return nil, ErrToolLoopExceeded
}

func appendToolFeedback(messages []llm.Message, call tools.ToolCall, result tools.ToolResult) []llm.Message {
	// 用 assistant 消息保留“模型请求工具”的语义，再用 user 消息把工具结果作为下一步输入回填。
	// 一些 OpenAI 兼容模型会弱化后置 system 消息，使用 user 消息能更明确地推动下一步决策。
	messages = append(messages, llm.Message{
		Role:    llm.RoleAssistant,
		Content: "请求调用工具: " + call.ToolName + "\n原因: " + call.Reason,
	})
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: buildToolResultMessage(result),
	})
	return messages
}

func formatToolResultLogJSON(result tools.ToolResult) string {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"marshal tool result log: %s"}`, err)
	}
	return string(data)
}

func buildToolResultMessage(result tools.ToolResult) string {
	return "工具执行结果如下。\n" +
		"如果该结果已经足以回答用户原问题，你现在必须进入 FINAL_ANSWER 阶段并直接回答；不要输出 tool_call JSON；不要输出 Markdown 代码块。\n" +
		"只有当该结果仍不足以完成回答时，才可以继续请求另一个不同工具；禁止重复请求相同工具和相同参数。\n" +
		tools.FormatToolResult(result)
}

func fallbackAnswerFromToolResult(call tools.ToolCall, result tools.ToolResult) string {
	if !result.Success {
		if strings.TrimSpace(result.Error) == "" {
			return "工具 " + call.ToolName + " 执行失败。"
		}
		return "工具 " + call.ToolName + " 执行失败：" + result.Error
	}

	switch call.ToolName {
	case "time":
		date, _ := result.Metadata["date"].(string)
		timeText, _ := result.Metadata["time"].(string)
		timezone, _ := result.Metadata["timezone"].(string)
		if date != "" && timeText != "" && timezone != "" {
			return "当前时间是 " + date + " " + timeText + "（" + timezone + "）。"
		}
	case "calculator":
		if strings.TrimSpace(result.Content) != "" {
			return "计算结果是 " + result.Content + "。"
		}
	case "file_read":
		if strings.TrimSpace(result.Content) != "" {
			return "文件内容如下：\n" + result.Content
		}
	case "web_search":
		if strings.TrimSpace(result.Content) != "" {
			return "搜索结果如下：\n" + result.Content
		}
	}

	if strings.TrimSpace(result.Content) == "" {
		return "工具 " + call.ToolName + " 已执行成功。"
	}
	return result.Content
}

// prepareRequest 负责把“无限增长的会话状态”整理成一次可发送的受控上下文。
// 这里会先做裁剪；若发生驱逐，再更新 summary 并重新构建一次最终请求消息。
func (b *ChatBot) prepareRequest(ctx context.Context) (*contextwindow.BuildResult, error) {
	input := contextwindow.BuildInput{
		SystemPrompt:   b.session.SystemPrompt(),
		RollingSummary: b.session.RollingSummary(),
		RecentMessages: b.session.RecentMessages(),
		Config:         b.contextCfg,
	}

	buildResult, err := b.builder.Build(input)
	if err != nil {
		return b.retryBuildWithCompressedSummary(ctx, err)
	}

	if len(buildResult.EvictedMessages) == 0 {
		b.session.SetRecentMessages(buildResult.RecentMessages)
		log.Printf("[chatbot] request_id=%s 上下文检查完成 budget=%d chars=%d trimmed=false", requestctx.FromContext(ctx), b.contextCfg.MaxChars, buildResult.EstimatedChars)
		return buildResult, nil
	}

	evictedCount := len(buildResult.EvictedMessages)
	b.session.SetRecentMessages(buildResult.RecentMessages)
	if b.contextCfg.EnableRollingSummary {
		// 摘要更新总是基于“旧摘要 + 本轮被驱逐消息”，保持单份滚动摘要持续演进。
		newSummary, err := b.summarizer.Update(ctx, b.session.RollingSummary(), buildResult.EvictedMessages, b.contextCfg.SummaryMaxChars)
		if err != nil {
			return nil, err
		}
		b.session.SetRollingSummary(newSummary)
	}

	buildResult, err = b.builder.Build(contextwindow.BuildInput{
		SystemPrompt:   b.session.SystemPrompt(),
		RollingSummary: b.session.RollingSummary(),
		RecentMessages: b.session.RecentMessages(),
		Config:         b.contextCfg,
	})
	if err != nil {
		return b.retryBuildWithCompressedSummary(ctx, err)
	}

	log.Printf(
		"[chatbot] request_id=%s 上下文检查完成 budget=%d chars=%d trimmed=true evicted_messages=%d summary_updated=%t",
		requestctx.FromContext(ctx),
		b.contextCfg.MaxChars,
		buildResult.EstimatedChars,
		evictedCount,
		b.contextCfg.EnableRollingSummary,
	)
	return buildResult, nil
}

func (b *ChatBot) retryBuildWithCompressedSummary(ctx context.Context, buildErr error) (*contextwindow.BuildResult, error) {
	if !errors.Is(buildErr, contextwindow.ErrContextBudgetExceeded) {
		return nil, buildErr
	}
	if strings.TrimSpace(b.session.RollingSummary()) == "" {
		return nil, buildErr
	}

	beforeChars := len(b.session.RollingSummary())
	target := contextwindow.RemainingSummaryBudget(
		b.session.SystemPrompt(),
		b.session.RecentMessages(),
		b.contextCfg.MaxChars,
	)
	if target <= 0 {
		log.Printf("[chatbot] request_id=%s summary 再压缩跳过 reason=no_remaining_budget budget=%d", requestctx.FromContext(ctx), b.contextCfg.MaxChars)
		return nil, buildErr
	}

	log.Printf(
		"[chatbot] request_id=%s summary 再压缩重试 remaining_summary_budget=%d summary_chars_before=%d budget=%d",
		requestctx.FromContext(ctx),
		target,
		beforeChars,
		b.contextCfg.MaxChars,
	)
	compressed, err := b.summarizer.Compress(ctx, b.session.RollingSummary(), target)
	if err != nil {
		return nil, err
	}
	b.session.SetRollingSummary(compressed)
	log.Printf(
		"[chatbot] request_id=%s summary 再压缩完成 summary_chars_before=%d summary_chars_after=%d remaining_summary_budget=%d",
		requestctx.FromContext(ctx),
		beforeChars,
		len(compressed),
		target,
	)

	retryResult, err := b.builder.Build(contextwindow.BuildInput{
		SystemPrompt:   b.session.SystemPrompt(),
		RollingSummary: b.session.RollingSummary(),
		RecentMessages: b.session.RecentMessages(),
		Config:         b.contextCfg,
	})
	if err != nil {
		return nil, err
	}

	log.Printf(
		"[chatbot] request_id=%s 上下文检查完成 retry_with_summary_compress=true budget=%d chars=%d summary_chars_after=%d",
		requestctx.FromContext(ctx),
		b.contextCfg.MaxChars,
		retryResult.EstimatedChars,
		len(compressed),
	)
	return retryResult, nil
}
