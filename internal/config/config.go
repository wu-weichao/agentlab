package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"agentlab/internal/prompt"

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
	MaxHistoryMessages int          `yaml:"max_history_messages"`
	Prompt             PromptConfig `yaml:"prompt"`
}

// PromptConfig 定义 system prompt 的模板和结构化变量。
type PromptConfig struct {
	Template string              `yaml:"template"`
	Role     PromptRoleConfig    `yaml:"role"`
	Context  PromptContextConfig `yaml:"context"`
}

// PromptRoleConfig 定义角色名称、目标和风格。
type PromptRoleConfig struct {
	Name  string `yaml:"name"`
	Goal  string `yaml:"goal"`
	Style string `yaml:"style"`
}

// PromptContextConfig 定义通用上下文参数。
type PromptContextConfig struct {
	Language string `yaml:"language"`
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
	if c.Chat.MaxHistoryMessages <= 0 {
		return errors.New("chat.max_history_messages must be greater than 0")
	}
	if strings.TrimSpace(c.Chat.Prompt.Template) == "" {
		return errors.New("chat.prompt.template is required")
	}

	requiredVariables := map[string]string{
		"role.name":        c.Chat.Prompt.Role.Name,
		"role.goal":        c.Chat.Prompt.Role.Goal,
		"role.style":       c.Chat.Prompt.Role.Style,
		"context.language": c.Chat.Prompt.Context.Language,
	}
	for key, value := range requiredVariables {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}

	if err := prompt.Validate(c.Chat.Prompt.Template, c.Chat.Prompt.VariableMap()); err != nil {
		return fmt.Errorf("validate chat.prompt.template: %w", err)
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

// VariableMap 返回 Prompt 模板可用的标准变量。
func (c PromptConfig) VariableMap() map[string]string {
	return map[string]string{
		"role.name":        strings.TrimSpace(c.Role.Name),
		"role.goal":        strings.TrimSpace(c.Role.Goal),
		"role.style":       strings.TrimSpace(c.Role.Style),
		"context.language": strings.TrimSpace(c.Context.Language),
	}
}

// isPlaceholderValue 用于识别示例配置中的占位值，避免误当成真实密钥使用。
func isPlaceholderValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}

	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}
