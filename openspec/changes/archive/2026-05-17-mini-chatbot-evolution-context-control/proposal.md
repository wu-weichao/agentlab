## Why

当前 Mini ChatBot 已具备基础单会话聊天能力，但对话一旦变长，请求上下文会持续膨胀，最终导致成本、延迟和稳定性不可控。现在需要补上一个最小可运行的短期上下文治理闭环，在不引入长期记忆和检索系统的前提下，先解决单会话内的上下文预算控制问题。

## What Changes

- 新增会话上下文预算控制能力，支持在模型请求前按预算组装上下文。
- 新增单份滚动摘要机制，在旧消息被移出近期窗口时通过 LLM 生成或更新结构化摘要，并在必要时继续压缩已有摘要。
- 为聊天配置新增 `chat.context.*` 配置，定义总上下文预算、近期保留轮数、摘要长度上限和摘要开关。
- **BREAKING** 移除 `chat.max_history_messages` 配置，后续上下文控制只使用 `chat.context.*`，不提供历史版本兼容。
- 调整会话运行时消息模型，支持内部 `summary` 语义消息，并在发送给模型前转换为 provider 可接受的消息格式。
- 调整 `clear` 与 `history` 行为，使其同时处理滚动摘要和近期原始消息。
- 增加上下文预算命中、裁剪、摘要更新、摘要再压缩和 `request_id` 贯穿链路的日志与测试。

## Capabilities

### New Capabilities
- `context-window-control`: 定义单会话内的上下文预算、近期消息保留、旧消息裁剪与单份滚动摘要行为。

### Modified Capabilities
- `chat-session-runtime`: 会话状态从仅维护原始消息列表扩展为维护渲染后的 system prompt、滚动摘要和近期消息，并更新清空与历史读取行为。
- `cli-chat-interface`: `history` 输出需要能展示滚动摘要与原始消息，`clear` 需要同时清空摘要和近期历史。
- `llm-provider-configuration`: 聊天运行时配置需要新增 `chat.context.*` 字段，并在启动阶段校验其合法性。

## Impact

- 受影响代码预计包括 `internal/session/`、`internal/app/`、`internal/config/`、`internal/llm/`、`cmd/chat/` 和新增的 `internal/contextwindow/`。
- 需要更新 `configs/config.example.yaml`、`README.md` 与相关演进文档，说明新的上下文控制配置和行为。
- 需要补充上下文构建、滚动摘要、配置校验、`clear/history` 行为的单元测试与集成测试。
