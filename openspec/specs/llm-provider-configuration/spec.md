## ADDED Requirements

### Requirement: Load chat runtime configuration
系统必须从本地配置文件加载聊天运行时所需参数，包括 provider、model、base URL、API Key、temperature、Prompt 模板配置和 `chat.context.*` 上下文预算配置。

#### Scenario: Read configuration successfully
- **WHEN** 本地 `config.yaml` 文件存在且内容合法
- **THEN** 系统必须构造完整的聊天运行时配置对象
- **THEN** 配置对象必须包含 `chat.prompt.template`、`chat.prompt.role`、`chat.prompt.context`、`chat.context.max_chars`、`chat.context.keep_recent_turns`、`chat.context.summary_max_chars` 和 `chat.context.enable_rolling_summary`

#### Scenario: Missing required API key
- **WHEN** 配置文件未提供所选 provider 所需的 API Key
- **THEN** 系统必须返回明确的配置错误
- **THEN** 系统不得启动聊天会话

#### Scenario: Reject placeholder API key at startup
- **WHEN** 配置文件中的 `api_key` 仍为 `{...}` 占位值
- **THEN** 系统必须在启动阶段返回明确的配置错误
- **THEN** 系统不得进入交互式聊天循环

#### Scenario: Reject missing prompt template configuration
- **WHEN** 配置文件缺少 `chat.prompt.template`、`chat.prompt.role` 或 `chat.prompt.context`
- **THEN** 系统必须在启动阶段返回明确的配置错误
- **THEN** 系统不得进入交互式聊天循环

#### Scenario: Reject invalid context control configuration
- **WHEN** 配置文件中的 `chat.context.max_chars`、`chat.context.keep_recent_turns` 或 `chat.context.summary_max_chars` 为零、负数或摘要上限超过总预算
- **THEN** 系统必须在启动阶段返回明确的配置错误
- **THEN** 系统不得进入交互式聊天循环

#### Scenario: Reject removed history-count configuration
- **WHEN** 配置文件仍包含已移除的 `chat.max_history_messages`
- **THEN** 系统必须在启动阶段返回明确的配置错误
- **THEN** 系统不得以兼容模式继续启动

### Requirement: OpenAI-compatible chat client
系统必须提供首个基于 OpenAI 兼容聊天接口的 LLM 客户端实现，并通过统一接口暴露给聊天运行时。

#### Scenario: Build provider client from config
- **WHEN** 配置中的 provider 为 `openai`
- **THEN** 系统必须创建一个实现统一 LLM 客户端接口的 OpenAI 兼容客户端

#### Scenario: Send chat completion request
- **WHEN** ChatBot 将会话消息传给 OpenAI 兼容客户端
- **THEN** 客户端必须把角色和消息文本转换为目标接口请求格式
- **THEN** 客户端必须返回首个助手文本回复或明确错误

### Requirement: Record request details to log file
系统必须记录接口请求和响应的关键调试信息，以支持本地排障。

#### Scenario: Record request summary
- **WHEN** OpenAI 兼容客户端发起一次聊天请求
- **THEN** 系统必须记录 provider、model、请求 URL、消息数量和最后一条消息摘要

#### Scenario: Record response summary
- **WHEN** OpenAI 兼容客户端收到接口响应
- **THEN** 系统必须记录响应状态、耗时、响应体大小，以及失败时的错误详情摘要或成功时的回复摘要
