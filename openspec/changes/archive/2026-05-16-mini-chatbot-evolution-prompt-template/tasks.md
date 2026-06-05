## 1. Prompt 配置与渲染基础

- [x] 1.1 调整 `internal/config` 配置结构，移除 `chat.system_prompt` 并新增 `chat.prompt.template`、`chat.prompt.role`、`chat.prompt.context`
- [x] 1.2 实现新配置校验逻辑，要求模板存在、标准变量完整且非空
- [x] 1.3 新增 `internal/prompt` 模块，实现模板变量解析、校验与渲染
- [x] 1.4 更新 `configs/config.example.yaml` 和 README，改用新 Prompt 配置结构

## 2. 会话启动链路接入

- [x] 2.1 在启动阶段构造 Prompt 变量并渲染最终 system prompt
- [x] 2.2 调整 `Session` 初始化与 `clear` 行为，始终恢复渲染后的 prompt 结果
- [x] 2.3 保持 `ChatBot` 主链路不变，只接收渲染完成后的会话对象
- [x] 2.4 为模板渲染和启动校验补充清晰日志与错误信息

## 3. 规范与测试

- [x] 3.1 为 `internal/prompt` 补充模板替换、缺失变量和普通文本保留测试
- [x] 3.2 为 `internal/config` 补充新结构加载与空值校验测试
- [x] 3.3 为 `internal/session` 或 `internal/app` 补充渲染后 prompt 初始化与 `clear` 恢复测试
- [x] 3.4 运行 `go test ./...` 验证第二版核心行为
