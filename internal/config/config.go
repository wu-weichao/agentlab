package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是聊天运行时的总配置结构。
type Config struct {
	LLM  LLMConfig  `yaml:"llm"`
	Chat ChatConfig `yaml:"chat"`
}

// LLMConfig 描述模型提供方和请求参数。
type LLMConfig struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	BaseURL     string  `yaml:"base_url"`
	APIKey      string  `yaml:"api_key"`
	Temperature float64 `yaml:"temperature"`
}

// ChatConfig 描述本地对话行为相关的配置项。
type ChatConfig struct {
	SystemPrompt       string `yaml:"system_prompt"`
	MaxHistoryMessages int    `yaml:"max_history_messages"`
}

// Load 从本地 YAML 文件读取并校验聊天配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate 校验配置是否足以在启动阶段构造聊天运行时。
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	if strings.TrimSpace(c.LLM.Provider) == "" {
		return errors.New("llm.provider is required")
	}
	if strings.TrimSpace(c.LLM.Model) == "" {
		return errors.New("llm.model is required")
	}
	if strings.TrimSpace(c.LLM.BaseURL) == "" {
		return errors.New("llm.base_url is required")
	}
	if strings.TrimSpace(c.Chat.SystemPrompt) == "" {
		return errors.New("chat.system_prompt is required")
	}
	if c.Chat.MaxHistoryMessages <= 0 {
		return errors.New("chat.max_history_messages must be greater than 0")
	}

	switch strings.ToLower(strings.TrimSpace(c.LLM.Provider)) {
	case "openai":
		if isPlaceholderValue(c.LLM.APIKey) {
			return errors.New("llm.api_key must be replaced with a real API key for openai provider")
		}
	default:
		return fmt.Errorf("unsupported llm.provider: %s", c.LLM.Provider)
	}

	return nil
}

// isPlaceholderValue 用于识别示例配置中的占位值，避免误当成真实密钥使用。
func isPlaceholderValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}

	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}
