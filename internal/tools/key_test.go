package tools

import (
	"encoding/json"
	"testing"
)

func TestBuildToolCallKeyIgnoresArgumentFieldOrder(t *testing.T) {
	first, err := BuildToolCallKey(ToolCall{
		ToolName: "calculator",
		Arguments: map[string]any{
			"expression": "1 + 2",
			"options": map[string]any{
				"precision": json.Number("2"),
				"mode":      "basic",
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildToolCallKey returned error: %v", err)
	}

	second, err := BuildToolCallKey(ToolCall{
		ToolName: "calculator",
		Arguments: map[string]any{
			"options": map[string]any{
				"mode":      "basic",
				"precision": float64(2),
			},
			"expression": "1 + 2",
		},
	})
	if err != nil {
		t.Fatalf("BuildToolCallKey returned error: %v", err)
	}

	if first != second {
		t.Fatalf("expected same key, got %#v and %#v", first, second)
	}
}

func TestBuildToolCallKeyIncludesToolName(t *testing.T) {
	args := map[string]any{"value": "same"}
	first, err := BuildToolCallKey(ToolCall{ToolName: "time", Arguments: args})
	if err != nil {
		t.Fatalf("BuildToolCallKey returned error: %v", err)
	}
	second, err := BuildToolCallKey(ToolCall{ToolName: "calculator", Arguments: args})
	if err != nil {
		t.Fatalf("BuildToolCallKey returned error: %v", err)
	}

	if first == second {
		t.Fatalf("expected different keys for different tools, got %#v", first)
	}
}

func TestBuildToolCallKeyIgnoresProviderCallID(t *testing.T) {
	first, err := BuildToolCallKey(ToolCall{
		ID:        "call-1",
		ToolName:  "calculator",
		Arguments: map[string]any{"expression": "1+1"},
	})
	if err != nil {
		t.Fatalf("BuildToolCallKey returned error: %v", err)
	}
	second, err := BuildToolCallKey(ToolCall{
		ID:        "call-2",
		ToolName:  "calculator",
		Arguments: map[string]any{"expression": "1+1"},
	})
	if err != nil {
		t.Fatalf("BuildToolCallKey returned error: %v", err)
	}
	if first != second {
		t.Fatalf("provider call id must not affect cache key: %#v != %#v", first, second)
	}
}

func TestNormalizeArgumentsHandlesEmptyAndNestedValues(t *testing.T) {
	empty, err := NormalizeArguments(nil)
	if err != nil {
		t.Fatalf("NormalizeArguments returned error: %v", err)
	}
	if empty != "{}" {
		t.Fatalf("expected empty args to normalize to {}, got %q", empty)
	}

	first, err := NormalizeArguments(map[string]any{
		"items": []any{
			map[string]any{"b": json.Number("2"), "a": "x"},
		},
	})
	if err != nil {
		t.Fatalf("NormalizeArguments returned error: %v", err)
	}
	second, err := NormalizeArguments(map[string]any{
		"items": []any{
			map[string]any{"a": "x", "b": int64(2)},
		},
	})
	if err != nil {
		t.Fatalf("NormalizeArguments returned error: %v", err)
	}
	if first != second {
		t.Fatalf("expected same normalized args, got %q and %q", first, second)
	}
}

func TestBuildToolCallKeyRejectsEmptyToolName(t *testing.T) {
	_, err := BuildToolCallKey(ToolCall{ToolName: " ", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("expected error")
	}
}
