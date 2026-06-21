## MODIFIED Requirements

### Requirement: Run a complete conversational turn through Agent
系统 MUST 提供结构化运行入口，独立完成从用户输入、受控上下文构建和模型调用到最终 assistant 回答写回的 Runtime 编排；现有 `Agent.Run(ctx, input)` MUST 委托该入口并保持简单文本返回接口。

#### Scenario: Complete ordinary chat without tools
- **WHEN** Agent 收到用户输入且模型第一次响应为普通 assistant 文本
- **THEN** Agent MUST 在模型调用前将用户输入写入会话并构建受预算约束的上下文
- **THEN** 结构化入口 MUST 返回包含最终文本和运行轨迹的 RunResult
- **THEN** Agent MUST 将最终文本作为 assistant 消息写入会话
- **THEN** Agent MUST 只调用模型一次

#### Scenario: Model call fails
- **WHEN** Agent 调用模型时返回错误
- **THEN** 结构化入口 MUST 返回原始错误和包含已收集步骤的部分 RunResult
- **THEN** Agent MUST NOT 为失败请求追加 assistant 消息

#### Scenario: Direct Agent call has request correlation
- **WHEN** 调用方直接调用结构化入口或 `Agent.Run()` 且未预先提供 request_id
- **THEN** Agent MUST 为该次运行建立稳定的 `request_id`
- **THEN** 上下文、摘要、模型、工具循环日志和结构化轨迹 MUST 复用该 request_id

#### Scenario: Simple Run delegates structured execution
- **WHEN** 调用方使用现有 `Agent.Run(ctx, input)`
- **THEN** Agent MUST 通过结构化运行入口执行完整请求
- **THEN** 成功时 `Run()` MUST 只返回 RunResult 中的最终文本
- **THEN** 失败时 `Run()` MUST 返回与结构化入口相同的错误

#### Scenario: Structured and simple entry preserve session behavior
- **WHEN** 相同 Agent 分别通过结构化入口或简单 `Run()` 完成请求
- **THEN** 两个入口的用户消息写入、最终 assistant 写回、工具执行和失败不写回语义 MUST 一致
