package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsPromptTemplateConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, validConfigYAML(
		"你是{{role.name}}，职责是{{role.goal}}。",
		"AI Agent 学习助理",
		"帮助用户理解 Agent",
		"简洁",
		"中文",
	))

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLM.APIKey != "real-key" {
		t.Fatalf("expected config api key, got %q", cfg.LLM.APIKey)
	}
	if cfg.Chat.Prompt.Template != "你是{{role.name}}，职责是{{role.goal}}。\n" {
		t.Fatalf("unexpected template: %q", cfg.Chat.Prompt.Template)
	}
	if cfg.Chat.Prompt.Role.Name != "AI Agent 学习助理" {
		t.Fatalf("unexpected role name: %q", cfg.Chat.Prompt.Role.Name)
	}
	if cfg.Chat.Prompt.Context.Language != "中文" {
		t.Fatalf("unexpected language: %q", cfg.Chat.Prompt.Context.Language)
	}
	if cfg.Chat.Context.MaxChars != 12000 {
		t.Fatalf("unexpected max chars: %d", cfg.Chat.Context.MaxChars)
	}
}

func TestValidateRejectsMissingAPIKey(t *testing.T) {
	cfg := validConfig()
	cfg.LLM.APIKey = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRejectsPlaceholderAPIKey(t *testing.T) {
	cfg := validConfig()
	cfg.LLM.APIKey = "{your-openai-api-key}"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for placeholder api key")
	}
}

func TestValidateRejectsMissingPromptTemplate(t *testing.T) {
	cfg := validConfig()
	cfg.Chat.Prompt.Template = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing prompt template")
	}
}

func TestValidateRejectsEmptyStandardVariable(t *testing.T) {
	cfg := validConfig()
	cfg.Chat.Prompt.Role.Name = " "

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty role.name")
	}
}

func TestValidateRejectsMissingTemplateVariable(t *testing.T) {
	cfg := validConfig()
	cfg.Chat.Prompt.Template = "你是{{role.name}}，目标是{{role.missing}}。"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing template variable")
	}
}

func TestValidateRejectsInvalidContextConfig(t *testing.T) {
	cfg := validConfig()
	cfg.Chat.Context.SummaryMaxChars = cfg.Chat.Context.MaxChars

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for invalid context config")
	}
}

func TestLoadRejectsDeprecatedMaxHistoryMessages(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, validConfigYAMLWithMaxHistory(
		"你是{{role.name}}，职责是{{role.goal}}。",
		"AI Agent 学习助理",
		"帮助用户理解 Agent",
		"简洁",
		"中文",
	))

	if _, err := Load(path); err == nil {
		t.Fatal("expected deprecated field error")
	}
}

func validConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			BaseURL:     "https://api.openai.com/v1",
			APIKey:      "real-key",
			Temperature: 0.5,
		},
		Chat: ChatConfig{
			Prompt: PromptConfig{
				Template: "你是{{role.name}}，请使用{{context.language}}回答。",
				Role: PromptRoleConfig{
					Name:  "AI Agent 学习助理",
					Goal:  "帮助用户理解 Agent",
					Style: "简洁",
				},
				Context: PromptContextConfig{
					Language: "中文",
				},
			},
			Context: ContextConfig{
				MaxChars:             12000,
				KeepRecentTurns:      6,
				SummaryMaxChars:      2400,
				EnableRollingSummary: true,
			},
		},
	}
}

func validConfigYAML(template string, roleName string, roleGoal string, roleStyle string, language string) string {
	return "" +
		"llm:\n" +
		"  provider: openai\n" +
		"  model: gpt-4o-mini\n" +
		"  base_url: https://api.openai.com/v1\n" +
		"  api_key: real-key\n" +
		"  temperature: 0.5\n" +
		"chat:\n" +
		"  context:\n" +
		"    max_chars: 12000\n" +
		"    keep_recent_turns: 6\n" +
		"    summary_max_chars: 2400\n" +
		"    enable_rolling_summary: true\n" +
		"  prompt:\n" +
		"    template: |\n" +
		"      " + template + "\n" +
		"    role:\n" +
		"      name: " + roleName + "\n" +
		"      goal: " + roleGoal + "\n" +
		"      style: " + roleStyle + "\n" +
		"    context:\n" +
		"      language: " + language + "\n"
}

func validConfigYAMLWithMaxHistory(template string, roleName string, roleGoal string, roleStyle string, language string) string {
	return "" +
		"llm:\n" +
		"  provider: openai\n" +
		"  model: gpt-4o-mini\n" +
		"  base_url: https://api.openai.com/v1\n" +
		"  api_key: real-key\n" +
		"  temperature: 0.5\n" +
		"chat:\n" +
		"  max_history_messages: 10\n" +
		"  context:\n" +
		"    max_chars: 12000\n" +
		"    keep_recent_turns: 6\n" +
		"    summary_max_chars: 2400\n" +
		"    enable_rolling_summary: true\n" +
		"  prompt:\n" +
		"    template: |\n" +
		"      " + template + "\n" +
		"    role:\n" +
		"      name: " + roleName + "\n" +
		"      goal: " + roleGoal + "\n" +
		"      style: " + roleStyle + "\n" +
		"    context:\n" +
		"      language: " + language + "\n"
}

func writeConfigFile(t *testing.T, dir string, content string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	path := filepath.Join(dir, "config.yaml")
	data := []byte(content)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}
