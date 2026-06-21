package agent

import (
	"context"
	"errors"
	"log"
	"strings"

	"agentlab/internal/config"
	"agentlab/internal/contextwindow"
	"agentlab/internal/llm"
	"agentlab/internal/requestctx"
	"agentlab/internal/session"
	"agentlab/internal/tools"
)

type Options struct {
	Client          llm.Client
	Session         *session.Session
	ContextConfig   config.ContextConfig
	Executor        *tools.Executor
	ToolLoopOptions ToolLoopOptions
	ToolCallingMode llm.ToolCallingMode
}

// Agent 负责把用户输入、会话上下文、模型调用和工具循环编排成一次完整运行。
type Agent struct {
	client          llm.Client
	session         *session.Session
	builder         *contextwindow.Builder
	summarizer      *contextwindow.Summarizer
	contextCfg      config.ContextConfig
	executor        *tools.Executor
	toolLoopOptions ToolLoopOptions
	toolCallingMode llm.ToolCallingMode
}

// New 创建一个可执行多轮对话的 Agent 实例。
// executor 为 nil 时使用当前工作区的默认工具集合；toolLoopOptions 为空值时使用默认工具循环配置。
func New(options Options) *Agent {
	if options.Executor == nil {
		options.Executor = tools.NewDefaultExecutorForCurrentWorkspace(nil)
	}
	tracedClient := tracingClient{delegate: options.Client}
	mode, err := llm.ParseToolCallingMode(string(options.ToolCallingMode))
	if err != nil {
		mode = llm.DefaultToolCallingMode
	}
	options.Session.AppendSystemPromptSection(buildToolInstructions(options.Executor, mode))
	return &Agent{
		client:          tracedClient,
		session:         options.Session,
		builder:         contextwindow.NewBuilder(),
		summarizer:      contextwindow.NewSummarizer(tracedClient),
		contextCfg:      options.ContextConfig,
		executor:        options.Executor,
		toolLoopOptions: normalizeToolLoopOptions(options.ToolLoopOptions),
		toolCallingMode: mode,
	}
}

// Run 执行一轮对话：记录用户输入，调用模型和受控工具循环，再把最终回复写回会话。
func (b *Agent) Run(ctx context.Context, input string) (string, error) {
	result, err := b.RunWithTrace(ctx, input)
	if err != nil {
		return "", err
	}
	return result.FinalAnswer, nil
}

// RunWithTrace 执行一轮对话，并返回当前运行的结构化步骤轨迹。
func (b *Agent) RunWithTrace(ctx context.Context, input string) (RunResult, error) {
	ctx, requestID := requestctx.WithNewRequestID(ctx)
	recorder := newTraceRecorder(requestID)
	ctx = withTraceRecorder(ctx, recorder)
	trimmed := strings.TrimSpace(input)
	log.Printf("[agent] request_id=%s 收到用户输入，长度=%d", requestID, len(trimmed))

	// 先写入 user 消息，再基于最新会话状态做预算控制和摘要更新。
	b.session.AddUserMessage(trimmed)

	buildResult, err := b.prepareRequest(ctx)
	if err != nil {
		log.Printf("[agent] request_id=%s 上下文构建失败: %v", requestID, err)
		return recorder.result("", TerminationContextError), err
	}

	runCache := map[tools.ToolCallKey]tools.ToolResult{}
	resp, err := b.runToolLoop(ctx, buildResult.Messages, runCache)
	if err != nil {
		log.Printf("[agent] request_id=%s 工具循环失败: %v", requestID, err)
		return recorder.result("", terminationReasonForError(err)), err
	}
	b.session.AddAssistantMessage(resp.Content)
	log.Printf("[agent] request_id=%s 本轮对话完成，回复长度=%d", requestID, len(resp.Content))
	return recorder.result(resp.Content, TerminationCompleted), nil
}

func terminationReasonForError(err error) TerminationReason {
	switch {
	case errors.Is(err, ErrToolLoopExceeded):
		return TerminationMaxStepsExceeded
	case errors.Is(err, tools.ErrMultipleToolCalls),
		errors.Is(err, tools.ErrInvalidArguments):
		return TerminationInvalidToolCall
	case errors.Is(err, errToolRuntime):
		return TerminationToolError
	default:
		return TerminationModelError
	}
}

// prepareRequest 负责把“无限增长的会话状态”整理成一次可发送的受控上下文。
// 这里会先做裁剪；若发生驱逐，再更新 summary 并重新构建一次最终请求消息。
func (b *Agent) prepareRequest(ctx context.Context) (*contextwindow.BuildResult, error) {
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
		log.Printf("[agent] request_id=%s 上下文检查完成 budget=%d chars=%d trimmed=false", requestctx.FromContext(ctx), b.contextCfg.MaxChars, buildResult.EstimatedChars)
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
		"[agent] request_id=%s 上下文检查完成 budget=%d chars=%d trimmed=true evicted_messages=%d summary_updated=%t",
		requestctx.FromContext(ctx),
		b.contextCfg.MaxChars,
		buildResult.EstimatedChars,
		evictedCount,
		b.contextCfg.EnableRollingSummary,
	)
	return buildResult, nil
}

func (b *Agent) retryBuildWithCompressedSummary(ctx context.Context, buildErr error) (*contextwindow.BuildResult, error) {
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
		log.Printf("[agent] request_id=%s summary 再压缩跳过 reason=no_remaining_budget budget=%d", requestctx.FromContext(ctx), b.contextCfg.MaxChars)
		return nil, buildErr
	}

	log.Printf(
		"[agent] request_id=%s summary 再压缩重试 remaining_summary_budget=%d summary_chars_before=%d budget=%d",
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
		"[agent] request_id=%s summary 再压缩完成 summary_chars_before=%d summary_chars_after=%d remaining_summary_budget=%d",
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
		"[agent] request_id=%s 上下文检查完成 retry_with_summary_compress=true budget=%d chars=%d summary_chars_after=%d",
		requestctx.FromContext(ctx),
		b.contextCfg.MaxChars,
		retryResult.EstimatedChars,
		len(compressed),
	)
	return retryResult, nil
}
