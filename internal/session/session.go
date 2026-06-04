package session

import "agentlab/internal/llm"

// Session 负责在内存中维护当前会话的消息历史。
type Session struct {
	systemPrompt string
	messages     []llm.Message
}

// New 创建一个新会话，并自动写入 system prompt 作为起始消息。
func New(systemPrompt string) *Session {
	s := &Session{
		systemPrompt: systemPrompt,
	}
	s.Reset()
	return s
}

// AddUserMessage 向会话中追加一条用户消息。
func (s *Session) AddUserMessage(content string) {
	s.messages = append(s.messages, llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	})
}

// AddAssistantMessage 向会话中追加一条助手消息。
func (s *Session) AddAssistantMessage(content string) {
	s.messages = append(s.messages, llm.Message{
		Role:    llm.RoleAssistant,
		Content: content,
	})
}

// Messages 返回当前消息快照，避免调用方直接修改内部切片。
func (s *Session) Messages() []llm.Message {
	messages := make([]llm.Message, len(s.messages))
	copy(messages, s.messages)
	return messages
}

// Reset 清空历史消息，并重新保留初始 system prompt。
func (s *Session) Reset() {
	s.messages = []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: s.systemPrompt,
		},
	}
}
