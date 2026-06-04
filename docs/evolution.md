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
- 实现 `max_history_messages` 驱动的短期记忆裁剪
- 在上下文增长后引入更稳的裁剪或摘要策略
- 再进入 Tool Calling 和更完整的 Agent Runtime 演进
