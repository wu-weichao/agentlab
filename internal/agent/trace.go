package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"agentlab/internal/llm"
	"agentlab/internal/requestctx"
	"agentlab/internal/tools"
)

// RunStepType 表示结构化运行轨迹中的稳定步骤类型。
type RunStepType string

const (
	RunStepModelCall   RunStepType = "model_call"
	RunStepToolCall    RunStepType = "tool_call"
	RunStepCacheReuse  RunStepType = "cache_reuse"
	RunStepFinalAnswer RunStepType = "final_answer"
)

// TerminationReason 表示一次运行结束的稳定分类。
type TerminationReason string

const (
	TerminationCompleted        TerminationReason = "completed"
	TerminationContextError     TerminationReason = "context_error"
	TerminationModelError       TerminationReason = "model_error"
	TerminationToolError        TerminationReason = "tool_error"
	TerminationMaxStepsExceeded TerminationReason = "max_steps_exceeded"
	TerminationInvalidToolCall  TerminationReason = "invalid_tool_call"
)

// RunErrorCode 表示步骤失败的稳定机器可读分类。
type RunErrorCode string

const (
	RunErrorNone            RunErrorCode = ""
	RunErrorModel           RunErrorCode = "model_error"
	RunErrorTool            RunErrorCode = "tool_error"
	RunErrorInvalidToolCall RunErrorCode = "invalid_tool_call"
	RunErrorMaxSteps        RunErrorCode = "max_steps_exceeded"
)

// RunResult 描述一次 Agent 运行的最终回答和进程内结构化轨迹。
type RunResult struct {
	RequestID         string
	FinalAnswer       string
	Steps             []RunStep
	TotalSteps        int
	Duration          time.Duration
	TerminationReason TerminationReason
}

// RunStep 描述一次模型、工具、缓存或最终回答事件。
type RunStep struct {
	Index           int
	RequestID       string
	StepID          string
	Type            RunStepType
	StartedAt       time.Time
	Duration        time.Duration
	Success         bool
	ErrorCode       RunErrorCode
	ToolCallingMode llm.ToolCallingMode
	ToolName        string
	ToolCallKey     string
	CacheHit        bool
	Content         string
	Metadata        map[string]any
}

type traceRecorder struct {
	requestID string
	startedAt time.Time
	steps     []RunStep
}

func newTraceRecorder(requestID string) *traceRecorder {
	return &traceRecorder{
		requestID: requestID,
		startedAt: time.Now(),
	}
}

func (r *traceRecorder) add(step RunStep) RunStep {
	step.Index = len(r.steps) + 1
	step.RequestID = r.requestID
	step.StepID = fmt.Sprintf("%s-step-%d", r.requestID, step.Index)
	if step.StartedAt.IsZero() {
		step.StartedAt = time.Now()
	}
	if step.Duration < 0 {
		step.Duration = 0
	}
	step.Metadata = cloneMetadata(step.Metadata)
	r.steps = append(r.steps, step)
	return step
}

func (r *traceRecorder) result(finalAnswer string, reason TerminationReason) RunResult {
	steps := make([]RunStep, len(r.steps))
	copy(steps, r.steps)
	return RunResult{
		RequestID:         r.requestID,
		FinalAnswer:       finalAnswer,
		Steps:             steps,
		TotalSteps:        len(steps),
		Duration:          time.Since(r.startedAt),
		TerminationReason: reason,
	}
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func cloneToolResult(result tools.ToolResult) tools.ToolResult {
	result.Metadata = cloneMetadata(result.Metadata)
	return result
}

type traceRecorderKey struct{}
type modelOperationKey struct{}

func withTraceRecorder(ctx context.Context, recorder *traceRecorder) context.Context {
	return context.WithValue(ctx, traceRecorderKey{}, recorder)
}

func traceRecorderFromContext(ctx context.Context) *traceRecorder {
	recorder, _ := ctx.Value(traceRecorderKey{}).(*traceRecorder)
	return recorder
}

func addTraceStep(ctx context.Context, step RunStep) RunStep {
	if recorder := traceRecorderFromContext(ctx); recorder != nil {
		return recorder.add(step)
	}
	return step
}

func withModelOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, modelOperationKey{}, operation)
}

func modelOperationFromContext(ctx context.Context) string {
	operation, _ := ctx.Value(modelOperationKey{}).(string)
	if operation == "" {
		return "summary"
	}
	return operation
}

type tracingClient struct {
	delegate llm.Client
}

func (c tracingClient) Chat(ctx context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	startedAt := time.Now()
	resp, err := c.delegate.Chat(ctx, request)
	recorder := traceRecorderFromContext(ctx)
	if recorder == nil {
		return resp, err
	}

	step := recorder.add(RunStep{
		Type:            RunStepModelCall,
		StartedAt:       startedAt,
		Duration:        time.Since(startedAt),
		Success:         err == nil,
		ErrorCode:       RunErrorNone,
		ToolCallingMode: request.ToolCallingMode,
		Metadata: map[string]any{
			"operation": modelOperationFromContext(ctx),
		},
	})
	if err != nil {
		recorder.steps[len(recorder.steps)-1].ErrorCode = RunErrorModel
	}
	log.Printf(
		"[agent] request_id=%s step_id=%s run_step=%s operation=%s success=%t duration=%s",
		requestctx.FromContext(ctx),
		step.StepID,
		step.Type,
		modelOperationFromContext(ctx),
		err == nil,
		step.Duration,
	)
	return resp, err
}
