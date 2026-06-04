package app

import (
	"context"
	"log"
	"strings"

	"agentlab/internal/llm"
	"agentlab/internal/session"
)

// ChatBot 负责把 CLI 输入、会话上下文和模型调用串成一次完整对话。
type ChatBot struct {
	client  llm.Client
	session *session.Session
}

// NewChatBot 创建一个可执行多轮对话的 ChatBot 实例。
func NewChatBot(client llm.Client, sess *session.Session) *ChatBot {
	return &ChatBot{
		client:  client,
		session: sess,
	}
}

// Send 执行一轮对话：记录用户输入，调用模型，再把回复写回会话。
func (b *ChatBot) Send(ctx context.Context, input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	log.Printf("[chatbot] 收到用户输入，长度=%d", len(trimmed))
	b.session.AddUserMessage(trimmed)

	resp, err := b.client.Chat(ctx, b.session.Messages())
	if err != nil {
		log.Printf("[chatbot] 模型调用失败: %v", err)
		return "", err
	}

	b.session.AddAssistantMessage(resp.Content)
	log.Printf("[chatbot] 本轮对话完成，回复长度=%d", len(resp.Content))
	return resp.Content, nil
}
