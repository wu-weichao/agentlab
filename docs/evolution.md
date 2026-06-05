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
