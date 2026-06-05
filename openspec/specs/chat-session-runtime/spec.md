## ADDED Requirements

### Requirement: Structured chat messages
系统必须使用统一消息结构表示对话上下文，至少支持 `system`、`user`、`assistant` 和内部 `summary` 四种角色。

#### Scenario: Initialize session with rendered system prompt
- **WHEN** 系统使用渲染后的 system prompt 创建新会话
- **THEN** 会话必须包含一条角色为 `system` 的初始消息作为会话起点
- **THEN** 初始消息内容必须等于启动阶段生成的最终 prompt 字符串

#### Scenario: Append user and assistant messages
- **WHEN** 用户完成一次成功对话
- **THEN** 会话必须先追加一条 `user` 消息
- **THEN** 在模型成功返回后，会话必须再追加一条 `assistant` 消息

#### Scenario: Store rolling summary as internal semantic message
- **WHEN** 上下文治理流程生成或更新滚动摘要
- **THEN** 会话必须以内部 `summary` 语义保存该摘要内容
- **THEN** 会话不得同时保存多份滚动摘要

### Requirement: ChatBot orchestrates a single turn
系统必须通过聊天运行时编排单轮请求，在模型调用前执行上下文预算构建和必要的摘要更新，同时保持会话管理和模型调用职责分离。

#### Scenario: Successful turn execution
- **WHEN** ChatBot 接收到一条用户输入且模型调用成功
- **THEN** ChatBot 必须先基于会话状态和上下文预算构建本轮请求消息
- **THEN** ChatBot 必须调用统一的 LLM 客户端接口并返回模型文本结果给 CLI 层

#### Scenario: Model call failure
- **WHEN** LLM 客户端返回错误
- **THEN** ChatBot 必须将错误返回给调用方
- **THEN** 系统不得为该轮失败请求追加 `assistant` 消息

#### Scenario: Summary update during turn preparation
- **WHEN** 本轮请求在预算检查后移出了旧历史消息
- **THEN** ChatBot 必须在调用主模型前完成滚动摘要更新
- **THEN** 发送给主模型的消息列表必须反映新的摘要和裁剪后的近期消息

#### Scenario: Rebuild with compressed summary
- **WHEN** 更新后的滚动摘要插回上下文后仍然超出预算
- **THEN** ChatBot 必须继续压缩已有摘要并重试一次上下文构建
- **THEN** 只有在重试后仍超预算时，系统才可以返回上下文预算错误

### Requirement: Reset session with the same rendered prompt
系统必须在清空会话历史时恢复到本次启动阶段生成的同一个 system prompt，并同时清空滚动摘要和近期原始消息。

#### Scenario: Clear command restores rendered prompt
- **WHEN** 用户执行清空会话历史操作
- **THEN** 系统必须移除当前会话中的滚动摘要以及历史 `user` 和 `assistant` 消息
- **THEN** 系统必须保留一条内容等于本次启动已渲染 prompt 的 `system` 消息作为新会话起点

### Requirement: Expose summary-aware session history
系统必须能够以稳定顺序暴露当前会话历史，使调用方可以区分滚动摘要与近期原始消息。

#### Scenario: Read history with summary and recent messages
- **WHEN** 调用方请求读取当前会话历史
- **THEN** 系统必须按 system prompt、滚动摘要、近期原始消息的顺序返回结果
- **THEN** 如不存在滚动摘要，系统必须只返回 system prompt 和近期原始消息
