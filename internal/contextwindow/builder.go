package contextwindow

import (
	"errors"

	"agentlab/internal/config"
	"agentlab/internal/llm"
)

// BuildInput 描述一次上下文构建所需的会话状态。
type BuildInput struct {
	SystemPrompt   string
	RollingSummary string
	RecentMessages []llm.Message
	Config         config.ContextConfig
}

// BuildResult 描述预算构建结果。
type BuildResult struct {
	Messages        []llm.Message
	RecentMessages  []llm.Message
	EvictedMessages []llm.Message
	EstimatedChars  int
	Trimmed         bool
}

// ErrContextBudgetExceeded 表示最小保留上下文仍超过预算。
var ErrContextBudgetExceeded = errors.New("context budget exceeded even after trimming")

// Builder 负责构建受预算约束的请求上下文。
type Builder struct{}

const minRecentTurns = 1

// NewBuilder 创建上下文构建器。
func NewBuilder() *Builder {
	return &Builder{}
}

// Build 基于会话状态和预算组装发送给模型的消息列表。
func (b *Builder) Build(input BuildInput) (*BuildResult, error) {
	// 先把 recent messages 切成“完整轮次”或“当前悬空 user 输入”的片段，
	// 后续裁剪时才能保证不拆半轮对话。
	segments := splitSegments(input.RecentMessages)
	preferredProtected, hardProtected := protectedSegments(segments, input.Config.KeepRecentTurns, minRecentTurns)
	evictable := evictableIndices(segments, preferredProtected, hardProtected)

	messages := composeMessages(input.SystemPrompt, input.RollingSummary, input.RecentMessages)
	estimated := EstimateMessagesChars(messages)
	if estimated <= input.Config.MaxChars {
		return &BuildResult{
			Messages:       messages,
			RecentMessages: cloneMessages(input.RecentMessages),
			EstimatedChars: estimated,
			Trimmed:        false,
		}, nil
	}

	keptSegments := append([]segment(nil), segments...)
	var evicted []llm.Message
	trimmed := false
	for _, idx := range evictable {
		// 按从旧到新的顺序逐段驱逐，直到上下文重新落回预算内。
		evicted = append(evicted, keptSegments[idx].Messages...)
		keptSegments[idx].Dropped = true
		trimmed = true

		keptRecent := flattenSegments(keptSegments, true)
		messages = composeMessages(input.SystemPrompt, input.RollingSummary, keptRecent)
		estimated = EstimateMessagesChars(messages)
		if estimated <= input.Config.MaxChars {
			return &BuildResult{
				Messages:        messages,
				RecentMessages:  keptRecent,
				EvictedMessages: evicted,
				EstimatedChars:  estimated,
				Trimmed:         trimmed,
			}, nil
		}
	}

	return nil, ErrContextBudgetExceeded
}

type segment struct {
	Messages     []llm.Message
	CompleteTurn bool
	Dropped      bool
}

// composeMessages 固定输出顺序：
// system prompt -> rolling summary -> recent raw messages。
func composeMessages(systemPrompt string, rollingSummary string, recent []llm.Message) []llm.Message {
	messages := []llm.Message{{
		Role:    llm.RoleSystem,
		Content: systemPrompt,
	}}
	if rollingSummary != "" {
		messages = append(messages, renderSummaryMessage(rollingSummary))
	}
	messages = append(messages, cloneMessages(recent)...)
	return messages
}

// splitSegments 把消息序列切成便于裁剪的片段：
// 完整的 user/assistant 作为一轮，其余单条消息单独成段。
func splitSegments(messages []llm.Message) []segment {
	var segments []segment
	for i := 0; i < len(messages); {
		current := messages[i]
		if current.Role == llm.RoleUser && i+1 < len(messages) && messages[i+1].Role == llm.RoleAssistant {
			segments = append(segments, segment{
				Messages:     cloneMessages(messages[i : i+2]),
				CompleteTurn: true,
			})
			i += 2
			continue
		}

		segments = append(segments, segment{
			Messages: cloneMessages(messages[i : i+1]),
		})
		i++
	}
	return segments
}

// protectedSegments 同时返回两层保护：
// 1. preferredProtected: 尽量保留的最近 N 轮
// 2. hardProtected: 即使超预算也尽量不再继续裁掉的最小底线（最近 1 轮 + 当前悬空 user）
func protectedSegments(segments []segment, keepRecentTurns int, minKeepTurns int) ([]bool, []bool) {
	preferredProtected := make([]bool, len(segments))
	hardProtected := make([]bool, len(segments))
	remainingPreferred := keepRecentTurns
	remainingHard := minKeepTurns

	for i := len(segments) - 1; i >= 0; i-- {
		if !segments[i].CompleteTurn {
			if len(segments[i].Messages) == 1 && segments[i].Messages[0].Role == llm.RoleUser {
				preferredProtected[i] = true
				hardProtected[i] = true
			}
			continue
		}
		if remainingPreferred > 0 {
			preferredProtected[i] = true
			remainingPreferred--
		}
		if remainingHard > 0 {
			hardProtected[i] = true
			remainingHard--
		}
	}

	return preferredProtected, hardProtected
}

// evictableIndices 返回允许被驱逐的完整轮次索引。
// 驱逐顺序分两段：
// 1. 先驱逐“不在优先保留窗口内”的旧轮次
// 2. 若仍超预算，再逐步降低 keepRecentTurns，驱逐优先窗口中的较旧轮次
// hardProtected 所标记的最小底线轮次不会出现在返回结果里。
func evictableIndices(segments []segment, preferredProtected []bool, hardProtected []bool) []int {
	var indices []int
	for i := range segments {
		if segments[i].CompleteTurn && !preferredProtected[i] {
			indices = append(indices, i)
		}
	}
	for i := range segments {
		if segments[i].CompleteTurn && preferredProtected[i] && !hardProtected[i] {
			indices = append(indices, i)
		}
	}
	return indices
}

// flattenSegments 把片段重新拍平成消息列表；在裁剪路径里会跳过已驱逐片段。
func flattenSegments(segments []segment, skipDropped bool) []llm.Message {
	var messages []llm.Message
	for _, seg := range segments {
		if skipDropped && seg.Dropped {
			continue
		}
		messages = append(messages, seg.Messages...)
	}
	return messages
}

// cloneMessages 统一做切片防御性拷贝，避免调用方共享底层数组。
func cloneMessages(messages []llm.Message) []llm.Message {
	cloned := make([]llm.Message, len(messages))
	copy(cloned, messages)
	return cloned
}
