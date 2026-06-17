## MODIFIED Requirements

### Requirement: Configure bounded tool loop execution
系统 MUST 为同一次用户请求内的工具循环提供 `ToolLoopOptions{MaxSteps int}`，并使用配置化最大步数防止无限工具调用。

#### Scenario: Use configured max steps
- **WHEN** 本地配置提供合法 `tools.max_steps`
- **THEN** ChatBot 工具循环 MUST 使用该值作为 `ToolLoopOptions.MaxSteps`

#### Scenario: Use default max steps
- **WHEN** 本地配置未提供 `tools.max_steps`
- **THEN** 系统 MUST 使用默认最大步数 `3`

#### Scenario: Stop when max steps exceeded
- **WHEN** 模型在达到 `MaxSteps` 后仍未生成普通最终回答
- **THEN** 系统 MUST 返回 `ErrToolLoopExceeded`
- **THEN** 系统 MUST NOT 向会话历史追加 assistant 消息

#### Scenario: No tool request uses one model call
- **WHEN** 模型第一次响应为普通 assistant 文本
- **THEN** 系统 MUST 将该文本保存为最终 assistant 回答
- **THEN** 系统 MUST NOT 进入额外工具执行步骤

### Requirement: Provide full diagnostic logs for tool loop debugging
系统 MUST 在本地日志中提供足够诊断信息，用于确认工具循环步骤、工具结果和 LLM 请求上下文是否按预期传递，同时避免新增工具循环日志暴露完整敏感内容。

#### Scenario: Tool execution log includes bounded ToolResult diagnostics
- **WHEN** ChatBot 完成一次工具执行
- **THEN** 日志 MUST 包含工具名、执行成功状态和错误文本
- **THEN** 日志 MUST 包含脱敏后的单行 `tool_result_json`
- **THEN** `tool_result_json` MUST 包含 `success`、`error`、`metadata` 和 `content_chars`
- **THEN** 日志 MUST NOT 包含完整文件内容、搜索结果全文或完整工具参数

#### Scenario: LLM request log includes full request body
- **WHEN** OpenAI 兼容客户端发送聊天补全请求
- **THEN** 日志 MUST 包含 provider、model、url 和消息数量
- **THEN** 日志 MUST 包含实际发送的完整 JSON request body
- **THEN** 日志 MUST NOT 输出 Authorization header

## ADDED Requirements

### Requirement: Record structured tool loop step logs
系统 MUST 为工具循环中的每一步记录结构化日志，使本地调试能够定位工具调用步骤、缓存命中、执行状态和超限原因。

#### Scenario: Log each tool loop step
- **WHEN** 模型在工具循环某一步请求合法工具调用
- **THEN** 系统 MUST 记录包含 `request_id`、`step`、`max_steps`、`tool_name`、`tool_call_key`、`cache_hit`、`success` 和 `error_code` 的日志
- **THEN** `step` MUST 表示当前工具循环步骤序号

#### Scenario: Log cache hit in step diagnostics
- **WHEN** 工具调用命中当前 run cache
- **THEN** step 日志 MUST 记录 `cache_hit=true`
- **THEN** step 日志 MUST 包含对应的 `tool_call_key`
- **THEN** 系统 MUST NOT 再次调用真实工具实现

#### Scenario: Log cache miss in step diagnostics
- **WHEN** 工具调用未命中当前 run cache
- **THEN** step 日志 MUST 记录 `cache_hit=false`
- **THEN** step 日志 MUST 包含对应的 `tool_call_key`

#### Scenario: Avoid sensitive diagnostic content
- **WHEN** 系统记录工具循环 step 日志
- **THEN** 日志 MUST NOT 输出完整工具参数
- **THEN** 日志 MUST NOT 输出完整文件内容或搜索结果全文
- **THEN** 日志 MUST NOT 输出明显密钥或隐私内容

### Requirement: Attach tool loop diagnostics to tool results
系统 MUST 在工具执行结果 metadata 中补充最小调试字段，使模型回填、日志和测试能够关联同一步工具调用。

#### Scenario: Tool result includes step diagnostics
- **WHEN** 工具循环完成一次工具调用
- **THEN** 返回给后续模型请求的 `ToolResult.Metadata` MUST 包含 `step`
- **THEN** 返回给后续模型请求的 `ToolResult.Metadata` MUST 包含 `cache_hit`
- **THEN** 返回给后续模型请求的 `ToolResult.Metadata` MUST 包含 `tool_call_key`

#### Scenario: Cached tool result updates diagnostics
- **WHEN** 工具循环复用 run cache 中的 `ToolResult`
- **THEN** 系统 MUST 在当前步骤返回前设置 `cache_hit=true`
- **THEN** 系统 MUST 在当前步骤返回前更新 `step` 为当前步骤序号
- **THEN** 系统 MUST 保留原工具结果的业务 metadata，除运行时调试字段外不得丢弃已有字段

#### Scenario: Failed tool result includes diagnostics
- **WHEN** 工具执行返回失败结果
- **THEN** 失败的 `ToolResult.Metadata` MUST 仍包含 `step`、`cache_hit` 和 `tool_call_key`
- **THEN** 系统 MUST 将失败结果作为受控上下文交给模型生成说明或下一步决策
