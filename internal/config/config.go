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
	LLM   LLMConfig   `yaml:"llm"`
	Chat  ChatConfig  `yaml:"chat"`
	Tools ToolsConfig `yaml:"tools"`
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
	Prompt  PromptConfig  `yaml:"prompt"`
	Context ContextConfig `yaml:"context"`
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

// ContextConfig 定义单会话上下文控制参数。
type ContextConfig struct {
	MaxChars             int  `yaml:"max_chars"`
	KeepRecentTurns      int  `yaml:"keep_recent_turns"`
	SummaryMaxChars      int  `yaml:"summary_max_chars"`
	EnableRollingSummary bool `yaml:"enable_rolling_summary"`
}

// ToolsConfig 描述可选工具配置。首版 web_search 未配置时仍允许启动。
type ToolsConfig struct {
	WebSearch WebSearchConfig `yaml:"web_search"`
}

// WebSearchConfig 为后续真实搜索客户端预留最小配置入口。
type WebSearchConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
	Endpoint string `yaml:"endpoint"`
}

// Load 从本地 YAML 文件读取并校验聊天配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if hasDeprecatedMaxHistoryMessages(data) {
		return nil, errors.New("chat.max_history_messages has been removed; use chat.context.* instead")
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
	if strings.TrimSpace(c.Chat.Prompt.Template) == "" {
		return errors.New("chat.prompt.template is required")
	}
	// v0.0.3 的上下文窗口完全由 chat.context.* 驱动，启动阶段直接拦截非法预算配置。
	if c.Chat.Context.MaxChars <= 0 {
		return errors.New("chat.context.max_chars must be greater than 0")
	}
	if c.Chat.Context.KeepRecentTurns <= 0 {
		return errors.New("chat.context.keep_recent_turns must be greater than 0")
	}
	if c.Chat.Context.SummaryMaxChars <= 0 {
		return errors.New("chat.context.summary_max_chars must be greater than 0")
	}
	if c.Chat.Context.SummaryMaxChars >= c.Chat.Context.MaxChars {
		return errors.New("chat.context.summary_max_chars must be less than chat.context.max_chars")
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

// isPlaceholderValue 用于识别示例配置中的占位值，避免用户带着示例 API Key 进入运行时。
func isPlaceholderValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}

	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}

// hasDeprecatedMaxHistoryMessages 在反序列化前扫描 YAML AST，
// 显式拦截已经移除的旧字段，避免它被结构体静默忽略。
func hasDeprecatedMaxHistoryMessages(data []byte) bool {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false
	}
	if len(root.Content) == 0 {
		return false
	}

	return mappingHasNestedKey(root.Content[0], "chat", "max_history_messages")
}

// mappingHasNestedKey 递归检查多层 mapping 中是否存在目标键路径。
func mappingHasNestedKey(node *yaml.Node, keys ...string) bool {
	if node == nil || len(keys) == 0 || node.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		if keyNode.Value != keys[0] {
			continue
		}
		if len(keys) == 1 {
			return true
		}
		return mappingHasNestedKey(valueNode, keys[1:]...)
	}

	return false
}
