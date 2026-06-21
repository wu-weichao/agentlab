## Context

当前 `internal/agent` 已经负责顺序多步工具循环、`MaxSteps`、请求内 runCache 和工具诊断，但模型与 Runtime 之间仍使用文本模拟协议：Agent 把工具说明和 JSON 输出格式写入 system prompt，OpenAI 兼容客户端只发送文本消息，模型再把 `tool_call` JSON 写进 assistant `content`。

这种方式已经验证了工具闭环，但存在三个结构问题：

- `llm.Message` 和 `llm.ChatResponse` 只能表达文本，无法保留 provider 的 tool call ID 和 tool result 角色。
- JSON 格式正确性依赖 Prompt，不同模型可能输出代码块、包装对象、多个调用或混合解释文本。
- 工具结果使用普通 user 消息回填，provider 无法利用原生 tool message 的关联语义。

本变更需要同时调整 `internal/llm`、`internal/agent`、配置和测试，但必须保持工具实现、顺序循环、缓存和用户可见会话行为稳定。

## Goals / Non-Goals

**Goals:**

- 让统一 LLM 协议能够表达工具定义、结构化工具调用和工具结果消息。
- OpenAI 兼容客户端发送原生 `tools`，并把原生 `tool_calls` 转换为仓库统一类型。
- Agent 在 native 模式下使用结构化调用和原生工具反馈完成现有顺序循环。
- 提供 `text_compat` 模式，保留当前 JSON 文本协议作为兼容和回退路径。
- 保持单步单工具、`MaxSteps`、runCache 和最终回答写回语义，并为学习调试保留完整执行内容日志。

**Non-Goals:**

- 不支持一次响应中的多个工具调用或并行工具执行。
- 不新增工具，不修改现有 `Tool` 执行接口。
- 不引入跨轮缓存、TTL、Planner、Workflow、Memory 或结构化 RunResult。
- 不自动探测 provider 是否支持原生 Tool Calling。
- 不在 native 请求失败时静默切换到文本兼容模式。

## Decisions

### Decision 1: 使用统一 `ChatRequest` 承载消息、工具定义和调用模式

将 `llm.Client` 从只接收消息切片调整为接收统一请求对象。请求至少包含：

- `Messages []llm.Message`
- `Tools []tools.ToolSpec`
- `ToolCallingMode`

`ChatResponse` 增加结构化 `ToolCalls`。`llm.Message` 增加 assistant tool calls、tool call ID 和 tool name 等可选字段。

理由：

- 工具定义属于单次模型请求输入，不应隐式保存在 provider client 中。
- Agent、Summarizer 和后续其他 LLM 调用可以明确决定是否允许工具。
- 避免继续扩展 `Chat(ctx, messages, tools, options...)` 形式的位置参数。

备选方案：

- 在 OpenAI client 构造时固定工具定义：会让摘要模型调用也携带工具，并难以支持不同 Agent 使用不同 Executor。
- 新增第二个 `ChatWithTools` 接口：会形成两套调用路径，增加 mock 和 provider 实现复杂度。

### Decision 2: 仓库统一工具调用继续使用 `tools.ToolCall`

为 `tools.ToolCall` 增加可选 `ID`，native 模式保存 provider tool call ID，text compatibility 模式允许 ID 为空。`ToolCallKey` 仍只由工具名和标准化参数构造，不包含 ID。

理由：

- Agent 和 ToolExecutor 已经围绕 `tools.ToolCall` 工作，不需要再引入一套等价调用模型。
- provider 私有的 `function.name`、参数 JSON 字符串等结构只在 OpenAI adapter 中存在。
- tool call ID 用于消息关联，不代表工具调用语义，不能影响 runCache 命中。

备选方案：

- 在 `llm` 和 `tools` 中各维护一套 ToolCall：边界更纯粹，但需要重复转换和校验，当前项目收益不足。

### Decision 3: native 模式默认使用 provider 原生消息语义

native 模式请求流程为：

```text
messages + tools
  -> assistant(tool_calls)
  -> ToolExecutor
  -> tool(tool_call_id, name, result)
  -> next chat request
  -> assistant(content)
```

工具轨迹仍只保存在本轮工作消息中，不写入 `Session`。Session 继续只保存 system、summary、user 和最终 assistant 文本。

理由：

- 保留现有用户可见历史边界。
- tool call ID 能让 provider 正确关联调用和结果。
- 不把内部工具协议长期塞入上下文治理和滚动摘要。

备选方案：

- 把完整原生工具轨迹写入 Session：有利于跨轮追踪，但会改变摘要、历史输出和上下文预算语义，超出本版本范围。

### Decision 4: native 与 text compatibility 使用显式配置

新增工具调用模式配置，合法值为：

- `native`：默认值，发送 provider 原生 `tools` 并解析原生 `tool_calls`。
- `text_compat`：保留现有 system prompt JSON 说明、文本解析和 user 消息反馈路径。

配置非法时启动失败。native 模式遇到 provider 不支持或响应非法时直接返回明确错误，不自动降级。

理由：

- 自动降级会重复请求模型，可能产生额外费用和不一致的副作用判断。
- 显式模式便于测试、诊断和兼容不完整的 OpenAI-compatible provider。
- 默认 native 符合本版本从模拟协议迁移到原生协议的目标。

备选方案：

- 始终同时发送原生 tools 和文本 JSON 指令：模型可能混用两种协议，增加歧义。
- 自动探测 provider：OpenAI-compatible 接口没有稳定统一的能力发现标准。

### Decision 5: native 模式只接受零个或一个结构化工具调用

OpenAI adapter 可以完整解析 provider 返回的 tool call 数组，但 Agent 收到多个调用时继续返回现有 `ErrMultipleToolCalls`，并且不执行任何工具。

native 工具参数必须是合法 JSON object；空字符串按空对象处理，数组、标量或非法 JSON 返回参数协议错误。

理由：

- 保持当前顺序单工具步骤边界和失败原子性。
- 避免借协议迁移同时引入并行或批量工具语义。

### Decision 6: 工具说明按模式分工

- native 模式：工具名称、描述和参数通过请求 `tools` 定义传递；system prompt 只保留最终回答、顺序执行、禁止重复调用等行为约束，不再要求输出 JSON。
- text compatibility 模式：继续生成完整 JSON 调用格式和工具参数说明。

工具参数 schema 由 `ToolSpec.Parameters` 转换为最小 JSON Schema object，必填参数进入 `required`。

理由：

- 避免 native 模式同时存在两套调用格式。
- 保留通用行为约束，防止模型在已有工具结果足够时继续调用。

### Decision 7: 为学习调试记录完整请求、响应和工具内容

每次模型请求和响应至少记录：

- `tool_calling_mode`
- `tools_count`
- `tool_calls_count`
- `tool_calls_parse_status`
- 实际发送的完整 `request_body`
- provider 返回的完整 `response_body`

Agent 的工具日志记录完整 `tool_call_json` 和完整 `tool_result_json`，用于观察参数解析、执行结果和下一次请求回填过程。日志不得输出 Authorization header 或 API Key。

理由：

- 当前仓库是本地学习项目，观察实际协议内容比生产级日志最小化更重要。
- 完整请求、响应和工具结果可以直接验证原生 tool call ID、arguments、tool result 和多步消息是否正确传递。

## Risks / Trade-offs

- [Risk] 部分 OpenAI-compatible provider 不支持原生 `tools`。  
  → Mitigation: 提供显式 `text_compat` 配置，启动和运行日志标明当前模式。

- [Risk] 修改 `llm.Client` 会影响 Agent、Summarizer 和全部测试 mock。  
  → Mitigation: 一次性迁移到 `ChatRequest`，并为普通无工具请求补充回归测试。

- [Risk] provider 返回缺失 ID、非法参数 JSON 或多个工具调用。  
  → Mitigation: adapter 做结构校验，Agent 在执行任何工具前完成调用数量校验。

- [Risk] 完整日志可能包含用户输入、文件正文、搜索结果或工具参数。  
  → Mitigation: README 明确日志仅适用于本地学习环境，不应直接用于生产；Authorization header 和 API Key 始终禁止记录。

- [Trade-off] 不自动降级会要求用户主动选择兼容模式。  
  → Benefit: 请求次数、行为和错误更可预测，不会隐藏 provider 能力问题。

## Migration Plan

1. 先扩展统一 LLM 类型和 Client 接口，迁移普通聊天与 Summarizer 调用。
2. 为 OpenAI adapter 增加 native 请求/响应 DTO 和独立单元测试。
3. 调整 Agent 工具循环，按模式构建工具说明、识别调用和回填结果。
4. 增加配置默认值与校验；未配置时使用 `native`。
5. 更新示例配置、README 和 `docs/evolution.md`。
6. 运行全部测试，并通过 text compatibility 回归测试确认旧协议仍可用。

回滚时可把配置切换为 `text_compat`；代码级回滚不需要迁移会话或持久化数据，因为本版本不改变 Session 存储格式。

## Open Questions

- OpenAI-compatible provider 对 tool message 的 `name` 字段支持程度存在差异；实现阶段应以标准 `tool_call_id` 为必要字段，`name` 仅在兼容需要时发送。
- 当前 `Parameter.Type` 是字符串，首版只映射已有工具使用的基础类型；后续复杂嵌套 schema 是否升级为原生 JSON Schema 不属于本版本。
