# 项目演进记录

本文档用于记录 AI Agent 学习项目的阶段性演进脉络。详细变更过程见 `openspec/changes/archive/`，这里仅保留每个阶段的高层摘要。

## v0.0.1 mini-chatbot

目标：实现最小可运行的 CLI ChatBot，作为后续 Agent 架构演进的起点。

新增：
- 基于 `spf13/cobra` 的命令行入口
- `system`、`user`、`assistant` 三类消息模型
- 内存会话 `Session`，支持多轮对话、清空上下文和历史查看
- `ChatBot -> LLMClient` 的最小对话编排链路
- OpenAI 兼容客户端实现
- 本地 `configs/config.yaml` 配置加载
- `configs/config.example.yaml` 示例配置文件
- 默认写入 `logs/chat.log` 的文件日志

关键决策：
- 第一版只做 `CLI -> ChatBot -> Session -> LLMClient`
- 不引入 Tool Calling、RAG、Workflow、Multi-Agent
- 运行时只使用本地 `config.yaml`，不使用 `.env` 或环境变量覆盖
- 启动阶段直接拦截占位 API Key，避免进入对话后才鉴权失败
- 调试日志不输出到终端，默认写入日志文件，避免影响交互体验

遗留问题：
- 还没有 Prompt 模板化
- 还没有上下文裁剪与短期记忆压缩
- 还没有 Tool Calling
- 还没有规划、工作流和多 Agent 能力

下一步：
- 从 Prompt 模板化开始
- 或先实现短期记忆裁剪
- 或进入 Tool Calling 能力演进

## v0.0.2 prompt-runtime

目标：把静态 `system prompt` 升级为受配置驱动、可渲染、可校验的 Prompt Runtime，为后续记忆、工具调用和工作流能力打基础。

新增：
- `chat.prompt.template` 模板配置
- `chat.prompt.role` 结构化角色设定
- `chat.prompt.context` 结构化上下文参数
- `internal/prompt` 模块，负责模板变量解析、校验和渲染
- 启动阶段 Prompt 变量完整性校验
- 渲染后的 system prompt 会话初始化与 `clear` 恢复测试

调整：
- 移除旧字段 `chat.system_prompt`
- 会话初始化改为使用渲染后的 system prompt
- `clear` 后恢复本次启动时已渲染的同一份 prompt
- README 和示例配置切换到新 Prompt 配置结构

关键决策：
- 第二版仍然保持 `CLI -> ChatBot -> Session -> LLMClient` 主链路不变
- Prompt 渲染发生在配置加载完成之后、会话创建之前
- 角色设定不是独立系统，而是 Prompt 模板变量的一部分
- 只支持点路径变量和纯替换语义，例如 `{{role.name}}`
- 缺失变量或空值在启动阶段直接报错，不做静默兜底
- 不兼容 `v0.0.1` 的 `chat.system_prompt`

当前边界：
- 只解决 Prompt 模板、角色设定和参数化渲染
- 不支持条件模板、循环、include、多模板管理
- 不支持运行时角色切换
- 不引入 Tool Calling、长期记忆、RAG、Workflow、Multi-Agent

下一步：
- 实现受 `chat.context.*` 控制的短期上下文裁剪
- 在上下文增长后引入更稳的裁剪或摘要策略
- 再进入 Tool Calling 和更完整的 Agent Runtime 演进

## v0.0.3 context-window-control

目标：在现有 Chat Runtime 基础上，引入受预算控制的短期上下文治理能力，通过“最近原文保留 + 单份滚动摘要”解决长对话膨胀问题。

新增：
- `chat.context.max_chars`、`chat.context.keep_recent_turns`、`chat.context.summary_max_chars`、`chat.context.enable_rolling_summary` 配置
- `internal/contextwindow` 模块，负责上下文预算估算、完整轮次裁剪和滚动摘要更新
- 内部 `summary` 语义消息，并在发给模型前渲染为 `system` 风格消息
- 基于 LLM 的结构化滚动摘要生成与二次压缩
- `request_id` 贯穿 `chatbot -> summary -> llm/openai` 的日志链路

调整：
- 移除 `chat.max_history_messages`，上下文控制只使用 `chat.context.*`
- 会话状态从单一消息切片调整为 `systemPrompt + rollingSummary + recentMessages`
- `keep_recent_turns` 从硬保留改为优先保留；超预算时允许继续缩小近期窗口，但至少保留最近 1 轮完整对话和当前用户输入
- 当插入摘要后仍超预算时，继续压缩已有摘要并重试一次上下文构建
- `history` 输出现在能区分 `summary` 与原始消息，`clear` 同时清空摘要和近期历史

关键决策：
- 预算单位首版仍使用字符数估算，不引入 provider 级精确 token 统计
- 裁剪单位按完整 `user + assistant` 轮次进行，不拆半轮
- 滚动摘要使用 LLM 做真实归纳，而不是规则摘录
- 只维护单份滚动摘要，不引入多摘要层级或长期记忆
- 通过 `request_id` 把单次对话中的裁剪、摘要、主请求串成一条可观测链路

当前边界：
- 只解决单会话短期上下文预算、裁剪、摘要和日志可观测性
- 不支持跨会话持久化记忆、RAG、Embedding 检索、多级摘要树
- 不支持用户手动编辑摘要或按重要性排序裁剪
- 仍不引入 Tool Calling、Workflow、Multi-Agent

下一步：
- 评估滚动摘要的质量与成本，决定是否拆出独立 summarizer 配置
- 继续收紧上下文预算策略，例如更精细的最小保留规则和摘要压缩阈值
- 在上下文治理稳定后再进入 Tool Calling 和更完整的 Agent Runtime 演进

## v0.0.4 tool-calling

目标：在现有 Chat Runtime 和上下文治理基础上，引入最小可运行的 Tool Calling 闭环，让模型可以请求一次受控工具调用，并基于工具结果生成最终回答。

新增：
- `internal/tools` 模块，定义工具元信息、`ToolCall`、`ToolResult` 和 `ToolExecutor`
- 启动阶段将工具说明合并进会话 system prompt，使工具能力成为 Agent 的全局能力说明
- `calculator` 工具，支持安全基础数学表达式
- `time` 工具，返回当前日期、时间、时区和 Unix 时间戳
- `file_read` 工具，只读取工作区内允许范围的文本文件
- `web_search` 工具和可替换 `SearchClient`，未配置真实客户端时返回结构化不可用错误
- ChatBot 单轮一次工具调用闭环：模型请求工具、执行器执行、结果回填、模型生成最终文本或由系统兜底生成回答

关键决策：
- 首版只允许单轮一次工具调用，不继续执行递归或并行工具调用
- 工具协议使用仓库内统一 JSON 文本结构，不直接绑定 provider 私有 tool schema
- 当前是“文本协议模拟 Tool Calling”，模型是否输出 `tool_call` 依赖 system prompt 约束，而不是 API 层原生 tool calling 强约束
- 工具说明在 ChatBot 初始化时基于已注册工具生成，并固化到 `Session` 的 system prompt 中，`clear` 后仍然保留
- `file_read` 默认拒绝 `.git/`、`.claude/`、`.codex/`、明显密钥文件、超大文件和非文本内容
- `web_search` 先通过接口隔离外部依赖，避免把项目强绑定到某个搜索供应商
- 如果工具结果回填后的第二次模型响应仍然输出 `tool_call`，系统不再报错或继续执行工具，而是基于已有 `ToolResult` 生成兜底最终回答

工具定义思路：
- 每个工具都实现统一的 `Tool` 接口，提供 `Name`、`Description`、`Parameters` 和 `Execute`
- `Name` 是工具调用标识，必须稳定且唯一，例如 `calculator`、`file_read`
- `Description` 和 `Parameters` 用来描述工具能力和参数边界，后续可用于生成模型可见的工具说明
- `Execute` 只接收结构化参数并返回 `ToolResult`，不直接修改 ChatBot 会话状态
- 工具执行成功和失败都返回统一结构，包含 `success`、`content`、`error` 和 `metadata`
- `ToolExecutor` 负责统一注册、查找、参数兜底和执行分发，ChatBot 只依赖执行器，不直接硬编码具体工具实现
- `ToolSpec` 是工具给模型看的定义快照，只包含名称、描述和参数，不暴露执行实现

工具识别思路：
- ChatBot 启动时基于 `ToolExecutor.Specs()` 生成全局工具说明，并追加到会话 system prompt
- 工具说明作为 Agent 的全局能力长期存在，包含可用工具、参数说明、调用格式和单工具限制，而不是每轮 `Send` 临时追加
- 每轮用户输入的第一次模型调用直接使用包含工具说明的 system prompt，让模型自行判断是否需要输出 `tool_call`
- 模型第一阶段回复可以是普通文本，也可以是基于工具说明生成的结构化 `tool_call`
- `tool_call` 使用仓库内部统一 JSON 协议，至少包含 `tool_name`、`arguments` 和可选 `reason`
- ChatBot 在第一次模型调用后尝试解析工具调用；解析不到时按普通 assistant 回复处理
- 如果识别到多个工具调用，直接返回不支持错误，因为首版只允许单轮一次工具调用
- 解析逻辑兼容纯 JSON、Markdown `json` 代码块、`tool_call` 包裹对象和 `tool_calls` 数组，但不会在解析阶段执行工具
- 这种识别方式可先验证 Tool Calling 闭环，同时保留后续接入 provider 原生 tool schema 的空间

工具调用思路：
- 单轮流程是“用户输入 -> 构建受控上下文 -> 第一次模型调用 -> 识别 tool_call -> ToolExecutor 执行 -> 回填 tool_result -> 第二次模型调用 -> 最终回答”
- 工具结果只追加到本轮第二次模型请求中，不写入 `Session`，避免内部协议污染用户可见历史
- 回填时使用稳定 JSON 文本，让模型明确区分工具内容、错误和 metadata，并追加 `FINAL_ANSWER` 阶段指令
- 工具失败不会直接中断进程，而是以结构化失败结果交给模型生成面向用户的说明；如果模型仍输出工具调用，则由系统基于失败结果兜底说明
- 如果第二次模型响应仍然请求工具调用，系统不继续执行第二个工具，而是基于已有工具结果生成兜底最终回答
- 这个设计优先保证闭环可理解、可测试、可回退，再逐步演进到多工具规划、授权和高风险工具

当前边界：
- 不支持 `file_write`、`ask_user`、`shell_exec`、长期记忆读写、任意 HTTP 抓取和外部插件市场
- 不支持复杂权限审批或交互式授权流程
- 不使用 provider 原生 tool calling，暂时依赖模型遵守 JSON 文本协议，因此需要系统级 fallback 保底
- 搜索结果只作为来源感知的外部信息，不应被直接视为未经验证的绝对事实

下一步：
- 评估是否接入 provider 原生 tool calling，减少文本协议下模型重复请求工具的问题
- 选择并接入真实 `web_search` 供应商实现
- 在单工具闭环稳定后，再设计多工具规划、授权确认和高风险写入/命令类工具

## v0.0.5 tool-call-key-and-run-cache

目标：在单次工具闭环基础上，补齐 Runtime 级工具调用标识和同轮 runCache 基础设施，并增强工具闭环诊断能力，为后续多步 Tool Loop 做准备。

新增：
- `internal/tools/key.go`，定义 `ToolCallKey{Name, ArgsHash}`，用工具名和参数 hash 标识一次工具调用
- `BuildToolCallKey(call ToolCall)`，统一从 `ToolCall` 构造稳定 key，并校验空工具名
- `NormalizeArguments(args map[string]any)`，把工具参数转换为稳定 JSON 字符串
- 参数 hash 使用 SHA-256，避免把完整参数直接塞进 key，同时保持可比较性
- `internal/app/tool_cache.go`，提供 `executeToolWithRunCache` 作为工具执行包装入口
- 当前单工具闭环已统一经过 runCache 执行包装，后续多步 Tool Loop 可复用同一入口
- 工具结果回填阶段使用 `user` 消息承载工具执行结果和 `FINAL_ANSWER` 约束，减少兼容模型在第二次响应中继续输出 `tool_call` 的概率
- 工具执行完成日志输出完整单行 `ToolResult` JSON；OpenAI 兼容客户端发送请求日志输出完整 request body，便于排查工具结果是否正确进入二次请求
- key 构建、参数标准化、执行包装层缓存复用和现有单工具闭环兼容性测试

关键决策：
- 本版本只做基础设施，不改变当前“单轮最多一次工具调用”的用户可见行为
- runCache 生命周期限定在一次请求内，不引入跨轮缓存、TTL 或 `ToolMetadata`
- 参数标准化只做 JSON 结构级归一化，不做字符串大小写、路径或搜索词等业务语义归一化
- 数字参数会做稳定化处理：整数形态的浮点数会归一为整数，`NaN` 和无穷大被拒绝
- map 和嵌套对象依赖 JSON marshal 的稳定 key 排序，数组保持原顺序，因为数组顺序通常有业务含义
- `Tool` 接口保持不变，现有工具不需要声明缓存策略
- 二次工具结果回填不写入 `Session`，只作为本轮第二次模型请求的临时上下文
- 完整请求与工具结果日志服务于本地调试；后续生产化需要再加入日志级别、脱敏和截断策略

ToolCallKey 思路：
- `ToolCallKey.Name` 直接来自规范化后的 `tool_name`
- `ToolCallKey.ArgsHash` 来自标准化参数 JSON 的 SHA-256
- 参数字段顺序不同但内容相同，应生成相同 key
- 工具名不同即使参数相同，也必须生成不同 key
- 空参数和 `nil` 参数都按空 JSON 对象处理
- 不支持的参数类型会返回参数错误，而不是隐式转字符串

runCache 执行思路：
- 每次工具执行前先构造 `ToolCallKey`
- 如果当前 run cache 已包含该 key，则直接复用已缓存的 `ToolResult`
- 如果未命中，则调用 `tools.Executor.Execute()` 执行真实工具，并把结果写入当前 run cache
- 当前 `ChatBot.Send()` 仍只允许单轮一次工具调用，所以 runCache 的端到端收益有限
- 该执行入口是给后续多步 Tool Loop 预留的基础设施，避免到多步阶段再重写工具执行路径

工具结果回填调整：
- 二次请求中先追加一条 `assistant` 消息，保留“模型请求调用某个工具”的语义
- 随后追加一条 `user` 消息，内容包含“工具执行结果如下”“工具调用阶段已经结束”和 `FINAL_ANSWER` 阶段约束
- 该 `user` 消息包含结构化 `ToolResult`，要求模型只能基于结果回答原问题
- 明确禁止再次请求工具调用、禁止输出 `tool_call` JSON、禁止输出 Markdown 代码块
- 如果第二次模型响应仍然输出工具调用，系统继续使用已有 fallback 基于 `ToolResult` 生成最终回答，不执行第二个工具

可观测性增强：
- `工具执行完成` 日志现在包含 `tool_result_json=<json>`，输出完整单行 `ToolResult` JSON 字符串
- `tool_result_json` 使用 `json.Marshal(ToolResult)` 生成，避免多行缩进 JSON 打散日志
- OpenAI 兼容客户端的 `发送请求` 日志输出完整 `request_body=<json>`
- `request_body` 是实际发送到 `/chat/completions` 的 JSON body，包含 model、messages 和 temperature
- 请求日志不输出 Authorization header
- 这些日志可以直接用于确认第二次请求是否包含工具结果、`FINAL_ANSWER` 约束以及完整上下文

测试覆盖：
- 参数字段顺序不同但语义相同，生成相同 `ToolCallKey`
- 工具名不同但参数相同，生成不同 `ToolCallKey`
- 空参数、嵌套参数和数字参数标准化
- runCache 命中时不重复执行工具
- calculator、time、file_read、web_search 的单工具闭环测试继续通过
- 第二次模型响应仍返回 `tool_call` 时，系统继续使用 fallback 回答

当前边界：
- 不支持多步 Tool Loop
- 不支持跨轮工具结果缓存
- 不支持 provider 原生 tool calling
- 不把工具轨迹写入 `history`
- 不在缓存结果中追加 `cache_hit`、执行耗时或命中来源等 metadata
- 不按工具类型区分可缓存性；当前缓存只在一次请求内有效，因此暂不需要 TTL 和副作用策略
- 完整日志可能包含用户输入、文件内容或搜索结果，只适合当前本地学习项目的调试场景

下一步：
- 引入 `MaxSteps` 和多步 Tool Loop，让 runCache 在完整 `ChatBot.Send()` 流程中发挥端到端去重作用
- 在多步 Tool Loop 中为缓存命中补充可观测 metadata，例如 `cache_hit=true`
- 评估日志级别、脱敏和截断策略，避免完整 request body 在生产化场景中过度暴露上下文
- 继续评估 provider 原生 tool calling，减少文本协议下模型重复请求工具的问题

## v0.0.6 max-steps-tool-loop

目标：把当前“单轮最多一次工具调用”升级为“同一次用户请求内允许多个顺序工具步骤”，并通过 `MaxSteps` 防止模型在工具调用阶段无限循环。

新增：
- `ToolLoopOptions{MaxSteps int}` 和默认最大步数 `3`
- `ErrToolLoopExceeded`，用于在达到最大循环步数仍未生成最终回答时返回明确错误
- `runToolLoop()`，统一编排模型响应解析、工具执行、工具结果回填和下一步模型调用
- 同一次 `ChatBot.Send()` 内共享 runCache，使相同工具和相同标准化参数只真实执行一次
- `ChatBotOptions`，将 ChatBot 初始化依赖和可选项收敛到单一构造入口
- runCache 命中日志，输出 `tool_cache_hit`、`tool_cache_store`、`args_hash` 和缓存条目数
- 多步工具链测试，覆盖 `tool -> tool -> final answer`
- 重复工具调用缓存命中、跨轮缓存不复用、步数超限和多工具调用拒绝测试
- LLM 输出约束：允许 thinking/reasoning，但最终用户可见回答必须进入 assistant `content`

实现思路：
- `ChatBot.Send()` 仍先写入用户消息并构建受控上下文，然后为本次请求创建 runCache
- `runToolLoop()` 使用显式 `for step := 1; step <= MaxSteps; step++` 循环调用模型
- 模型返回普通文本时视为最终回答并写入会话历史
- 模型返回单个合法 `tool_call` 时，通过 `executeToolWithRunCache()` 执行或复用工具结果，再把工具反馈追加到本轮工作消息
- 模型返回多个工具调用时立即返回 `ErrMultipleToolCalls`，不执行任何工具
- 达到 `MaxSteps` 仍没有最终文本时返回 `ErrToolLoopExceeded`，不追加 assistant 消息
- `executeToolWithRunCache()` 是唯一缓存观测入口，缓存未命中、写入和命中都会记录日志
- OpenAI 兼容客户端只接受 `content` 作为有效 assistant 回答；如果只返回 `reasoning_content`，系统返回明确错误，不做文字匹配提取

可观测性：
- 工具执行日志继续输出工具名、成功状态、错误文本和完整单行 `ToolResult` JSON
- 工具缓存日志可直接观察 `tool_cache_hit=true/false` 和 `tool_cache_store=true`
- 缓存日志只输出参数 hash，不输出完整工具参数，降低上下文泄露风险
- 多步循环日志包含当前 step 和最大步数，便于定位是哪一步触发工具调用或失败
- OpenAI 兼容客户端仍保留完整 request body 日志，便于确认工具结果是否进入下一次请求

当前边界：
- `MaxSteps` 暂不接入配置文件，首版固定默认值为 `3`
- 不支持并行工具调用，也不支持一次响应中的多个工具调用
- 不引入跨轮工具缓存、TTL 或副作用工具缓存策略
- 不把完整工具轨迹写入 `history`，工具反馈只存在于本轮工作消息
- 仍使用文本协议模拟 Tool Calling，不接入 provider 原生 tool calling
- 不从 `reasoning_content` 中按自然语言 marker 提取最终回答；字段归属应通过 prompt 约束或 provider 参数解决

下一步：
- 评估是否把 `MaxSteps` 暴露到 `configs/config.yaml`
- 为 runCache 日志继续补充 step index、耗时等可观测 metadata
- 继续评估日志脱敏与截断策略，避免完整 request body 在生产化场景中过度暴露上下文
- 评估具体 provider 是否支持保留 reasoning 的同时稳定返回 assistant `content` 的请求参数
- 在多步工具循环稳定后，再设计授权确认和高风险写入/命令类工具

## v0.0.7 tool-loop-config-observability

目标：让 v0.0.6 的多步 Tool Loop 可配置、可解释、可调试，同时不改变工具调用协议、runCache 生命周期或工具执行语义。

新增：
- `tools.max_steps` 配置，缺失时默认 `3`
- 启动阶段校验 `tools.max_steps`，小于 `1` 或大于 `10` 直接报错
- CLI 初始化 ChatBot 时将配置值注入 `ToolLoopOptions.MaxSteps`
- 工具执行包装路径返回 `cache_hit` 和稳定 `tool_call_key`
- 每次工具调用返回前向 `ToolResult.Metadata` 写入 `step`、`cache_hit` 和 `tool_call_key`
- 工具循环 step 结构化日志，包含 `request_id`、`step`、`max_steps`、`tool_name`、`tool_call_key`、`cache_hit`、`success` 和 `error_code`
- 工具执行完成日志包含脱敏后的 `tool_result_json`，保留状态、错误、metadata 和 `content_chars`，不输出完整 `content`
- README 和示例配置补充工具循环配置、边界和可观测性说明

实现思路：
- 配置字段放在顶层 `tools.max_steps`，继续兼容已有 `tools.web_search`
- 配置加载阶段负责默认值填充和范围校验，避免运行时静默纠正误配置
- `runToolLoop()` 负责 step 级日志，因为它同时知道 request、step、maxSteps、工具调用和工具结果
- runCache 命中时不重复执行真实工具，但返回前会更新当前步骤的 `step` 和 `cache_hit=true`
- 工具结果 metadata 保留原有业务字段，只覆盖运行时调试字段

可观测性：
- step 日志能直接判断第几步触发工具、是否缓存命中、工具是否成功和失败分类
- `tool_call_key` 使用工具名和参数 hash 定位重复调用，不输出完整参数
- `tool_result_json` 继续提供工具完成结果摘要，但通过空 `content` 和 `content_chars` 避免泄露文件正文或搜索结果全文
- 新增 step 日志不输出完整文件内容、搜索结果全文或明显敏感内容
- 工具反馈仍会回填给模型，但不写入用户可见 `history`

当前边界：
- 不新增工具能力
- 不支持并行工具调用或一次响应中的多个工具调用
- 不引入跨轮缓存、TTL 或副作用工具缓存策略
- 不引入 OpenTelemetry、指标面板或 UI 工具轨迹展示
- 仍使用文本协议模拟 Tool Calling，不接入 provider 原生 tool calling

下一步：
- 评估是否按工具类型声明更细粒度的可缓存性和脱敏策略
- 评估是否将工具循环轨迹写入独立调试文件，而不是进入会话历史
- 继续设计授权确认和高风险写入/命令类工具的安全边界

## v0.0.8 agent-runtime-lite

目标：将已经膨胀的 ChatBot Runtime 职责抽取为轻量 `Agent`，建立后续 ToolCache、Memory 和 Workflow 演进所需的稳定运行时边界，同时保持现有 CLI 和用户可见行为兼容。

新增：
- `internal/agent` 包，提供 `Agent`、`Options` 和 `Agent.Run(ctx, input)`
- Agent 独立持有 LLM 客户端、Session、上下文 Builder、Summarizer、工具 Executor 和工具循环选项
- Agent 层普通聊天、上下文摘要、单工具、多步工具、请求内去重、失败路径和日志脱敏测试
- app 层 ChatBot 委托测试，覆盖构造参数映射、成功写回、错误传播和最大步数兼容

实现思路：
- `Agent.Run()` 统一建立 request_id、写入用户输入、准备受控上下文、创建本轮 runCache、执行工具循环并写入最终 assistant 回答
- 上下文裁剪、滚动摘要更新和摘要再压缩整体迁移到 Agent，继续复用现有 `contextwindow` 与 `session` 模块
- 工具说明在 `agent.New()` 阶段基于实际 Executor 生成并固化到 Session system prompt，单次 Run 不重复追加
- 工具循环、ToolCallKey 去重、结果回填和 MaxSteps 控制迁移到 Agent；runCache 仍只存在于单次 Run 内
- `internal/app.ChatBot` 收敛为兼容适配器，保留 `ChatBotOptions` 和 `Send()`，内部委托 `Agent.Run()`
- app 层通过类型和错误别名保留原有 ToolLoop 配置入口，CLI 无需改变组装和调用方式

可观测性：
- 直接调用 `Agent.Run()` 也会自动创建 request_id，并贯穿摘要、LLM 和工具循环日志
- 工具循环继续记录 step、max_steps、tool_name、tool_call_key、cache_hit、success 和 error_code
- 工具结果日志继续只保留状态、metadata 和 content_chars，不输出完整文件内容、搜索结果或工具参数
- Runtime 日志组件标识从 chatbot 调整为 agent，结构化诊断字段保持兼容

当前边界：
- 不引入跨轮 ToolCache、TTL、缓存持久化或副作用工具缓存策略
- 不修改 Tool、LLM Client、Session 或配置文件契约
- 不新增工具，不接入 provider 原生 Tool Calling
- 不实现 Planner、Workflow、Memory、RAG 或 Multi-Agent
- ChatBot 兼容层暂时保留，CLI 仍通过 `ChatBot.Send()` 使用 Agent Runtime
- Session 仍面向当前单线程 CLI 使用，不声明并发安全

下一步：
- 评估 CLI 是否逐步直接依赖 Agent，并明确 ChatBot 兼容层的废弃计划
- 在 Agent Runtime 边界上设计跨轮 ToolCache 的生命周期、TTL 和副作用工具策略
- 当出现第二种上下文或工具循环实现时，再评估是否提炼窄接口
- 继续设计高风险工具授权、Memory 和 Workflow 能力
