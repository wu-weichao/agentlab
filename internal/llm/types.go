package llm

// Role 表示一条对话消息的角色类型。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSummary   Role = "summary"
)

// Message 是聊天上下文中的基础消息结构。
type Message struct {
	Role    Role
	Content string
}

// ChatResponse 表示模型返回的最小响应结果。
type ChatResponse struct {
	Content string
}
