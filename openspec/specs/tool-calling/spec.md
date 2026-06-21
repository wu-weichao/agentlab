## Purpose

定义工具注册与发现、结构化调用协议、原生和文本兼容模式、顺序多步工具循环、请求内结果缓存、安全执行边界以及完整学习诊断日志行为。
## Requirements
### Requirement: Define a structured tool calling protocol
系统 MUST 使用统一结构表示工具调用和工具执行结果，使 Agent、工具执行器和模型回填流程之间的契约清晰可测试。

#### Scenario: Model requests a tool call
- **WHEN** 模型判断需要调用工具
- **THEN** 模型输出必须能够被解析为包含 `tool_name`、`arguments` 和 `reason` 的结构化 `tool_call`
- **THEN** `tool_name` 必须匹配已注册工具名称

#### Scenario: Tool execution returns a structured result
- **WHEN** 工具执行完成
- **THEN** 系统必须返回包含 `success`、`content`、`error` 和 `metadata` 的结构化 `tool_result`
- **THEN** 成功结果必须设置 `success=true` 并提供可回填给模型的 `content`
- **THEN** 失败结果必须设置 `success=false` 并提供明确 `error`

### Requirement: Expose registered tools in system prompt
系统 MUST 根据工具调用模式向模型暴露已注册工具，并确保工具能力在会话 reset 后继续可用。

#### Scenario: Build native tool behavior instructions at startup
- **WHEN** Agent 使用 `native` 模式和工具执行器初始化
- **THEN** system prompt MUST 包含顺序工具使用、避免重复调用和最终回答输出约束
- **THEN** system prompt MUST NOT 包含要求模型输出工具 JSON 的格式说明
- **THEN** 具体工具名称、描述和参数 MUST 通过每次模型请求的原生工具定义传递

#### Scenario: Build text compatibility instructions at startup
- **WHEN** Agent 使用 `text_compat` 模式和工具执行器初始化
- **THEN** 系统必须基于已注册工具生成包含工具名称、描述、参数和调用 JSON 格式的工具说明
- **THEN** 系统必须将该工具说明追加到会话 system prompt

#### Scenario: Preserve mode instructions after clear
- **WHEN** 会话执行 `clear` 或 reset
- **THEN** 系统必须清空摘要和近期消息
- **THEN** system prompt 中当前模式对应的工具行为说明必须继续保留

### Requirement: Register and execute tools through a ToolExecutor
系统 MUST 通过统一 ToolExecutor 注册、查找和执行工具，而不是在 ChatBot 主流程中硬编码具体工具实现。

#### Scenario: Register supported tools
- **WHEN** ChatBot 初始化工具执行器
- **THEN** 系统必须注册 `calculator`、`time`、`file_read` 和 `web_search`
- **THEN** ToolExecutor 必须能够提供 `ToolSpec` 作为模型可见工具定义快照
- **THEN** 系统不得在本版本注册 `file_write`、`ask_user`、`shell_exec`、`http_fetch` 或 memory 工具

#### Scenario: Reject unknown tool
- **WHEN** Agent 请求调用未注册工具
- **THEN** ToolExecutor 必须返回结构化错误
- **THEN** 系统不得尝试执行任何隐式 fallback 操作

#### Scenario: Validate tool arguments before execution
- **WHEN** 工具调用缺少必需参数或参数类型非法
- **THEN** ToolExecutor 或具体工具必须返回结构化参数错误
- **THEN** 错误必须能够回填给 Agent 生成最终说明

### Requirement: Support calculator tool
系统 MUST 提供 `calculator` 工具执行安全的基础数学计算。

#### Scenario: Evaluate a valid expression
- **WHEN** Agent 调用 `calculator` 并传入合法基础数学表达式
- **THEN** 工具必须返回确定性计算结果
- **THEN** 工具结果必须包含表达式和计算结果相关 metadata

#### Scenario: Reject unsafe or invalid expression
- **WHEN** 表达式为空、语法非法、发生除零或包含非数学执行语义
- **THEN** 工具必须返回结构化错误
- **THEN** 系统不得执行任意代码

### Requirement: Support time tool
系统 MUST 提供 `time` 工具查询当前环境时间。

#### Scenario: Return current local time
- **WHEN** Agent 调用 `time`
- **THEN** 工具必须返回当前日期、时间、时区和 Unix 时间戳
- **THEN** 返回内容必须使用稳定格式，便于 Agent 引用

### Requirement: Support file_read tool with workspace boundary
系统 MUST 提供 `file_read` 工具读取工作区内允许范围的文本文件。

#### Scenario: Read allowed text file
- **WHEN** Agent 调用 `file_read` 并传入工作区内允许的文本文件路径
- **THEN** 工具必须返回文件内容
- **THEN** 工具 metadata 必须包含规范化路径和读取字节数

#### Scenario: Reject disallowed path
- **WHEN** 请求路径位于工作区外、`.git/`、`.claude/`、`.codex/`、敏感配置目录或明显密钥文件
- **THEN** 工具必须返回结构化安全错误
- **THEN** 系统不得读取该文件内容

#### Scenario: Reject oversized or non-text file
- **WHEN** 文件超过单次读取大小限制或被判断为非文本文件
- **THEN** 工具必须返回结构化错误
- **THEN** 系统不得把超大或二进制内容注入模型上下文

### Requirement: Support web_search tool with source-aware results
系统 MUST 提供 `web_search` 工具获取外部搜索结果，并以结构化方式回填给 Agent。

#### Scenario: Return bounded search results
- **WHEN** Agent 调用 `web_search` 并传入查询词
- **THEN** 工具必须返回数量受限的结果列表
- **THEN** 每条结果必须尽量包含标题、摘要、URL、来源和时间信息

#### Scenario: Search dependency unavailable
- **WHEN** 搜索客户端未配置或外部搜索失败
- **THEN** 工具必须返回结构化错误
- **THEN** Agent 不得编造搜索结果

#### Scenario: Final answer uses source awareness
- **WHEN** Agent 基于 `web_search` 结果生成最终回答
- **THEN** 最终回答必须体现结果来自搜索工具
- **THEN** 系统不得把搜索摘要直接视为未经验证的绝对事实

### Requirement: Complete one tool call loop per user turn
系统 MUST 支持单轮对话中的受控顺序多步工具调用闭环，并根据当前模式使用原生工具消息或文本兼容消息回填结果。

#### Scenario: Complete one native tool call successfully
- **WHEN** native 模式下模型响应包含一个合法结构化工具调用
- **THEN** Agent 必须执行该工具
- **THEN** Agent 必须保留 assistant tool call 并追加关联 tool call ID 的 tool result 消息
- **THEN** 如果下一次模型响应为普通文本，Agent 必须将其保存为最终 assistant 回答
- **THEN** 成功后会话历史必须只包含用户输入和最终 assistant 回答，不包含内部工具轨迹

#### Scenario: Complete sequential native tool calls successfully
- **WHEN** 模型在同一次用户请求内先后返回多个顺序的单个原生工具调用
- **THEN** Agent 必须按步骤逐个执行工具
- **THEN** 每一步最多执行一个工具调用
- **THEN** Agent 必须把每次原生 tool result 追加到本轮工作消息后继续下一步
- **THEN** 当模型返回普通文本时 Agent 必须保存该文本作为最终 assistant 回答

#### Scenario: Tool execution fails safely
- **WHEN** 工具执行返回失败结果
- **THEN** Agent 必须将失败结果通过当前模式对应的受控工具反馈交给模型
- **THEN** 系统不得中断进程或追加伪造的工具成功结果

#### Scenario: Continue after tool result when another tool is needed
- **WHEN** 工具结果回填后的下一次模型响应仍请求一个合法工具调用
- **THEN** 系统必须在未超过 `MaxSteps` 时继续执行该工具调用
- **THEN** 系统必须继续使用同一次请求内的 runCache

#### Scenario: Multiple native tool calls are unsupported
- **WHEN** native 模式下模型一次响应中请求多个工具调用
- **THEN** 系统必须返回明确的不支持错误
- **THEN** 系统不得执行其中任何工具

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

### Requirement: Reuse run cache across the full tool loop
系统 MUST 在完整 `ChatBot.Send()` 工具循环中复用同一个 run-level 工具结果缓存，使相同工具和相同标准化参数在同一次请求内只真实执行一次。

#### Scenario: Duplicate call in same tool loop
- **WHEN** 模型在同一次 `Send()` 的多个步骤中重复请求相同 `tool_name` 和相同标准化 `arguments`
- **THEN** 系统 MUST 复用第一次执行产生的 `ToolResult`
- **THEN** 系统 MUST NOT 再次调用真实工具实现

#### Scenario: Different call in same tool loop
- **WHEN** 模型在同一次 `Send()` 中请求相同工具但参数不同
- **THEN** 系统 MUST 使用不同 run cache entry
- **THEN** 系统 MUST 按正常工具执行路径处理该调用

#### Scenario: Cache discarded after send
- **WHEN** 当前 `Send()` 处理结束
- **THEN** 系统 MUST 丢弃本次工具循环的 run cache
- **THEN** 后续用户请求 MUST NOT 复用上一轮请求的缓存结果

#### Scenario: Log cache miss and store
- **WHEN** 工具调用未命中当前 run cache 并完成真实执行
- **THEN** 系统 MUST 记录 `tool_cache_hit=false`
- **THEN** 系统 MUST 在写入缓存后记录 `tool_cache_store=true`
- **THEN** 日志 MUST 包含工具名、参数 hash 和当前 cache entry 数量
- **THEN** 日志 MUST NOT 输出完整工具参数

#### Scenario: Log cache hit
- **WHEN** 工具调用命中当前 run cache
- **THEN** 系统 MUST 记录 `tool_cache_hit=true`
- **THEN** 系统 MUST 包含工具名、参数 hash 和当前 cache entry 数量
- **THEN** 系统 MUST NOT 再次调用真实工具实现

### Requirement: Guide model behavior for sequential tool use
系统 MUST 在模型可见工具说明中表达顺序工具使用边界，使模型每次回复最多请求一个工具，并在已有结果足够时生成最终回答。

#### Scenario: Tool instructions allow sequential steps
- **WHEN** ChatBot 基于已注册工具生成 system prompt 工具说明
- **THEN** 工具说明 MUST 声明每次回复最多请求一个工具
- **THEN** 工具说明 MUST 声明只有当已有工具结果不足以回答用户问题时才能继续请求另一个工具

#### Scenario: Tool instructions discourage duplicate calls
- **WHEN** ChatBot 生成工具说明
- **THEN** 工具说明 MUST 要求模型不要重复请求相同工具和相同参数
- **THEN** 工具说明 MUST 要求已有工具结果足以回答时直接输出最终回答

### Requirement: Require final answers in assistant content
系统 MUST 要求模型把最终用户可见回答输出到 assistant message 的 `content` 字段，并且不得依赖从 reasoning 字段做语言相关文本提取。

#### Scenario: Prompt requires content output
- **WHEN** ChatBot 生成 system prompt 工具说明
- **THEN** 工具说明 MUST 要求即使模型内部进行 thinking 或 reasoning，最终给用户看的回答也必须输出在 assistant message 的 `content` 中
- **THEN** 工具说明 MUST 要求最终回答不能只输出在 `reasoning_content` 或 thinking 字段中

#### Scenario: Reasoning-only response is rejected
- **WHEN** OpenAI 兼容响应中的 assistant `content` 为空且 `reasoning_content` 非空
- **THEN** 系统 MUST 返回明确错误说明响应只包含 reasoning 内容
- **THEN** 系统 MUST NOT 通过自然语言 marker 从 `reasoning_content` 提取最终回答

### Requirement: Construct ChatBot with options
系统 MUST 使用单一 `ChatBotOptions` 构造入口初始化 ChatBot，避免多个构造函数或过长位置参数导致初始化路径分散。

#### Scenario: Create ChatBot with default optional values
- **WHEN** 调用方通过 `ChatBotOptions` 创建 ChatBot 且未提供 `Executor`
- **THEN** 系统 MUST 使用当前工作区默认工具执行器
- **THEN** 系统 MUST 在 `ToolLoopOptions.MaxSteps` 小于等于 0 时使用默认最大步数 `3`

#### Scenario: Create ChatBot with custom executor
- **WHEN** 测试或后续接入方通过 `ChatBotOptions.Executor` 提供自定义工具执行器
- **THEN** 系统 MUST 使用该执行器生成工具说明并执行工具
- **THEN** 系统 MUST NOT 通过其他 ChatBot 构造函数初始化

### Requirement: Build stable tool call keys
系统 MUST 为每次结构化工具调用构造稳定的 `ToolCallKey`，用于在 Runtime 内识别相同工具和相同参数的重复调用。

#### Scenario: Same arguments with different field order
- **WHEN** 两个工具调用使用相同 `tool_name`，且 `arguments` 仅 JSON 字段顺序不同
- **THEN** 系统 MUST 为它们生成相同的 `ToolCallKey`

#### Scenario: Different tools with same arguments
- **WHEN** 两个工具调用使用不同 `tool_name`，即使 `arguments` 完全相同
- **THEN** 系统 MUST 为它们生成不同的 `ToolCallKey`

#### Scenario: Nested arguments are normalized
- **WHEN** 工具调用参数包含嵌套对象
- **THEN** 系统 MUST 对嵌套对象字段顺序进行稳定标准化
- **THEN** 语义相同但字段顺序不同的嵌套参数 MUST 生成相同的 `ToolCallKey`

### Requirement: Provide run-level tool result cache infrastructure
系统 MUST 提供一次请求作用域内可使用的 run-level 工具结果缓存基础设施，并能够复用相同 `ToolCallKey` 对应的工具执行结果。

#### Scenario: Duplicate tool call through execution wrapper
- **WHEN** 工具执行包装逻辑在同一个 run cache 中收到相同 `tool_name` 和相同标准化 `arguments` 的工具调用
- **THEN** 系统 MUST 复用该 `ToolCallKey` 已缓存的工具结果
- **THEN** 系统 MUST 不要求工具接口声明 TTL 或跨轮缓存策略

#### Scenario: Different arguments use different cache entries
- **WHEN** 工具执行包装逻辑收到相同工具但参数不同的工具调用
- **THEN** 系统 MUST 将它们视为不同工具调用
- **THEN** 系统 MUST 使用不同 run cache entry

#### Scenario: Run cache is scoped to current request
- **WHEN** 一次用户请求处理结束
- **THEN** 系统 MUST 丢弃本次请求的 run-level 工具调用缓存
- **THEN** 后续新的用户请求 MUST 不依赖上一轮请求的 run-level 缓存结果

### Requirement: Preserve current tool calling boundaries
系统 MUST 在引入 provider 原生 Tool Calling 时保持现有工具集合、执行边界、缓存生命周期和文本兼容能力。

#### Scenario: Existing text tool call loop remains available
- **WHEN** 系统配置为 `text_compat` 且模型返回合法单个文本 `tool_call`
- **THEN** Agent MUST 继续执行该工具并回填文本协议 `tool_result`
- **THEN** Agent MUST 继续基于最终模型响应保存 assistant 回答

#### Scenario: Native tool result is returned with call correlation
- **WHEN** Agent 已执行 native 模式中的合法工具调用
- **THEN** 下一次模型请求 MUST 包含原 assistant tool call
- **THEN** 下一次模型请求 MUST 包含使用相同 tool call ID 的 tool result 消息

#### Scenario: No cross-turn cache is introduced
- **WHEN** 用户在后续对话轮次中再次请求相同工具和相同参数
- **THEN** 系统 MUST 不复用上一轮请求的工具结果
- **THEN** 是否执行工具 MUST 继续遵循当前工具调用循环行为

#### Scenario: Tool interface remains unchanged
- **WHEN** 本变更完成
- **THEN** 现有工具 MUST 继续通过当前 `Tool` 接口注册和执行
- **THEN** 系统 MUST 不要求工具实现 TTL、`ToolMetadata` 或跨轮缓存策略

### Requirement: Provide full diagnostic logs for tool loop debugging
系统 MUST 在本地学习日志中记录足够且完整的诊断信息，用于确认工具调用参数、工具执行结果和 LLM 请求上下文是否按预期传递。

#### Scenario: Tool execution log includes complete ToolResult
- **WHEN** Agent 完成一次工具执行
- **THEN** 日志 MUST 包含工具名、执行成功状态和错误文本
- **THEN** 日志 MUST 包含完整单行 `tool_result_json`
- **THEN** `tool_result_json` MUST 包含完整 `success`、`content`、`error` 和 `metadata`

#### Scenario: Tool call log includes complete arguments
- **WHEN** Agent 识别到合法工具调用
- **THEN** 日志 MUST 包含完整单行 `tool_call_json`
- **THEN** `tool_call_json` MUST 包含调用 ID、工具名称、完整 arguments 和 reason

#### Scenario: LLM request and response logs include full bodies
- **WHEN** OpenAI 兼容客户端发送聊天补全请求并收到响应
- **THEN** 请求日志 MUST 包含实际发送的完整 JSON `request_body`
- **THEN** 响应日志 MUST 包含 provider 返回的完整 `response_body`
- **THEN** 日志 MUST NOT 输出 Authorization header 或 API Key

### Requirement: Support provider-native tool calling
系统 MUST 在 `native` 模式下使用 provider 原生工具定义、结构化工具调用和工具结果消息完成 Tool Calling，同时保持仓库内部统一工具协议。

#### Scenario: Expose registered tools through native definitions
- **WHEN** Agent 在 `native` 模式下调用模型
- **THEN** 请求 MUST 包含当前 Executor 已注册工具的名称、描述和参数 schema
- **THEN** system prompt MUST NOT 要求模型输出 JSON 文本工具调用

#### Scenario: Convert provider call to unified ToolCall
- **WHEN** provider 返回一个合法原生工具调用
- **THEN** 系统 MUST 将调用 ID、工具名称和参数转换为统一 `ToolCall`
- **THEN** ToolExecutor MUST 继续通过统一 `ToolCall` 执行工具

#### Scenario: Keep tool call ID out of cache key
- **WHEN** 两个原生工具调用的 ID 不同但工具名和标准化参数相同
- **THEN** 系统 MUST 为它们生成相同 `ToolCallKey`
- **THEN** 同一次 Run 中第二个调用 MUST 能够复用 runCache

### Requirement: Preserve text tool calling compatibility
系统 MUST 提供显式 `text_compat` 模式，保留现有 JSON 文本工具调用能力。

#### Scenario: Parse text compatibility tool call
- **WHEN** 系统处于 `text_compat` 模式且模型在 assistant content 中返回合法 JSON 工具调用
- **THEN** Agent MUST 使用现有文本解析逻辑识别并执行该调用

#### Scenario: Use text compatibility feedback
- **WHEN** `text_compat` 模式完成一次工具执行
- **THEN** Agent MUST 使用现有 assistant 意图消息和 user 工具结果消息回填下一次请求
- **THEN** 工具轨迹 MUST NOT 写入 Session

#### Scenario: Do not silently fall back
- **WHEN** `native` 模式请求失败或 provider 返回非法原生工具调用
- **THEN** 系统 MUST 返回明确错误
- **THEN** 系统 MUST NOT 自动重试为 `text_compat`

