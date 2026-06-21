## 1. 结构化运行模型

- [x] 1.1 在 `internal/agent` 定义 `RunResult`、`RunStep`、步骤类型、终止原因和稳定错误代码常量。
- [x] 1.2 实现运行级轨迹记录器，生成从 1 开始的步骤序号、request 内唯一 step ID，并汇总总步骤数与总耗时。
- [x] 1.3 实现 ToolResult 与 metadata 的安全复制，确保缓存复用步骤不会修改先前轨迹数据。

## 2. Agent 结构化入口

- [x] 2.1 新增 `Agent.RunWithTrace(ctx, input) (RunResult, error)`，迁移现有 `Run()` 的请求建立、上下文准备、工具循环和会话写回流程。
- [x] 2.2 让现有 `Agent.Run()` 委托 `RunWithTrace()`，成功时只返回最终回答，失败时保持原始错误语义。
- [x] 2.3 为上下文构建失败、模型失败、非法工具调用、工具 Runtime 错误和 MaxSteps 超限设置稳定终止原因，并在失败时返回部分轨迹。
- [x] 2.4 确认结构化入口只在成功时写入最终 assistant 消息，且不把任何轨迹步骤写入 Session、rolling summary 或 system prompt。

## 3. 模型与工具步骤采集

- [x] 3.1 在每次 LLM 调用边界记录 `model_call` 步骤，包含调用模式、耗时、成功状态和模型错误分类。
- [x] 3.2 在 runCache 未命中时记录 `tool_call` 步骤，包含工具名、tool call key、工具正文、复制后的 metadata、真实执行耗时和错误代码。
- [x] 3.3 在 runCache 命中时记录 `cache_reuse` 步骤，包含当前调用的诊断 metadata，并确保不重复执行真实工具。
- [x] 3.4 在模型返回用户可见普通文本时记录 `final_answer` 步骤，并完成 RunResult 的最终回答、步骤数、总耗时和完成终止原因。
- [x] 3.5 保持 native 与 text compatibility 两种模式、顺序工具循环、工具反馈、runCache key 和 MaxSteps 行为不变。

## 4. 日志关联

- [x] 4.1 为模型调用、工具执行、缓存复用和最终回答日志增加与 RunStep 一致的 `step_id`。
- [x] 4.2 保留既有 `request_id`、工具循环 `step`、`max_steps`、tool call key、cache hit 和完整工具协议日志字段语义。
- [x] 4.3 增加日志测试，验证 request ID 和 step ID 能将结构化轨迹与对应运行日志关联。

## 5. 结构化轨迹测试

- [x] 5.1 增加普通回答测试，断言 `model_call -> final_answer` 顺序、最终回答、唯一 step ID、步骤数、非负耗时和完成原因。
- [x] 5.2 增加单工具与顺序多工具测试，断言模型、工具和最终回答步骤按实际发生顺序返回。
- [x] 5.3 增加重复工具调用测试，断言第二次记录为 `cache_reuse`、真实工具只执行一次且先前 metadata 不被修改。
- [x] 5.4 增加 ToolResult 业务失败测试，断言工具步骤失败信息可见且后续模型成功时运行仍正常完成。
- [x] 5.5 增加模型错误、非法工具调用和 MaxSteps 超限测试，断言原始 error、部分轨迹及稳定终止原因。
- [x] 5.6 增加 `Run()` 与 `RunWithTrace()` 兼容测试，覆盖最终文本、错误、Session 写回和 ChatBot 委托行为。
- [x] 5.7 增加轨迹数据边界测试，确认不写入 Session、不自动持久化且不暴露 reasoning/thinking 字段。

## 6. 文档与验证

- [x] 6.1 更新 `docs/evolution.md`，新增 v0.0.10 的能力、实现思路、可观测性、边界和下一步。
- [x] 6.2 对新增和修改的 Go 文件运行 `gofmt` 或 `go fmt ./...`。
- [x] 6.3 运行 `go test ./...`，确认结构化轨迹、现有 Agent、ChatBot、上下文和两种工具调用模式测试全部通过。
- [x] 6.4 运行 OpenSpec 校验与状态命令，确认变更产物合法且全部实施任务可追踪。
