## Why

经过 v0.0.5 至 v0.0.7 的工具循环、请求内去重、配置与可观测性演进后，`ChatBot` 已同时承担会话上下文治理、摘要更新、工具循环和缓存诊断等 Runtime 职责，继续扩展会使 CLI/app 层与 Agent 执行策略深度耦合。现在需要建立轻量 `Agent Runtime` 边界，为后续 ToolCache、Memory、Workflow 等能力提供稳定入口，同时保持现有用户行为不变。

## What Changes

- 新增 `internal/agent` 包与可独立调用的 `Agent.Run(ctx, input)` Runtime 入口。
- 将上下文准备、滚动摘要更新、工具循环、请求内 runCache 和相关诊断逻辑迁移到 `internal/agent`。
- 由 `Agent` 统一持有 `llm.Client`、`session.Session`、`tools.Executor`、上下文组件和 Runtime options。
- 将 `ChatBot` 收敛为 app/CLI 适配器，保留现有 `ChatBot.Send()` API，并委托 `Agent.Run()` 执行。
- 保持 CLI 命令、配置格式、工具协议、会话历史语义、错误语义和用户可见交互兼容。
- 补充 Agent 层普通聊天、上下文治理、单工具、多步工具、请求内去重和失败路径测试，并保留适配器兼容测试。
- 更新演进文档，记录 v0.0.8 的能力、实现思路、可观测性、边界和下一步。

## Capabilities

### New Capabilities

- `agent-runtime`: 定义轻量 Agent Runtime 的构造、独立运行入口、上下文与工具循环编排边界，以及 ChatBot 兼容委托行为。

### Modified Capabilities

无。现有 `chat-session-runtime`、`context-window-control`、`tool-calling` 等能力的外部需求保持不变，本变更只调整其实现归属。

## Impact

- 新增代码：`internal/agent/` 下的 Runtime、options、上下文准备和工具循环实现及测试。
- 调整代码：`internal/app/chatbot.go`、`internal/app/tool.go` 及相关测试，主要职责改为组装和兼容适配。
- CLI：`cmd/chat` 的命令、配置读取和用户交互不变；对象组装可能调整为创建 Agent 后注入 ChatBot。
- API：保留 `app.NewChatBot(...)` 与 `ChatBot.Send(...)`；新增内部 `agent.New(...)` 与 `Agent.Run(...)`。
- 依赖与配置：不新增外部依赖，不改变 `configs/config.yaml`。
- 后续扩展：为跨轮 ToolCache、Memory、Workflow 和 Planner 提供稳定 Runtime 承载点，但本变更不实现这些能力。
