package session

import "agentlab/internal/llm"

// Session 负责在内存中维护当前会话的结构化上下文状态。
type Session struct {
	systemPrompt string
	summary      string
	// recent 只保存尚未被折叠进 rolling summary 的原始消息窗口。
	recent []llm.Message
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
	s.recent = append(s.recent, llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	})
}

// AddAssistantMessage 向会话中追加一条助手消息。
func (s *Session) AddAssistantMessage(content string) {
	s.recent = append(s.recent, llm.Message{
		Role:    llm.RoleAssistant,
		Content: content,
	})
}

// SetRollingSummary 更新滚动摘要。
func (s *Session) SetRollingSummary(content string) {
	s.summary = content
}

// RollingSummary 返回当前滚动摘要。
func (s *Session) RollingSummary() string {
	return s.summary
}

// SetRecentMessages 用裁剪后的消息窗口整体替换 recent 状态。
func (s *Session) SetRecentMessages(messages []llm.Message) {
	s.recent = cloneMessages(messages)
}

// RecentMessages 返回近期原始消息快照。
func (s *Session) RecentMessages() []llm.Message {
	return cloneMessages(s.recent)
}

// SystemPrompt 返回启动时渲染后的 system prompt。
func (s *Session) SystemPrompt() string {
	return s.systemPrompt
}

// AppendSystemPromptSection 在启动阶段向 system prompt 追加全局能力说明。
// 该方法用于把工具能力等 Agent 级说明固化到会话系统提示中，而不是每轮临时注入。
func (s *Session) AppendSystemPromptSection(section string) {
	if section == "" {
		return
	}
	if s.systemPrompt == "" {
		s.systemPrompt = section
		return
	}
	s.systemPrompt = s.systemPrompt + "\n\n" + section
}

// History 返回按 system -> summary -> recent 顺序组织的会话历史。
func (s *Session) History() []llm.Message {
	messages := []llm.Message{{
		Role:    llm.RoleSystem,
		Content: s.systemPrompt,
	}}
	if s.summary != "" {
		messages = append(messages, llm.Message{
			Role:    llm.RoleSummary,
			Content: s.summary,
		})
	}
	messages = append(messages, s.recent...)
	return messages
}

// Reset 只清空 summary 和 recent；
// system prompt 由独立字段保存，因此不需要重新写回消息切片。
func (s *Session) Reset() {
	s.summary = ""
	s.recent = nil
}

// cloneMessages 返回独立副本，避免会话内部状态被外部直接修改。
func cloneMessages(messages []llm.Message) []llm.Message {
	cloned := make([]llm.Message, len(messages))
	copy(cloned, messages)
	return cloned
}
