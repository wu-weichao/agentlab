## Why

当前调用方只能从 `Agent.Run()` 获得最终文本，运行中的模型步骤、工具执行、缓存复用、耗时和错误原因主要依赖日志排查。原生 Tool Calling 已稳定后，需要把单次运行过程提升为结构化数据，使调用方能够直接消费和测试运行轨迹，而不再解析日志字符串。

## What Changes

- 新增结构化 `RunResult`，同时返回最终回答、步骤轨迹、总耗时、总步骤数和稳定终止原因。
- 新增 `RunStep`，区分模型调用、工具调用、缓存复用和最终回答，并记录步骤序号、request ID、step ID、耗时、成功状态和稳定错误分类。
- 工具步骤额外记录工具名、tool call key、cache hit、执行耗时和工具错误代码；工具结果正文与诊断 metadata 分离。
- 为 Agent 增加结构化运行入口，完整收集本轮执行轨迹；现有 `Agent.Run()` 保留并委托新入口，只返回最终文本。
- 轨迹只描述当前运行，不写入 Session，不持久化，也不暴露模型隐藏推理内容。
- 日志和结构化轨迹复用同一 request ID 与 step ID，保持既有日志和工具执行语义不变。
- 更新 `docs/evolution.md`，记录 v0.0.10 的能力、实现思路、可观测性、边界和下一步。

## Capabilities

### New Capabilities

- `structured-run-trace`: 定义单次 Agent 运行结果、步骤类型、耗时与错误分类、工具诊断字段、终止原因及轨迹数据边界。

### Modified Capabilities

- `agent-runtime`: 增加结构化运行入口，并规定现有 `Agent.Run()` 委托该入口且保持返回值、错误和会话写回兼容。

## Impact

- 影响 `internal/agent`：新增运行结果与步骤类型，调整运行和工具循环内部返回值以收集轨迹。
- 影响 `internal/requestctx` 或 Agent 内部步骤标识逻辑：为日志与轨迹提供一致的 request ID 和 step ID。
- 影响 `internal/app`：继续通过兼容 `Agent.Run()` 工作，无需改变 CLI 命令和交互方式。
- 影响测试：补充普通回答、顺序工具、缓存命中、工具失败、模型失败和 MaxSteps 终止的结构化轨迹断言。
- 不改变 LLM provider 协议、Tool 接口、runCache 生命周期和 Session 历史语义；不引入 OpenTelemetry、指标平台、轨迹持久化或隐藏推理暴露。
