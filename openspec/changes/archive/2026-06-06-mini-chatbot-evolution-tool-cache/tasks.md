## 1. ToolCallKey 基础设施

- [x] 1.1 新增 `internal/tools/key.go`，定义 `ToolCallKey` 结构。
- [x] 1.2 实现 `BuildToolCallKey(call ToolCall) (ToolCallKey, error)`，基于工具名和标准化参数生成 key。
- [x] 1.3 实现 `NormalizeArguments(args map[string]any) (string, error)`，确保 map key 和嵌套对象顺序稳定。
- [x] 1.4 实现稳定 hash 逻辑，用于生成 `ToolCallKey.ArgsHash`。

## 2. runCache 执行包装接入

- [x] 2.1 新增或抽出带 `runCache` 参数的工具执行包装函数。
- [x] 2.2 在执行工具前构造 `ToolCallKey` 并检查本轮缓存。
- [x] 2.3 缓存未命中时真实调用 `tools.Executor.Execute()` 并写入 `runCache`。
- [x] 2.4 缓存命中时复用本轮工具结果。
- [x] 2.5 保持当前单工具闭环、fallback 行为和 `Tool` 接口兼容。

## 3. 测试覆盖

- [x] 3.1 新增 `internal/tools/key_test.go`，覆盖参数顺序不同但 key 相同的场景。
- [x] 3.2 覆盖不同工具名但参数相同时 key 不同的场景。
- [x] 3.3 覆盖空参数和嵌套参数标准化场景。
- [x] 3.4 在 app 层补充执行包装函数复用相同 `ToolCallKey` 结果的测试。
- [x] 3.5 确认现有单工具闭环测试继续通过。

## 4. 文档与验证

- [x] 4.1 更新 README 或 `docs/evolution.md`，说明本版本增加 `ToolCallKey` 和 runCache 基础设施。
- [x] 4.2 运行 `go fmt ./...`。
- [x] 4.3 运行 `go test ./...`。
- [x] 4.4 使用 OpenSpec 状态命令确认 change artifacts 已满足实施条件。
- [x] 4.5 同步记录工具结果二次回填使用 `user` 消息承载 `FINAL_ANSWER` 阶段约束。
- [x] 4.6 同步记录诊断日志增强：工具执行完成输出完整单行 `ToolResult` JSON，LLM 发送请求输出完整 request body。
