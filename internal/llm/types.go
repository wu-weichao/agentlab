package llm

import (
	"fmt"
	"strings"

	"agentlab/internal/tools"
)

// Role 表示一条对话消息的角色类型。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSummary   Role = "summary"
)

// ToolCallingMode 控制 Agent 使用 provider 原生工具协议或文本兼容协议。
type ToolCallingMode string

const (
	ToolCallingModeNative     ToolCallingMode = "native"
	ToolCallingModeTextCompat ToolCallingMode = "text_compat"
	DefaultToolCallingMode                    = ToolCallingModeNative
)

func ParseToolCallingMode(value string) (ToolCallingMode, error) {
	mode := ToolCallingMode(strings.ToLower(strings.TrimSpace(value)))
	if mode == "" {
		return DefaultToolCallingMode, nil
	}
	switch mode {
	case ToolCallingModeNative, ToolCallingModeTextCompat:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported tool calling mode: %s", value)
	}
}

// Message 是聊天上下文中的基础消息结构。
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []tools.ToolCall
	ToolCallID string
	ToolName   string
}

// ChatRequest 是 provider 无关的统一聊天请求。
type ChatRequest struct {
	Messages        []Message
	Tools           []tools.ToolSpec
	ToolCallingMode ToolCallingMode
}

// ChatResponse 表示 provider 返回的统一文本和工具调用结果。
type ChatResponse struct {
	Content   string
	ToolCalls []tools.ToolCall
}
