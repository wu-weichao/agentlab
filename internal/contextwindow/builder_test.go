package contextwindow

import (
	"testing"

	"agentlab/internal/config"
	"agentlab/internal/llm"
)

func TestBuildReturnsMessagesWhenWithinBudget(t *testing.T) {
	builder := NewBuilder()
	result, err := builder.Build(BuildInput{
		SystemPrompt:   "system prompt",
		RollingSummary: "",
		RecentMessages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
		Config:         testContextConfig(),
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if result.Trimmed {
		t.Fatal("expected no trimming")
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
}

func TestBuildEvictsOldestCompleteTurns(t *testing.T) {
	builder := NewBuilder()
	result, err := builder.Build(BuildInput{
		SystemPrompt: "system prompt",
		RecentMessages: []llm.Message{
			{Role: llm.RoleUser, Content: "u1 long long long"},
			{Role: llm.RoleAssistant, Content: "a1 long long long"},
			{Role: llm.RoleUser, Content: "u2 keep"},
			{Role: llm.RoleAssistant, Content: "a2 keep"},
			{Role: llm.RoleUser, Content: "u3 current"},
		},
		Config: config.ContextConfig{
			MaxChars:             95,
			KeepRecentTurns:      1,
			SummaryMaxChars:      80,
			EnableRollingSummary: true,
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !result.Trimmed {
		t.Fatal("expected trimming")
	}
	if len(result.EvictedMessages) != 2 {
		t.Fatalf("expected 2 evicted messages, got %d", len(result.EvictedMessages))
	}
	if result.RecentMessages[0].Content != "u2 keep" {
		t.Fatalf("expected oldest turn removed, got first recent %q", result.RecentMessages[0].Content)
	}
	if result.RecentMessages[len(result.RecentMessages)-1].Role != llm.RoleUser {
		t.Fatalf("expected trailing user preserved")
	}
}

func TestBuildReturnsBudgetErrorWhenProtectedContextStillTooLarge(t *testing.T) {
	builder := NewBuilder()
	_, err := builder.Build(BuildInput{
		SystemPrompt:   "system prompt that is too large",
		RollingSummary: "[关键事实]\n- summary is also large",
		RecentMessages: []llm.Message{{Role: llm.RoleUser, Content: "current input that cannot be removed"}},
		Config: config.ContextConfig{
			MaxChars:             20,
			KeepRecentTurns:      1,
			SummaryMaxChars:      10,
			EnableRollingSummary: true,
		},
	})
	if err == nil {
		t.Fatal("expected budget error")
	}
}

func TestBuildDegradesPreferredRecentTurnsWhenOtherwiseNoEvictionPossible(t *testing.T) {
	builder := NewBuilder()
	result, err := builder.Build(BuildInput{
		SystemPrompt: "system prompt",
		RecentMessages: []llm.Message{
			{Role: llm.RoleUser, Content: "u1 old old old old old old old old"},
			{Role: llm.RoleAssistant, Content: "a1 old old old old old old old old"},
			{Role: llm.RoleUser, Content: "u2 recent recent recent recent"},
			{Role: llm.RoleAssistant, Content: "a2 recent recent recent recent"},
			{Role: llm.RoleUser, Content: "u3 current question"},
		},
		Config: config.ContextConfig{
			MaxChars:             150,
			KeepRecentTurns:      6,
			SummaryMaxChars:      80,
			EnableRollingSummary: true,
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !result.Trimmed {
		t.Fatal("expected trimming after degrading preferred recent turns")
	}
	if len(result.EvictedMessages) != 2 {
		t.Fatalf("expected one complete turn to be evicted, got %d messages", len(result.EvictedMessages))
	}
	if result.RecentMessages[0].Content != "u2 recent recent recent recent" {
		t.Fatalf("expected newest complete turn to remain, got first recent %q", result.RecentMessages[0].Content)
	}
}

func testContextConfig() config.ContextConfig {
	return config.ContextConfig{
		MaxChars:             12000,
		KeepRecentTurns:      6,
		SummaryMaxChars:      2400,
		EnableRollingSummary: true,
	}
}
