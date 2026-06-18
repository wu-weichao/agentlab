package app

import (
	"context"

	"agentlab/internal/agent"
	"agentlab/internal/config"
	"agentlab/internal/llm"
	"agentlab/internal/session"
	"agentlab/internal/tools"
)

// ChatBotOptions 保留原有 ChatBot 构造契约，并映射到 Agent Runtime。
type ChatBotOptions struct {
	Client          llm.Client
	Session         *session.Session
	ContextConfig   config.ContextConfig
	Executor        *tools.Executor
	ToolLoopOptions ToolLoopOptions
}

// ChatBot 是 CLI/app 层兼容适配器，Runtime 行为由 Agent 负责。
type ChatBot struct {
	agent *agent.Agent
}

// NewChatBot 创建兼容现有调用方式的 ChatBot。
func NewChatBot(options ChatBotOptions) *ChatBot {
	return &ChatBot{
		agent: agent.New(agent.Options{
			Client:          options.Client,
			Session:         options.Session,
			ContextConfig:   options.ContextConfig,
			Executor:        options.Executor,
			ToolLoopOptions: options.ToolLoopOptions,
		}),
	}
}

// Send 委托 Agent.Run 执行一轮对话。
func (b *ChatBot) Send(ctx context.Context, input string) (string, error) {
	return b.agent.Run(ctx, input)
}
