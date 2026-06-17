## Why

v0.0.6 已经把工具调用升级为多步 Tool Loop，但最大步数仍停留在代码默认值，工具循环的每一步也缺少统一的调试字段。现在需要把工具循环的运行边界配置化，并补齐最小可观测性，便于定位重复调用、缓存命中、失败步骤和超限原因。

## What Changes

- 新增 `tools.max_steps` 配置，缺失时默认使用 `3`。
- 启动阶段校验 `tools.max_steps`，小于 `1` 或大于 `10` 时返回明确配置错误。
- 保持现有 `tools.web_search` 配置兼容，不改变工具注册和执行语义。
- 工具循环每一步记录结构化日志，至少包含 `request_id`、`step`、`max_steps`、`tool_name`、`tool_call_key`、`cache_hit`、`success` 和 `error_code`。
- 工具结果 `metadata` 增加调试字段：`step`、`cache_hit`、`tool_call_key`。
- 日志不得输出完整工具参数、完整文件内容、搜索结果全文或明显敏感内容。
- README 补充多步工具循环配置、边界和可观测性说明。
- `docs/evolution.md` 增加 `v0.0.7` 演进记录。
- 不新增 UI、OpenTelemetry、指标面板，也不把完整工具轨迹写入 `history`。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `tool-calling`: 工具循环必须具备可配置最大步数、逐步结构化日志，以及工具结果 metadata 中的最小调试字段。
- `llm-provider-configuration`: 本地配置加载必须支持 `tools.max_steps`，并在启动阶段执行默认值填充和合法性校验。

## Impact

- 影响配置结构、配置示例和启动阶段校验逻辑。
- 影响 `internal/app` 中 Tool Loop 选项注入、每步日志和 runCache 观测路径。
- 影响 `internal/tools` 或工具执行包装层中 `ToolResult.Metadata` 的调试字段补充。
- 需要补充配置默认值、非法配置、每步日志、缓存命中 metadata 和超限行为相关测试。
- 不引入新的外部依赖，不改变 `ToolCallKey`、runCache 生命周期、工具接口或 Tool Loop 核心执行语义。
