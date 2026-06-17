package app

import (
	"context"
	"errors"
	"log"

	"agentlab/internal/requestctx"
	"agentlab/internal/tools"
)

func executeToolWithRunCache(
	ctx context.Context,
	executor *tools.Executor,
	call tools.ToolCall,
	runCache map[tools.ToolCallKey]tools.ToolResult,
) (tools.ToolResult, error) {
	if executor == nil {
		return tools.ToolResult{}, errors.New("tool executor is not configured")
	}

	key, err := tools.BuildToolCallKey(call)
	if err != nil {
		return tools.ToolResult{}, err
	}

	if runCache != nil {
		if result, ok := runCache[key]; ok {
			log.Printf(
				"[chatbot] request_id=%s tool_cache_hit=true tool=%s args_hash=%s cache_entries=%d",
				requestctx.FromContext(ctx),
				key.Name,
				key.ArgsHash,
				len(runCache),
			)
			return result, nil
		}
		log.Printf(
			"[chatbot] request_id=%s tool_cache_hit=false tool=%s args_hash=%s cache_entries=%d",
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
			"[chatbot] request_id=%s tool_cache_store=true tool=%s args_hash=%s cache_entries=%d",
			requestctx.FromContext(ctx),
			key.Name,
			key.ArgsHash,
			len(runCache),
		)
	}
	return result, nil
}
