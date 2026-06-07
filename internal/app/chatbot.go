package app

import (
	"context"
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

// ChatBot 负责把 CLI 输入、会话上下文和模型调用串成一次完整对话。
type ChatBot struct {
	client     llm.Client
	session    *session.Session
	builder    *contextwindow.Builder
	summarizer *contextwindow.Summarizer
	contextCfg config.ContextConfig
	executor   *tools.Executor
}

// NewChatBot 创建一个可执行多轮对话的 ChatBot 实例。
func NewChatBot(client llm.Client, sess *session.Session, contextCfg config.ContextConfig) *ChatBot {
	return NewChatBotWithTools(client, sess, contextCfg, nil)
}

// NewChatBotWithTools 创建一个带自定义工具执行器的 ChatBot，主要用于测试或后续真实搜索客户端接入。
func NewChatBotWithTools(client llm.Client, sess *session.Session, contextCfg config.ContextConfig, executor *tools.Executor) *ChatBot {
	if executor == nil {
		executor = tools.NewDefaultExecutorForCurrentWorkspace(nil)
	}
	sess.AppendSystemPromptSection(buildToolInstructions(executor))
	return &ChatBot{
		client:     client,
		session:    sess,
		builder:    contextwindow.NewBuilder(),
		summarizer: contextwindow.NewSummarizer(client),
		contextCfg: contextCfg,
		executor:   executor,
	}
}

// Send 执行一轮对话：记录用户输入，调用模型，再把回复写回会话。
// 首版 Tool Calling 的设计边界是“单轮最多一次工具调用”：
// 第一次模型响应可以是普通文本，也可以是结构化 tool_call；
// 如果是 tool_call，则执行工具并把 tool_result 回填给模型生成最终文本。
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

	resp, err := b.client.Chat(ctx, buildResult.Messages)
	if err != nil {
		log.Printf("[chatbot] request_id=%s 模型调用失败: %v", requestID, err)
		return "", err
	}

	// 识别工具调用只发生在第一次模型响应后。
	// ParseToolCall 返回 ErrToolCallNotPresent 时表示普通回复，不应当视为错误。
	if call, err := tools.ParseToolCall(resp.Content); err == nil {
		log.Printf("[chatbot] request_id=%s 检测到工具调用 tool=%s reason=%q", requestID, call.ToolName, call.Reason)
		resp, err = b.completeToolCall(ctx, buildResult.Messages, *call)
		if err != nil {
			log.Printf("[chatbot] request_id=%s 工具调用闭环失败: %v", requestID, err)
			return "", err
		}
	} else if errors.Is(err, tools.ErrMultipleToolCalls) {
		return "", tools.ErrMultipleToolCalls
	} else if !errors.Is(err, tools.ErrToolCallNotPresent) {
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

	builder.WriteString("\n约束：每轮最多调用一个工具；不要请求未列出的工具；如果上下文中已经包含“工具执行结果”，说明工具调用阶段已经结束，你必须直接给出最终回答，禁止再次输出 tool_call JSON。")
	return builder.String()
}

// completeToolCall 执行单次工具调用闭环。
// requestMessages 是已经通过上下文预算控制后的基础模型请求，不包含第一次请求专用的工具说明。
// 工具结果只追加到本轮二次请求中，
// 不直接写入 Session，避免内部协议污染用户可见历史。
func (b *ChatBot) completeToolCall(ctx context.Context, requestMessages []llm.Message, call tools.ToolCall) (*llm.ChatResponse, error) {
	if b.executor == nil {
		return nil, errors.New("tool executor is not configured")
	}

	result := b.executor.Execute(ctx, call)
	log.Printf(
		"[chatbot] request_id=%s 工具执行完成 tool=%s success=%t error=%q",
		requestctx.FromContext(ctx),
		call.ToolName,
		result.Success,
		result.Error,
	)

	messages := append([]llm.Message(nil), requestMessages...)
	// 用 assistant 消息保留“模型请求工具”的语义，再用 system 消息注入工具结果。
	// 这样 provider 仍只接收基础 chat roles，同时模型能区分原始对话和工具反馈。
	messages = append(messages, llm.Message{
		Role:    llm.RoleAssistant,
		Content: "请求调用工具: " + call.ToolName + "\n原因: " + call.Reason,
	})
	messages = append(messages, llm.Message{
		Role: llm.RoleSystem,
		Content: "以下是工具执行结果。你现在处于 FINAL_ANSWER 阶段：必须基于该结果直接生成最终回答，禁止再次请求工具调用，禁止输出 tool_call JSON。\n" +
			tools.FormatToolResult(result),
	})

	resp, err := b.client.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	if _, err := tools.ParseToolCall(resp.Content); err == nil {
		// 首版不递归执行第二个工具。若模型在 FINAL_ANSWER 阶段仍输出 tool_call，
		// 说明它没有遵守回填指令；此时直接用已经拿到的 ToolResult 生成兜底回答。
		log.Printf(
			"[chatbot] request_id=%s FINAL_ANSWER 阶段仍收到 tool_call，使用工具结果兜底回答 tool=%s",
			requestctx.FromContext(ctx),
			call.ToolName,
		)
		return &llm.ChatResponse{Content: fallbackAnswerFromToolResult(call, result)}, nil
	} else if errors.Is(err, tools.ErrMultipleToolCalls) {
		return nil, fmt.Errorf("%w after tool result", tools.ErrMultipleToolCalls)
	} else if !errors.Is(err, tools.ErrToolCallNotPresent) {
		return nil, err
	}

	return resp, nil
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
