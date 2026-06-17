## ADDED Requirements

### Requirement: Configure bounded tool loop execution
系统 MUST 为同一次用户请求内的工具循环提供 `ToolLoopOptions{MaxSteps int}`，并使用最大步数防止无限工具调用。

#### Scenario: Use default max steps
- **WHEN** ChatBot 未显式配置 `MaxSteps` 或配置值小于等于 0
- **THEN** 系统 MUST 使用默认最大步数 `3`

#### Scenario: Stop when max steps exceeded
- **WHEN** 模型在达到 `MaxSteps` 后仍未生成普通最终回答
- **THEN** 系统 MUST 返回 `ErrToolLoopExceeded`
- **THEN** 系统 MUST NOT 向会话历史追加 assistant 消息

#### Scenario: No tool request uses one model call
- **WHEN** 模型第一次响应为普通 assistant 文本
- **THEN** 系统 MUST 将该文本保存为最终 assistant 回答
- **THEN** 系统 MUST NOT 进入额外工具执行步骤

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

## MODIFIED Requirements

### Requirement: Complete one tool call loop per user turn
系统 MUST 支持单轮对话中的受控顺序多步工具调用闭环，并明确避免执行无限递归、并行工具调用或一次响应中的多个工具调用。

#### Scenario: Complete one tool call successfully
- **WHEN** 模型第一阶段响应为合法 `tool_call`
- **THEN** ChatBot 必须执行该工具
- **THEN** ChatBot 必须将 `tool_result` 回填到本轮工作消息
- **THEN** 如果下一次模型响应为普通文本，ChatBot 必须将其保存为最终 assistant 回答
- **THEN** 成功后会话历史必须包含用户输入和最终 assistant 回答

#### Scenario: Complete sequential tool calls successfully
- **WHEN** 模型在同一次用户请求内先后返回多个顺序 `tool_call`
- **THEN** ChatBot 必须按步骤逐个执行工具
- **THEN** 每一步最多执行一个工具调用
- **THEN** ChatBot 必须把每次 `tool_result` 追加到本轮工作消息后继续下一步
- **THEN** 当模型返回普通文本时，ChatBot 必须保存该文本作为最终 assistant 回答

#### Scenario: Tool execution fails safely
- **WHEN** 工具执行返回失败结果
- **THEN** ChatBot 必须将失败结果作为受控上下文交给模型生成说明或下一步决策
- **THEN** 系统不得中断进程或追加伪造的工具成功结果

#### Scenario: Continue after tool result when another tool is needed
- **WHEN** 工具结果回填后的下一次模型响应仍请求一个合法工具调用
- **THEN** 系统必须在未超过 `MaxSteps` 时继续执行该工具调用
- **THEN** 系统必须继续使用同一次请求内的 run cache

#### Scenario: Parallel tool calls are unsupported
- **WHEN** 模型一次响应中请求多个工具调用
- **THEN** 系统必须返回明确的不支持错误
- **THEN** 系统不得并行或顺序执行多个工具调用
