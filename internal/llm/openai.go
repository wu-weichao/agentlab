package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"agentlab/internal/requestctx"
	"agentlab/internal/tools"
)

// OpenAIConfig 描述 OpenAI 兼容客户端的初始化参数。
type OpenAIConfig struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
}

// OpenAIClient 负责把内部消息结构转换成 OpenAI 兼容聊天请求。
type OpenAIClient struct {
	baseURL     string
	apiKey      string
	model       string
	temperature float64
	httpClient  *http.Client
}

// NewOpenAIClient 创建一个 OpenAI 兼容客户端。
func NewOpenAIClient(cfg OpenAIConfig) *OpenAIClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIClient{
		baseURL:     baseURL,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

// Chat 发送聊天补全请求，并返回首个 assistant 文本或结构化工具调用。
func (c *OpenAIClient) Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	mode, err := ParseToolCallingMode(string(request.ToolCallingMode))
	if err != nil {
		return nil, err
	}
	requestBody := openAIChatRequest{
		Model:       c.model,
		Messages:    make([]openAIMessage, 0, len(request.Messages)),
		Temperature: c.temperature,
	}

	for _, message := range request.Messages {
		converted, err := toOpenAIMessage(message)
		if err != nil {
			return nil, err
		}
		requestBody.Messages = append(requestBody.Messages, converted)
	}
	if mode == ToolCallingModeNative && len(request.Tools) > 0 {
		requestBody.Tools = make([]openAITool, 0, len(request.Tools))
		for _, spec := range request.Tools {
			requestBody.Tools = append(requestBody.Tools, toOpenAITool(spec))
		}
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	requestURL := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create openai request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	log.Printf(
		"[llm/openai] request_id=%s 发送请求 provider=openai model=%s url=%s messages=%d tools_count=%d tool_calling_mode=%s request_body=%s",
		requestctx.FromContext(ctx),
		c.model,
		requestURL,
		len(request.Messages),
		len(requestBody.Tools),
		mode,
		string(payload),
	)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[llm/openai] request_id=%s 请求发送失败 model=%s err=%v", requestctx.FromContext(ctx), c.model, err)
		return nil, fmt.Errorf("send openai request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[llm/openai] request_id=%s 读取响应失败 status=%s err=%v", requestctx.FromContext(ctx), resp.Status, err)
		return nil, fmt.Errorf("read openai response: %w", err)
	}

	log.Printf(
		"[llm/openai] request_id=%s 收到响应 status=%s duration=%s body_bytes=%d response_body=%s",
		requestctx.FromContext(ctx),
		resp.Status,
		time.Since(start),
		len(body),
		strings.TrimSpace(string(body)),
	)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai request failed: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}

	var completion openAIChatResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("openai response missing choices")
	}

	message := completion.Choices[0].Message
	toolCalls, err := parseOpenAIToolCalls(message.ToolCalls)
	if err != nil {
		log.Printf(
			"[llm/openai] request_id=%s 解析失败 tool_calling_mode=%s tool_calls_count=%d tool_calls_parse_status=error err=%v",
			requestctx.FromContext(ctx),
			mode,
			len(message.ToolCalls),
			err,
		)
		return nil, err
	}
	content := strings.TrimSpace(message.Content)
	if len(toolCalls) > 0 {
		log.Printf(
			"[llm/openai] request_id=%s 解析成功 choice_count=%d tool_calling_mode=%s tool_calls_count=%d tool_calls_parse_status=success",
			requestctx.FromContext(ctx),
			len(completion.Choices),
			mode,
			len(toolCalls),
		)
		return &ChatResponse{Content: content, ToolCalls: toolCalls}, nil
	}
	if content == "" {
		if strings.TrimSpace(message.ReasoningContent) != "" {
			return nil, fmt.Errorf("openai response missing assistant content: response only contains reasoning_content")
		}
		return nil, fmt.Errorf("openai response missing assistant content")
	}

	log.Printf(
		"[llm/openai] request_id=%s 解析成功 choice_count=%d reply=%q tool_calling_mode=%s tool_calls_count=0 tool_calls_parse_status=success",
		requestctx.FromContext(ctx),
		len(completion.Choices),
		summarizeText(content, 120),
		mode,
	)
	return &ChatResponse{Content: content}, nil
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []openAITool    `json:"tools,omitempty"`
}

type openAIMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	Name             string           `json:"name,omitempty"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIFunctionSpec `json:"function"`
}

type openAIFunctionSpec struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Parameters  openAIJSONSchema `json:"parameters"`
}

type openAIJSONSchema struct {
	Type       string                        `json:"type"`
	Properties map[string]openAIJSONProperty `json:"properties"`
	Required   []string                      `json:"required,omitempty"`
}

type openAIJSONProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type,omitempty"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

func summarizeText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= limit {
		return trimmed
	}

	return trimmed[:limit] + "..."
}

func toOpenAIMessage(message Message) (openAIMessage, error) {
	converted := openAIMessage{
		Role:       string(message.Role),
		Content:    message.Content,
		ToolCallID: message.ToolCallID,
		Name:       message.ToolName,
	}
	if len(message.ToolCalls) == 0 {
		return converted, nil
	}

	converted.ToolCalls = make([]openAIToolCall, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		if strings.TrimSpace(call.ID) == "" {
			return openAIMessage{}, fmt.Errorf("native tool call id is required")
		}
		arguments, err := json.Marshal(call.Arguments)
		if err != nil {
			return openAIMessage{}, fmt.Errorf("marshal native tool arguments: %w", err)
		}
		converted.ToolCalls = append(converted.ToolCalls, openAIToolCall{
			ID:   call.ID,
			Type: "function",
			Function: openAIFunctionCall{
				Name:      call.ToolName,
				Arguments: string(arguments),
			},
		})
	}
	return converted, nil
}

func toOpenAITool(spec tools.ToolSpec) openAITool {
	properties := make(map[string]openAIJSONProperty, len(spec.Parameters))
	required := make([]string, 0, len(spec.Parameters))
	for _, parameter := range spec.Parameters {
		properties[parameter.Name] = openAIJSONProperty{
			Type:        parameter.Type,
			Description: parameter.Description,
		}
		if parameter.Required {
			required = append(required, parameter.Name)
		}
	}
	return openAITool{
		Type: "function",
		Function: openAIFunctionSpec{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters: openAIJSONSchema{
				Type:       "object",
				Properties: properties,
				Required:   required,
			},
		},
	}
}

func parseOpenAIToolCalls(rawCalls []openAIToolCall) ([]tools.ToolCall, error) {
	if len(rawCalls) == 0 {
		return nil, nil
	}
	calls := make([]tools.ToolCall, 0, len(rawCalls))
	for _, raw := range rawCalls {
		if strings.TrimSpace(raw.ID) == "" {
			return nil, fmt.Errorf("native tool call id is required")
		}
		name := strings.TrimSpace(raw.Function.Name)
		if name == "" {
			return nil, fmt.Errorf("native tool name is required")
		}
		argumentsText := strings.TrimSpace(raw.Function.Arguments)
		if argumentsText == "" {
			argumentsText = "{}"
		}
		decoder := json.NewDecoder(strings.NewReader(argumentsText))
		decoder.UseNumber()
		var arguments map[string]any
		if err := decoder.Decode(&arguments); err != nil {
			return nil, fmt.Errorf("decode native tool arguments for %s: %w", name, err)
		}
		if arguments == nil {
			return nil, fmt.Errorf("native tool arguments for %s must be an object", name)
		}
		calls = append(calls, tools.ToolCall{
			ID:        strings.TrimSpace(raw.ID),
			ToolName:  name,
			Arguments: arguments,
		})
	}
	return calls, nil
}
