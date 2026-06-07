package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Executor 负责工具注册、查找和执行分发。
// 它是 ChatBot 与具体工具之间的唯一入口，避免在对话编排层硬编码工具名称和实现细节。
type Executor struct {
	tools map[string]Tool
}

// NewExecutor 创建一个空工具执行器。
func NewExecutor() *Executor {
	return &Executor{
		tools: map[string]Tool{},
	}
}

// NewDefaultExecutor 注册当前版本支持的四个默认工具。
// 未提供 searchClient 时仍注册 web_search，但该工具会在调用时返回“未配置”的结构化错误。
func NewDefaultExecutor(workspaceRoot string, searchClient SearchClient) *Executor {
	executor := NewExecutor()
	_ = executor.Register(NewCalculatorTool())
	_ = executor.Register(NewTimeTool())
	_ = executor.Register(NewFileReadTool(workspaceRoot, DefaultFileReadMaxBytes))
	if searchClient == nil {
		searchClient = UnavailableSearchClient{}
	}
	_ = executor.Register(NewWebSearchTool(searchClient, DefaultSearchLimit))
	return executor
}

// NewDefaultExecutorForCurrentWorkspace 使用当前工作目录作为 file_read 的工作区边界。
func NewDefaultExecutorForCurrentWorkspace(searchClient SearchClient) *Executor {
	workspace, err := os.Getwd()
	if err != nil {
		workspace = "."
	}
	return NewDefaultExecutor(workspace, searchClient)
}

// Register 注册单个工具，并拒绝空名称和重复名称。
func (e *Executor) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("%w: nil tool", ErrInvalidArguments)
	}
	name := strings.TrimSpace(tool.Name())
	if name == "" {
		return fmt.Errorf("%w: empty tool name", ErrInvalidArguments)
	}
	if _, exists := e.tools[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateTool, name)
	}
	e.tools[name] = tool
	return nil
}

// Execute 根据 ToolCall 分发到具体工具。
// 执行器只负责查找、兜底参数 map 和统一 metadata，不吞掉工具返回的结构化错误。
func (e *Executor) Execute(ctx context.Context, call ToolCall) ToolResult {
	name := strings.TrimSpace(call.ToolName)
	if name == "" {
		return errorResult("tool_name is required", map[string]any{"code": "invalid_arguments"})
	}

	tool, ok := e.tools[name]
	if !ok {
		return errorResult("tool not found: "+name, map[string]any{
			"code":      "tool_not_found",
			"tool_name": name,
		})
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}

	result := tool.Execute(ctx, call.Arguments)
	if result.Metadata == nil {
		result.Metadata = map[string]any{}
	}
	result.Metadata["tool_name"] = name
	return result
}

// Names 返回当前已注册的工具名称快照。
func (e *Executor) Names() []string {
	names := make([]string, 0, len(e.tools))
	for name := range e.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Specs 返回当前已注册工具的模型可见定义快照。
// ChatBot 会用这些定义生成 tool instruction，告诉模型哪些工具可用以及如何组织参数。
func (e *Executor) Specs() []ToolSpec {
	names := e.Names()
	specs := make([]ToolSpec, 0, len(names))
	for _, name := range names {
		tool := e.tools[name]
		specs = append(specs, ToolSpec{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}
	return specs
}
