package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, ""+
		"llm:\n"+
		"  provider: openai\n"+
		"  model: gpt-4o-mini\n"+
		"  base_url: https://api.openai.com/v1\n"+
		"  api_key: config-key\n"+
		"  temperature: 0.5\n"+
		"chat:\n"+
		"  system_prompt: test prompt\n"+
		"  max_history_messages: 10\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLM.APIKey != "config-key" {
		t.Fatalf("expected config api key, got %q", cfg.LLM.APIKey)
	}
}

func TestLoadReadsAllConfigFields(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, ""+
		"llm:\n"+
		"  provider: openai\n"+
		"  model: gpt-4.1-mini\n"+
		"  base_url: https://api.example.com/v1\n"+
		"  api_key: real-key\n"+
		"  temperature: 0.3\n"+
		"chat:\n"+
		"  system_prompt: production prompt\n"+
		"  max_history_messages: 32\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLM.Provider != "openai" {
		t.Fatalf("expected provider from config, got %q", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gpt-4.1-mini" {
		t.Fatalf("expected model from config, got %q", cfg.LLM.Model)
	}
	if cfg.LLM.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected base url from config, got %q", cfg.LLM.BaseURL)
	}
	if cfg.LLM.APIKey != "real-key" {
		t.Fatalf("expected api key from config, got %q", cfg.LLM.APIKey)
	}
	if cfg.LLM.Temperature != 0.3 {
		t.Fatalf("expected temperature from config, got %v", cfg.LLM.Temperature)
	}
	if cfg.Chat.SystemPrompt != "production prompt" {
		t.Fatalf("expected system prompt from config, got %q", cfg.Chat.SystemPrompt)
	}
	if cfg.Chat.MaxHistoryMessages != 32 {
		t.Fatalf("expected history size from config, got %d", cfg.Chat.MaxHistoryMessages)
	}
}

func TestValidateRejectsMissingAPIKey(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o-mini",
			BaseURL:  "https://api.openai.com/v1",
		},
		Chat: ChatConfig{
			SystemPrompt:       "prompt",
			MaxHistoryMessages: 10,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRejectsPlaceholderAPIKey(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o-mini",
			BaseURL:  "https://api.openai.com/v1",
			APIKey:   "{your-openai-api-key}",
		},
		Chat: ChatConfig{
			SystemPrompt:       "prompt",
			MaxHistoryMessages: 10,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for placeholder api key")
	}
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
