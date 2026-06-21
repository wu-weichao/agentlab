package agent

import (
	"bytes"
	"context"
	"errors"
	"log"
	"reflect"
	"strings"
	"testing"

	"agentlab/internal/llm"
	"agentlab/internal/session"
	"agentlab/internal/tools"
)

func TestRunWithTraceOrdinaryAnswer(t *testing.T) {
	sess := session.New("system prompt")
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{{content: "hello"}},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        tools.NewExecutor(),
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "hi")
	if err != nil {
		t.Fatalf("RunWithTrace returned error: %v", err)
	}
	if result.FinalAnswer != "hello" || result.TerminationReason != TerminationCompleted {
		t.Fatalf("unexpected result: %#v", result)
	}
	assertStepTypes(t, result.Steps, RunStepModelCall, RunStepFinalAnswer)
	if result.TotalSteps != len(result.Steps) || result.Duration < 0 || result.RequestID == "" {
		t.Fatalf("invalid result summary: %#v", result)
	}
	seen := map[string]bool{}
	for index, step := range result.Steps {
		if step.Index != index+1 || step.RequestID != result.RequestID || step.StepID == "" || seen[step.StepID] {
			t.Fatalf("invalid step identity: %#v", step)
		}
		if step.Duration < 0 || !step.Success {
			t.Fatalf("unexpected step status: %#v", step)
		}
		seen[step.StepID] = true
	}
}

func TestRunWithTraceSequentialToolsAndCacheReuse(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{
				{toolCalls: []tools.ToolCall{{ID: "call-1", ToolName: "counting", Arguments: map[string]any{"a": 1}}}},
				{toolCalls: []tools.ToolCall{{ID: "call-2", ToolName: "counting", Arguments: map[string]any{"a": 1.0}}}},
				{content: "done"},
			},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        executor,
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "run")
	if err != nil {
		t.Fatalf("RunWithTrace returned error: %v", err)
	}
	assertStepTypes(
		t,
		result.Steps,
		RunStepModelCall,
		RunStepToolCall,
		RunStepModelCall,
		RunStepCacheReuse,
		RunStepModelCall,
		RunStepFinalAnswer,
	)
	if tool.calls != 1 {
		t.Fatalf("expected one real tool execution, got %d", tool.calls)
	}
	firstTool := result.Steps[1]
	cachedTool := result.Steps[3]
	if firstTool.CacheHit || !cachedTool.CacheHit || firstTool.ToolCallKey != cachedTool.ToolCallKey {
		t.Fatalf("unexpected cache trace: first=%#v cached=%#v", firstTool, cachedTool)
	}
	if firstTool.Metadata["cache_hit"] != false || cachedTool.Metadata["cache_hit"] != true {
		t.Fatalf("unexpected cache metadata: first=%#v cached=%#v", firstTool.Metadata, cachedTool.Metadata)
	}
	cachedTool.Metadata["cache_hit"] = "mutated"
	if firstTool.Metadata["cache_hit"] != false {
		t.Fatalf("prior metadata was mutated: %#v", firstTool.Metadata)
	}
}

func TestRunWithTraceSequentialRealTools(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{
				{toolCalls: []tools.ToolCall{{ID: "call-1", ToolName: "counting", Arguments: map[string]any{"step": 1}}}},
				{toolCalls: []tools.ToolCall{{ID: "call-2", ToolName: "counting", Arguments: map[string]any{"step": 2}}}},
				{content: "done"},
			},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        executor,
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "run")
	if err != nil {
		t.Fatalf("RunWithTrace returned error: %v", err)
	}
	assertStepTypes(
		t,
		result.Steps,
		RunStepModelCall,
		RunStepToolCall,
		RunStepModelCall,
		RunStepToolCall,
		RunStepModelCall,
		RunStepFinalAnswer,
	)
	if tool.calls != 2 {
		t.Fatalf("expected two real tool executions, got %d", tool.calls)
	}
}

func TestRunWithTraceToolBusinessFailureCanComplete(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(&failingTool{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{
				{toolCalls: []tools.ToolCall{{ID: "call-fail", ToolName: "failing"}}},
				{content: "failure explained"},
			},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        executor,
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "run")
	if err != nil {
		t.Fatalf("RunWithTrace returned error: %v", err)
	}
	if result.TerminationReason != TerminationCompleted {
		t.Fatalf("expected completed termination, got %s", result.TerminationReason)
	}
	toolStep := result.Steps[1]
	if toolStep.Type != RunStepToolCall || toolStep.Success || toolStep.ErrorCode != RunErrorTool {
		t.Fatalf("unexpected failed tool step: %#v", toolStep)
	}
	if toolStep.Metadata["error_code"] != "planned" {
		t.Fatalf("expected tool metadata, got %#v", toolStep.Metadata)
	}
}

func TestRunWithTraceReturnsPartialTraceForModelFailure(t *testing.T) {
	wantErr := errors.New("planned model failure")
	sess := session.New("system prompt")
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{{err: wantErr}},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        tools.NewExecutor(),
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "run")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if result.TerminationReason != TerminationModelError {
		t.Fatalf("unexpected termination reason: %s", result.TerminationReason)
	}
	assertStepTypes(t, result.Steps, RunStepModelCall)
	if result.Steps[0].Success || result.Steps[0].ErrorCode != RunErrorModel {
		t.Fatalf("unexpected model failure step: %#v", result.Steps[0])
	}
	if history := sess.History(); len(history) != 2 || history[1].Role != llm.RoleUser {
		t.Fatalf("failed run must not append assistant: %#v", history)
	}
}

func TestRunWithTraceClassifiesInvalidToolCall(t *testing.T) {
	sess := session.New("system prompt")
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{{toolCalls: []tools.ToolCall{
				{ID: "call-1", ToolName: "counting"},
				{ID: "call-2", ToolName: "counting"},
			}}},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        tools.NewExecutor(),
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "run")
	if !errors.Is(err, tools.ErrMultipleToolCalls) {
		t.Fatalf("expected ErrMultipleToolCalls, got %v", err)
	}
	if result.TerminationReason != TerminationInvalidToolCall {
		t.Fatalf("unexpected termination reason: %s", result.TerminationReason)
	}
	assertStepTypes(t, result.Steps, RunStepModelCall)
}

func TestRunWithTraceClassifiesMaxSteps(t *testing.T) {
	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	tool := &countingTool{}
	if err := executor.Register(tool); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{
				{toolCalls: []tools.ToolCall{{ID: "call-1", ToolName: "counting"}}},
			},
		},
		Session:       sess,
		ContextConfig: testContextConfig(),
		Executor:      executor,
		ToolLoopOptions: ToolLoopOptions{
			MaxSteps: 1,
		},
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "run")
	if !errors.Is(err, ErrToolLoopExceeded) {
		t.Fatalf("expected ErrToolLoopExceeded, got %v", err)
	}
	if result.TerminationReason != TerminationMaxStepsExceeded {
		t.Fatalf("unexpected termination reason: %s", result.TerminationReason)
	}
	assertStepTypes(t, result.Steps, RunStepModelCall, RunStepToolCall)
}

func TestRunDelegatesStructuredExecution(t *testing.T) {
	sess := session.New("system prompt")
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{{content: "hello"}},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        tools.NewExecutor(),
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	reply, err := runtime.Run(context.Background(), "hi")
	if err != nil || reply != "hello" {
		t.Fatalf("unexpected Run result: reply=%q err=%v", reply, err)
	}
	if history := sess.History(); len(history) != 3 || history[2].Content != "hello" {
		t.Fatalf("unexpected session writeback: %#v", history)
	}
}

func TestRunWithTraceLogsCorrelatedStepIDs(t *testing.T) {
	var buf bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})

	sess := session.New("system prompt")
	executor := tools.NewExecutor()
	if err := executor.Register(&countingTool{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{
				{toolCalls: []tools.ToolCall{{ID: "call-1", ToolName: "counting"}}},
				{content: "done"},
			},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        executor,
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "run")
	if err != nil {
		t.Fatalf("RunWithTrace returned error: %v", err)
	}
	logText := buf.String()
	for _, step := range result.Steps {
		if !strings.Contains(logText, "request_id="+result.RequestID) ||
			!strings.Contains(logText, "step_id="+step.StepID) {
			t.Fatalf("missing trace correlation for %#v in logs %q", step, logText)
		}
	}
	for _, want := range []string{"tool_loop_step=true", "step=1", "max_steps=3", "tool_call_key=counting:", "cache_hit=false"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected existing log field %q, got %q", want, logText)
		}
	}
}

func TestRunWithTraceKeepsTraceOutOfSessionAndReasoningAPI(t *testing.T) {
	sess := session.New("system prompt")
	runtime := New(Options{
		Client: &scriptedChatClient{
			responses: []scriptedChatResponse{{content: "visible answer"}},
		},
		Session:         sess,
		ContextConfig:   testContextConfig(),
		Executor:        tools.NewExecutor(),
		ToolCallingMode: llm.ToolCallingModeNative,
	})

	result, err := runtime.RunWithTrace(context.Background(), "hi")
	if err != nil {
		t.Fatalf("RunWithTrace returned error: %v", err)
	}
	history := sess.History()
	if len(history) != 3 || history[1].Content != "hi" || history[2].Content != "visible answer" {
		t.Fatalf("trace leaked into session: %#v", history)
	}
	stepType := reflect.TypeOf(RunStep{})
	for _, field := range []string{"Reasoning", "ReasoningContent", "Thinking"} {
		if _, ok := stepType.FieldByName(field); ok {
			t.Fatalf("RunStep must not expose hidden reasoning field %s", field)
		}
	}
	if result.Steps[len(result.Steps)-1].Content != "visible answer" {
		t.Fatalf("final step must contain only visible answer: %#v", result.Steps)
	}
}

func assertStepTypes(t *testing.T, steps []RunStep, want ...RunStepType) {
	t.Helper()
	if len(steps) != len(want) {
		t.Fatalf("expected %d steps, got %d: %#v", len(want), len(steps), steps)
	}
	for index, stepType := range want {
		if steps[index].Type != stepType {
			t.Fatalf("step %d: expected %s, got %s", index, stepType, steps[index].Type)
		}
	}
}
