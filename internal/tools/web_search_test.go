package tools

import (
	"context"
	"errors"
	"testing"
)

func TestWebSearchReturnsBoundedResults(t *testing.T) {
	tool := NewWebSearchTool(fakeSearchClient{
		results: []SearchResult{
			{Title: "A", Snippet: "first", URL: "https://example.com/a", Source: "example"},
			{Title: "B", Snippet: "second", URL: "https://example.com/b", Source: "example"},
			{Title: "C", Snippet: "third", URL: "https://example.com/c", Source: "example"},
		},
	}, 2)

	result := tool.Execute(context.Background(), map[string]any{
		"query": "agent",
		"limit": float64(5),
	})
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Error)
	}
	if result.Metadata["result_count"] != 2 {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestWebSearchReturnsUnavailableError(t *testing.T) {
	tool := NewWebSearchTool(UnavailableSearchClient{}, 2)
	result := tool.Execute(context.Background(), map[string]any{"query": "agent"})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Metadata["code"] != "search_failed" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestWebSearchRejectsMissingQuery(t *testing.T) {
	tool := NewWebSearchTool(fakeSearchClient{}, 2)
	result := tool.Execute(context.Background(), map[string]any{})
	if result.Success {
		t.Fatal("expected failure")
	}
}

type fakeSearchClient struct {
	results []SearchResult
	err     error
}

func (c fakeSearchClient) Search(_ context.Context, _ string, _ int) ([]SearchResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.results == nil {
		return nil, errors.New("search failed")
	}
	return c.results, nil
}
