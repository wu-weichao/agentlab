package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentlab/internal/config"
	"agentlab/internal/llm"
	"agentlab/internal/session"
	"agentlab/internal/tools"
)

func TestChatBotSendAppendsAssistantOnSuccess(t *testing.T) {
	sess := session.New("system prompt")
	bot := NewChatBot(&scriptedChatClient{
		responses: []scriptedChatResponse{{content: "hello"}},
	}, sess, testContextConfig())

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

func TestChatBotStoresToolInstructionsInSystemPromptAtStartup(t *testing.T) {
	sess := session.New("system prompt")
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{{content: "hello"}},
	}
	bot := NewChatBot(client, sess, testContextConfig())

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
	bot := NewChatBotWithTools(client, sess, testContextConfig(), executor)

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
	bot := NewChatBot(&scriptedChatClient{
		responses: []scriptedChatResponse{{err: errors.New("boom")}},
	}, sess, testContextConfig())

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
	bot := NewChatBotWithTools(client, sess, config.ContextConfig{
		MaxChars:             380,
		KeepRecentTurns:      1,
		SummaryMaxChars:      120,
		EnableRollingSummary: true,
	}, tools.NewExecutor())

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
	bot := NewChatBotWithTools(client, sess, config.ContextConfig{
		MaxChars:             280,
		KeepRecentTurns:      1,
		SummaryMaxChars:      120,
		EnableRollingSummary: true,
	}, tools.NewExecutor())

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
	bot := NewChatBotWithTools(client, sess, testContextConfig(), executor)

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
	if finalRequest[len(finalRequest)-1].Role != llm.RoleSystem {
		t.Fatalf("expected final tool result message to be system, got %s", finalRequest[len(finalRequest)-1].Role)
	}
	if !strings.Contains(finalRequest[0].Content, "可用工具") {
		t.Fatal("tool instructions should live in the global system prompt")
	}
	if !strings.Contains(finalRequest[len(finalRequest)-1].Content, "FINAL_ANSWER") {
		t.Fatalf("expected final request to disable recursive tools, got %q", finalRequest[len(finalRequest)-1].Content)
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
	bot := NewChatBotWithTools(client, sess, testContextConfig(), executor)

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
	bot := NewChatBotWithTools(client, sess, testContextConfig(), executor)

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
	bot := NewChatBotWithTools(client, sess, testContextConfig(), executor)

	reply, err := bot.Send(context.Background(), "查一下 tool calling")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "搜索工具返回了 1 条结果。" {
		t.Fatalf("unexpected reply: %q", reply)
	}
}

func TestChatBotSendFallsBackWhenFinalAnswerRequestsToolAgain(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(tools.NewCalculatorTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	client := &scriptedChatClient{
		responses: []scriptedChatResponse{
			{content: `{"tool_name":"calculator","arguments":{"expression":"1 + 1"}}`},
			{content: `{"tool_name":"calculator","arguments":{"expression":"2 + 2"}}`},
		},
	}
	bot := NewChatBotWithTools(client, sess, testContextConfig(), executor)

	reply, err := bot.Send(context.Background(), "算一下")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "计算结果是 2。" {
		t.Fatalf("unexpected fallback reply: %q", reply)
	}
	history := sess.History()
	if len(history) != 3 || history[len(history)-1].Role != llm.RoleAssistant {
		t.Fatalf("expected fallback assistant appended, got %#v", history)
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
	bot := NewChatBotWithTools(client, sess, testContextConfig(), executor)

	_, err := bot.Send(context.Background(), "算一下")
	if err == nil {
		t.Fatal("expected error")
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
	responses []scriptedChatResponse
	calls     [][]llm.Message
}

type scriptedSearchClient struct {
	results []tools.SearchResult
	err     error
}

func (c scriptedSearchClient) Search(_ context.Context, _ string, _ int) ([]tools.SearchResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.results, nil
}

func (c *scriptedChatClient) Chat(_ context.Context, messages []llm.Message) (*llm.ChatResponse, error) {
	c.calls = append(c.calls, append([]llm.Message(nil), messages...))
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

func testContextConfig() config.ContextConfig {
	return config.ContextConfig{
		MaxChars:             12000,
		KeepRecentTurns:      6,
		SummaryMaxChars:      2400,
		EnableRollingSummary: true,
	}
}
