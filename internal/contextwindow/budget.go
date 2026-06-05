package contextwindow

import "agentlab/internal/llm"

const (
	// summarySystemPrefix 把内部 summary 语义转换成标准 system 消息，避免侵入 LLM 协议层。
	summarySystemPrefix = "以下是对较早对话历史的结构化摘要，请将其视为已确认上下文：\n"
	// messageOverhead 用一个固定常数近似 role、JSON 包装等额外开销。
	messageOverhead = 8
)

// EstimateMessagesChars 使用粗粒度字符数估算消息长度。
func EstimateMessagesChars(messages []llm.Message) int {
	total := 0
	for _, message := range messages {
		total += EstimateMessageChars(message)
	}
	return total
}

// EstimateMessageChars 估算单条消息的长度。
func EstimateMessageChars(message llm.Message) int {
	return len(message.Content) + len(message.Role) + messageOverhead
}

// RemainingSummaryBudget 估算在保留 system prompt 和 recent messages 后，
// rolling summary 还能占用多少字符预算。
func RemainingSummaryBudget(systemPrompt string, recent []llm.Message, maxChars int) int {
	base := []llm.Message{{
		Role:    llm.RoleSystem,
		Content: systemPrompt,
	}}
	base = append(base, cloneMessages(recent)...)

	remaining := maxChars - EstimateMessagesChars(base) - EstimateMessageChars(renderSummaryMessage(""))
	if remaining < 0 {
		return 0
	}
	return remaining
}

// renderSummaryMessage 把内部滚动摘要包装成可直接发给模型的 system 风格消息。
func renderSummaryMessage(summary string) llm.Message {
	return llm.Message{
		Role:    llm.RoleSystem,
		Content: summarySystemPrefix + summary,
	}
}
