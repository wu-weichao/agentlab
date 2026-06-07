## Why

当前 Mini ChatBot 已具备基础单会话聊天、Prompt 模板和上下文控制能力，但仍只能依赖模型直接生成回答，无法在需要确定性计算、本地上下文读取、实时信息检索或当前时间查询时调用受控工具。现在需要补上一个最小可运行的 Tool Calling 闭环，让 Agent 能够生成结构化工具调用、执行受控工具，并把工具结果回填给模型生成最终回答。

本次变更遵循“先简单、后复杂”的演进原则，只完成低风险且可测试的工具调用基础能力。交互类、写入类、授权类和命令执行类操作暂不纳入本版本，避免过早引入权限模型和复杂安全边界。

## What Changes

- 新增 Tool Calling 协议，定义 `tool_call`、`tool_result`、工具元信息和参数校验的统一结构。
- 新增 `internal/tools/` 模块，提供工具接口、`ToolSpec`、注册表、执行器和结构化错误返回。
- 新增四个首版工具：`calculator`、`time`、`file_read`、`web_search`。
- 调整 ChatBot 编排逻辑，在启动阶段将工具说明合并进会话 system prompt，使模型能够基于全局工具能力说明生成结构化工具调用。
- 使单轮对话可在模型请求后识别工具调用、执行工具，并基于工具结果继续生成最终回答。
- 为工具调用增加清晰边界：单轮最多一次工具调用，不继续执行并行工具调用或递归工具调用。
- 当工具结果回填后的第二次模型响应仍输出 `tool_call` 时，系统基于已有 `tool_result` 生成兜底最终回答。
- 为工具注册、参数校验、执行成功、执行失败和 Agent 工具回填流程补充测试。

## Capabilities

### New Capabilities

- `tool-calling`: 定义 Mini ChatBot 的最小 Tool Calling 协议、工具执行器、安全边界和首版工具能力。

### Modified Capabilities

- `chat-session-runtime`: 聊天运行时需要支持一次受控工具调用闭环，并确保工具失败不会破坏会话主流程。
- `llm-provider-configuration`: 需要为 `web_search` 的外部依赖预留最小配置入口，并在未配置时返回结构化不可用错误。

## Impact

- 受影响代码预计包括 `internal/app/`、`internal/llm/`，以及新增的 `internal/tools/`。
- 受影响会话代码包括 `internal/session/`，用于启动阶段将工具说明追加到 system prompt 并在 `clear` 后保留。
- 可能需要更新 `configs/config.example.yaml`、`README.md` 和 `docs/evolution.md`，说明工具范围、配置方式和非目标。
- 需要补充工具单元测试、执行器测试和 ChatBot 工具调用集成测试。

## Non-Goals

- 不实现 `file_write`、`ask_user`、`shell_exec`、`http_fetch`、长期记忆读写或外部插件市场。
- 不实现并行工具调用、多轮递归工具调用或工具链规划。
- 不引入复杂权限审批、用户确认流、交互式授权或高风险命令执行。
- 不把搜索结果视为事实来源；最终回答仍需要体现来源意识和不确定性。
