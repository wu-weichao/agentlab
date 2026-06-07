package tools

import (
	"context"
	"errors"
	"testing"
)

func TestExecutorRegistersAndExecutesTool(t *testing.T) {
	executor := NewExecutor()
	if err := executor.Register(NewCalculatorTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	result := executor.Execute(context.Background(), ToolCall{
		ToolName:  "calculator",
		Arguments: map[string]any{"expression": "1 + 2 * 3"},
	})
	if !result.Success {
		t.Fatalf("expected success, got error %q", result.Error)
	}
	if result.Content != "7" {
		t.Fatalf("expected content 7, got %q", result.Content)
	}
	if result.Metadata["tool_name"] != "calculator" {
		t.Fatalf("expected metadata tool_name, got %#v", result.Metadata)
	}
}

func TestExecutorRejectsDuplicateRegistration(t *testing.T) {
	executor := NewExecutor()
	if err := executor.Register(NewCalculatorTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if err := executor.Register(NewCalculatorTool()); !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestExecutorReturnsStructuredErrorForUnknownTool(t *testing.T) {
	executor := NewExecutor()
	result := executor.Execute(context.Background(), ToolCall{ToolName: "missing"})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Metadata["code"] != "tool_not_found" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestExecutorReturnsToolArgumentError(t *testing.T) {
	executor := NewExecutor()
	if err := executor.Register(NewCalculatorTool()); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	result := executor.Execute(context.Background(), ToolCall{ToolName: "calculator"})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Metadata["code"] != "invalid_arguments" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestParseToolCall(t *testing.T) {
	call, err := ParseToolCall("```json\n{\"tool_name\":\"time\",\"arguments\":{},\"reason\":\"now\"}\n```")
	if err != nil {
		t.Fatalf("ParseToolCall returned error: %v", err)
	}
	if call.ToolName != "time" || call.Reason != "now" {
		t.Fatalf("unexpected call: %#v", call)
	}
}

func TestParseToolCallRejectsMultipleCalls(t *testing.T) {
	_, err := ParseToolCall(`{"tool_calls":[{"tool_name":"time"},{"tool_name":"calculator"}]}`)
	if !errors.Is(err, ErrMultipleToolCalls) {
		t.Fatalf("expected multiple tool calls error, got %v", err)
	}
}
