package tools

import (
	"context"
	"testing"
	"time"
)

func TestTimeToolReturnsStableFields(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	fixed := time.Date(2026, 6, 5, 20, 30, 10, 0, loc)
	tool := NewTimeToolWithClock(func() time.Time { return fixed })

	result := tool.Execute(context.Background(), nil)
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Error)
	}
	if result.Content != "2026-06-05T20:30:10+08:00" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if result.Metadata["date"] != "2026-06-05" {
		t.Fatalf("unexpected date metadata: %#v", result.Metadata)
	}
	if result.Metadata["timezone"] != "CST" {
		t.Fatalf("unexpected timezone metadata: %#v", result.Metadata)
	}
}
