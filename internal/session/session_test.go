package session

import (
	"testing"

	"agentlab/internal/llm"
)

func TestNewAddsSystemPrompt(t *testing.T) {
	sess := New("system prompt")

	messages := sess.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Role != llm.RoleSystem {
		t.Fatalf("expected system role, got %s", messages[0].Role)
	}
}

func TestResetKeepsSystemPrompt(t *testing.T) {
	sess := New("system prompt")
	sess.AddUserMessage("hi")
	sess.AddAssistantMessage("hello")

	sess.Reset()

	messages := sess.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message after reset, got %d", len(messages))
	}
	if messages[0].Content != "system prompt" {
		t.Fatalf("expected system prompt to remain, got %q", messages[0].Content)
	}
}
