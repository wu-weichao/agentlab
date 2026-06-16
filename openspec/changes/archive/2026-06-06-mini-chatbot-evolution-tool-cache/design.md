## Context

当前项目已经具备最小 Tool Calling 闭环：模型按内部 JSON 文本协议输出 `tool_call`，`ChatBot` 解析后通过 `tools.Executor` 执行工具，再把结构化 `ToolResult` 回填给模型生成最终回答。

当前边界是“单轮最多一次工具调用”。如果模型在工具结果回填后仍请求工具，系统会使用 fallback 生成最终回答，不继续递归执行。这个策略能保证闭环安全，但还没有 Runtime 级工具调用标识和 runCache 基础设施，无法为后续多步 Tool Loop 提供稳定基础。

本变更只引入同一次请求作用域内可使用的 run-level cache 基础设施。当前 `ChatBot.Send()` 仍然最多执行一次工具，但工具执行路径已经统一经过 runCache 包装；完整多步重复调用治理会在后续多步 Tool Loop 中发挥更明显作用。

实现过程中还同步收紧了工具结果回填和诊断日志：工具执行完成后会记录完整单行 `ToolResult` JSON，OpenAI 兼容客户端发送请求时会记录完整 request body，便于排查模型是否在第二次请求中收到工具结果。

## Goals / Non-Goals

**Goals:**

- 为每次工具调用构造稳定 `ToolCallKey`。
- 对 `ToolCall.Arguments` 做确定性标准化，避免 JSON 字段顺序影响 key。
- 新增一次请求作用域的 `runCache` 数据结构。
- 新增工具执行包装逻辑，在给定 `runCache` 时可复用相同 `ToolCallKey` 的结果。
- 为后续 `MaxSteps` 多步 Tool Loop 复用该基础设施。
- 让工具结果二次回填更明确：以 `user` 消息携带工具结果和 `FINAL_ANSWER` 阶段约束。
- 让关键诊断日志可复现：记录完整 `ToolResult` JSON 和完整 LLM 请求 body。

**Non-Goals:**

- 不实现跨轮工具缓存、TTL 或 `ToolMetadata`。
- 不新增多步 Tool Loop。
- 不接入 provider 原生 tool calling。
- 不改变 `Tool` 接口。
- 不把工具轨迹写入用户可见 `history`。
- 不新增高风险工具或权限审批流程。

## Decisions

### Decision 1: 在 `internal/tools` 中定义 `ToolCallKey`

`ToolCallKey` 属于工具协议基础设施，放在 `internal/tools/key.go`，而不是放在 `internal/app`。

理由：

- `ToolCall` 已定义在 `internal/tools`。
- key 构建只依赖工具名和参数，不依赖 ChatBot 状态。
- 后续 Agent Runtime 或 ToolCache 可以复用同一 key 构建逻辑。

备选方案：

- 放在 `internal/app`：会把去重基础设施绑定到 ChatBot，不利于后续迁移到 `internal/agent`。
- 放在新包 `internal/toolcache`：当前还没有跨轮缓存，过早引入包边界会增加抽象成本。

### Decision 2: 参数标准化只做结构化 JSON 级别归一化

`NormalizeArguments` 只保证 map key 顺序稳定、嵌套结构稳定和数字 JSON 表达稳定，不做业务语义归一化。

理由：

- 同轮去重必须确定、可测试、可解释。
- `"New York"` 和 `"new york"` 是否等价属于业务语义，不应由通用 key 构建层判断。
- 文件路径、搜索 query 等参数如果过度归一化，可能导致错误复用。

备选方案：

- 对字符串做大小写、空格、路径等语义归一化：短期看能提高命中率，但会引入隐式行为和安全风险。

### Decision 3: runCache 生命周期限定为一次请求

runCache 在一次用户请求处理开始时创建，结束后丢弃。当前版本可以在工具执行包装层验证该生命周期；完整多步循环由后续版本接入。

理由：

- 本版本只提供同轮结果复用基础设施。
- 跨轮缓存需要 TTL、工具时效性、副作用和刷新语义，属于后续版本。
- 生命周期短，易于测试和回滚。

备选方案：

- 放到 `Session` 或 `Executor` 中跨轮复用：会提前引入缓存失效和工具元信息问题。

### Decision 4: 不改 `Tool` 接口

所有现有工具继续实现 `Name`、`Description`、`Parameters`、`Execute`。

理由：

- 本变更不需要工具声明缓存策略。
- 保持四个默认工具和测试稳定。
- 避免把 `ToolMetadata` 和 TTL 提前引入。

备选方案：

- 直接改为 `Metadata()` 接口：更接近 `v0.2.3`，但会扩大本次变更范围。

### Decision 5: 工具结果回填使用 `user` 消息推进最终回答阶段

工具执行后，二次模型请求保留一条 `assistant` 消息表示“模型曾请求工具”，随后追加一条 `user` 消息承载工具执行结果和 `FINAL_ANSWER` 阶段约束。

理由：

- 当前仍使用 OpenAI 兼容的基础 chat roles，没有 provider 原生 tool role。
- 部分兼容模型会弱化后置 `system` 消息，导致第二次响应仍输出 `tool_call`。
- 使用 `user` 消息表达“工具结果已返回，请基于结果回答原问题”更符合普通聊天补全的对话推进语义。
- 工具结果仍不写入 `Session`，不会污染用户可见历史。

备选方案：

- 继续使用后置 `system` 消息：实现简单，但已观察到兼容模型可能忽略该阶段约束。
- 引入 provider 原生 tool role：更规范，但超出本次文本协议和基础设施变更范围。

### Decision 6: 诊断日志输出完整结构化内容

工具执行完成日志输出 `tool_result_json=<json>`，其中 JSON 由 `json.Marshal(ToolResult)` 生成，保持单行完整字符串。OpenAI 兼容客户端发送请求日志输出实际发送的 `request_body=<json>`。

理由：

- 排查工具闭环问题时，需要确认第二次请求是否真的包含工具结果和 `FINAL_ANSWER` 约束。
- 单行 JSON 便于在 `logs/chat.log` 中检索和复制。
- request body 不包含 Authorization header，可直接反映模型实际收到的消息数组。

备选方案：

- 只输出 `last_message`：日志更短，但无法判断完整上下文和 system prompt 是否影响模型行为。
- 输出缩进 JSON：可读性略高，但会拆成多行日志，不利于按 request_id 排查。

## Risks / Trade-offs

- [Risk] 当前仍是单次工具闭环，runCache 在完整 `ChatBot.Send()` 中可观察收益有限。  
  Mitigation: 本版本先交付 key 和 runCache 基础设施，为下一版多步 Tool Loop 提供测试好的基础。

- [Risk] 参数标准化过窄可能无法识别自然语言等价参数。  
  Mitigation: 保持通用层确定性，不做语义推断；业务级归一化留给具体工具或后续策略。

- [Risk] 参数标准化过宽可能导致错误复用。  
  Mitigation: 只做 JSON 结构级标准化，不改写字符串和业务字段。

- [Risk] 工具结果如果被缓存后复用，metadata 中可能混入第一次执行的上下文信息。  
  Mitigation: 本版本只在一次请求作用域内复用，并保持工具结果结构不变；后续可在多步循环中追加 `cache_hit` 标记。

- [Risk] 完整 LLM request body 和完整工具结果日志可能增加日志体积，且可能包含用户输入、文件读取内容或搜索结果。  
  Mitigation: 当前项目日志本就是本地调试用途；后续如进入生产化，需要增加日志级别、脱敏和内容截断策略。
