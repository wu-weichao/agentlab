## ADDED Requirements

### Requirement: Select tool calling behavior per Agent
Agent Runtime MUST 使用构造时注入的工具调用模式，统一决定工具说明、模型请求、响应识别和工具结果回填路径。

#### Scenario: Construct native tool calling Agent
- **WHEN** 调用方以 `native` 模式构造 Agent
- **THEN** Agent MUST 在模型请求中提供 Executor 的工具定义
- **THEN** Agent MUST 优先消费 `ChatResponse` 中的结构化工具调用

#### Scenario: Construct text compatibility Agent
- **WHEN** 调用方以 `text_compat` 模式构造 Agent
- **THEN** Agent MUST 使用现有文本工具说明和 `ParseToolCall` 路径
- **THEN** Agent MUST NOT 要求 LLM provider 支持原生工具调用

### Requirement: Record tool calling mode diagnostics
Agent Runtime MUST 为模型请求和工具循环记录当前工具调用模式、完整工具调用和完整工具执行结果，以支持学习和调试。

#### Scenario: Log native tool call diagnostics
- **WHEN** Agent 在 native 模式收到结构化工具调用
- **THEN** 日志 MUST 包含 `tool_calling_mode=native`、调用数量和解析状态
- **THEN** 现有 step 日志 MUST 继续包含 `request_id`、`tool_name`、`tool_call_key` 和 `cache_hit`
- **THEN** 日志 MUST 包含完整 `tool_call_json` 和完整 `tool_result_json`

#### Scenario: Log text compatibility diagnostics
- **WHEN** Agent 在 text compatibility 模式识别文本工具调用
- **THEN** 日志 MUST 包含 `tool_calling_mode=text_compat`
- **THEN** 日志 MUST 输出解析后的完整工具参数和完整执行结果

## MODIFIED Requirements

### Requirement: Own bounded sequential tool execution
Agent Runtime MUST 负责原生和文本兼容两种模式下的顺序多步工具循环、请求内 runCache、工具反馈和诊断 metadata，并保持最大步数与执行安全边界不变。

#### Scenario: Complete a single native tool call
- **WHEN** native 模式下模型响应包含一个合法结构化工具调用
- **THEN** Agent MUST 通过注入的 ToolExecutor 执行工具
- **THEN** Agent MUST 使用调用 ID 关联 assistant tool call 和 tool result 消息
- **THEN** 下一次模型响应为普通文本时 Agent MUST 将其作为最终回答返回并写入会话

#### Scenario: Complete sequential native tool calls
- **WHEN** 模型在同一次 Run 中按步骤返回多个合法的单个结构化工具调用并最终返回普通文本
- **THEN** Agent MUST 在最大步数内顺序执行这些工具
- **THEN** Agent MUST 在各步骤间保留本轮原生工具消息但不得把工具轨迹写入会话历史

#### Scenario: Complete a text compatibility tool call
- **WHEN** text compatibility 模式下模型 content 包含一个合法文本工具调用
- **THEN** Agent MUST 使用现有文本解析和反馈路径完成工具闭环

#### Scenario: Reuse duplicate native tool result within one Run
- **WHEN** 同一次 `Agent.Run()` 中出现 ID 不同但工具名和标准化参数相同的原生工具调用
- **THEN** Agent MUST 复用第一次执行产生的 ToolResult
- **THEN** Agent MUST NOT 再次调用真实工具实现
- **THEN** 当前步骤反馈 MUST 使用当前调用 ID 并标记 `cache_hit=true`

#### Scenario: Discard run cache between Runs
- **WHEN** 两次不同的 `Agent.Run()` 请求调用相同工具和参数
- **THEN** 第二次 Run MUST NOT 复用第一次 Run 的工具结果
- **THEN** 本变更 MUST NOT 引入跨轮缓存或 TTL

#### Scenario: Stop unsupported or excessive native execution
- **WHEN** 模型一次响应包含多个原生工具调用，或达到 MaxSteps 后仍未返回最终文本
- **THEN** Agent MUST 返回现有对应错误
- **THEN** Agent MUST NOT 执行多调用响应中的任何工具
- **THEN** Agent MUST NOT 为失败请求追加 assistant 消息

### Requirement: Preserve Runtime observability boundaries
Agent Runtime MUST 保留上下文和工具循环的关键结构化诊断字段，并为本地学习调试记录完整工具调用和工具执行结果，同时禁止记录认证凭据。

#### Scenario: Record tool loop diagnostics from Agent
- **WHEN** Agent 执行工具循环步骤
- **THEN** 日志 MUST 包含 `request_id`、`step`、`max_steps`、`tool_name`、`tool_call_key`、`cache_hit`、`success` 和 `error_code`
- **THEN** 工具反馈 metadata MUST 包含 `step`、`cache_hit` 和 `tool_call_key`

#### Scenario: Record complete tool protocol content
- **WHEN** Agent 识别工具调用并完成工具执行
- **THEN** 日志 MUST 记录完整 `tool_call_json`
- **THEN** 日志 MUST 记录完整 `tool_result_json`
- **THEN** 日志 MUST 允许包含工具参数、文件正文或搜索结果全文，以支持本地学习和协议排查

#### Scenario: Exclude authentication credentials
- **WHEN** Agent 或 LLM 客户端记录完整学习日志
- **THEN** 日志 MUST NOT 输出 Authorization header
- **THEN** 日志 MUST NOT 输出 API Key
