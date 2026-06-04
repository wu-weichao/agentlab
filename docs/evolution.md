# 项目演进记录

本文件用于记录 AI Agent 学习项目的阶段性演进脉络。
详细变更过程见 `openspec/changes/archive/`，这里仅保留每一阶段的高层摘要。

## v0.0.1 mini-chatbot

目标：
实现最小可运行的 CLI ChatBot，作为后续 Agent 架构演进的起点。

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
- 可从 Prompt 模板化开始
- 或先实现短期记忆裁剪
- 或进入 Tool Calling 能力演进
