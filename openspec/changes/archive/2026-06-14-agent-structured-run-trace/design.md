## Context

`internal/agent.Agent.Run()` 当前完成 request ID 创建、用户消息写入、上下文准备、模型调用、顺序工具循环、runCache 和最终 assistant 写回，但只向调用方返回最终文本或错误。模型步骤、工具执行、缓存命中和终止原因只存在于日志及 `ToolResult.Metadata` 中，调用方无法稳定消费，也难以对完整运行过程做结构化断言。

v0.0.9 已将工具协议升级为 provider 原生 Tool Calling，并保留文本兼容模式。v0.0.10 需要在不改变模型协议、工具行为、Session 历史和 CLI 的前提下，为同一执行路径增加内存态结构化轨迹。

## Goals / Non-Goals

**Goals:**

- 提供包含最终回答、完整步骤、总耗时、总步骤数和终止原因的 `RunResult`。
- 通过 `RunStep` 表达模型调用、真实工具调用、缓存复用和最终回答四类事件。
- 为每个步骤提供稳定序号、request ID、step ID、耗时、成功状态和错误分类。
- 让结构化轨迹与现有日志复用 request ID 和 step ID。
- 保留 `Agent.Run()` 简单接口以及现有会话写回、工具循环和错误返回语义。
- 在失败时向结构化调用方返回已收集的部分轨迹。

**Non-Goals:**

- 不持久化或跨运行聚合轨迹。
- 不引入 OpenTelemetry、span、指标平台或外部依赖。
- 不暴露 provider 的 reasoning、thinking 或其他隐藏推理字段。
- 不改变 Tool、LLM provider、runCache、`MaxSteps` 或 Session 数据模型。
- 不为 CLI 增加轨迹展示命令。

## Decisions

### Decision 1: 新增 `RunWithTrace`，`Run` 委托并丢弃轨迹

在 `internal/agent` 增加：

```go
func (a *Agent) RunWithTrace(ctx context.Context, input string) (RunResult, error)
```

`RunResult` 使用值类型返回。即使运行失败，也返回已经建立的 request ID、已完成步骤、总耗时和失败终止原因；`error` 继续保留 Go 调用方现有的错误处理方式。`Agent.Run()` 调用 `RunWithTrace()`，成功时返回 `FinalAnswer`，失败时原样返回错误。

理由：

- 单一执行路径可避免简单接口与结构化接口出现行为漂移。
- `RunResult + error` 同时满足稳定机器可读诊断和 Go 习惯错误传播。
- 失败时返回部分结果，调用方才能观察失败发生前的模型或工具步骤。

备选方案是修改 `Run()` 返回类型，这会破坏 ChatBot 和现有调用方；另一方案是用回调收集事件，但当前需求是一次运行完成后获得完整数据，回调会增加生命周期和并发复杂度。

### Decision 2: 轨迹采用扁平有序 `RunStep`，不建立通用 span 树

`RunResult.Steps` 按实际发生顺序追加。`RunStep` 至少包含：

- `Index`、`RequestID`、`StepID`
- `Type`
- `StartedAt`、`Duration`
- `Success`、`ErrorCode`
- 模型步骤的调用模式
- 工具步骤的工具名、tool call key、cache hit、工具正文和诊断 metadata
- 最终回答步骤的可见回答

步骤类型使用稳定常量：`model_call`、`tool_call`、`cache_reuse`、`final_answer`。真实工具执行记录为 `tool_call`，runCache 命中记录为 `cache_reuse`，两者不同时出现。

理由：

- 当前 Runtime 是严格顺序循环，扁平序列足以还原执行过程。
- 显式区分真实执行和缓存复用，调用方无需组合多个布尔字段推断。
- 避免过早引入父子 span、并行步骤和通用 tracing 抽象。

### Decision 3: 统一轨迹序号和 step ID，并注入对应日志

每次 `RunWithTrace()` 创建一个运行级记录器，按事件发生顺序生成从 1 开始的 `Index` 和唯一 `StepID`。step ID 只要求在当前 request ID 内稳定唯一，不把具体字符串格式作为公共契约。

模型调用、工具执行或缓存复用、最终回答的日志增加对应 `step_id`；既有工具循环 `step` 和 `max_steps` 字段继续表示模型循环轮次，不改为轨迹全局序号。

理由：

- 保留现有日志字段含义，避免破坏排查习惯和测试。
- 独立 step ID 可把一轮工具循环中的模型事件与工具事件分别关联到轨迹。

备选方案是直接复用工具循环 step 作为唯一标识，但普通回答和同轮模型/工具事件会发生冲突。

### Decision 4: 在现有工具循环边界采集事件，不改变执行决策

`runToolLoop()` 接收运行级轨迹记录器或等价内部收集对象：

1. 每次 `client.Chat()` 完成后追加一个 `model_call` 步骤；失败时追加失败模型步骤并返回。
2. 识别到工具调用后，通过现有 runCache 包装执行。
3. cache miss 追加 `tool_call`，cache hit 追加 `cache_reuse`。
4. 模型返回普通可见文本时追加 `final_answer` 并结束。
5. 达到 `MaxSteps` 时设置稳定终止原因并返回已有轨迹。

工具执行计时只覆盖真实 `Executor.Execute()`；缓存复用计时覆盖查找和复制结果路径。模型步骤计时只覆盖 `client.Chat()`，总耗时覆盖整个 `RunWithTrace()`，包括上下文准备。

理由：

- 采集点与现有执行边界一致，可保持工具协议和副作用语义。
- 不通过解析日志反向构建轨迹，避免格式耦合。

### Decision 5: 使用稳定终止原因和错误代码，不复制错误字符串作为协议

`RunResult.TerminationReason` 使用稳定常量，至少覆盖：

- `completed`
- `context_error`
- `model_error`
- `tool_error`
- `max_steps_exceeded`
- `invalid_tool_call`

`RunStep.ErrorCode` 使用同样稳定的分类原则；原始 Go `error` 继续通过返回值提供，轨迹字段不得要求调用方匹配错误文本。工具返回的业务失败继续保留在工具正文和 metadata 中，并允许模型基于失败结果生成最终回答；只有中断 Runtime 的错误才决定失败终止原因。

理由：

- 调用方可建立稳定分支逻辑。
- 不把 provider、工具或内部实现的错误文本固化为 API。

### Decision 6: 工具正文、诊断 metadata 和隐藏推理保持明确边界

工具步骤分别保存工具结果正文、成功状态、稳定工具错误代码和复制后的 metadata。轨迹不得引用随后会被修改的 runCache map 或 metadata map。模型步骤不保存隐藏推理字段；最终回答只保存用户可见 `ChatResponse.Content`。

轨迹只存在于返回的 `RunResult`，不得追加到 Session recent messages、rolling summary 或 system prompt。

理由：

- 调用方可读取工具结果，又能独立处理 cache hit、tool call key 等诊断数据。
- 复制数据避免缓存命中步骤更新 metadata 时反向污染早先轨迹。
- 明确排除隐藏推理，保持模型安全边界。

## Risks / Trade-offs

- [Risk] 轨迹保存完整工具正文会增加单次返回对象大小并可能包含本地敏感数据。  
  → Mitigation: 轨迹仅由显式 `RunWithTrace()` 返回且不持久化；文档说明调用方负责后续存储和脱敏。

- [Risk] 为模型和工具增加计时会使测试受时间波动影响。  
  → Mitigation: 测试只断言耗时非负、步骤顺序和分类，不断言精确时长。

- [Risk] `Run()` 委托后错误路径可能意外改变 Session 写回行为。  
  → Mitigation: 保留现有 Agent 与 ChatBot 回归测试，并新增结构化入口和简单入口的等价性测试。

- [Risk] cache hit 返回的 `ToolResult.Metadata` 会在当前步骤补充诊断字段。  
  → Mitigation: 在写入轨迹和回填消息前复制结果与 metadata，禁止多个步骤共享可变 map。

- [Trade-off] 扁平步骤不能表达未来并行工具或嵌套工作流。  
  → Benefit: 与当前顺序 Runtime 匹配，避免为尚不存在的能力设计通用 tracing 模型。

## Migration Plan

1. 新增结构化类型、稳定常量和内部轨迹记录器，不改变现有公开调用路径。
2. 将当前 `Run()` 主体迁移到 `RunWithTrace()`，再让 `Run()` 委托。
3. 在上下文、模型、工具和终止边界接入步骤采集及关联日志。
4. 补充失败部分轨迹、缓存复制和 Session 不写入轨迹测试。
5. 更新 `docs/evolution.md` 并运行格式化、全量测试和 OpenSpec 校验。

回滚时可移除结构化入口并恢复 `Run()` 主体；没有持久化数据或配置需要迁移。

## Open Questions

无。首版固定为进程内同步返回的顺序轨迹；流式事件、轨迹导出和持久化在后续版本单独设计。
