## 1. 统一 LLM 协议扩展

- [x] 1.1 定义 `ToolCallingMode`，支持 `native` 和 `text_compat` 两个合法值及默认值。
- [x] 1.2 新增统一 `llm.ChatRequest`，承载消息、工具定义和工具调用模式，并迁移 `llm.Client` 接口。
- [x] 1.3 扩展 `llm.Message`，支持 assistant tool calls、tool role、tool call ID 和工具名称。
- [x] 1.4 扩展 `llm.ChatResponse`，支持结构化 `tools.ToolCall` 列表。
- [x] 1.5 为 `tools.ToolCall` 增加可选调用 ID，并确认 `ToolCallKey` 继续忽略 ID。
- [x] 1.6 迁移 Summarizer、Agent 和测试 fake client 的普通无工具模型调用到 `ChatRequest`。

## 2. 配置与运行时组装

- [x] 2.1 在工具配置中增加工具调用模式字段，缺失时默认填充 `native`。
- [x] 2.2 在配置校验中拒绝 `native` 和 `text_compat` 之外的值。
- [x] 2.3 将工具调用模式从 CLI 组装层注入 Agent Options，并保持 ChatBot 兼容入口可用。
- [x] 2.4 更新示例配置和配置测试，覆盖默认 native、显式 text compatibility 和非法模式。

## 3. OpenAI 原生 Tool Calling 适配

- [x] 3.1 扩展 OpenAI 请求 DTO，支持 `tools` 定义、assistant `tool_calls` 和 tool result 消息。
- [x] 3.2 实现 `ToolSpec` 到 OpenAI function tool JSON Schema 的转换，包含 properties 和 required。
- [x] 3.3 在 native 模式请求中发送已注册工具定义，并在 text compatibility 模式中省略原生 `tools`。
- [x] 3.4 解析 provider 返回的 tool call ID、function name 和 arguments JSON object 为统一 `tools.ToolCall`。
- [x] 3.5 对缺失调用 ID、空工具名、非法 JSON、数组或标量 arguments 返回明确协议错误。
- [x] 3.6 保持普通 assistant content、reasoning-only 拒绝和 HTTP 错误处理行为兼容。
- [x] 3.7 新增 OpenAI client 测试，覆盖工具 schema、普通回答、单个原生调用、多个调用和非法参数。

## 4. Agent 原生工具循环

- [x] 4.1 根据工具调用模式生成 system prompt 工具说明：native 仅保留行为约束，text compatibility 保留 JSON 格式说明。
- [x] 4.2 在每次 native 模型请求中传入 Executor 的 `ToolSpec`，普通摘要请求不得携带工具。
- [x] 4.3 调整工具调用识别逻辑：native 消费 `ChatResponse.ToolCalls`，text compatibility 继续解析 content。
- [x] 4.4 在执行前拒绝一次响应中的多个原生工具调用，并确保不执行其中任何工具。
- [x] 4.5 实现 native assistant tool call 与 tool result 工作消息回填，使用相同 tool call ID 建立关联。
- [x] 4.6 保持工具轨迹只存在于本轮工作消息，不写入 Session history 或 rolling summary。
- [x] 4.7 保持顺序多步调用、`MaxSteps`、失败工具反馈和最终 assistant 写回行为。
- [x] 4.8 确认不同调用 ID 但相同工具和参数仍命中同一个 runCache，并使用当前调用 ID 回填结果。
- [x] 4.9 新增 Agent 测试，覆盖 native 普通回答、单工具、多步工具、缓存命中、多调用拒绝、超步数和工具失败。
- [x] 4.10 保留并补充 text compatibility 回归测试，确认现有 JSON 文本协议行为不变。

## 5. 学习项目详细可观测性

- [x] 5.1 为 LLM 请求和响应日志增加 `tool_calling_mode`、`tools_count`、`tool_calls_count` 和解析状态。
- [x] 5.2 为 Agent 工具识别日志增加调用模式，并保留 request ID、step、tool call key 和 cache hit。
- [x] 5.3 记录完整 request body 和 response body，同时确保不输出 Authorization header 或 API Key。
- [x] 5.4 增加日志测试，覆盖完整模型请求、响应、工具调用参数和工具执行结果。

## 6. 文档与验证

- [x] 6.1 更新 README，说明 native 默认模式、`text_compat` 适用场景和 provider 兼容要求。
- [x] 6.2 更新 `docs/evolution.md`，新增 v0.0.9 的能力、实现思路、可观测性、边界和下一步。
- [x] 6.3 运行 `gofmt` 或 `go fmt ./...`，确保新增 Go 代码格式正确。
- [x] 6.4 运行 `go test ./...`，确认普通聊天、上下文治理、两种工具模式和现有工具测试全部通过。
- [x] 6.5 使用 OpenSpec 校验和状态命令确认 change artifacts 合法且实施任务可追踪。
