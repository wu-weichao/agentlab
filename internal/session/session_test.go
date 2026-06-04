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

func TestNewUsesRenderedPromptAsInitialMessage(t *testing.T) {
	renderedPrompt := "你是一个 AI Agent 学习助理。"
	sess := New(renderedPrompt)

	messages := sess.Messages()
	if messages[0].Content != renderedPrompt {
		t.Fatalf("expected rendered prompt %q, got %q", renderedPrompt, messages[0].Content)
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

func TestResetRestoresRenderedPromptAfterConversation(t *testing.T) {
	renderedPrompt := "你是一个 AI Agent 学习助理。"
	sess := New(renderedPrompt)
	sess.AddUserMessage("你好")
	sess.AddAssistantMessage("你好，我来帮助你。")

	sess.Reset()

	messages := sess.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message after reset, got %d", len(messages))
	}
	if messages[0].Role != llm.RoleSystem {
		t.Fatalf("expected system role, got %s", messages[0].Role)
	}
	if messages[0].Content != renderedPrompt {
		t.Fatalf("expected rendered prompt to remain, got %q", messages[0].Content)
	}
}
