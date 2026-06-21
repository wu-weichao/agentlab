## Why

当前 Agent 的工具调用依赖模型在 assistant 文本中严格生成约定 JSON，格式正确性和工具反馈语义主要由 Prompt 保证。随着顺序多步工具循环已经稳定，继续使用文本模拟协议会放大模型兼容、解析失败和重复请求问题，因此需要把 provider 原生 Tool Calling 接入现有统一 Runtime。

## What Changes

- 扩展统一 LLM 消息和响应模型，使其能够表达 assistant tool call、tool result 和结构化工具调用响应。
- OpenAI 兼容客户端基于已注册 `ToolSpec` 发送原生 `tools` 定义，并解析 provider 返回的 `tool_calls`。
- 在 LLM provider 适配层将 provider 私有字段转换为仓库内部统一的 `tools.ToolCall`，Agent 和 ToolExecutor 不直接依赖 OpenAI 私有协议。
- Agent 工具循环优先消费结构化工具调用，并使用原生 assistant/tool 消息回填后续模型请求。
- 保留现有 JSON 文本工具协议作为显式兼容模式，普通文本回答行为保持不变。
- 保持顺序单工具步骤、`MaxSteps`、请求内 runCache、工具安全边界和会话历史语义不变。
- 增加原生与兼容模式、完整模型请求体、完整响应体、工具调用和工具执行结果日志；Authorization header 和 API Key 不进入日志。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `llm-provider-configuration`: OpenAI 兼容客户端需要发送原生工具定义、解析结构化工具调用，并支持配置化工具调用模式。
- `tool-calling`: 工具调用协议从仅依赖 assistant 文本 JSON 扩展为优先使用 provider 原生 Tool Calling，同时保留文本兼容模式。
- `agent-runtime`: Agent 工具循环需要消费结构化工具调用并使用原生工具消息完成顺序多步执行。

## Impact

- 影响 `internal/llm`：扩展统一消息、响应、请求 DTO 和 OpenAI provider 适配逻辑。
- 影响 `internal/agent`：调整工具发现、模型调用、工具调用识别和工具结果回填路径。
- 影响 `internal/config`、示例配置和 CLI 组装：增加工具调用模式配置并注入 LLM 客户端。
- 影响测试：补充原生普通回答、单工具、多步工具、非法参数、多工具拒绝、`MaxSteps`、runCache 和文本兼容模式测试。
- 不改变现有 `Tool` 接口，不新增工具和外部依赖，不引入并行调用、跨轮缓存、Planner、Workflow 或 Memory。
