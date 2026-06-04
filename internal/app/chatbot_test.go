package app

import (
	"context"
	"errors"
	"testing"

	"agentlab/internal/llm"
	"agentlab/internal/session"
)

func TestChatBotSendAppendsAssistantOnSuccess(t *testing.T) {
	sess := session.New("system prompt")
	bot := NewChatBot(stubClient{
		response: &llm.ChatResponse{Content: "hello"},
	}, sess)

	reply, err := bot.Send(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if reply != "hello" {
		t.Fatalf("expected reply hello, got %q", reply)
	}

	messages := sess.Messages()
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[2].Role != llm.RoleAssistant {
		t.Fatalf("expected last role assistant, got %s", messages[2].Role)
	}
}

func TestChatBotSendDoesNotAppendAssistantOnFailure(t *testing.T) {
	sess := session.New("system prompt")
	bot := NewChatBot(stubClient{
		err: errors.New("boom"),
	}, sess)

	_, err := bot.Send(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error")
	}

	messages := sess.Messages()
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[len(messages)-1].Role != llm.RoleUser {
		t.Fatalf("expected last role user, got %s", messages[len(messages)-1].Role)
	}
}

type stubClient struct {
	response *llm.ChatResponse
	err      error
}

func (s stubClient) Chat(context.Context, []llm.Message) (*llm.ChatResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}
