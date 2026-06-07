package tools

import (
	"context"
	"testing"
)

func TestCalculatorEvaluatesValidExpression(t *testing.T) {
	tool := NewCalculatorTool()
	result := tool.Execute(context.Background(), map[string]any{
		"expression": "(1 + 2) * 3 - 4 / 2",
	})
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Error)
	}
	if result.Content != "7" {
		t.Fatalf("expected 7, got %q", result.Content)
	}
}

func TestCalculatorRejectsInvalidExpression(t *testing.T) {
	tool := NewCalculatorTool()
	result := tool.Execute(context.Background(), map[string]any{
		"expression": "1 + alert(2)",
	})
	if result.Success {
		t.Fatal("expected failure")
	}
}

func TestCalculatorRejectsDivisionByZero(t *testing.T) {
	tool := NewCalculatorTool()
	result := tool.Execute(context.Background(), map[string]any{
		"expression": "1 / 0",
	})
	if result.Success {
		t.Fatal("expected failure")
	}
}

func TestCalculatorRejectsEmptyInput(t *testing.T) {
	tool := NewCalculatorTool()
	result := tool.Execute(context.Background(), map[string]any{
		"expression": " ",
	})
	if result.Success {
		t.Fatal("expected failure")
	}
}
