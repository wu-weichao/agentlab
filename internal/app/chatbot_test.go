package app

import (
	"context"
	"errors"
	"testing"

	"agentlab/internal/config"
	"agentlab/internal/llm"
	"agentlab/internal/session"
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
	bot := NewChatBot(client, sess, config.ContextConfig{
		MaxChars:             380,
		KeepRecentTurns:      1,
		SummaryMaxChars:      120,
		EnableRollingSummary: true,
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
	bot := NewChatBot(client, sess, config.ContextConfig{
		MaxChars:             280,
		KeepRecentTurns:      1,
		SummaryMaxChars:      120,
		EnableRollingSummary: true,
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

type scriptedChatResponse struct {
	content string
	err     error
}

type scriptedChatClient struct {
	responses []scriptedChatResponse
	calls     [][]llm.Message
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
