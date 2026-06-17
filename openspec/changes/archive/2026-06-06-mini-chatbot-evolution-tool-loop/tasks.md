## 1. Tool Loop 基础结构

- [x] 1.1 新增 `ToolLoopOptions{MaxSteps int}` 与默认最大步数常量 `3`
- [x] 1.2 新增或整理 `ErrToolLoopExceeded`、`ErrMultipleToolCalls` 等工具循环错误
- [x] 1.3 将 ChatBot 初始化逻辑接入工具循环选项，并在未配置或非法值时使用默认值

## 2. 主流程重构

- [x] 2.1 将现有固定二次请求工具闭环重构为 `runToolLoop()`
- [x] 2.2 在 `ChatBot.Send()` 内创建覆盖完整本次请求的 runCache，并传入工具循环
- [x] 2.3 实现循环内普通文本、单个 `tool_call`、多个工具调用和步数超限的分支处理
- [x] 2.4 保持工具反馈消息只追加到本轮工作消息，不写入 `Session` 或用户可见 `history`
- [x] 2.5 确保步数超限时返回明确错误且不追加 assistant 消息

## 3. 工具反馈与 Prompt 调整

- [x] 3.1 更新工具说明 Prompt，将“每轮最多一次工具调用”调整为“每次回复最多请求一个工具”
- [x] 3.2 在 Prompt 中要求已有工具结果足以回答时直接给最终回答
- [x] 3.3 在 Prompt 中要求不要重复请求相同工具和相同参数
- [x] 3.4 确保工具结果回填消息适用于多步循环，并保留 `FINAL_ANSWER` 约束

## 4. runCache 端到端验证

- [x] 4.1 确保重复 `tool_name + arguments` 在同一次 `Send()` 内命中 runCache
- [x] 4.2 确保相同工具不同参数使用不同 cache entry
- [x] 4.3 确保每次 `Send()` 结束后丢弃 runCache，后续请求不复用上一轮结果

## 5. 测试覆盖

- [x] 5.1 补充无工具请求仍只调用一次模型的测试
- [x] 5.2 补充单工具请求兼容旧行为的测试
- [x] 5.3 补充 `tool -> tool -> final answer` 顺序工具链测试
- [x] 5.4 补充同一次 `Send()` 内重复工具调用只真实执行一次的测试
- [x] 5.5 补充达到 `MaxSteps` 返回 `ErrToolLoopExceeded` 且不追加 assistant 的测试
- [x] 5.6 补充多工具调用数组仍返回 `ErrMultipleToolCalls` 且不执行工具的测试

## 6. 验证与文档

- [x] 6.1 运行 `go fmt ./...`
- [x] 6.2 运行 `go test ./...`
- [x] 6.3 在 `docs/evolution.md` 增加 v0.0.6 阶段摘要，包含能力、实现思路、可观测性、边界和下一步

## 7. 实现反馈修正

- [x] 7.1 将 ChatBot 构造函数收敛为 `NewChatBot(ChatBotOptions)`，移除多个构造函数和过长位置参数
- [x] 7.2 在 runCache 执行入口补充 `tool_cache_hit`、`tool_cache_store`、`args_hash` 和 cache entry 数量日志
- [x] 7.3 保持日志不输出完整工具参数，避免泄露文件路径、搜索词或用户上下文
- [x] 7.4 移除基于自然语言 marker 从 `reasoning_content` 提取最终回答的处理
- [x] 7.5 在工具说明 Prompt 中要求最终用户可见回答必须进入 assistant `content`
- [x] 7.6 运行 `go fmt ./...`
- [x] 7.7 运行 `go test ./...`
