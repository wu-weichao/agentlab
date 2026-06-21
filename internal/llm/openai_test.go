package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentlab/internal/tools"
)

func TestOpenAIClientSendsNativeToolsAndParsesToolCall(t *testing.T) {
	var captured openAIChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"calculator","arguments":"{\"expression\":\"1+2\"}"}}]}}]}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		BaseURL: server.URL,
		APIKey:  "secret-key",
		Model:   "test-model",
	})
	response, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "calculate"}},
		Tools: []tools.ToolSpec{{
			Name:        "calculator",
			Description: "calculate expressions",
			Parameters: []tools.Parameter{{
				Name:        "expression",
				Type:        "string",
				Required:    true,
				Description: "expression to calculate",
			}},
		}},
		ToolCallingMode: ToolCallingModeNative,
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if len(captured.Tools) != 1 {
		t.Fatalf("expected one native tool, got %d", len(captured.Tools))
	}
	schema := captured.Tools[0].Function.Parameters
	if schema.Properties["expression"].Type != "string" || len(schema.Required) != 1 || schema.Required[0] != "expression" {
		t.Fatalf("unexpected tool schema: %#v", schema)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected one parsed tool call, got %#v", response.ToolCalls)
	}
	call := response.ToolCalls[0]
	if call.ID != "call-1" || call.ToolName != "calculator" || call.Arguments["expression"] != "1+2" {
		t.Fatalf("unexpected parsed tool call: %#v", call)
	}
}

func TestOpenAIClientTextCompatOmitsToolsAndReturnsContent(t *testing.T) {
	var captured openAIChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, Model: "test-model"})
	response, err := client.Chat(context.Background(), ChatRequest{
		Messages:        []Message{{Role: RoleUser, Content: "hi"}},
		Tools:           []tools.ToolSpec{{Name: "time"}},
		ToolCallingMode: ToolCallingModeTextCompat,
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if len(captured.Tools) != 0 {
		t.Fatalf("text compatibility request should omit native tools, got %#v", captured.Tools)
	}
	if response.Content != "hello" || len(response.ToolCalls) != 0 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestParseOpenAIToolCallsRejectsInvalidCalls(t *testing.T) {
	tests := []struct {
		name string
		call openAIToolCall
	}{
		{name: "missing id", call: openAIToolCall{Function: openAIFunctionCall{Name: "time", Arguments: "{}"}}},
		{name: "missing name", call: openAIToolCall{ID: "call-1", Function: openAIFunctionCall{Arguments: "{}"}}},
		{name: "invalid json", call: openAIToolCall{ID: "call-1", Function: openAIFunctionCall{Name: "time", Arguments: "{"}}},
		{name: "array arguments", call: openAIToolCall{ID: "call-1", Function: openAIFunctionCall{Name: "time", Arguments: "[]"}}},
		{name: "scalar arguments", call: openAIToolCall{ID: "call-1", Function: openAIFunctionCall{Name: "time", Arguments: `"x"`}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseOpenAIToolCalls([]openAIToolCall{tt.call}); err == nil {
				t.Fatal("expected protocol error")
			}
		})
	}
}

func TestParseOpenAIToolCallsPreservesMultipleCallsForRuntimeValidation(t *testing.T) {
	calls, err := parseOpenAIToolCalls([]openAIToolCall{
		{ID: "call-1", Function: openAIFunctionCall{Name: "time", Arguments: "{}"}},
		{ID: "call-2", Function: openAIFunctionCall{Name: "calculator", Arguments: `{"expression":"1+1"}`}},
	})
	if err != nil {
		t.Fatalf("parseOpenAIToolCalls returned error: %v", err)
	}
	if len(calls) != 2 || calls[0].ID != "call-1" || calls[1].ID != "call-2" {
		t.Fatalf("expected adapter to preserve calls for Agent validation, got %#v", calls)
	}
}

func TestOpenAIClientRejectsReasoningOnlyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"hidden"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, Model: "test-model"})
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages:        []Message{{Role: RoleUser, Content: "hi"}},
		ToolCallingMode: ToolCallingModeNative,
	})
	if err == nil || !strings.Contains(err.Error(), "reasoning_content") {
		t.Fatalf("expected reasoning-only error, got %v", err)
	}
}

func TestOpenAIClientLogsFullRequestAndResponseWithoutAuthorizationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"done"}}]}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})

	client := NewOpenAIClient(OpenAIConfig{
		BaseURL: server.URL,
		APIKey:  "super-secret-api-key",
		Model:   "test-model",
	})
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "private user content"},
			{Role: RoleTool, Content: "sensitive tool result", ToolCallID: "call-1"},
		},
		ToolCallingMode: ToolCallingModeNative,
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	logText := buf.String()
	for _, forbidden := range []string{"super-secret-api-key", "Authorization", "Bearer "} {
		if strings.Contains(logText, forbidden) {
			t.Fatalf("log should not contain %q: %s", forbidden, logText)
		}
	}
	for _, required := range []string{
		"tool_calling_mode=native",
		"tools_count=0",
		"tool_calls_count=0",
		"request_body=",
		"private user content",
		"sensitive tool result",
		"response_body=",
		`"content":"done"`,
	} {
		if !strings.Contains(logText, required) {
			t.Fatalf("log should contain %q: %s", required, logText)
		}
	}
}
