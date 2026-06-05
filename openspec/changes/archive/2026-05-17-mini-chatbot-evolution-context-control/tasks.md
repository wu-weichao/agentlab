## 1. 配置与状态建模

- [x] 1.1 在 `internal/config/` 中新增 `chat.context.max_chars`、`chat.context.keep_recent_turns`、`chat.context.summary_max_chars`、`chat.context.enable_rolling_summary` 配置结构
- [x] 1.2 移除 `chat.max_history_messages` 配置，并在配置校验中拒绝旧字段、零值、负值和摘要上限超过总预算的非法配置
- [x] 1.3 调整 `internal/session/` 会话状态，支持渲染后的 system prompt、单份滚动摘要和近期原始消息

## 2. 上下文治理核心实现

- [x] 2.1 新增 `internal/contextwindow/` 模块，实现基于字符数预算的上下文长度估算
- [x] 2.2 实现上下文构建器，按固定顺序组装 system prompt、滚动摘要、近期消息和当前用户输入
- [x] 2.3 实现按完整 `user/assistant` 轮次裁剪旧消息的逻辑，并输出被驱逐消息供摘要更新使用
- [x] 2.4 实现滚动摘要更新器，基于旧摘要和被驱逐消息生成结构化摘要并在超限时再次压缩

## 3. 聊天编排与 CLI 行为

- [x] 3.1 在 `internal/app/chatbot.go` 中接入“追加用户输入 -> 预算构建 -> 必要时更新摘要 -> 调用模型 -> 追加 assistant 回复”的主流程
- [x] 3.2 调整发送给 LLM 的消息转换逻辑，将内部 `summary` 语义渲染为固定前缀的 `system` 风格消息
- [x] 3.3 更新 `clear` 和 `history` 命令行为，使其能正确处理滚动摘要和近期原始消息
- [x] 3.4 增加上下文预算命中、裁剪轮次和摘要更新结果的日志记录，且不污染 CLI 交互输出

## 4. 测试与文档

- [x] 4.1 为配置校验补充测试，覆盖上下文预算字段的成功与失败场景，以及旧 `chat.max_history_messages` 字段的拒绝行为
- [x] 4.2 为 context builder 和 summarizer 补充测试，覆盖未超预算、超预算裁剪、整轮保留和摘要再压缩场景
- [x] 4.3 为 ChatBot、`clear`、`history` 和失败回退补充测试，确保 assistant 历史不会在失败请求中被错误写入
- [x] 4.4 更新 `configs/config.example.yaml`、`README.md` 和相关演进文档，说明新的上下文控制配置和行为边界
