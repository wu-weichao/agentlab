package session

import (
	"testing"

	"agentlab/internal/llm"
)

func TestNewAddsSystemPrompt(t *testing.T) {
	sess := New("system prompt")

	messages := sess.History()
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

	messages := sess.History()
	if messages[0].Content != renderedPrompt {
		t.Fatalf("expected rendered prompt %q, got %q", renderedPrompt, messages[0].Content)
	}
}

func TestResetKeepsSystemPrompt(t *testing.T) {
	sess := New("system prompt")
	sess.AddUserMessage("hi")
	sess.AddAssistantMessage("hello")

	sess.Reset()

	messages := sess.History()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message after reset, got %d", len(messages))
	}
	if messages[0].Content != "system prompt" {
		t.Fatalf("expected system prompt to remain, got %q", messages[0].Content)
	}
}

func TestAppendSystemPromptSectionSurvivesReset(t *testing.T) {
	sess := New("system prompt")
	sess.AppendSystemPromptSection("[工具能力]\n- time")
	sess.AddUserMessage("hi")

	sess.Reset()

	messages := sess.History()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message after reset, got %d", len(messages))
	}
	if messages[0].Content != "system prompt\n\n[工具能力]\n- time" {
		t.Fatalf("unexpected system prompt: %q", messages[0].Content)
	}
}

func TestResetRestoresRenderedPromptAfterConversation(t *testing.T) {
	renderedPrompt := "你是一个 AI Agent 学习助理。"
	sess := New(renderedPrompt)
	sess.AddUserMessage("你好")
	sess.AddAssistantMessage("你好，我来帮助你。")

	sess.Reset()

	messages := sess.History()
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

func TestHistoryIncludesRollingSummaryBeforeRecentMessages(t *testing.T) {
	sess := New("system prompt")
	sess.SetRollingSummary("[关键事实]\n- 用户想学习 Agent")
	sess.AddUserMessage("你好")

	history := sess.History()
	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}
	if history[1].Role != llm.RoleSummary {
		t.Fatalf("expected second role summary, got %s", history[1].Role)
	}
	if history[2].Role != llm.RoleUser {
		t.Fatalf("expected third role user, got %s", history[2].Role)
	}
}
