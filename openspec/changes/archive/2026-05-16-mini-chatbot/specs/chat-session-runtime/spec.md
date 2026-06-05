## ADDED Requirements

### Requirement: Structured chat messages
系统必须使用统一消息结构表示对话上下文，至少支持 `system`、`user` 和 `assistant` 三种角色。

#### Scenario: Initialize session with system prompt
- **WHEN** 系统使用配置中的 system prompt 创建新会话
- **THEN** 会话消息列表必须包含一条角色为 `system` 的初始消息

#### Scenario: Append user and assistant messages
- **WHEN** 用户完成一次成功对话
- **THEN** 会话必须先追加一条 `user` 消息
- **THEN** 在模型成功返回后，会话必须再追加一条 `assistant` 消息

### Requirement: ChatBot orchestrates a single turn
系统必须通过聊天运行时编排单轮请求，使会话管理和模型调用职责分离。

#### Scenario: Successful turn execution
- **WHEN** ChatBot 接收到一条用户输入且模型调用成功
- **THEN** ChatBot 必须读取当前会话消息并调用统一的 LLM 客户端接口
- **THEN** ChatBot 必须返回模型文本结果给 CLI 层

#### Scenario: Model call failure
- **WHEN** LLM 客户端返回错误
- **THEN** ChatBot 必须将错误返回给调用方
- **THEN** 系统不得为该轮失败请求追加 assistant 消息
