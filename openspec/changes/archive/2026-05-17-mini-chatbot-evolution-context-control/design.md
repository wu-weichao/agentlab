## Context

当前仓库已经具备最小可运行的 CLI ChatBot 闭环，主链路为 `cmd/chat -> internal/app -> internal/session -> internal/llm`。现有实现默认把会话历史作为持续增长的消息列表直接传给模型，缺少对 system prompt、近期轮次、旧消息裁剪和摘要压缩的统一治理。

明确这次演进的边界：先解决单会话内的短期上下文控制，不引入长期记忆、RAG、向量检索或多会话复用。同时仓库遵循“先简单、后复杂”的原则，因此设计需要优先保证实现透明、可观察、可测试，而不是一次性做成完整 memory runtime。

## Goals / Non-Goals

**Goals:**
- 在每次模型调用前，按预算构建受控上下文，而不是无界传递历史消息。
- 保留渲染后的 system prompt、当前用户输入、最近若干轮原始对话，并对更早历史做整轮裁剪。
- 仅维护一份结构化滚动摘要，用于承载被移出近期窗口的历史事实、约束和决策。
- 为 `clear`、`history`、配置校验和日志增加与上下文控制一致的行为。
- 让实现能够通过单元测试验证预算命中、裁剪、摘要更新和失败回退。

**Non-Goals:**
- 不实现跨会话持久化记忆、用户画像、检索增强或长期事实库。
- 不做精确 tokenizer 计数，首版不引入 provider 级 token 适配。
- 不支持多份摘要、摘要树、人工编辑摘要或重要性排序裁剪。
- 不把上下文治理逻辑塞进 `LLMClient`，也不把 `Session` 演化成完整策略执行器。

## Decisions

### 1. 新增独立 `internal/contextwindow` 模块，封装预算构建和摘要更新

建议新增如下模块：

```text
internal/contextwindow/
  budget.go
  builder.go
  summarizer.go
  builder_test.go
  summarizer_test.go
```

职责划分：
- `internal/session` 只负责保存会话状态：渲染后的 system prompt、滚动摘要、近期消息。
- `internal/contextwindow/builder.go` 负责根据预算决定发送哪些消息、裁剪哪些完整轮次。
- `internal/contextwindow/summarizer.go` 负责用旧摘要和被移出的消息生成新摘要。
- `internal/app/chatbot.go` 负责协调“追加用户输入 -> 预算构建 -> 必要时更新摘要 -> 调用模型 -> 追加 assistant 回复”。

原因：
- 这是一次跨 `config/session/app/llm/cli` 的横切变更，单独模块最利于后续演进和测试。
- 将预算策略从会话存储中拆开，可以保持 `Session` 的职责清晰。

备选方案：
- 把裁剪和摘要逻辑直接放进 `internal/session/session.go`。
  缺点是状态管理和策略执行耦合过深，后续测试和替换都会更困难，因此不采用。

### 2. 首版预算单位使用字符数估算，而不是精确 token 统计，并完全移除 `max_history_messages`

新增配置：

```yaml
chat:
  context:
    max_chars: 12000
    keep_recent_turns: 6
    summary_max_chars: 2400
    enable_rolling_summary: true
```

预算策略：
- 用粗粒度字符数估算总上下文长度。
- `summary_max_chars` 必须小于 `max_chars`，建议限制在总预算的一定比例内。
- 不再保留 `chat.max_history_messages`，上下文控制唯一配置入口为 `chat.context.*`。

原因：
- 字符数估算足够支撑当前学习型项目的最小闭环。
- 实现不依赖外部 tokenizer，便于调试和单元测试。
- 删除旧字段后，配置语义更单一，避免“消息条数限制”和“上下文预算限制”并存带来的冲突。

备选方案：
- 为不同 provider 接入精确 token 计数。
  这会把复杂度提前引入 provider 层，不符合本次“先简单、后复杂”的边界，因此不采用。

### 3. 会话内部引入 `summary` 语义消息，但对外发送前统一转换为 `system` 风格消息

会话结构调整为：

```text
Session
  - systemPrompt
  - rollingSummary
  - recentMessages
```

运行时语义：
- `summary` 只在 session 和 context builder 内部存在，表示“已离开原文窗口的旧历史摘要”。
- 真正调用 LLM 前，context builder 将其渲染为一条固定前缀的 `system` 风格消息。
- provider 客户端仍只接收模型原生支持的消息角色，避免修改底层接口契约。

原因：
- 内部语义清晰，便于在 `history` 中区分摘要和原始消息。
- 对现有 `LLMClient` 侵入最小，不要求 provider 原生支持 `summary` 角色。

备选方案：
- 直接让 `LLMClient` 支持新的 `summary` 角色。
  这会把上层上下文策略泄漏到 provider 协议层，因此不采用。

### 4. 裁剪单位按完整轮次进行，且只维护单份滚动摘要

上下文构建顺序固定为：

```text
System Prompt
-> Rolling Summary (if any)
-> Recent User/Assistant Messages
-> Current User Input
```

当预算超限时：
- 永远保留 system prompt。
- 永远保留当前用户输入。
- 优先保留最近 `N` 轮完整的 `user + assistant` 原始对话，但这不是绝对硬约束。
- 从最老的完整轮次开始移出，不拆半轮；如果连“最近 `N` 轮”都放不下，则允许继续缩小近期保留窗口。
- 真正的硬底线是：system prompt、当前用户输入以及至少 1 轮最近完整对话。
- 若启用滚动摘要，则以 `oldSummary + evictedMessages` 生成 `newSummary`。
- 若插入新摘要后仍超预算，则继续压缩已有摘要并重试一次上下文构建。
- 若摘要再压缩后仍无法满足最小预算，则返回明确错误，本轮不写入 assistant 消息。

原因：
- 保留最近原文比过早摘要化更有利于连续对话质量。
- 单份摘要足以支撑 v0.0.3，不会过早引入多层记忆结构。

备选方案：
- 直接对全部历史做整体摘要，或维护多段摘要池。
  前者会让最近语义损失过大，后者实现过重，因此都不采用。

### 5. 摘要必须结构化且可压缩，生成时机仅在旧消息被驱逐时触发

摘要模板固定为类似以下结构：

```text
[用户目标]
- ...

[已确认约束]
- ...

[已做决策]
- ...

[关键事实]
- ...

[未解决问题]
- ...
```

生成规则：
- 正常短对话不生成摘要。
- 只有当旧原始消息因预算超限被移出近期窗口时，才触发摘要更新。
- 摘要由现有 `LLMClient` 通过固定 system instruction 生成，而不是手写规则摘录。
- 摘要内容只保留事实、约束、决策和待解决问题，不复述寒暄或冗余措辞。
- 若摘要本身超出 `summary_max_chars`，则先做一次 summary-of-summary 压缩；如仍超限，再做最终截断兜底。

原因：
- 结构化格式更稳定，更利于多次滚动压缩。
- 使用 LLM 真正做归纳，比规则拼接更符合“短期记忆折叠”的目标。
- 触发时机明确，日志和测试都更容易建立。

备选方案：
- 使用规则法摘要，或每轮都生成摘要。
  前者只能做摘录和截断，无法稳定提炼高层事实；后者则会引入额外开销，因此不采用。

### 6. 日志通过 request_id 串联单次对话链路

运行时为每次 `Send()` 生成唯一 `request_id`，并沿 `chatbot -> summary -> llm/openai` 透传。

日志要求：
- ChatBot 记录预算检查、裁剪、摘要更新、摘要再压缩重试和最终请求结果。
- Summarizer 记录首轮摘要、二次压缩和最终兜底截断。
- LLM 客户端记录请求、响应和错误，并带上同一个 `request_id`。

原因：
- 长对话触发裁剪和摘要后，请求链路不再是单次 LLM 调用。
- 没有 request_id 时，很难把一次用户输入引发的多次内部调用串起来排查。

## Risks / Trade-offs

- [Risk] 字符数预算与真实 token 消耗不完全一致。 -> Mitigation：首版采用保守阈值，并记录日志指标，为后续 token 化演进提供依据。
- [Risk] 摘要生成与二次压缩会引入额外模型调用，增加延迟和失败场景。 -> Mitigation：仅在旧消息被驱逐或已有摘要仍超预算时触发，并保证失败时不写入错误的 assistant 历史。
- [Risk] 新增 `summary` 语义后，`history` 和测试基线需要整体调整。 -> Mitigation：在 session/CLI 层显式区分摘要消息，并补齐对应测试。
- [Risk] 移除 `chat.max_history_messages` 会让旧配置文件直接失效。 -> Mitigation：在 proposal、spec、示例配置和 README 中明确这是 breaking change，并在启动时对旧字段给出明确报错。
- [Risk] `keep_recent_turns` 如果被当成硬约束，在超长输出场景下会导致 summary 永远无法触发。 -> Mitigation：实现中将其定义为优先保留窗口，而不是绝对硬保留，仅保留最近 1 轮作为最小底线。

## Migration Plan

1. 在 `internal/config` 中新增 `chat.context.*` 配置结构和校验逻辑，同时移除 `chat.max_history_messages` 并对旧字段报错，更新示例配置与文档。
2. 在 `internal/session` 中调整会话状态，支持渲染后的 system prompt、滚动摘要和近期消息。
3. 新增 `internal/contextwindow` 模块，实现预算估算、候选上下文构建和摘要更新逻辑。
4. 在 `internal/app/chatbot.go` 中接入上下文治理流程，并保证失败时不会污染 assistant 历史。
5. 调整 `history` 与 `clear` 命令行为，补充日志输出。
6. 运行 `go test ./...` 验证相关行为；如需回退，整体回退该变更即可，当前不涉及持久化数据迁移。

## Open Questions

- 滚动摘要生成是否直接复用现有主模型客户端，还是后续引入独立 summarizer 配置；本次设计默认先复用同一客户端。
- `history` 命令展示摘要时，是否需要固定标记为 `[summary]` 并显示摘要分段；本次设计默认需要显式标记。
