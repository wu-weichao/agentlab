## ADDED Requirements

### Requirement: Support provider-native tool calling
系统 MUST 在 `native` 模式下使用 provider 原生工具定义、结构化工具调用和工具结果消息完成 Tool Calling，同时保持仓库内部统一工具协议。

#### Scenario: Expose registered tools through native definitions
- **WHEN** Agent 在 `native` 模式下调用模型
- **THEN** 请求 MUST 包含当前 Executor 已注册工具的名称、描述和参数 schema
- **THEN** system prompt MUST NOT 要求模型输出 JSON 文本工具调用

#### Scenario: Convert provider call to unified ToolCall
- **WHEN** provider 返回一个合法原生工具调用
- **THEN** 系统 MUST 将调用 ID、工具名称和参数转换为统一 `ToolCall`
- **THEN** ToolExecutor MUST 继续通过统一 `ToolCall` 执行工具

#### Scenario: Keep tool call ID out of cache key
- **WHEN** 两个原生工具调用的 ID 不同但工具名和标准化参数相同
- **THEN** 系统 MUST 为它们生成相同 `ToolCallKey`
- **THEN** 同一次 Run 中第二个调用 MUST 能够复用 runCache

### Requirement: Preserve text tool calling compatibility
系统 MUST 提供显式 `text_compat` 模式，保留现有 JSON 文本工具调用能力。

#### Scenario: Parse text compatibility tool call
- **WHEN** 系统处于 `text_compat` 模式且模型在 assistant content 中返回合法 JSON 工具调用
- **THEN** Agent MUST 使用现有文本解析逻辑识别并执行该调用

#### Scenario: Use text compatibility feedback
- **WHEN** `text_compat` 模式完成一次工具执行
- **THEN** Agent MUST 使用现有 assistant 意图消息和 user 工具结果消息回填下一次请求
- **THEN** 工具轨迹 MUST NOT 写入 Session

#### Scenario: Do not silently fall back
- **WHEN** `native` 模式请求失败或 provider 返回非法原生工具调用
- **THEN** 系统 MUST 返回明确错误
- **THEN** 系统 MUST NOT 自动重试为 `text_compat`

## MODIFIED Requirements

### Requirement: Expose registered tools in system prompt
系统 MUST 根据工具调用模式向模型暴露已注册工具，并确保工具能力在会话 reset 后继续可用。

#### Scenario: Build native tool behavior instructions at startup
- **WHEN** Agent 使用 `native` 模式和工具执行器初始化
- **THEN** system prompt MUST 包含顺序工具使用、避免重复调用和最终回答输出约束
- **THEN** system prompt MUST NOT 包含要求模型输出工具 JSON 的格式说明
- **THEN** 具体工具名称、描述和参数 MUST 通过每次模型请求的原生工具定义传递

#### Scenario: Build text compatibility instructions at startup
- **WHEN** Agent 使用 `text_compat` 模式和工具执行器初始化
- **THEN** 系统必须基于已注册工具生成包含工具名称、描述、参数和调用 JSON 格式的工具说明
- **THEN** 系统必须将该工具说明追加到会话 system prompt

#### Scenario: Preserve mode instructions after clear
- **WHEN** 会话执行 `clear` 或 reset
- **THEN** 系统必须清空摘要和近期消息
- **THEN** system prompt 中当前模式对应的工具行为说明必须继续保留

### Requirement: Complete one tool call loop per user turn
系统 MUST 支持单轮对话中的受控顺序多步工具调用闭环，并根据当前模式使用原生工具消息或文本兼容消息回填结果。

#### Scenario: Complete one native tool call successfully
- **WHEN** native 模式下模型响应包含一个合法结构化工具调用
- **THEN** Agent 必须执行该工具
- **THEN** Agent 必须保留 assistant tool call 并追加关联 tool call ID 的 tool result 消息
- **THEN** 如果下一次模型响应为普通文本，Agent 必须将其保存为最终 assistant 回答
- **THEN** 成功后会话历史必须只包含用户输入和最终 assistant 回答，不包含内部工具轨迹

#### Scenario: Complete sequential native tool calls successfully
- **WHEN** 模型在同一次用户请求内先后返回多个顺序的单个原生工具调用
- **THEN** Agent 必须按步骤逐个执行工具
- **THEN** 每一步最多执行一个工具调用
- **THEN** Agent 必须把每次原生 tool result 追加到本轮工作消息后继续下一步
- **THEN** 当模型返回普通文本时 Agent 必须保存该文本作为最终 assistant 回答

#### Scenario: Tool execution fails safely
- **WHEN** 工具执行返回失败结果
- **THEN** Agent 必须将失败结果通过当前模式对应的受控工具反馈交给模型
- **THEN** 系统不得中断进程或追加伪造的工具成功结果

#### Scenario: Continue after tool result when another tool is needed
- **WHEN** 工具结果回填后的下一次模型响应仍请求一个合法工具调用
- **THEN** 系统必须在未超过 `MaxSteps` 时继续执行该工具调用
- **THEN** 系统必须继续使用同一次请求内的 runCache

#### Scenario: Multiple native tool calls are unsupported
- **WHEN** native 模式下模型一次响应中请求多个工具调用
- **THEN** 系统必须返回明确的不支持错误
- **THEN** 系统不得执行其中任何工具

### Requirement: Preserve current tool calling boundaries
系统 MUST 在引入 provider 原生 Tool Calling 时保持现有工具集合、执行边界、缓存生命周期和文本兼容能力。

#### Scenario: Existing text tool call loop remains available
- **WHEN** 系统配置为 `text_compat` 且模型返回合法单个文本 `tool_call`
- **THEN** Agent MUST 继续执行该工具并回填文本协议 `tool_result`
- **THEN** Agent MUST 继续基于最终模型响应保存 assistant 回答

#### Scenario: Native tool result is returned with call correlation
- **WHEN** Agent 已执行 native 模式中的合法工具调用
- **THEN** 下一次模型请求 MUST 包含原 assistant tool call
- **THEN** 下一次模型请求 MUST 包含使用相同 tool call ID 的 tool result 消息

#### Scenario: No cross-turn cache is introduced
- **WHEN** 用户在后续对话轮次中再次请求相同工具和相同参数
- **THEN** 系统 MUST 不复用上一轮请求的工具结果
- **THEN** 是否执行工具 MUST 继续遵循当前工具调用循环行为

#### Scenario: Tool interface remains unchanged
- **WHEN** 本变更完成
- **THEN** 现有工具 MUST 继续通过当前 `Tool` 接口注册和执行
- **THEN** 系统 MUST 不要求工具实现 TTL、`ToolMetadata` 或跨轮缓存策略

### Requirement: Provide full diagnostic logs for tool loop debugging
系统 MUST 在本地学习日志中记录足够且完整的诊断信息，用于确认工具调用参数、工具执行结果和 LLM 请求上下文是否按预期传递。

#### Scenario: Tool execution log includes complete ToolResult
- **WHEN** Agent 完成一次工具执行
- **THEN** 日志 MUST 包含工具名、执行成功状态和错误文本
- **THEN** 日志 MUST 包含完整单行 `tool_result_json`
- **THEN** `tool_result_json` MUST 包含完整 `success`、`content`、`error` 和 `metadata`

#### Scenario: Tool call log includes complete arguments
- **WHEN** Agent 识别到合法工具调用
- **THEN** 日志 MUST 包含完整单行 `tool_call_json`
- **THEN** `tool_call_json` MUST 包含调用 ID、工具名称、完整 arguments 和 reason

#### Scenario: LLM request and response logs include full bodies
- **WHEN** OpenAI 兼容客户端发送聊天补全请求并收到响应
- **THEN** 请求日志 MUST 包含实际发送的完整 JSON `request_body`
- **THEN** 响应日志 MUST 包含 provider 返回的完整 `response_body`
- **THEN** 日志 MUST NOT 输出 Authorization header 或 API Key
