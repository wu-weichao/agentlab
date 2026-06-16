package app

import (
	"context"
	"errors"

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
			return result, nil
		}
	}

	result := executor.Execute(ctx, call)
	if runCache != nil {
		runCache[key] = result
	}
	return result, nil
}
