package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// DefaultFileReadMaxBytes 是 file_read 单次读取的默认上限，避免超大文件污染模型上下文。
	DefaultFileReadMaxBytes = 64 * 1024
	// DefaultSearchLimit 是 web_search 返回结果数的默认上限。
	DefaultSearchLimit = 5
)

var (
	// ErrToolNotFound 表示请求调用的工具未注册。
	ErrToolNotFound = errors.New("tool not found")
	// ErrDuplicateTool 表示工具名重复注册。
	ErrDuplicateTool = errors.New("duplicate tool")
	// ErrInvalidArguments 表示工具调用参数不满足协议。
	ErrInvalidArguments = errors.New("invalid tool arguments")
	// ErrMultipleToolCalls 表示模型请求了首版不支持的多工具调用。
	ErrMultipleToolCalls = errors.New("multiple tool calls are unsupported")
	// ErrToolCallNotPresent 表示模型响应不是工具调用。
	ErrToolCallNotPresent = errors.New("tool call not present")
)

// Tool 描述一个可注册、可执行的受控工具。
// 设计上工具只接收结构化参数并返回 ToolResult，不直接读写 ChatBot 会话状态，
// 这样可以把“模型编排”和“工具副作用边界”隔离开，便于后续做权限控制。
type Tool interface {
	Name() string
	Description() string
	Parameters() []Parameter
	Execute(ctx context.Context, args map[string]any) ToolResult
}

// Parameter 描述工具参数的最小元信息。
type Parameter struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// ToolSpec 是可展示给模型的工具定义快照。
// 它只包含工具能力描述和参数约束，不包含具体执行实现。
type ToolSpec struct {
	Name        string
	Description string
	Parameters  []Parameter
}

// ToolCall 是模型请求工具调用时使用的统一结构。
// 该结构是仓库内部协议，不绑定 OpenAI 等 provider 的私有 tool schema；
// provider 适配层后续可以把原生 tool call 转换成此结构。
type ToolCall struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
	Reason    string         `json:"reason,omitempty"`
}

// ToolResult 是工具执行完成后的统一结构。
// 成功和失败都必须返回结构化结果，避免工具错误直接打断 Agent 主流程或诱导模型编造结果。
type ToolResult struct {
	Success  bool           `json:"success"`
	Content  string         `json:"content"`
	Error    string         `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func successResult(content string, metadata map[string]any) ToolResult {
	return ToolResult{
		Success:  true,
		Content:  content,
		Metadata: metadata,
	}
}

func errorResult(message string, metadata map[string]any) ToolResult {
	return ToolResult{
		Success:  false,
		Error:    message,
		Metadata: metadata,
	}
}

func requiredString(args map[string]any, name string) (string, bool) {
	value, ok := args[name]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	return text, text != ""
}

func optionalInt(args map[string]any, name string, fallback int) (int, bool) {
	value, ok := args[name]
	if !ok {
		return fallback, true
	}

	switch v := value.(type) {
	case int:
		return v, v > 0
	case int64:
		return int(v), v > 0
	case float64:
		if v != float64(int(v)) {
			return 0, false
		}
		return int(v), v > 0
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), i > 0
	default:
		return 0, false
	}
}

// ParseToolCall 从模型文本响应中解析单个工具调用。
// 支持纯 JSON、Markdown json 代码块、{"tool_call": {...}} 和 {"tool_calls": [...]}。
// 解析不到工具调用时返回 ErrToolCallNotPresent，让调用方可以把响应当作普通 assistant 文本处理。
// 一旦识别到多个工具调用则直接返回 ErrMultipleToolCalls，因为首版只允许单轮一次工具调用。
func ParseToolCall(text string) (*ToolCall, error) {
	payload := strings.TrimSpace(text)
	if payload == "" {
		return nil, ErrToolCallNotPresent
	}
	payload = unwrapJSONFence(payload)

	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()

	var raw any
	if err := decoder.Decode(&raw); err != nil {
		return nil, ErrToolCallNotPresent
	}

	call, err := parseToolCallValue(raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(call.ToolName) == "" {
		return nil, fmt.Errorf("%w: tool_name is required", ErrInvalidArguments)
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	return call, nil
}

func unwrapJSONFence(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 {
		return trimmed
	}
	first := strings.TrimSpace(lines[0])
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(first, "```") || last != "```" {
		return trimmed
	}
	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}

func parseToolCallValue(value any) (*ToolCall, error) {
	switch v := value.(type) {
	case []any:
		if len(v) > 1 {
			return nil, ErrMultipleToolCalls
		}
		if len(v) == 0 {
			return nil, ErrToolCallNotPresent
		}
		return parseToolCallValue(v[0])
	case map[string]any:
		// 兼容两种常见模型输出：直接输出 tool_call，或用 tool_calls 数组包裹。
		// 这里不执行任何工具，只做协议识别和首版能力边界校验。
		if calls, ok := v["tool_calls"]; ok {
			list, ok := calls.([]any)
			if !ok {
				return nil, fmt.Errorf("%w: tool_calls must be an array", ErrInvalidArguments)
			}
			if len(list) > 1 {
				return nil, ErrMultipleToolCalls
			}
			if len(list) == 0 {
				return nil, ErrToolCallNotPresent
			}
			return parseToolCallValue(list[0])
		}
		if wrapped, ok := v["tool_call"]; ok {
			return parseToolCallValue(wrapped)
		}

		toolName, _ := v["tool_name"].(string)
		if strings.TrimSpace(toolName) == "" {
			return nil, ErrToolCallNotPresent
		}

		args := map[string]any{}
		if rawArgs, ok := v["arguments"]; ok {
			rawMap, ok := rawArgs.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%w: arguments must be an object", ErrInvalidArguments)
			}
			args = rawMap
		}
		reason, _ := v["reason"].(string)
		return &ToolCall{
			ToolName:  strings.TrimSpace(toolName),
			Arguments: args,
			Reason:    strings.TrimSpace(reason),
		}, nil
	default:
		return nil, ErrToolCallNotPresent
	}
}

// FormatToolResult 将工具结果序列化为可回填给模型的稳定 JSON 文本。
// 回填时保持 JSON 结构，目的是让模型明确区分工具内容、错误和 metadata。
func FormatToolResult(result ToolResult) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"marshal tool result: %s"}`, err)
	}
	return string(data)
}
