## ADDED Requirements

### Requirement: Configure tool calling mode
系统 MUST 允许通过本地配置显式选择 LLM 工具调用模式，并在未配置时使用原生模式。

#### Scenario: Use native mode by default
- **WHEN** 配置文件未提供工具调用模式
- **THEN** 系统 MUST 使用 `native` 模式
- **THEN** OpenAI 兼容客户端 MUST 使用 provider 原生 Tool Calling

#### Scenario: Use text compatibility mode
- **WHEN** 配置文件将工具调用模式设置为 `text_compat`
- **THEN** 系统 MUST 使用现有 JSON 文本工具协议
- **THEN** 系统 MUST NOT 在该模式下向 provider 发送原生 `tools` 定义

#### Scenario: Reject unsupported tool calling mode
- **WHEN** 配置文件提供的工具调用模式不是 `native` 或 `text_compat`
- **THEN** 系统 MUST 在启动阶段返回明确配置错误
- **THEN** 系统 MUST NOT 进入交互式聊天循环

## MODIFIED Requirements

### Requirement: OpenAI-compatible chat client
系统 MUST 提供基于 OpenAI 兼容聊天接口的 LLM 客户端实现，通过统一请求和响应类型同时支持普通文本聊天与 provider 原生 Tool Calling。

#### Scenario: Build provider client from config
- **WHEN** 配置中的 provider 为 `openai`
- **THEN** 系统必须创建一个实现统一 LLM 客户端接口的 OpenAI 兼容客户端
- **THEN** 客户端必须使用配置的工具调用模式处理后续请求

#### Scenario: Send ordinary chat completion request
- **WHEN** Agent 将不包含工具定义的普通聊天请求传给 OpenAI 兼容客户端
- **THEN** 客户端必须把角色和消息文本转换为目标接口请求格式
- **THEN** 客户端必须返回首个助手文本回复或明确错误

#### Scenario: Send native tool definitions
- **WHEN** 请求处于 `native` 模式且包含已注册工具定义
- **THEN** 客户端 MUST 把工具名称、描述和参数转换为 OpenAI 兼容 `tools` 定义
- **THEN** 客户端 MUST 将工具参数必填项转换为 JSON Schema 的 `required`

#### Scenario: Parse native tool call response
- **WHEN** provider 返回 assistant `tool_calls`
- **THEN** 客户端 MUST 将每个调用的 ID、工具名称和参数转换为仓库统一结构化工具调用
- **THEN** 客户端 MUST NOT 要求调用方解析 assistant `content` 中的 JSON

#### Scenario: Reject invalid native tool arguments
- **WHEN** provider 返回的工具参数不是合法 JSON object
- **THEN** 客户端 MUST 返回明确的工具调用协议错误
- **THEN** Agent MUST NOT 执行该工具

#### Scenario: Preserve assistant content response
- **WHEN** provider 未返回工具调用且 assistant `content` 包含文本
- **THEN** 客户端 MUST 将该文本作为普通最终回答返回

### Requirement: Record request details to log file
系统 MUST 为本地学习和调试记录接口请求与响应的完整协议内容，同时不得记录认证请求头或 API Key。

#### Scenario: Record request summary
- **WHEN** OpenAI 兼容客户端发起一次聊天请求
- **THEN** 系统必须记录 provider、model、请求 URL、消息数量、工具数量和工具调用模式
- **THEN** 系统 MUST 记录实际发送的完整 JSON `request_body`
- **THEN** 系统 MUST NOT 记录 Authorization header

#### Scenario: Record native response summary
- **WHEN** OpenAI 兼容客户端收到接口响应
- **THEN** 系统必须记录响应状态、耗时、响应体大小、工具调用数量和解析状态
- **THEN** 系统 MUST 记录 provider 返回的完整 `response_body`
- **THEN** 成功和失败响应都 MUST 保留完整响应内容
