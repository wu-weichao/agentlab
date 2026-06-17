## Why

当前 ChatBot 只支持单轮最多一次工具调用；当模型在工具结果基础上还需要继续读取、计算或查询时，系统会直接进入 fallback，无法表达真实 Agent 常见的轻量串行工具链。v0.0.5 已补齐 `ToolCallKey` 和同轮 runCache 基础设施，现在适合把工具闭环升级为受 `MaxSteps` 约束的多步 Tool Loop。

## What Changes

- 引入一次用户请求内的顺序多步 Tool Loop，让模型可以在同一轮 `Send()` 中按步骤请求多个工具。
- 新增 `ToolLoopOptions{MaxSteps int}`，首版使用默认常量 `3`，暂不接入配置文件。
- 将现有单工具闭环重构为 `runToolLoop()`，每一步最多解析并执行一个工具调用。
- 当模型返回普通文本时，将其视为最终回答并结束循环。
- 当模型返回合法 `tool_call` 时，执行工具、通过 runCache 去重，并把工具结果追加到本轮工作消息后继续下一步。
- 当达到 `MaxSteps` 仍没有最终回答时，返回明确的 `ErrToolLoopExceeded`，且不追加 assistant 消息。
- 继续拒绝一次响应中的多个工具调用，不支持并行执行或批量执行。
- 调整工具说明 Prompt：允许顺序工具步骤，但要求不要重复请求相同工具和相同参数。
- 使用 `ChatBotOptions` 收敛 ChatBot 初始化参数，避免多个构造函数或过长位置参数。
- 增强 runCache 日志，显式输出 `tool_cache_hit`、`tool_cache_store`、`args_hash` 和缓存条目数，便于确认是否通过缓存返回。
- 增加 LLM 输出约束：允许模型内部 thinking/reasoning，但最终用户可见回答必须进入 assistant `content`，系统不从 `reasoning_content` 做文字匹配提取。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `tool-calling`: 将“单轮一次工具调用闭环”扩展为“受 `MaxSteps` 约束的顺序多步工具循环”，并要求 runCache 在完整 `ChatBot.Send()` 流程中端到端生效。

## Impact

- 影响 `internal/app` 中 ChatBot 的工具调用编排、工具结果回填和错误处理路径。
- 复用 `internal/tools` 中现有 `ToolCall`、`ToolResult`、`ToolCallKey` 与 runCache 基础设施。
- 需要更新工具说明 Prompt 文案，使模型理解每次回复最多请求一个工具，但允许在已有结果不足时继续请求下一个工具。
- 需要补充单次模型调用、单工具兼容、多步工具链、重复工具调用缓存命中、步数超限、多工具调用拒绝和输出约束等测试。
- 不引入新的外部依赖，不改变现有工具接口，不写入跨轮缓存或完整工具轨迹。
