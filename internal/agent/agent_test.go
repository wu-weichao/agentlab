package agent

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentlab/internal/config"
	"agentlab/internal/llm"
	"agentlab/internal/requestctx"
	"agentlab/internal/session"
	"agentlab/internal/tools"
)

// 以下别名只用于复用迁移前的行为测试名称和调用形式。
// 生产代码只暴露 Agent、Options、New 和 Run。
type ChatBotOptions = Options
type ChatBot = Agent

func NewChatBot(options Options) *Agent {
	return New(options)
}

func (b *Agent) Send(ctx context.Context, input string) (string, error) {
	return b.Run(ctx, input)
}

func TestChatBotSendAppendsAssistantOnSuccess(t *testing.T) {
	sess := session.New("system prompt")
	bot := newTestChatBot(&scriptedChatClient{
		responses: []scriptedChatResponse{{content: "hello"}},
	}, sess, nil)

	reply, err := bot.Send(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "hello" {
		t.Fatalf("expected reply hello, got %q", reply)
	}

	messages := sess.History()
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[2].Role != llm.RoleAssistant {
		t.Fatalf("expected last role assistant, got %s", messages[2].Role)
	}
}

func TestAgentRunCreatesRequestIDForDirectCall(t *testing.T) {
	sess := session.New("system prompt")
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{{content: "hello"}},
	}
	runtime := New(Options{
		Client:        client,
		Session:       sess,
		ContextConfig: testContextConfig(),
		Executor:      tools.NewExecutor(),
	})

	if _, err := runtime.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(client.requestIDs) != 1 || client.requestIDs[0] == "" {
		t.Fatalf("expected direct Agent.Run to create request_id, got %#v", client.requestIDs)
	}
}

func TestAgentResetPreservesToolInstructionsWithoutDuplicateAppend(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewTimeTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{{content: "hello"}},
	}
	runtime := New(Options{
		Client:        client,
		Session:       sess,
		ContextConfig: testContextConfig(),
		Executor:      executor,
	})

	initialPrompt := sess.SystemPrompt()
	if strings.Count(initialPrompt, "[工具能力]") != 1 {
		t.Fatalf("expected one tool instruction section, got %q", initialPrompt)
	}
	sess.Reset()
	if sess.SystemPrompt() != initialPrompt {
		t.Fatalf("expected reset to preserve tool instructions, got %q", sess.SystemPrompt())
	}
	if _, err := runtime.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if strings.Count(sess.SystemPrompt(), "[工具能力]") != 1 {
		t.Fatalf("Run should not append tool instructions again, got %q", sess.SystemPrompt())
	}
}

func TestChatBotStoresToolInstructionsInSystemPromptAtStartup(t *testing.T) {
	sess := session.New("system prompt")
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{{content: "hello"}},
	}
	bot := newTestChatBot(client, sess, nil)

	if !strings.Contains(sess.SystemPrompt(), "可用工具") {
		t.Fatalf("expected session system prompt to contain tool instructions, got %q", sess.SystemPrompt())
	}
	if _, err := bot.Send(context.Background(), "hi"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(client.calls) != 1 {
		t.Fatalf("expected 1 llm call, got %d", len(client.calls))
	}

	request := client.calls[0]
	first := request[0]
	if first.Role != llm.RoleSystem {
		t.Fatalf("expected system prompt first, got %s", first.Role)
	}
	for _, want := range []string{"可用工具", "calculator", "time", "file_read", "web_search", "tool_name", "arguments"} {
		if !strings.Contains(first.Content, want) {
			t.Fatalf("expected system prompt to contain %q, got %q", want, first.Content)
		}
	}
	for _, want := range []string{"assistant message 的 content", "不能只输出在 reasoning_content", "每次回复最多请求一个工具", "不要重复请求相同工具和相同参数", "已有工具结果足以回答"} {
		if !strings.Contains(first.Content, want) {
			t.Fatalf("expected system prompt to contain sequential tool loop guidance %q, got %q", want, first.Content)
		}
	}
	if strings.Contains(first.Content, "每轮最多调用一个工具") {
		t.Fatalf("system prompt should not keep old single-tool limit, got %q", first.Content)
	}
}

func TestChatBotBuildsToolInstructionsAtStartup(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewTimeTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{{content: "hello"}},
	}
	bot := newTestChatBot(client, sess, executor)

	// ChatBot 启动后再修改 executor，不应改变已缓存的工具说明。
	if err := executor.Register(tools.NewCalculatorTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if _, err := bot.Send(context.Background(), "hi"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	first := client.calls[0][0]
	if !strings.Contains(first.Content, "time") {
		t.Fatalf("expected startup tool instruction to contain time, got %q", first.Content)
	}
	if strings.Contains(first.Content, "calculator") {
		t.Fatalf("tool instruction should be built at startup, got %q", first.Content)
	}
}

func TestChatBotSendDoesNotAppendAssistantOnFailure(t *testing.T) {
	sess := session.New("system prompt")
	bot := newTestChatBot(&scriptedChatClient{
		responses: []scriptedChatResponse{{err: errors.New("boom")}},
	}, sess, nil)

	_, err := bot.Send(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error")
	}

	messages := sess.History()
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[len(messages)-1].Role != llm.RoleUser {
		t.Fatalf("expected last role user, got %s", messages[len(messages)-1].Role)
	}
}

func TestChatBotSendBuildsSummaryWhenBudgetExceeded(t *testing.T) {
	sess := session.New("system prompt")
	sess.AddUserMessage("第一轮用户输入包含很多很多上下文信息，需要被裁剪。第一轮用户输入包含很多很多上下文信息，需要被裁剪。")
	sess.AddAssistantMessage("第一轮助手回复也包含很多很多上下文信息，需要被裁剪。第一轮助手回复也包含很多很多上下文信息，需要被裁剪。")
	sess.AddUserMessage("第二轮用户输入")
	sess.AddAssistantMessage("第二轮助手回复")

	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: "[用户目标]\n- 学习 Agent，并继续当前对话。\n\n[已确认约束]\n- 暂无。\n\n[已做决策]\n- 先裁剪旧轮次并合并历史。\n\n[关键事实]\n- 第一轮旧消息已被折叠进摘要。\n\n[未解决问题]\n- 如何继续优化上下文窗口实现。"},
			{content: "[用户目标]\n- 学习 Agent\n\n[已确认约束]\n- 暂无\n\n[已做决策]\n- 先裁剪旧轮次\n\n[关键事实]\n- 第一轮已折叠\n\n[未解决问题]\n- 继续当前对话"},
			{content: "latest"},
		},
	}
	bot := NewChatBot(ChatBotOptions{
		Client:  client,
		Session: sess,
		ContextConfig: config.ContextConfig{
			MaxChars:             380,
			KeepRecentTurns:      1,
			SummaryMaxChars:      120,
			EnableRollingSummary: true,
		},
		Executor: tools.NewExecutor(),
	})

	reply, err := bot.Send(context.Background(), "第三轮用户输入")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "latest" {
		t.Fatalf("unexpected reply: %q", reply)
	}

	history := sess.History()
	if len(history) < 4 {
		t.Fatalf("expected summary-aware history, got %d messages", len(history))
	}
	if history[1].Role != llm.RoleSummary {
		t.Fatalf("expected summary at index 1, got %s", history[1].Role)
	}
	if len(client.calls) != 3 {
		t.Fatalf("expected 3 llm calls, got %d", len(client.calls))
	}
	finalRequest := client.calls[len(client.calls)-1]
	if len(finalRequest) < 2 {
		t.Fatalf("expected final llm request with summary, got %d messages", len(finalRequest))
	}
	if finalRequest[1].Role != llm.RoleSystem {
		t.Fatalf("expected summary to be rendered as system for llm, got %s", finalRequest[1].Role)
	}
}

func TestChatBotSendCompressesExistingSummaryWhenRebuildStillExceedsBudget(t *testing.T) {
	sess := session.New("system prompt")
	sess.SetRollingSummary("[用户目标]\n- 很长很长的历史摘要，需要继续压缩。很长很长的历史摘要，需要继续压缩。\n\n[已确认约束]\n- 暂无\n\n[已做决策]\n- 暂无\n\n[关键事实]\n- 暂无\n\n[未解决问题]\n- 暂无")
	sess.AddUserMessage("最近一轮用户输入")
	sess.AddAssistantMessage("最近一轮助手回复")

	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: "[用户目标]\n- 压缩后的摘要\n\n[已确认约束]\n- 暂无\n\n[已做决策]\n- 暂无\n\n[关键事实]\n- 暂无\n\n[未解决问题]\n- 暂无"},
			{content: "final reply"},
		},
	}
	bot := NewChatBot(ChatBotOptions{
		Client:  client,
		Session: sess,
		ContextConfig: config.ContextConfig{
			MaxChars:             280,
			KeepRecentTurns:      1,
			SummaryMaxChars:      120,
			EnableRollingSummary: true,
		},
		Executor: tools.NewExecutor(),
	})

	reply, err := bot.Send(context.Background(), "当前用户继续提问")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "final reply" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if len(client.calls) != 2 {
		t.Fatalf("expected 2 llm calls, got %d", len(client.calls))
	}
}

func TestChatBotSendCompletesCalculatorToolCall(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewCalculatorTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"calculator","arguments":{"expression":"1 + 2 * 3"},"reason":"需要准确计算"}`},
			{content: "计算结果是 7。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	reply, err := bot.Send(context.Background(), "算一下 1 + 2 * 3")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "计算结果是 7。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if len(client.calls) != 2 {
		t.Fatalf("expected 2 llm calls, got %d", len(client.calls))
	}
	finalRequest := client.calls[1]
	if finalRequest[len(finalRequest)-1].Role != llm.RoleUser {
		t.Fatalf("expected final tool result message to be user, got %s", finalRequest[len(finalRequest)-1].Role)
	}
	if !strings.Contains(finalRequest[0].Content, "可用工具") {
		t.Fatal("tool instructions should live in the global system prompt")
	}
	if !strings.Contains(finalRequest[len(finalRequest)-1].Content, "FINAL_ANSWER") {
		t.Fatalf("expected final request to guide final answer, got %q", finalRequest[len(finalRequest)-1].Content)
	}
	if !strings.Contains(finalRequest[len(finalRequest)-1].Content, "工具执行结果如下") {
		t.Fatalf("expected final request to include tool result, got %q", finalRequest[len(finalRequest)-1].Content)
	}
	if len(sess.History()) != 3 {
		t.Fatalf("expected user and final assistant history only, got %d", len(sess.History()))
	}
}

func TestChatBotSendCompletesTimeToolCall(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewTimeTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"time","arguments":{},"reason":"需要当前时间"}`},
			{content: "当前时间已查询。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	reply, err := bot.Send(context.Background(), "现在几点")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "当前时间已查询。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
}

func TestChatBotSendCompletesFileReadToolCall(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("file context"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewFileReadTool(dir, 1024)); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"file_read","arguments":{"path":"notes.md"},"reason":"读取本地上下文"}`},
			{content: "文件内容是 file context。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	reply, err := bot.Send(context.Background(), "读 notes")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "文件内容是 file context。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
}

func TestChatBotSendCompletesWebSearchToolCall(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewWebSearchTool(scriptedSearchClient{
		results: []tools.SearchResult{{
			Title:   "AgentLab",
			Snippet: "Tool calling added",
			URL:     "https://example.com",
			Source:  "example",
		}},
	}, 3)); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"web_search","arguments":{"query":"agentlab tool calling","limit":1},"reason":"查询外部信息"}`},
			{content: "搜索工具返回了 1 条结果。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	reply, err := bot.Send(context.Background(), "查一下 tool calling")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "搜索工具返回了 1 条结果。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
}

func TestChatBotSendCompletesSequentialToolCalls(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewCalculatorTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"calculator","arguments":{"expression":"1 + 1"}}`},
			{content: `{"tool_name":"calculator","arguments":{"expression":"2 + 2"}}`},
			{content: "两次计算结果分别是 2 和 4。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	reply, err := bot.Send(context.Background(), "算一下")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "两次计算结果分别是 2 和 4。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if len(client.calls) != 3 {
		t.Fatalf("expected 3 llm calls, got %d", len(client.calls))
	}
	history := sess.History()
	if len(history) != 3 || history[len(history)-1].Role != llm.RoleAssistant {
		t.Fatalf("expected final assistant appended, got %#v", history)
	}
}

func TestChatBotSendDoesNotAppendAssistantWhenSecondModelCallFails(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewCalculatorTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"calculator","arguments":{"expression":"1 + 1"}}`},
			{err: errors.New("second call failed")},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	_, err := bot.Send(context.Background(), "算一下")
	if err == nil {
		t.Fatal("expected error")
	}
	history := sess.History()
	if len(history) != 2 || history[len(history)-1].Role != llm.RoleUser {
		t.Fatalf("expected no assistant appended, got %#v", history)
	}
}

func TestExecuteToolWithRunCacheReusesSameToolCallKey(t *testing.T) {
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	runCache := map[tools.ToolCallKey]tools.ToolResult{}
	first, err := executeToolWithRunCache(context.Background(), executor, tools.ToolCall{
		ToolName: "counting",
		Arguments: map[string]any{
			"a": float64(1),
			"b": "same",
		},
	}, runCache)
	if err != nil {
		t.Fatalf("executeToolWithRunCache returned error: %v", err)
	}
	second, err := executeToolWithRunCache(context.Background(), executor, tools.ToolCall{
		ToolName: "counting",
		Arguments: map[string]any{
			"b": "same",
			"a": int64(1),
		},
	}, runCache)
	if err != nil {
		t.Fatalf("executeToolWithRunCache returned error: %v", err)
	}

	if tool.calls != 1 {
		t.Fatalf("expected tool to execute once, got %d", tool.calls)
	}
	if first.Result.Content != second.Result.Content {
		t.Fatalf("expected cached result, got %q and %q", first.Result.Content, second.Result.Content)
	}
	if first.CacheHit {
		t.Fatal("expected first execution to miss cache")
	}
	if !second.CacheHit {
		t.Fatal("expected second execution to hit cache")
	}
	if first.Key != second.Key {
		t.Fatalf("expected same tool call key, got %#v and %#v", first.Key, second.Key)
	}
}

func TestChatBotSendReusesRunCacheAcrossToolLoop(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"counting","arguments":{"a":1,"b":"same"},"reason":"第一次调用"}`},
			{content: `{"tool_name":"counting","arguments":{"b":"same","a":1.0},"reason":"重复确认"}`},
			{content: "已完成。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	reply, err := bot.Send(context.Background(), "测试重复工具调用")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "已完成。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if tool.calls != 1 {
		t.Fatalf("expected duplicate tool call to hit run cache, got %d executions", tool.calls)
	}
	if len(client.calls) != 3 {
		t.Fatalf("expected 3 llm calls, got %d", len(client.calls))
	}
	secondToolFeedback := client.calls[2][len(client.calls[2])-1].Content
	for _, want := range []string{`"step": 2`, `"cache_hit": true`, `"tool_call_key": "counting:`} {
		if !strings.Contains(secondToolFeedback, want) {
			t.Fatalf("expected cached tool feedback to contain %q, got %q", want, secondToolFeedback)
		}
	}
}

func TestChatBotSendDoesNotReuseRunCacheAcrossRequests(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"counting","arguments":{"a":1},"reason":"第一次请求"}`},
			{content: "第一次完成。"},
			{content: `{"tool_name":"counting","arguments":{"a":1},"reason":"第二次请求"}`},
			{content: "第二次完成。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	if _, err := bot.Send(context.Background(), "第一次"); err != nil {
		t.Fatalf("first Send returned error: %v", err)
	}
	if _, err := bot.Send(context.Background(), "第二次"); err != nil {
		t.Fatalf("second Send returned error: %v", err)
	}
	if tool.calls != 2 {
		t.Fatalf("expected cache to be scoped per Send, got %d executions", tool.calls)
	}
}

func TestChatBotSendReturnsErrToolLoopExceededWithoutAssistant(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"counting","arguments":{"step":1}}`},
			{content: `{"tool_name":"counting","arguments":{"step":2}}`},
			{content: `{"tool_name":"counting","arguments":{"step":3}}`},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	_, err := bot.Send(context.Background(), "一直调用工具")
	if !errors.Is(err, ErrToolLoopExceeded) {
		t.Fatalf("expected ErrToolLoopExceeded, got %v", err)
	}
	history := sess.History()
	if len(history) != 2 || history[len(history)-1].Role != llm.RoleUser {
		t.Fatalf("expected no assistant appended after max steps, got %#v", history)
	}
	if tool.calls != DefaultMaxToolLoopSteps {
		t.Fatalf("expected %d tool executions, got %d", DefaultMaxToolLoopSteps, tool.calls)
	}
}

func TestChatBotSendUsesConfiguredMaxSteps(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"counting","arguments":{"step":1}}`},
		},
	}
	bot := NewChatBot(ChatBotOptions{
		Client:        client,
		Session:       sess,
		ContextConfig: testContextConfig(),
		Executor:      executor,
		ToolLoopOptions: ToolLoopOptions{
			MaxSteps: 1,
		},
	})

	_, err := bot.Send(context.Background(), "只允许一步")
	if !errors.Is(err, ErrToolLoopExceeded) {
		t.Fatalf("expected ErrToolLoopExceeded, got %v", err)
	}
	if tool.calls != 1 {
		t.Fatalf("expected 1 tool execution, got %d", tool.calls)
	}
}

func TestChatBotSendAddsDiagnosticsToFailedToolResult(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(&failingTool{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"failing","arguments":{},"reason":"验证失败路径"}`},
			{content: "工具失败已说明。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	if _, err := bot.Send(context.Background(), "触发失败工具"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	feedback := client.calls[1][len(client.calls[1])-1].Content
	for _, want := range []string{`"step": 1`, `"cache_hit": false`, `"tool_call_key": "failing:`, "planned failure"} {
		if !strings.Contains(feedback, want) {
			t.Fatalf("expected failed tool feedback to contain %q, got %q", want, feedback)
		}
	}
}

func TestChatBotToolLoopStepLogAvoidsFullToolResultContent(t *testing.T) {
	var buf bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})

	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &contentTool{content: "sensitive file content"}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"content","arguments":{"path":"secret.txt"},"reason":"读取内容"}`},
			{content: "完成。"},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	if _, err := bot.Send(context.Background(), "测试日志"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	logText := buf.String()
	for _, want := range []string{
		"tool_loop_step=true",
		"step=1",
		"max_steps=3",
		"tool_name=content",
		"tool_call_key=content:",
		"cache_hit=false",
		"success=true",
		"error_code=",
		"tool_result_json=",
		`"content":""`,
		`"content_chars":22`,
		`"cache_hit":false`,
		`"step":1`,
		`"tool_call_key":"content:`,
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected log to contain %q, got %q", want, logText)
		}
	}
	if strings.Contains(logText, "sensitive file content") || strings.Contains(logText, "secret.txt") {
		t.Fatalf("step log should not include full content or arguments, got %q", logText)
	}
}

func TestChatBotSendRejectsMultipleToolCallsWithoutExecuting(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_calls":[{"tool_name":"counting","arguments":{"a":1}},{"tool_name":"counting","arguments":{"a":2}}]}`},
		},
	}
	bot := newTestChatBot(client, sess, executor)

	_, err := bot.Send(context.Background(), "一次请求多个工具")
	if !errors.Is(err, tools.ErrMultipleToolCalls) {
		t.Fatalf("expected ErrMultipleToolCalls, got %v", err)
	}
	if tool.calls != 0 {
		t.Fatalf("expected multiple tool calls to execute no tools, got %d", tool.calls)
	}
	history := sess.History()
	if len(history) != 2 || history[len(history)-1].Role != llm.RoleUser {
		t.Fatalf("expected no assistant appended, got %#v", history)
	}
}

type scriptedChatResponse struct {
	content string
	err     error
}

type scriptedChatClient struct {
	responses  []scriptedChatResponse
	calls      [][]llm.Message
	requestIDs []string
}

type scriptedSearchClient struct {
	results []tools.SearchResult
	err     error
}

type countingTool struct {
	calls int
}

type failingTool struct{}

type contentTool struct {
	content string
}

func (t *countingTool) Name() string {
	return "counting"
}

func (t *countingTool) Description() string {
	return "counts executions"
}

func (t *countingTool) Parameters() []tools.Parameter {
	return nil
}

func (t *countingTool) Execute(_ context.Context, _ map[string]any) tools.ToolResult {
	t.calls++
	return tools.ToolResult{
		Success: true,
		Content: "execution result",
		Metadata: map[string]any{
			"calls": t.calls,
		},
	}
}

func (t *failingTool) Name() string {
	return "failing"
}

func (t *failingTool) Description() string {
	return "fails for tests"
}

func (t *failingTool) Parameters() []tools.Parameter {
	return nil
}

func (t *failingTool) Execute(_ context.Context, _ map[string]any) tools.ToolResult {
	return tools.ToolResult{
		Success: false,
		Error:   "planned failure",
		Metadata: map[string]any{
			"error_code": "planned",
		},
	}
}

func (t *contentTool) Name() string {
	return "content"
}

func (t *contentTool) Description() string {
	return "returns content"
}

func (t *contentTool) Parameters() []tools.Parameter {
	return []tools.Parameter{{
		Name:     "path",
		Type:     "string",
		Required: true,
	}}
}

func (t *contentTool) Execute(_ context.Context, _ map[string]any) tools.ToolResult {
	return tools.ToolResult{
		Success: true,
		Content: t.content,
		Metadata: map[string]any{
			"source": "fixture",
		},
	}
}

func (c scriptedSearchClient) Search(_ context.Context, _ string, _ int) ([]tools.SearchResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.results, nil
}

func (c *scriptedChatClient) Chat(ctx context.Context, messages []llm.Message) (*llm.ChatResponse, error) {
	c.calls = append(c.calls, append([]llm.Message(nil), messages...))
	c.requestIDs = append(c.requestIDs, requestctx.FromContext(ctx))
	if len(c.responses) == 0 {
		return nil, errors.New("no scripted response")
	}

	next := c.responses[0]
	c.responses = c.responses[1:]
	if next.err != nil {
		return nil, next.err
	}
	return &llm.ChatResponse{Content: next.content}, nil
}

func newTestChatBot(client llm.Client, sess *session.Session, executor *tools.Executor) *ChatBot {
	return NewChatBot(ChatBotOptions{
		Client:        client,
		Session:       sess,
		ContextConfig: testContextConfig(),
		Executor:      executor,
	})
}

func testContextConfig() config.ContextConfig {
	return config.ContextConfig{
		MaxChars:             12000,
		KeepRecentTurns:      6,
		SummaryMaxChars:      2400,
		EnableRollingSummary: true,
	}
}
