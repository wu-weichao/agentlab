## ADDED Requirements

### Requirement: Render system prompt from template
系统必须支持通过 Prompt 模板和结构化变量在启动阶段生成最终的 system prompt。

#### Scenario: Render prompt from structured variables
- **WHEN** 配置文件提供 `chat.prompt.template`、`chat.prompt.role` 和 `chat.prompt.context`
- **THEN** 系统必须使用模板与变量生成最终的 system prompt 字符串
- **THEN** 生成结果必须可直接作为会话中的首条 `system` 消息

### Requirement: Validate prompt variables before session startup
系统必须在进入交互式聊天循环前校验 Prompt 模板中引用的变量是否完整且非空。

#### Scenario: Reject missing template variable
- **WHEN** 模板中引用了未在结构化配置中提供的变量
- **THEN** 系统必须在启动阶段返回明确的配置错误
- **THEN** 系统不得进入交互式聊天循环

#### Scenario: Reject empty standard variable value
- **WHEN** `role.name`、`role.goal`、`role.style` 或 `context.language` 的值为空
- **THEN** 系统必须在启动阶段返回明确的配置错误
- **THEN** 系统不得进入交互式聊天循环

### Requirement: Support standard prompt variable paths
系统必须支持第二版定义的标准 Prompt 变量路径，并按点路径语义进行替换。

#### Scenario: Replace standard variable paths
- **WHEN** 模板中包含 `{{role.name}}`、`{{role.goal}}`、`{{role.style}}` 或 `{{context.language}}`
- **THEN** 系统必须使用对应结构化配置值替换这些变量
- **THEN** 未被替换的原始变量占位符不得出现在最终 system prompt 中
