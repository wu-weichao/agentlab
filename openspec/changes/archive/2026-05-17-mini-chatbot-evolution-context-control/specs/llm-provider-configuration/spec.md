## MODIFIED Requirements

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
