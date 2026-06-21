package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"agentlab/internal/llm"
	"agentlab/internal/requestctx"
	"agentlab/internal/tools"
)

const DefaultMaxToolLoopSteps = 3

var ErrToolLoopExceeded = errors.New("tool loop exceeded max steps")

type ToolLoopOptions struct {
	MaxSteps int
}

type toolExecutionResult struct {
	Result   tools.ToolResult
	Key      tools.ToolCallKey
	CacheHit bool
}

func normalizeToolLoopOptions(options ToolLoopOptions) ToolLoopOptions {
	if options.MaxSteps <= 0 {
		options.MaxSteps = DefaultMaxToolLoopSteps
	}
	return options
}

func buildToolInstructions(executor *tools.Executor, mode llm.ToolCallingMode) string {
	if executor == nil {
		return ""
	}
	specs := executor.Specs()
	if len(specs) == 0 {
		return ""
	}
	if mode == llm.ToolCallingModeNative {
		return renderNativeToolInstructions()
	}
	return renderToolInstructions(specs)
}

func renderNativeToolInstructions() string {
	return "[工具能力]\n" +
		"工具名称、描述和参数由模型接口的原生工具定义提供。\n" +
		"即使模型内部进行 thinking/reasoning，最终给用户看的回答也必须输出在 assistant message 的 content 中，不能只输出在 reasoning_content 或 thinking 字段中。\n" +
		"你可以在确实需要外部能力时调用工具。若不需要工具，请直接正常回答。\n" +
		"约束：每次回复最多请求一个工具；不要请求未提供的工具；不要重复请求相同工具和相同参数；如果已有工具结果足以回答用户问题，必须直接给出最终回答；只有当已有工具结果仍不足以完成回答时，才继续请求另一个工具。"
}

func renderToolInstructions(specs []tools.ToolSpec) string {
	var builder strings.Builder
	builder.WriteString("[工具能力]\n")
	builder.WriteString("即使模型内部进行 thinking/reasoning，最终给用户看的回答也必须输出在 assistant message 的 content 中，不能只输出在 reasoning_content 或 thinking 字段中。\n")
	builder.WriteString("你可以在确实需要外部能力时调用工具。若不需要工具，请直接正常回答。\n")
	builder.WriteString("如果需要调用工具，你的本次回复必须只输出一个 JSON 对象，不要包含解释、Markdown 或其他文本。\n")
	builder.WriteString("JSON 格式如下：\n")
	builder.WriteString(`{"tool_name":"工具名","arguments":{"参数名":"参数值"},"reason":"调用原因"}`)
	builder.WriteString("\n\n可用工具：\n")

	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	for _, spec := range specs {
		builder.WriteString("- ")
		builder.WriteString(spec.Name)
		builder.WriteString(": ")
		builder.WriteString(spec.Description)
		if len(spec.Parameters) == 0 {
			builder.WriteString("。参数：无")
			builder.WriteString("\n")
			continue
		}
		builder.WriteString("。参数：")
		for i, param := range spec.Parameters {
			if i > 0 {
				builder.WriteString("；")
			}
			builder.WriteString(param.Name)
			builder.WriteString("(")
			builder.WriteString(param.Type)
			if param.Required {
				builder.WriteString(", required")
			} else {
				builder.WriteString(", optional")
			}
			builder.WriteString(")")
			if strings.TrimSpace(param.Description) != "" {
				builder.WriteString(": ")
				builder.WriteString(param.Description)
			}
		}
		builder.WriteString("\n")
	}

	builder.WriteString("\n约束：每次回复最多请求一个工具；不要请求未列出的工具；不要重复请求相同工具和相同参数；如果已有工具结果足以回答用户问题，必须直接给出最终回答；只有当已有工具结果仍不足以完成回答时，才继续请求另一个工具。")
	return builder.String()
}

// runToolLoop 在同一次用户请求内执行受 MaxSteps 限制的顺序工具循环。
// 工具轨迹只保存在本轮工作消息中，不写入 Session。
func (b *Agent) runToolLoop(
	ctx context.Context,
	requestMessages []llm.Message,
	runCache map[tools.ToolCallKey]tools.ToolResult,
) (*llm.ChatResponse, error) {
	messages := append([]llm.Message(nil), requestMessages...)
	maxSteps := normalizeToolLoopOptions(b.toolLoopOptions).MaxSteps

	for step := 1; step <= maxSteps; step++ {
		request := llm.ChatRequest{
			Messages:        messages,
			ToolCallingMode: b.toolCallingMode,
		}
		if b.toolCallingMode == llm.ToolCallingModeNative {
			request.Tools = b.executor.Specs()
		}
		resp, err := b.client.Chat(ctx, request)
		if err != nil {
			log.Printf("[agent] request_id=%s 模型调用失败 step=%d: %v", requestctx.FromContext(ctx), step, err)
			return nil, err
		}

		call, err := b.toolCallFromResponse(resp)
		if errors.Is(err, tools.ErrToolCallNotPresent) {
			return resp, nil
		}
		if errors.Is(err, tools.ErrMultipleToolCalls) {
			return nil, tools.ErrMultipleToolCalls
		}
		if err != nil {
			return nil, err
		}

		log.Printf(
			"[agent] request_id=%s 检测到工具调用 tool_calling_mode=%s tool_calls_count=1 tool_calls_parse_status=success step=%d/%d tool=%s reason=%q tool_call_json=%s",
			requestctx.FromContext(ctx),
			b.toolCallingMode,
			step,
			maxSteps,
			call.ToolName,
			call.Reason,
			formatToolCallLogJSON(*call),
		)
		execution, err := executeToolWithRunCache(ctx, b.executor, *call, runCache)
		if err != nil {
			return nil, err
		}
		result := withToolLoopDiagnostics(execution.Result, step, execution.CacheHit, execution.Key)
		logToolLoopStep(ctx, step, maxSteps, call.ToolName, execution.Key, execution.CacheHit, result)
		log.Printf(
			"[agent] request_id=%s 工具执行完成 step=%d/%d tool=%s success=%t error=%q tool_result_json=%s",
			requestctx.FromContext(ctx),
			step,
			maxSteps,
			call.ToolName,
			result.Success,
			result.Error,
			formatToolResultLogJSON(result),
		)
		if b.toolCallingMode == llm.ToolCallingModeNative {
			messages = appendNativeToolFeedback(messages, *call, result)
		} else {
			messages = appendTextToolFeedback(messages, *call, result)
		}
	}

	return nil, ErrToolLoopExceeded
}

func (b *Agent) toolCallFromResponse(resp *llm.ChatResponse) (*tools.ToolCall, error) {
	if b.toolCallingMode == llm.ToolCallingModeNative {
		if len(resp.ToolCalls) == 0 {
			return nil, tools.ErrToolCallNotPresent
		}
		if len(resp.ToolCalls) > 1 {
			return nil, tools.ErrMultipleToolCalls
		}
		call := resp.ToolCalls[0]
		if strings.TrimSpace(call.ID) == "" {
			return nil, fmt.Errorf("%w: native tool call id is required", tools.ErrInvalidArguments)
		}
		if strings.TrimSpace(call.ToolName) == "" {
			return nil, fmt.Errorf("%w: tool_name is required", tools.ErrInvalidArguments)
		}
		if call.Arguments == nil {
			call.Arguments = map[string]any{}
		}
		return &call, nil
	}
	return tools.ParseToolCall(resp.Content)
}

func executeToolWithRunCache(
	ctx context.Context,
	executor *tools.Executor,
	call tools.ToolCall,
	runCache map[tools.ToolCallKey]tools.ToolResult,
) (toolExecutionResult, error) {
	if executor == nil {
		return toolExecutionResult{}, errors.New("tool executor is not configured")
	}

	key, err := tools.BuildToolCallKey(call)
	if err != nil {
		return toolExecutionResult{}, err
	}

	if runCache != nil {
		if result, ok := runCache[key]; ok {
			log.Printf(
				"[agent] request_id=%s tool_cache_hit=true tool=%s args_hash=%s cache_entries=%d",
				requestctx.FromContext(ctx),
				key.Name,
				key.ArgsHash,
				len(runCache),
			)
			return toolExecutionResult{
				Result:   result,
				Key:      key,
				CacheHit: true,
			}, nil
		}
		log.Printf(
			"[agent] request_id=%s tool_cache_hit=false tool=%s args_hash=%s cache_entries=%d",
			requestctx.FromContext(ctx),
			key.Name,
			key.ArgsHash,
			len(runCache),
		)
	}

	result := executor.Execute(ctx, call)
	if runCache != nil {
		runCache[key] = result
		log.Printf(
			"[agent] request_id=%s tool_cache_store=true tool=%s args_hash=%s cache_entries=%d",
			requestctx.FromContext(ctx),
			key.Name,
			key.ArgsHash,
			len(runCache),
		)
	}
	return toolExecutionResult{
		Result:   result,
		Key:      key,
		CacheHit: false,
	}, nil
}

func appendTextToolFeedback(messages []llm.Message, call tools.ToolCall, result tools.ToolResult) []llm.Message {
	// 用 assistant 消息保留“模型请求工具”的语义，再用 user 消息把工具结果作为下一步输入回填。
	// 一些 OpenAI 兼容模型会弱化后置 system 消息，使用 user 消息能更明确地推动下一步决策。
	messages = append(messages, llm.Message{
		Role:    llm.RoleAssistant,
		Content: "请求调用工具: " + call.ToolName + "\n原因: " + call.Reason,
	})
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: buildToolResultMessage(result),
	})
	return messages
}

func appendNativeToolFeedback(messages []llm.Message, call tools.ToolCall, result tools.ToolResult) []llm.Message {
	messages = append(messages, llm.Message{
		Role:      llm.RoleAssistant,
		ToolCalls: []tools.ToolCall{call},
	})
	messages = append(messages, llm.Message{
		Role:       llm.RoleTool,
		Content:    tools.FormatToolResult(result),
		ToolCallID: call.ID,
		ToolName:   call.ToolName,
	})
	return messages
}

func withToolLoopDiagnostics(result tools.ToolResult, step int, cacheHit bool, key tools.ToolCallKey) tools.ToolResult {
	metadata := make(map[string]any, len(result.Metadata)+3)
	for k, v := range result.Metadata {
		metadata[k] = v
	}
	metadata["step"] = step
	metadata["cache_hit"] = cacheHit
	metadata["tool_call_key"] = formatToolCallKey(key)
	result.Metadata = metadata
	return result
}

func logToolLoopStep(
	ctx context.Context,
	step int,
	maxSteps int,
	toolName string,
	key tools.ToolCallKey,
	cacheHit bool,
	result tools.ToolResult,
) {
	log.Printf(
		"[agent] request_id=%s tool_loop_step=true step=%d max_steps=%d tool_name=%s tool_call_key=%s cache_hit=%t success=%t error_code=%s",
		requestctx.FromContext(ctx),
		step,
		maxSteps,
		toolName,
		formatToolCallKey(key),
		cacheHit,
		result.Success,
		toolResultErrorCode(result),
	)
}

func formatToolCallKey(key tools.ToolCallKey) string {
	return key.Name + ":" + key.ArgsHash
}

func toolResultErrorCode(result tools.ToolResult) string {
	if result.Success || strings.TrimSpace(result.Error) == "" {
		return ""
	}
	return "tool_error"
}

func formatToolResultLogJSON(result tools.ToolResult) string {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"marshal tool result log: %s"}`, err)
	}
	return string(data)
}

func formatToolCallLogJSON(call tools.ToolCall) string {
	data, err := json.Marshal(call)
	if err != nil {
		return fmt.Sprintf(`{"tool_name":%q,"error":"marshal tool call log: %s"}`, call.ToolName, err)
	}
	return string(data)
}

func buildToolResultMessage(result tools.ToolResult) string {
	return "工具执行结果如下。\n" +
		"如果该结果已经足以回答用户原问题，你现在必须进入 FINAL_ANSWER 阶段并直接回答；不要输出 tool_call JSON；不要输出 Markdown 代码块。\n" +
		"只有当该结果仍不足以完成回答时，才可以继续请求另一个不同工具；禁止重复请求相同工具和相同参数。\n" +
		tools.FormatToolResult(result)
}

func fallbackAnswerFromToolResult(call tools.ToolCall, result tools.ToolResult) string {
	if !result.Success {
		if strings.TrimSpace(result.Error) == "" {
			return "工具 " + call.ToolName + " 执行失败。"
		}
		return "工具 " + call.ToolName + " 执行失败：" + result.Error
	}

	switch call.ToolName {
	case "time":
		date, _ := result.Metadata["date"].(string)
		timeText, _ := result.Metadata["time"].(string)
		timezone, _ := result.Metadata["timezone"].(string)
		if date != "" && timeText != "" && timezone != "" {
			return "当前时间是 " + date + " " + timeText + "（" + timezone + "）。"
		}
	case "calculator":
		if strings.TrimSpace(result.Content) != "" {
			return "计算结果是 " + result.Content + "。"
		}
	case "file_read":
		if strings.TrimSpace(result.Content) != "" {
			return "文件内容如下：\n" + result.Content
		}
	case "web_search":
		if strings.TrimSpace(result.Content) != "" {
			return "搜索结果如下：\n" + result.Content
		}
	}

	if strings.TrimSpace(result.Content) == "" {
		return "工具 " + call.ToolName + " 已执行成功。"
	}
	return result.Content
}
