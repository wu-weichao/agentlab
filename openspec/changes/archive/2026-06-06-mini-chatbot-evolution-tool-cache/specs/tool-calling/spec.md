## ADDED Requirements

### Requirement: Build stable tool call keys
系统 MUST 为每次结构化工具调用构造稳定的 `ToolCallKey`，用于在 Runtime 内识别相同工具和相同参数的重复调用。

#### Scenario: Same arguments with different field order
- **WHEN** 两个工具调用使用相同 `tool_name`，且 `arguments` 仅 JSON 字段顺序不同
- **THEN** 系统 MUST 为它们生成相同的 `ToolCallKey`

#### Scenario: Different tools with same arguments
- **WHEN** 两个工具调用使用不同 `tool_name`，即使 `arguments` 完全相同
- **THEN** 系统 MUST 为它们生成不同的 `ToolCallKey`

#### Scenario: Nested arguments are normalized
- **WHEN** 工具调用参数包含嵌套对象
- **THEN** 系统 MUST 对嵌套对象字段顺序进行稳定标准化
- **THEN** 语义相同但字段顺序不同的嵌套参数 MUST 生成相同的 `ToolCallKey`

### Requirement: Provide run-level tool result cache infrastructure
系统 MUST 提供一次请求作用域内可使用的 run-level 工具结果缓存基础设施，并能够复用相同 `ToolCallKey` 对应的工具执行结果。

#### Scenario: Duplicate tool call through execution wrapper
- **WHEN** 工具执行包装逻辑在同一个 run cache 中收到相同 `tool_name` 和相同标准化 `arguments` 的工具调用
- **THEN** 系统 MUST 复用该 `ToolCallKey` 已缓存的工具结果
- **THEN** 系统 MUST 不要求工具接口声明 TTL 或跨轮缓存策略

#### Scenario: Different arguments use different cache entries
- **WHEN** 工具执行包装逻辑收到相同工具但参数不同的工具调用
- **THEN** 系统 MUST 将它们视为不同工具调用
- **THEN** 系统 MUST 使用不同 run cache entry

#### Scenario: Run cache is scoped to current request
- **WHEN** 一次用户请求处理结束
- **THEN** 系统 MUST 丢弃本次请求的 run-level 工具调用缓存
- **THEN** 后续新的用户请求 MUST 不依赖上一轮请求的 run-level 缓存结果

### Requirement: Preserve current tool calling boundaries
系统 MUST 在引入 runCache 基础设施时保持当前 Tool Calling 能力边界不变。

#### Scenario: Existing single tool call loop remains compatible
- **WHEN** 模型第一阶段响应为合法单个 `tool_call`
- **THEN** ChatBot MUST 继续执行该工具并回填 `tool_result`
- **THEN** ChatBot MUST 继续基于最终模型响应保存 assistant 回答

#### Scenario: Tool result is returned as final-answer input
- **WHEN** ChatBot 已执行第一阶段模型请求中的合法单个 `tool_call`
- **THEN** ChatBot MUST 在第二次模型请求中保留一条表示工具请求意图的 assistant 消息
- **THEN** ChatBot MUST 使用 user 消息回填工具执行结果和 `FINAL_ANSWER` 阶段约束
- **THEN** 该回填消息 MUST 明确禁止再次请求工具调用或输出 `tool_call` JSON

#### Scenario: No cross-turn cache is introduced
- **WHEN** 用户在后续对话轮次中再次请求相同工具和相同参数
- **THEN** 系统 MUST 不因本变更复用上一轮请求的工具结果
- **THEN** 是否执行工具 MUST 继续遵循当前工具调用闭环行为

#### Scenario: Tool interface remains unchanged
- **WHEN** 本变更完成
- **THEN** 现有工具 MUST 继续通过当前 `Tool` 接口注册和执行
- **THEN** 系统 MUST 不要求工具实现 TTL、`ToolMetadata` 或跨轮缓存策略

### Requirement: Provide full diagnostic logs for tool loop debugging
系统 MUST 在本地日志中提供足够诊断信息，用于确认工具结果和 LLM 请求上下文是否按预期传递。

#### Scenario: Tool execution log includes full ToolResult JSON
- **WHEN** ChatBot 完成一次工具执行
- **THEN** 日志 MUST 包含工具名、执行成功状态和错误文本
- **THEN** 日志 MUST 包含完整单行 `ToolResult` JSON 字符串

#### Scenario: LLM request log includes full request body
- **WHEN** OpenAI 兼容客户端发送聊天补全请求
- **THEN** 日志 MUST 包含 provider、model、url 和消息数量
- **THEN** 日志 MUST 包含实际发送的完整 JSON request body
- **THEN** 日志 MUST NOT 输出 Authorization header
