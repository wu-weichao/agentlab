## 1. Agent Runtime 基础结构

- [x] 1.1 新增 `internal/agent` 包，定义 `Agent`、`Options`、`ToolLoopOptions`、默认最大步数和工具循环错误
- [x] 1.2 在 `agent.New()` 中完成依赖持有、默认 Executor/MaxSteps 处理和上下文 Builder、Summarizer 初始化
- [x] 1.3 将工具说明生成逻辑迁移到 `internal/agent`，并确保构造时只向 Session system prompt 固化一次

## 2. Agent 单轮执行与上下文治理

- [x] 2.1 实现 `Agent.Run(ctx, input)`，建立 request_id、写入用户消息、执行 Runtime 流程并仅在成功时写入最终 assistant 消息
- [x] 2.2 将上下文构建、旧消息裁剪和滚动摘要更新逻辑从 `internal/app` 迁移到 `internal/agent`
- [x] 2.3 将已有摘要再压缩与上下文重试逻辑迁移到 `internal/agent`，保持预算错误和会话状态语义不变
- [x] 2.4 补充 Agent 普通聊天、模型失败、摘要更新、摘要再压缩和 request_id 关联测试

## 3. 工具循环与请求内缓存迁移

- [x] 3.1 将顺序多步工具循环、工具调用解析和工具结果回填逻辑迁移到 `internal/agent`
- [x] 3.2 将 `executeToolWithRunCache`、ToolCallKey 复用和每次 Run 独立缓存生命周期迁移到 `internal/agent`
- [x] 3.3 将 step、cache_hit、tool_call_key metadata 和脱敏工具日志迁移到 Agent Runtime，并保留现有结构化字段
- [x] 3.4 迁移并补充单工具、多步工具、重复调用命中、跨 Run 不复用、工具失败、多工具拒绝和 MaxSteps 超限测试
- [x] 3.5 补充工具说明只初始化一次、Session reset 后仍保留工具说明和日志不泄露完整工具内容的测试

## 4. ChatBot 兼容适配

- [x] 4.1 将 `internal/app.ChatBot` 收敛为持有 `*agent.Agent` 的适配器，并让 `Send()` 委托 `Agent.Run()`
- [x] 4.2 保留 `ChatBotOptions` 构造入口，并将其稳定映射到 `agent.Options`
- [x] 4.3 为 app 层已有 ToolLoop 类型、默认值和错误保留必要的兼容别名，移除 app 层重复 Runtime 实现
- [x] 4.4 收敛 `internal/app` 测试为构造映射、成功委托、错误返回和会话写回兼容测试
- [x] 4.5 验证 `cmd/chat` 继续使用原命令、配置、clear、history 和退出交互，不产生用户可见行为变化

## 5. 验证与演进文档

- [x] 5.1 运行 `go fmt ./...`
- [x] 5.2 运行 `go test ./...` 并修复所有迁移导致的回归
- [x] 5.3 在 `docs/evolution.md` 增加 v0.0.8 记录，包含能力、实现思路、可观测性、边界和下一步
- [x] 5.4 核对本版本未引入跨轮 ToolCache、TTL、新工具、配置变更、Workflow、Memory、RAG 或 Multi-Agent
