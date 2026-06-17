## Context

v0.0.5 已经完成 `ToolCallKey`、参数标准化和一次请求作用域内的 runCache 包装入口，但当前 ChatBot 主流程仍然是“第一次模型响应请求工具 -> 执行一次工具 -> 第二次模型响应必须给最终回答”。如果第二次响应继续请求工具，系统会直接 fallback，无法完成“读取文件 -> 计算 -> 最终回答”这类顺序工具链。

本次变更把工具闭环从固定两次模型调用改成受 `MaxSteps` 约束的循环，同时保持现有工具接口、文本协议、上下文治理和会话历史边界不变。

实现过程中还补充了两个工程约束：ChatBot 初始化使用 `ChatBotOptions` 聚合依赖和可选项；OpenAI 兼容响应只接受 assistant `content` 作为用户可见回答，不从 `reasoning_content` 或 thinking 字段做语言相关的文本提取。

## Goals / Non-Goals

**Goals:**

- 支持同一次 `ChatBot.Send()` 内的顺序多步工具调用。
- 每一步最多解析并执行一个工具调用。
- 使用 `ToolLoopOptions{MaxSteps int}` 控制最大工具循环步数，首版默认值为 `3`。
- 让 v0.0.5 的 runCache 在完整工具循环中端到端生效。
- 保持无工具请求、单工具请求和工具失败路径的既有用户可见行为兼容。
- 为步数超限、多工具调用和重复工具调用提供明确错误或可观测行为。
- 通过日志明确区分 runCache 命中、未命中和写入。
- 保证最终用户可见回答进入 assistant `content`，避免依赖不可控语言 marker 从 reasoning 字段抽取答案。

**Non-Goals:**

- 不把 `MaxSteps` 接入配置文件。
- 不支持并行工具调用或一次响应中的多个工具调用。
- 不引入 provider 原生 tool calling。
- 不把完整工具轨迹保存进 `Session` 或 `history`。
- 不引入跨轮缓存、TTL、副作用工具策略或工具权限审批。
- 不新增 `file_write`、`shell_exec`、`ask_user` 等高风险工具。
- 不通过字符串匹配从 `reasoning_content` 中提取最终回答。

## Decisions

### Decision: 用 `runToolLoop()` 替代固定二次请求闭环

`ChatBot.Send()` 在加入用户消息并构建受控上下文后，创建本轮工作消息副本和 runCache，然后调用 `runToolLoop()`。循环内每一步调用 LLM，解析响应：

- 普通 assistant 文本：保存为最终回答并结束。
- 单个合法 `tool_call`：通过 `executeToolWithRunCache()` 执行或复用结果，追加工具反馈消息后继续。
- 多个工具调用：返回 `ErrMultipleToolCalls`，不执行任何工具。
- 达到 `MaxSteps`：返回 `ErrToolLoopExceeded`，不追加 assistant 消息。

备选方案是继续保留 `completeToolCall()` 并在内部递归调用，但递归会让步数控制、错误退出和测试断言更分散。显式循环更适合当前学习项目，也更容易观察每一步状态。

### Decision: `MaxSteps` 首版使用默认常量

新增 `ToolLoopOptions{MaxSteps int}`，ChatBot 初始化时可持有该选项；当未显式设置或小于等于 0 时使用默认值 `3`。

备选方案是立即增加 `chat.tools.max_steps` 配置。当前项目仍处于能力演进阶段，v0.0.6 的目标是先验证多步循环主路径；配置接入会扩大配置校验、示例文件和兼容性范围，因此暂缓。

### Decision: 使用 `ChatBotOptions` 作为唯一构造入口

ChatBot 只有一个 `NewChatBot(options ChatBotOptions)` 构造函数。`Client`、`Session` 和 `ContextConfig` 是必需依赖，`Executor` 和 `ToolLoopOptions` 是可选扩展点；当 `Executor` 为空时使用默认工具集合，当 `ToolLoopOptions.MaxSteps` 非法时使用默认值。

备选方案是保留 `NewChatBot`、`NewChatBotWithTools`、`NewChatBotWithToolsAndOptions` 多个构造函数，或使用一个包含较多位置参数的构造函数。多构造函数会扩散初始化路径，位置参数会降低调用点可读性；`ChatBotOptions` 更适合后续加入 logger、工具配置或 provider 适配选项。

### Decision: 工具反馈继续只存在于本轮工作消息

每次工具执行后，系统把“模型请求工具”的 assistant 语义和工具结果回填 user 消息追加到本轮 `messages` 副本，而不是写入 `Session`。最终只有用户原始输入和最终 assistant 回答进入会话历史。

备选方案是记录完整工具轨迹到历史。该方案更利于审计，但会污染用户可见历史并增加上下文预算压力；本阶段先保持 v0.0.5 的边界。

### Decision: runCache 生命周期覆盖完整 `Send()`

runCache 在 `ChatBot.Send()` 开始处理本次请求时创建，并传入整个 `runToolLoop()`。如果同一次请求中模型重复请求相同工具和相同标准化参数，执行包装层复用已有 `ToolResult`，不再次调用真实工具。

备选方案是每步创建独立缓存。那样无法验证 v0.0.5 的端到端价值，也不能避免模型重复请求相同工具造成的浪费。

### Decision: 在工具缓存入口记录命中状态

`executeToolWithRunCache()` 记录 `tool_cache_hit=true/false`、`tool_cache_store=true`、工具名、参数 hash 和当前缓存条目数。日志不输出完整参数，避免把文件路径、搜索词或用户上下文进一步扩散到日志。

备选方案是在每个具体工具内记录缓存状态，但具体工具并不知道 runCache 是否命中；把日志放在统一包装入口能覆盖所有工具。

### Decision: Prompt 从“工具阶段结束”改为“结果足够则最终回答”

工具说明不再表达“每轮最多调用一个工具”。新的约束是：每次回复最多请求一个工具；已有工具结果足以回答时必须最终回答；不要重复请求相同工具和相同参数；只有结果不足时才继续请求另一个工具。

备选方案是完全依赖系统代码处理重复请求，不调整 prompt。但当前仍是文本协议模拟 Tool Calling，Prompt 约束可以减少无效循环和重复调用概率。

### Decision: 要求最终回答进入 assistant `content`

工具说明前追加通用输出约束：即使模型内部进行 thinking/reasoning，最终给用户看的回答也必须输出在 assistant message 的 `content` 中，不能只输出在 `reasoning_content` 或 thinking 字段中。OpenAI 兼容客户端只把 `content` 作为有效回答；如果 `content` 为空但 `reasoning_content` 有值，返回明确错误，提示响应只包含 reasoning 内容。

备选方案是从 `reasoning_content` 中通过“最终回答”等文字 marker 提取答案。该方案依赖自然语言和模型输出习惯，语言不可控且容易误提取推理过程，因此不采用。

## Risks / Trade-offs

- 模型可能反复请求不同但无意义的工具 → 使用 `MaxSteps` 硬限制，并在超限时返回 `ErrToolLoopExceeded`。
- 文本协议下模型仍可能输出多个工具调用或格式不稳定 → 保留现有解析兼容能力，并继续对多工具调用返回明确错误。
- 多步循环增加 LLM 调用次数和成本 → 默认 `MaxSteps=3`，无工具请求路径仍只调用一次模型。
- 工具反馈消息可能导致上下文变大 → 工具结果仍只保留在本轮工作消息中，不写入长期会话历史。
- runCache 复用可能掩盖副作用工具的重复执行需求 → 当前工具集不包含写入或命令类副作用工具，后续引入高风险工具前再设计缓存策略。
- 某些兼容模型可能只返回 `reasoning_content` 而不填充 `content` → 通过 prompt 明确要求最终回答进入 `content`，如果供应商仍不遵守，则暴露清晰错误并在后续评估 provider 参数或模型模式。

## Migration Plan

1. 新增工具循环选项、默认值和错误定义。
2. 将当前工具闭环重构为 `runToolLoop()`，让单工具路径继续通过现有测试。
3. 更新工具反馈消息和工具说明 Prompt。
4. 补充多步循环、重复调用缓存命中、步数超限和多工具拒绝测试。
5. 收敛 ChatBot 构造函数为 `ChatBotOptions`。
6. 增加 runCache 命中日志和 assistant `content` 输出约束。
7. 执行 `go fmt ./...` 和 `go test ./...`。

回滚方式是恢复固定二次请求闭环，并保留 v0.0.5 的 runCache 包装入口；本变更不涉及数据迁移。

## Open Questions

- 是否需要在后续版本把 `MaxSteps` 暴露到 `configs/config.yaml`。
- 是否需要为工具结果日志继续增加 step index、耗时和脱敏策略。
- 是否继续沿用文本协议，还是在后续接入 provider 原生 tool calling。
- 是否需要根据具体 provider 增加请求参数，要求保留 reasoning 的同时稳定返回 assistant `content`。
