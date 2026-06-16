## Why

当前 Tool Calling 已完成单次工具闭环，但重复工具调用仍主要依赖 Prompt 约束。后续进入多步 Tool Loop 前，Runtime 需要先具备稳定识别“相同工具调用”的基础设施，否则无法可靠复用同轮工具结果。

本变更基于 `v0.0.5-tool-call-key-and-run-cache.md`，先补齐 `ToolCallKey`、参数标准化和 runCache 执行包装，为后续 `MaxSteps` 多步工具循环打基础。

## What Changes

- 新增 `ToolCallKey`，用工具名和标准化参数 hash 唯一标识一次工具调用。
- 新增工具参数标准化逻辑，确保 JSON 字段顺序不同但语义相同的参数生成相同 key。
- 新增一次请求作用域的 run-level tool cache 数据结构。
- 新增工具执行包装逻辑，在给定 run cache 时可复用相同 `ToolCallKey` 的结果。
- 将执行包装接入当前单工具闭环；当前每轮仍最多执行一次工具，但执行路径已经统一经过 run cache。
- 工具结果回填给第二次模型请求时使用 `user` 消息承载工具结果和 `FINAL_ANSWER` 阶段约束，降低兼容模型在回填后继续输出 `tool_call` 的概率。
- 增强诊断日志：工具执行完成日志输出完整单行 `ToolResult` JSON 字符串，OpenAI 兼容客户端发送请求日志输出完整请求 body。
- 不引入跨轮缓存、TTL、`ToolMetadata`、`MaxSteps` 或 Agent Runtime。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `tool-calling`: 增加 Runtime 级工具调用标识和 runCache 基础设施要求，为后续同轮重复工具调用去重提供基础。

## Impact

- 影响 `internal/tools`：新增 `ToolCallKey`、参数标准化和 hash 逻辑。
- 影响 `internal/app`：新增或抽出带 run cache 的工具执行包装逻辑，并调整工具结果二次回填消息与工具结果日志。
- 影响 `internal/llm`：OpenAI 兼容客户端发送请求日志输出完整 JSON request body。
- 影响测试：新增 key 构建、参数标准化和执行包装层 runCache 复用测试。
- 不改 `Tool` 接口，不新增外部依赖，不改变 CLI 使用方式。
- 不实现跨轮 `ToolCache`、TTL、`ToolMetadata`、多步 Tool Loop 或 Agent Runtime。
