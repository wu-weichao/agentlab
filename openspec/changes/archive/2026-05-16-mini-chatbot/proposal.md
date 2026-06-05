## Why

仓库当前只有 OpenSpec 规范骨架，缺少一个可运行的最小 Agent 学习起点。先落地一个 Golang CLI ChatBot，可以验证消息模型、会话管理、配置加载、日志记录和 LLM 调用抽象是否稳定，并为后续逐步演进到 Memory、Tool Calling、Workflow 和 Multi-Agent 提供基础。

## What Changes

- 新增一个基于 Golang 和 `spf13/cobra` 的命令行 ChatBot 运行时，支持用户输入、模型回复和基础退出流程。
- 新增会话上下文管理，维护 `system`、`user`、`assistant` 三类消息，并支持多轮对话。
- 新增统一的 LLM 客户端抽象和首个 OpenAI 兼容实现，屏蔽具体模型服务差异。
- 新增本地配置加载能力，运行时只读取 `configs/config.yaml`，并提供 `configs/config.example.yaml` 作为示例文件。
- 新增启动期配置校验，若 `api_key` 为空或仍为 `{...}` 占位值，则在进入交互前直接报错。
- 新增基础日志记录能力，请求与调试日志默认写入 `logs/chat.log`，避免污染命令行交互输出。
- 为核心对象和主流程补充中文注释，降低后续维护和学习成本。
- 明确第一版边界，不包含 Tool Calling、持久化 Memory、RAG、Workflow、Multi-Agent 和 Web UI。

## Capabilities

### New Capabilities
- `cli-chat-interface`: 定义命令行聊天入口、交互命令、日志输出策略和终端展示行为。
- `chat-session-runtime`: 定义消息模型、会话上下文维护和单次对话编排行为。
- `llm-provider-configuration`: 定义本地配置加载、启动期校验和 OpenAI 兼容调用要求。

### Modified Capabilities

无。

## Impact

- 受影响代码路径包括 `cmd/chat/`、`internal/app/`、`internal/session/`、`internal/llm/`、`internal/config/`、`configs/`、`logs/`、`README.md` 和 `.gitignore`。
- 需要初始化 Go module，引入 `spf13/cobra` 作为命令行库，并增加一个 OpenAI 兼容 HTTP 客户端实现及对应配置结构。
- 需要补充基础测试，覆盖消息追加、会话重置、配置解析、占位 API Key 校验和错误路径。
