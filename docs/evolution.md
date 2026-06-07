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
