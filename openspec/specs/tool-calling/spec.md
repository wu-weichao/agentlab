## ADDED Requirements

### Requirement: Define a structured tool calling protocol
系统必须使用统一结构表示工具调用和工具执行结果，使 Agent、工具执行器和模型回填流程之间的契约清晰可测试。

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
系统必须在 ChatBot 启动阶段将已注册工具的模型可见说明合并进会话 system prompt。

#### Scenario: Build tool instructions at startup
- **WHEN** ChatBot 使用工具执行器初始化
- **THEN** 系统必须基于已注册工具生成包含工具名称、描述、参数和调用 JSON 格式的工具说明
- **THEN** 系统必须将该工具说明追加到会话 system prompt
- **THEN** 系统不得在每次 `Send` 时重新生成并临时追加工具说明

#### Scenario: Preserve tool instructions after clear
- **WHEN** 会话执行 `clear` 或 reset
- **THEN** 系统必须清空摘要和近期消息
- **THEN** system prompt 中的工具说明必须继续保留

### Requirement: Register and execute tools through a ToolExecutor
系统必须通过统一 ToolExecutor 注册、查找和执行工具，而不是在 ChatBot 主流程中硬编码具体工具实现。

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
系统必须提供 `calculator` 工具执行安全的基础数学计算。

#### Scenario: Evaluate a valid expression
- **WHEN** Agent 调用 `calculator` 并传入合法基础数学表达式
- **THEN** 工具必须返回确定性计算结果
- **THEN** 工具结果必须包含表达式和计算结果相关 metadata

#### Scenario: Reject unsafe or invalid expression
- **WHEN** 表达式为空、语法非法、发生除零或包含非数学执行语义
- **THEN** 工具必须返回结构化错误
- **THEN** 系统不得执行任意代码

### Requirement: Support time tool
系统必须提供 `time` 工具查询当前环境时间。

#### Scenario: Return current local time
- **WHEN** Agent 调用 `time`
- **THEN** 工具必须返回当前日期、时间、时区和 Unix 时间戳
- **THEN** 返回内容必须使用稳定格式，便于 Agent 引用

### Requirement: Support file_read tool with workspace boundary
系统必须提供 `file_read` 工具读取工作区内允许范围的文本文件。

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
系统必须提供 `web_search` 工具获取外部搜索结果，并以结构化方式回填给 Agent。

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
系统必须支持单轮对话中的一次工具调用闭环，并明确避免执行递归或并行工具调用。

#### Scenario: Complete one tool call successfully
- **WHEN** 模型第一阶段响应为合法 `tool_call`
- **THEN** ChatBot 必须执行该工具
- **THEN** ChatBot 必须将 `tool_result` 回填给模型生成最终文本回答
- **THEN** 成功后会话历史必须包含用户输入和最终 assistant 回答

#### Scenario: Tool execution fails safely
- **WHEN** 工具执行返回失败结果
- **THEN** ChatBot 必须将失败结果作为受控上下文交给模型生成说明
- **THEN** 系统不得中断进程或追加伪造的工具成功结果

#### Scenario: Handle recursive tool request after result
- **WHEN** 工具结果回填后的第二次模型响应仍请求工具调用
- **THEN** 系统不得继续执行第二个工具调用
- **THEN** 系统必须基于已有 `tool_result` 生成兜底最终回答

#### Scenario: Parallel tool calls are unsupported
- **WHEN** 模型一次响应中请求多个工具调用
- **THEN** 系统必须返回明确的不支持错误
- **THEN** 系统不得并行或顺序执行多个工具调用
