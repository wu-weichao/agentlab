package contextwindow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agentlab/internal/llm"
)

func TestSummarizerUpdateUsesLLMToProduceStructuredSummary(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{content: "[用户目标]\n- 学习 Agent\n\n[已确认约束]\n- 使用中文\n\n[已做决策]\n- 先做上下文窗口\n\n[关键事实]\n- 已有 prompt runtime\n\n[未解决问题]\n- 如何压缩历史"},
		},
	}

	summarizer := NewSummarizer(client)
	summary, err := summarizer.Update(context.Background(), "", []llm.Message{
		{Role: llm.RoleUser, Content: "我想学习 Agent，并且默认使用中文"},
		{Role: llm.RoleAssistant, Content: "先从 contextwindow 开始"},
	}, 300)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	for _, section := range summarySections {
		if !strings.Contains(summary, "["+section+"]") {
			t.Fatalf("missing section %s in summary: %s", section, summary)
		}
	}
	if len(client.calls) != 1 {
		t.Fatalf("expected 1 llm call, got %d", len(client.calls))
	}
}

func TestSummarizerUpdateCompressesOversizedSummaryWithSecondPass(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{content: strings.Repeat("很长的摘要内容 ", 20)},
			{content: "[用户目标]\n- 学习 Agent\n\n[已确认约束]\n- 中文\n\n[已做决策]\n- 先做上下文窗口\n\n[关键事实]\n- 有滚动摘要\n\n[未解决问题]\n- 下一步优化"},
		},
	}

	summarizer := NewSummarizer(client)
	summary, err := summarizer.Update(context.Background(), "", []llm.Message{
		{Role: llm.RoleUser, Content: "用户输入很长"},
	}, 120)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if len(summary) > 120 {
		t.Fatalf("expected compressed summary <= 120, got %d", len(summary))
	}
	if len(client.calls) != 2 {
		t.Fatalf("expected 2 llm calls, got %d", len(client.calls))
	}
}

func TestSummarizerUpdateReturnsClientError(t *testing.T) {
	summarizer := NewSummarizer(&scriptedClient{
		responses: []scriptedResponse{{err: errors.New("boom")}},
	})

	if _, err := summarizer.Update(context.Background(), "", []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, 120); err == nil {
		t.Fatal("expected error")
	}
}

type scriptedResponse struct {
	content string
	err     error
}

type scriptedClient struct {
	responses []scriptedResponse
	calls     [][]llm.Message
}

func (c *scriptedClient) Chat(_ context.Context, messages []llm.Message) (*llm.ChatResponse, error) {
	c.calls = append(c.calls, append([]llm.Message(nil), messages...))
	if len(c.responses) == 0 {
		return nil, errors.New("no scripted response")
	}

	next := c.responses[0]
	c.responses = c.responses[1:]
	if next.err != nil {
		return nil, next.err
	}
	return &llm.ChatResponse{Content: next.content}, nil
}
