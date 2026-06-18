## Context

当前 `internal/app.ChatBot` 直接持有 LLM 客户端、Session、上下文 Builder、Summarizer、工具 Executor 和 Tool Loop options，并在 `Send()` 内完成请求标识、上下文裁剪、摘要更新、工具循环、runCache 与会话写回。该结构已经能够支撑 v0.0.7，但 Runtime 策略与 CLI/app 适配职责集中在同一类型中，后续加入 ToolCache、Memory 或 Workflow 会继续放大耦合。

本变更是 v0.0.8 的架构边界调整。现有 `chat-session-runtime`、`context-window-control` 和 `tool-calling` 规范仍是行为基线；迁移不得改变 CLI、配置、工具文本协议、请求内缓存生命周期、日志字段或会话历史语义。

## Goals / Non-Goals

**Goals:**

- 建立 `internal/agent.Agent` 作为用户输入到最终回答的统一 Runtime 编排入口。
- 让 `Agent.Run(ctx, input)` 独立具备当前 `ChatBot.Send()` 的普通聊天、上下文治理、摘要更新和多步工具循环能力。
- 将 Runtime 状态和策略从 `internal/app` 迁移到 `internal/agent`，使 app 层只负责兼容适配和对象组装。
- 保留 `app.NewChatBot(ChatBotOptions)` 与 `ChatBot.Send()`，避免 CLI 和现有调用方迁移。
- 通过 Agent 层测试承接 Runtime 行为，通过 app 层测试验证委托兼容性。

**Non-Goals:**

- 不引入跨轮 ToolCache、TTL、缓存持久化或副作用工具缓存策略。
- 不修改 `Tool`、`llm.Client`、`session.Session` 或配置结构的公开契约。
- 不新增工具，不接入 provider 原生 Tool Calling。
- 不实现 Planner、Workflow、Memory、RAG 或 Multi-Agent。
- 不改变 CLI 命令、输出文本、历史展示和 clear 语义。
- 不为未来能力预先设计通用插件框架或多层抽象。

## Decisions

### Decision: `Agent` 成为 Runtime 状态与执行策略的唯一持有者

新增 `internal/agent` 包，建议包含：

```text
internal/agent/
├── agent.go
├── options.go
├── context.go
├── tool_loop.go
├── agent_test.go
└── tool_loop_test.go
```

`Agent` 持有 `llm.Client`、`session.Session`、`contextwindow.Builder`、`contextwindow.Summarizer`、`config.ContextConfig`、`tools.Executor` 和 `ToolLoopOptions`。`Run()` 负责追加用户消息、准备上下文、创建本轮 runCache、运行工具循环并仅在成功时追加最终 assistant 消息。

备选方案是只把 `runToolLoop()` 抽成独立服务，但上下文准备和会话写回仍会留在 ChatBot，无法形成完整 Runtime 边界。另一方案是同时拆出 ContextManager、ToolRunner、ConversationRuntime 等多个接口；当前逻辑只有一套实现，过早抽象会增加学习成本，因此不采用。

### Decision: `ChatBot` 使用组合与委托保持兼容

`internal/app.ChatBot` 只持有一个 `*agent.Agent`，`Send()` 直接委托 `Agent.Run()`。保留 `ChatBotOptions` 作为兼容构造参数，并在 `NewChatBot()` 中转换为 `agent.Options`。

`app.ToolLoopOptions`、`app.DefaultMaxToolLoopSteps` 和 `app.ErrToolLoopExceeded` 如已被测试或调用方引用，应通过类型别名或变量别名映射到 `internal/agent`，避免迁移造成不必要的内部 API 破坏。新的 Runtime 测试直接使用 `agent` 包定义。

备选方案是修改 CLI 直接使用 `agent.Agent` 并删除 ChatBot。该方案边界更直接，但会破坏本版本明确要求的 `ChatBot.Send()` 兼容性，也扩大迁移范围。

### Decision: 工具说明由 `Agent` 初始化一次并固化到 Session

工具说明生成、排序和 prompt 拼接逻辑迁移到 `internal/agent`。`agent.New(options)` 在 Executor 默认化后生成工具说明，并通过 `Session.AppendSystemPromptSection()` 固化一次。`Run()` 不重复生成或追加说明，`Session.Reset()` 后说明继续保留。

备选方案是在 app 层提前修改 Session 后再构造 Agent，但这会让 Agent 无法独立满足“带工具运行”的能力，也会把 Runtime 初始化策略继续泄漏到适配层。

### Decision: 上下文治理整体迁移，不改变 Session 所有权

`prepareRequest()`、摘要更新和摘要再压缩逻辑迁移到 `internal/agent`，继续直接读写注入的 `session.Session`。CLI 仍持有同一个 Session 实例处理 `clear` 和 `history`，Agent 与 CLI 观察到的是同一会话状态。

备选方案是让 Agent 隐藏 Session 并暴露 `Reset()`、`History()`，但这会同时改变 CLI 组装和会话访问契约，超出本次最小迁移目标。

### Decision: 工具循环及请求内 runCache 原样迁移

`ToolLoopOptions`、`ErrToolLoopExceeded`、工具反馈构建、诊断 metadata、脱敏日志和 `executeToolWithRunCache()` 迁移到 `internal/agent`。每次 `Run()` 新建一个 `map[tools.ToolCallKey]tools.ToolResult`，循环结束后自然释放；不提升为 Agent 字段。

日志组件名可从 `[chatbot]` 调整为 `[agent]`，但必须继续携带现有 `request_id`、step、max_steps、tool_call_key、cache_hit、success 和 error_code 等诊断字段，且不能扩大敏感内容输出。

备选方案是保留工具循环在 app 层并由 Agent 回调，这会形成反向依赖或职责穿透，违背 Runtime 统一编排目标。

### Decision: 请求标识由 `Agent.Run()` 建立

`Agent.Run()` 在入口调用 `requestctx.WithNewRequestID(ctx)`，使直接调用 Agent 与通过 ChatBot 调用都获得完整日志关联。下游 Summarizer、LLM 和工具循环继续复用该 context。

不由 ChatBot 创建 request_id，因为直接使用 Agent 时将失去同等可观测性；也不要求调用方预先注入 request_id，以保持当前调用简洁性。

### Decision: 测试按职责迁移并保留少量适配器契约测试

普通聊天、摘要治理、工具调用、多步循环、runCache、步数超限、日志脱敏等测试迁移到 `internal/agent`。`internal/app` 保留构造映射、`Send()` 委托、成功返回和失败不追加 assistant 等兼容测试。测试使用现有 fake LLM、fake Tool 和内存 Session，不引入外部模型依赖。

直接复制全部 app 测试会造成双份高成本维护，因此 Runtime 行为只在 Agent 层详细覆盖，app 层只验证适配边界。

## Risks / Trade-offs

- [迁移遗漏导致行为漂移] → 以现有 app 测试为迁移清单，先让 Agent 层承接全部关键场景，再收敛 app 测试。
- [工具说明被重复追加] → 只允许 `agent.New()` 执行一次初始化追加，`Run()` 禁止修改 system prompt，并增加构造与 reset 测试。
- [兼容别名形成短期重复 API] → 仅为 v0.0.8 保留 app 层别名，新增代码统一依赖 `internal/agent` 定义。
- [Session 同时被 CLI 和 Agent 持有] → 保持当前单线程 CLI 使用模型，不在本版本声明并发安全；后续并发 Runtime 单独设计。
- [包迁移后日志标签变化影响排查] → 保留结构化字段和 request_id 语义，测试关注稳定字段而非源码文件名。
- [为了未来扩展过度抽象] → 只引入一个具体 `Agent` 与 options，不新增尚无第二实现的 Runtime 接口。

## Migration Plan

1. 新增 `internal/agent` options、错误和 Agent 构造结构，迁移工具说明生成逻辑。
2. 迁移上下文准备、摘要更新和摘要再压缩逻辑，并建立普通聊天与上下文测试。
3. 迁移工具循环、runCache、反馈构建和诊断日志，并迁移对应测试。
4. 将 `internal/app.ChatBot` 改为持有 Agent 的兼容适配器，保留原构造参数与 `Send()`。
5. 调整 CLI 对象组装但不改变配置、命令和交互；运行全部测试验证兼容性。
6. 更新 `docs/evolution.md` 的 v0.0.8 记录。

回滚时可恢复原 `internal/app` Runtime 实现并删除 `internal/agent`；本变更不涉及配置或持久化数据迁移。

## Open Questions

- 后续版本是否直接让 CLI 依赖 `Agent`，并逐步废弃 `ChatBot` 兼容层。
- 在引入跨轮 ToolCache 或 Memory 前，是否需要为 Agent 增加显式的 Run 生命周期对象。
- 当出现第二种上下文策略或工具循环实现时，是否再提炼窄接口，而不是现在预先抽象。
