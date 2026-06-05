## 1. 项目骨架与配置

- [x] 1.1 初始化 Go module，引入 `spf13/cobra`，并创建 `cmd/chat`、`internal/app`、`internal/session`、`internal/llm`、`internal/config`、`configs` 目录骨架
- [x] 1.2 定义聊天运行时配置结构，支持从配置文件读取 provider、model、base URL、API Key、temperature、system prompt、max history
- [x] 1.3 提供 `configs/config.example.yaml`、本地 `configs/config.yaml` 约定和基础 README 使用说明，明确第一版边界与启动方式
- [x] 1.4 保持配置加载逻辑简单，只使用 `config.yaml` 作为运行时配置来源
- [x] 1.5 在启动阶段校验 `api_key`，拒绝空值和 `{...}` 占位值

## 2. 对话运行时实现

- [x] 2.1 实现 `llm.Message`、角色枚举、`ChatResponse` 和统一 `LLMClient` 接口
- [x] 2.2 实现内存会话对象，支持初始化 system prompt、追加 user/assistant 消息、重置会话和读取历史
- [x] 2.3 实现 `ChatBot` 主流程，串联用户输入、会话更新、LLM 调用、错误返回和回复写回
- [x] 2.4 为核心结构和关键方法补充中文注释，降低阅读和维护成本

## 3. Provider 与 CLI 交互

- [x] 3.1 实现 OpenAI 兼容客户端，支持将消息列表转换为聊天请求并提取文本回复
- [x] 3.2 使用 `spf13/cobra` 实现命令入口和 CLI 主循环，支持欢迎语、普通聊天输入、`exit`/`quit`、`clear`、`history`
- [x] 3.3 将配置、会话、LLM 客户端和 ChatBot 在 `cmd/chat/main.go` 中组装为最小可运行闭环
- [x] 3.4 为接口请求、响应和错误详情增加日志记录
- [x] 3.5 将标准日志默认写入本地日志文件，避免打断命令行交互

## 4. 验证与测试

- [x] 4.1 为消息追加、会话重置、配置校验和 ChatBot 错误路径补充基础单元测试
- [x] 4.2 运行 `go test ./...` 验证核心逻辑，并记录暂未覆盖的外部 API 场景
- [x] 4.3 为配置文件加载和占位 API Key 拦截补充测试
