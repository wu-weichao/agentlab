package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ToolCallKey 标识一次可复用的工具调用。
// 相同工具名和相同标准化参数会生成相同 key。
type ToolCallKey struct {
	Name     string
	ArgsHash string
}

// BuildToolCallKey 基于工具名和标准化参数生成稳定 key。
func BuildToolCallKey(call ToolCall) (ToolCallKey, error) {
	name := strings.TrimSpace(call.ToolName)
	if name == "" {
		return ToolCallKey{}, fmt.Errorf("%w: tool_name is required", ErrInvalidArguments)
	}

	normalized, err := NormalizeArguments(call.Arguments)
	if err != nil {
		return ToolCallKey{}, err
	}

	return ToolCallKey{
		Name:     name,
		ArgsHash: hashString(normalized),
	}, nil
}

// NormalizeArguments 将工具参数转换为稳定 JSON 字符串。
func NormalizeArguments(args map[string]any) (string, error) {
	if args == nil {
		args = map[string]any{}
	}

	normalized, err := normalizeValue(args)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("%w: normalize arguments: %v", ErrInvalidArguments, err)
	}
	return string(data), nil
}

func normalizeValue(value any) (any, error) {
	switch v := value.(type) {
	case nil, bool, string:
		return v, nil
	case json.Number:
		return normalizeJSONNumber(v)
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case float32:
		return normalizeFloat(float64(v))
	case float64:
		return normalizeFloat(v)
	case map[string]any:
		normalized := make(map[string]any, len(v))
		for key, item := range v {
			normalizedItem, err := normalizeValue(item)
			if err != nil {
				return nil, err
			}
			normalized[key] = normalizedItem
		}
		return normalized, nil
	case []any:
		normalized := make([]any, len(v))
		for i, item := range v {
			normalizedItem, err := normalizeValue(item)
			if err != nil {
				return nil, err
			}
			normalized[i] = normalizedItem
		}
		return normalized, nil
	default:
		return nil, fmt.Errorf("%w: unsupported argument value type %T", ErrInvalidArguments, value)
	}
}

func normalizeJSONNumber(value json.Number) (any, error) {
	if i, err := value.Int64(); err == nil {
		return i, nil
	}
	f, err := value.Float64()
	if err != nil {
		return nil, fmt.Errorf("%w: invalid json number %q", ErrInvalidArguments, value.String())
	}
	return normalizeFloat(f)
}

func normalizeFloat(value float64) (any, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("%w: invalid number", ErrInvalidArguments)
	}
	if math.Trunc(value) == value && value >= math.MinInt64 && value <= math.MaxInt64 {
		return int64(value), nil
	}
	return value, nil
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
