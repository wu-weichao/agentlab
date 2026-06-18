package app

import (
	"context"
	"errors"
	"testing"

	"agentlab/internal/config"
	"agentlab/internal/llm"
	"agentlab/internal/session"
	"agentlab/internal/tools"
)

func TestChatBotSendDelegatesToAgent(t *testing.T) {
	sess := session.New("system prompt")
	bot := NewChatBot(ChatBotOptions{
		Client:        &stubClient{content: "hello"},
		Session:       sess,
		ContextConfig: testContextConfig(),
		Executor:      tools.NewExecutor(),
	})

	reply, err := bot.Send(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "hello" {
		t.Fatalf("expected hello, got %q", reply)
	}
	history := sess.History()
	if len(history) != 3 || history[2].Role != llm.RoleAssistant {
		t.Fatalf("expected compatible session writeback, got %#v", history)
	}
}

func TestChatBotSendReturnsAgentErrorWithoutAssistant(t *testing.T) {
	sess := session.New("system prompt")
	wantErr := errors.New("planned failure")
	bot := NewChatBot(ChatBotOptions{
		Client:        &stubClient{err: wantErr},
		Session:       sess,
		ContextConfig: testContextConfig(),
		Executor:      tools.NewExecutor(),
	})

	_, err := bot.Send(context.Background(), "hi")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	history := sess.History()
	if len(history) != 2 || history[1].Role != llm.RoleUser {
		t.Fatalf("expected no assistant on failure, got %#v", history)
	}
}

func TestChatBotOptionsMapConfiguredMaxSteps(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(&countingTool{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	bot := NewChatBot(ChatBotOptions{
		Client: &stubClient{
			content: `{"tool_name":"counting","arguments":{}}`,
		},
		Session:       sess,
		ContextConfig: testContextConfig(),
		Executor:      executor,
		ToolLoopOptions: ToolLoopOptions{
			MaxSteps: 1,
		},
	})

	_, err := bot.Send(context.Background(), "call tool")
	if !errors.Is(err, ErrToolLoopExceeded) {
		t.Fatalf("expected ErrToolLoopExceeded, got %v", err)
	}
}

type stubClient struct {
	content string
	err     error
}

func (c *stubClient) Chat(_ context.Context, _ []llm.Message) (*llm.ChatResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &llm.ChatResponse{Content: c.content}, nil
}

type countingTool struct{}

func (t *countingTool) Name() string {
	return "counting"
}

func (t *countingTool) Description() string {
	return "counts"
}

func (t *countingTool) Parameters() []tools.Parameter {
	return nil
}

func (t *countingTool) Execute(_ context.Context, _ map[string]any) tools.ToolResult {
	return tools.ToolResult{Success: true, Content: "done"}
}

func testContextConfig() config.ContextConfig {
	return config.ContextConfig{
		MaxChars:             12000,
		KeepRecentTurns:      6,
		SummaryMaxChars:      2400,
		EnableRollingSummary: true,
	}
}
