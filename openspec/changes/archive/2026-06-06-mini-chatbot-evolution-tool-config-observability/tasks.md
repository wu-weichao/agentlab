## 1. 配置加载与校验

- [x] 1.1 扩展配置结构，新增 `tools.max_steps` 字段并保持 `tools.web_search` 兼容
- [x] 1.2 在配置加载阶段为缺失的 `tools.max_steps` 填充默认值 `3`
- [x] 1.3 在启动阶段拒绝 `tools.max_steps < 1` 或 `tools.max_steps > 10`，并返回明确错误
- [x] 1.4 更新 `configs/config.example.yaml`，展示 `tools.max_steps` 与现有 `tools.web_search` 配置
- [x] 1.5 补充配置测试，覆盖缺失默认值、合法配置、小于 `1` 和大于 `10`

## 2. Tool Loop 配置注入

- [x] 2.1 在 CLI 初始化 ChatBot 时把配置中的 `tools.max_steps` 注入 `ChatBotOptions.ToolLoopOptions.MaxSteps`
- [x] 2.2 保留 ChatBot 直接构造路径的默认值兜底，避免测试或内部调用传入空选项时失效
- [x] 2.3 补充 ChatBot 初始化或端到端测试，确认配置值会影响 `ErrToolLoopExceeded` 的触发步数

## 3. 工具循环可观测性

- [x] 3.1 调整工具执行包装路径，使调用方能够获得 `cache_hit` 和稳定 `tool_call_key`
- [x] 3.2 在每次工具调用返回前向 `ToolResult.Metadata` 写入 `step`、`cache_hit` 和 `tool_call_key`
- [x] 3.3 缓存命中时更新当前步骤的 `step` 与 `cache_hit=true`，并保留原有业务 metadata
- [x] 3.4 在 `runToolLoop()` 每一步记录结构化日志，包含 `request_id`、`step`、`max_steps`、`tool_name`、`tool_call_key`、`cache_hit`、`success` 和 `error_code`
- [x] 3.5 确认新增 step 日志不输出完整工具参数、文件内容、搜索结果全文或明显敏感内容
- [x] 3.6 补充测试，覆盖普通工具执行、缓存命中、工具失败和步数超限时的 metadata 与日志字段

## 4. 文档与演进记录

- [x] 4.1 更新 README，说明 `tools.max_steps`、默认值、合法范围和多步工具循环边界
- [x] 4.2 更新 README，说明工具循环日志与 metadata 中的调试字段，以及敏感内容不进入 step 日志的边界
- [x] 4.3 更新 `docs/evolution.md`，新增 `v0.0.7 tool-loop-config-observability` 记录，包含能力、实现思路、可观测性、边界和下一步

## 5. 验证

- [x] 5.1 执行 `go fmt ./...`
- [x] 5.2 执行 `go test ./...`
- [x] 5.3 检查 OpenSpec 变更状态，确认 proposal、design、specs、tasks 均完成
