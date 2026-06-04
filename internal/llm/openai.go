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
			Timeout: 60 * time.Second,
		},
	}
}

// Chat 发送聊天补全请求，并返回首个 assistant 文本回复。
func (c *OpenAIClient) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	requestBody := openAIChatRequest{
		Model:       c.model,
		Messages:    make([]openAIMessage, 0, len(messages)),
		Temperature: c.temperature,
	}

	for _, message := range messages {
		requestBody.Messages = append(requestBody.Messages, openAIMessage{
			Role:    string(message.Role),
			Content: message.Content,
		})
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
	log.Printf("[llm/openai] 发送请求 provider=openai model=%s url=%s messages=%d last_message=%q", c.model, requestURL, len(messages), summarizeLastMessage(messages))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[llm/openai] 请求发送失败 model=%s err=%v", c.model, err)
		return nil, fmt.Errorf("send openai request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[llm/openai] 读取响应失败 status=%s err=%v", resp.Status, err)
		return nil, fmt.Errorf("read openai response: %w", err)
	}

	log.Printf("[llm/openai] 收到响应 status=%s duration=%s body_bytes=%d", resp.Status, time.Since(start), len(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[llm/openai] 请求失败详情 body=%s", summarizeText(strings.TrimSpace(string(body)), 300))
		return nil, fmt.Errorf("openai request failed: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}

	var completion openAIChatResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}
	log.Printf("[llm/openai] 响应内容 body=%s", strings.TrimSpace(string(body)))

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("openai response missing choices")
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		return nil, fmt.Errorf("openai response missing assistant content")
	}

	log.Printf("[llm/openai] 解析成功 choice_count=%d reply=%q", len(completion.Choices), summarizeText(content, 120))
	return &ChatResponse{Content: content}, nil
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

func summarizeLastMessage(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	last := messages[len(messages)-1]
	return summarizeText(string(last.Role)+": "+last.Content, 120)
}

func summarizeText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= limit {
		return trimmed
	}

	return trimmed[:limit] + "..."
}
