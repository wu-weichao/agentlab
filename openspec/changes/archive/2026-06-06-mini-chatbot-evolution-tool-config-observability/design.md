## Context

v0.0.6 已经引入 `ToolLoopOptions{MaxSteps int}`、`runToolLoop()` 和同一次 `ChatBot.Send()` 内共享的 runCache。当前限制是 `MaxSteps` 仍由代码默认值控制，配置文件无法约束工具循环成本；同时工具循环日志分散在模型调用、工具执行和缓存入口中，缺少统一的 step 级诊断字段。

本次变更基于 `docs/evolution.md` 和 `v0.0.7-tool-loop-config-observability.md`，只治理配置与可观测性，不改变工具调用协议、工具注册、runCache 生命周期或多步 Tool Loop 的核心执行语义。

## Goals / Non-Goals

**Goals:**

- 从本地配置加载 `tools.max_steps`，缺失时默认 `3`。
- 启动阶段拒绝非法 `tools.max_steps`：小于 `1` 或大于 `10`。
- 将配置值注入 `ChatBotOptions.ToolLoopOptions.MaxSteps`。
- 每个工具循环步骤输出统一结构化日志。
- 在 `ToolResult.Metadata` 中补充 `step`、`cache_hit` 和 `tool_call_key`。
- 保持日志可诊断，但不输出完整参数、文件内容、搜索结果全文或敏感信息。
- 更新 README 和 `docs/evolution.md`。

**Non-Goals:**

- 不新增工具能力。
- 不改变 `ToolCallKey` 构造、runCache 命中规则或 Tool Loop 循环语义。
- 不引入 OpenTelemetry、指标面板或 UI 轨迹展示。
- 不把完整工具轨迹写入 `Session` 或 `history`。
- 不接入 provider 原生 tool calling。

## Decisions

### Decision: 配置字段使用顶层 `tools.max_steps`

配置示例采用：

```yaml
tools:
  max_steps: 3
  web_search:
    enabled: false
    provider: ""
    endpoint: ""
```

这样可以继续承载现有 `tools.web_search` 配置，并把工具循环运行边界放在工具域下。备选方案是使用 `chat.tools.max_steps` 或 `tool_loop.max_steps`，但当前配置已经有 `tools.web_search`，继续扩展 `tools` 更少迁移成本。

### Decision: 配置层负责默认值和合法性校验

配置缺失时填充默认值 `3`；显式配置小于 `1` 或大于 `10` 时启动失败。`ChatBotOptions` 仍可保留内部默认值作为测试或直接构造兜底，但 CLI 启动路径必须在配置加载阶段暴露错误。

备选方案是在 `runToolLoop()` 中动态纠正非法值。该方案会隐藏配置错误，且不利于用户在启动阶段发现成本边界配置失误。

### Decision: step 级日志在工具循环编排层统一输出

`runToolLoop()` 拥有 `request_id`、`step`、`max_steps`、解析出的工具名和执行结果，因此它应负责输出每步结构化日志。缓存命中状态由 `executeToolWithRunCache()` 返回或附加到结果 metadata，再由循环层统一记录。

备选方案是在具体工具中记录 step 日志，但具体工具不知道循环步数、最大步数和缓存来源，日志会不完整且重复。

### Decision: 调试字段写入 `ToolResult.Metadata`

每次工具调用完成后，系统在返回给模型前补充：

- `step`
- `cache_hit`
- `tool_call_key`

字段用于本地调试和测试断言。若具体工具已有同名 metadata，应由运行时调试字段覆盖，以保证语义一致。备选方案是新增独立 `ToolExecutionTrace` 结构，但当前不保存完整轨迹，单独结构会增加传递复杂度。

### Decision: 日志输出 key，不输出完整参数

日志中的 `tool_call_key` 可定位重复调用和缓存命中，不需要记录完整参数。对于 `file_read`、`web_search` 等工具，完整参数或结果可能包含用户上下文、文件路径、搜索词和外部内容，因此本版本只记录 hash/key、工具名、状态和错误码。

备选方案是保留 v0.0.5 的完整 request body 调试习惯。v0.0.7 的目标是开始收敛敏感内容暴露面，因此新增 step 日志必须默认更克制。

## Risks / Trade-offs

- 非法配置导致旧配置启动失败 -> 只新增 `tools.max_steps`，缺失时默认 `3`，现有配置不需要立即修改。
- `ToolResult.Metadata` 被调试字段污染业务结果 -> 字段命名稳定且仅限运行时诊断，后续若需要可迁移到独立 trace。
- 缓存命中结果的 metadata 可能来自首次执行 -> 命中缓存后返回前重写 `step` 和 `cache_hit`，保证当前步骤可观测。
- 不记录完整参数会降低排障细节 -> 使用 `tool_call_key` 和 `args_hash` 定位重复调用；需要内容级排查时再临时提高更细日志。
- 最大值 `10` 可能限制少数复杂任务 -> 当前是学习项目且工具集有限，先用硬上限防止误配置导致成本失控。

## Migration Plan

1. 扩展配置结构和示例配置，新增 `tools.max_steps` 默认值与校验。
2. CLI 初始化 ChatBot 时把配置值传入 `ToolLoopOptions.MaxSteps`。
3. 调整工具循环和 runCache 包装返回值，补充 `cache_hit`、`tool_call_key` 和当前 step。
4. 在工具循环每步输出结构化日志。
5. 补充配置、日志、metadata 和缓存命中测试。
6. 更新 README 与 `docs/evolution.md`。

回滚方式是移除配置字段注入并恢复代码默认 `MaxSteps=3`；本变更不涉及数据迁移。

## Open Questions

- 后续是否需要区分开发日志与生产日志级别。
- 后续是否需要把工具循环轨迹保存为单独调试文件，而不是进入会话历史。
- 后续是否需要按工具类型声明更细粒度的可缓存性和日志脱敏策略。
