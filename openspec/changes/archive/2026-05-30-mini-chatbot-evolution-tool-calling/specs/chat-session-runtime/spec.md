## MODIFIED Requirements

### Requirement: ChatBot orchestrates a single turn
系统必须通过聊天运行时编排单轮请求，在模型调用前执行上下文预算构建和必要的摘要更新，同时保持会话管理、模型调用和工具执行职责分离。

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

#### Scenario: Tool-enabled turn execution
- **WHEN** ChatBot 收到模型第一阶段返回的合法单个 `tool_call`
- **THEN** ChatBot 必须通过 ToolExecutor 执行该工具
- **THEN** ChatBot 必须将结构化 `tool_result` 回填给模型，并要求模型进入最终回答阶段
- **THEN** 成功后会话历史必须只保存用户输入和最终 assistant 文本，不把内部工具调用协议作为普通用户消息保存

#### Scenario: Tool instructions are part of system prompt
- **WHEN** ChatBot 使用工具执行器完成初始化
- **THEN** ChatBot 必须将工具说明追加到会话 system prompt
- **THEN** 本轮请求构建出的第一条 system 消息必须包含该工具说明
- **THEN** ChatBot 不得在每次 `Send` 时额外追加临时工具说明消息

#### Scenario: Clear keeps tool instructions
- **WHEN** 用户执行清空会话历史操作
- **THEN** 系统必须移除滚动摘要和近期原始消息
- **THEN** system prompt 中的工具说明必须继续保留

#### Scenario: Tool failure during turn execution
- **WHEN** 工具执行失败或工具结果回填后的模型调用失败
- **THEN** ChatBot 必须返回明确错误或由模型基于失败结果生成说明
- **THEN** 系统不得追加伪造的工具成功结果
- **THEN** 系统不得为二次模型调用失败的请求追加 `assistant` 消息

#### Scenario: Recursive tool request after tool result
- **WHEN** 工具结果回填后的第二次模型响应仍请求工具调用
- **THEN** ChatBot 不得继续执行第二个工具调用
- **THEN** ChatBot 必须基于已有工具结果生成兜底最终回答
- **THEN** 成功后会话历史必须保存用户输入和兜底 assistant 回答
