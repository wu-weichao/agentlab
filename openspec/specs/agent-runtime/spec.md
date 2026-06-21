## Purpose

定义轻量 Agent Runtime 对模型调用、上下文治理、工具循环、会话写回和兼容入口的统一编排行为。
## Requirements
### Requirement: Provide a lightweight Agent Runtime
系统 MUST 提供 `internal/agent` 中的轻量 Agent Runtime，并通过单一构造入口统一持有完成一轮 Agent 执行所需的模型客户端、会话、工具执行器、上下文组件和运行选项。

#### Scenario: Construct Agent with explicit dependencies
- **WHEN** 调用方提供 `llm.Client`、`session.Session`、上下文配置、自定义 `tools.Executor` 和工具循环选项
- **THEN** Agent MUST 使用这些依赖执行后续请求
- **THEN** Agent MUST NOT 要求调用方通过 app 或 CLI 层才能运行

#### Scenario: Construct Agent with default executor and tool loop options
- **WHEN** 调用方未提供工具执行器或提供的最大工具步数小于等于 0
- **THEN** Agent MUST 使用当前工作区默认工具执行器
- **THEN** Agent MUST 使用默认最大工具步数 `3`

### Requirement: Run a complete conversational turn through Agent
系统 MUST 提供 `Agent.Run(ctx, input)`，独立完成从用户输入、受控上下文构建和模型调用到最终 assistant 回答写回的 Runtime 编排。

#### Scenario: Complete ordinary chat without tools
- **WHEN** Agent 收到用户输入且模型第一次响应为普通 assistant 文本
- **THEN** Agent MUST 在模型调用前将用户输入写入会话并构建受预算约束的上下文
- **THEN** Agent MUST 返回模型文本并将其作为最终 assistant 消息写入会话
- **THEN** Agent MUST 只调用模型一次

#### Scenario: Model call fails
- **WHEN** Agent 调用模型时返回错误
- **THEN** Agent MUST 将错误返回给调用方
- **THEN** Agent MUST NOT 为失败请求追加 assistant 消息

#### Scenario: Direct Agent call has request correlation
- **WHEN** 调用方直接调用 `Agent.Run()` 且未预先提供 request_id
- **THEN** Agent MUST 为该次运行建立稳定的 `request_id`
- **THEN** 上下文、摘要、模型和工具循环日志 MUST 复用该 request_id

### Requirement: Own context preparation and summary updates
Agent Runtime MUST 负责调用现有上下文治理组件，完成近期消息裁剪、滚动摘要更新和必要的摘要再压缩，并保持既有上下文预算语义。

#### Scenario: Evicted history updates rolling summary
- **WHEN** 本轮上下文构建因预算超限移出旧消息且启用了滚动摘要
- **THEN** Agent MUST 在主模型请求前基于旧摘要和被移出消息更新单份滚动摘要
- **THEN** Agent MUST 使用更新后的摘要和裁剪后的近期消息重新构建请求

#### Scenario: Existing summary requires compression
- **WHEN** 最小近期上下文与现有滚动摘要组合后仍超出预算且存在剩余摘要预算
- **THEN** Agent MUST 压缩现有摘要并重试一次上下文构建
- **THEN** 重试后仍超预算时 Agent MUST 返回明确的上下文预算错误

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

### Requirement: Initialize stable tool instructions in Agent
Agent MUST 在初始化阶段基于实际 Executor 生成工具说明并固化到 Session system prompt，使直接使用 Agent 时具备与现有 ChatBot 相同的工具发现能力。

#### Scenario: Build tool instructions once
- **WHEN** Agent 使用包含已注册工具的 Executor 完成构造
- **THEN** Session system prompt MUST 包含工具名称、参数、调用格式和顺序工具约束
- **THEN** 后续每次 `Agent.Run()` MUST NOT 重复追加工具说明

#### Scenario: Reset preserves tool instructions
- **WHEN** 调用方对 Agent 使用的 Session 执行 reset
- **THEN** Session MUST 清空滚动摘要和近期消息
- **THEN** 初始化时固化的工具说明 MUST 继续保留

### Requirement: Preserve ChatBot compatibility through delegation
系统 MUST 保留 app 层 ChatBot 兼容入口，并使其委托 Agent Runtime，而不是继续实现独立的上下文和工具循环策略。

#### Scenario: ChatBot Send delegates compatible behavior
- **WHEN** 现有调用方通过 `app.NewChatBot(ChatBotOptions)` 创建 ChatBot 并调用 `Send()`
- **THEN** ChatBot MUST 通过内部 Agent 执行该请求
- **THEN** 返回值、错误、会话写回、工具行为和最大步数语义 MUST 与迁移前兼容

#### Scenario: CLI behavior remains unchanged
- **WHEN** 用户通过现有 `chat run` 命令进行普通对话、clear、history 或退出操作
- **THEN** 用户 MUST NOT 需要修改命令、配置或交互方式
- **THEN** CLI MUST 继续观察与 Agent 共享的同一个 Session 状态

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

