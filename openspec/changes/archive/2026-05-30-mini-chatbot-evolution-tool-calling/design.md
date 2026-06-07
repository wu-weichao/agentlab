## Context

当前主链路为 `cmd/chat -> internal/app -> internal/session -> internal/llm`，并已具备会话历史、Prompt 模板和上下文窗口控制。Tool Calling 的首要目标不是完整工具生态，而是让一次聊天请求能够跑通“模型提出工具调用 -> 执行器执行 -> 工具结果回填 -> 模型生成最终回答”的最小闭环。

本版本只实现 `file_read`、`web_search`、`calculator`、`time`。`file_write`、`ask_user` 和其他涉及交互确认、授权审批或环境修改的工具暂缓到后续版本单独设计。

## Goals / Non-Goals

**Goals:**
- 定义统一的工具元信息、工具调用和工具结果结构。
- 实现工具注册、查找、参数校验、执行分发和结构化错误返回。
- 支持 `calculator`、`time`、`file_read`、`web_search` 四个工具。
- 让 ChatBot 能在单轮请求内执行至多一次工具调用，并基于工具结果生成最终回答。
- 保证工具失败时返回可解释错误，不污染会话状态，不让 Agent 编造工具结果。

**Non-Goals:**
- 不实现文件写入、命令执行、用户澄清、人工授权、长期记忆或任意 HTTP 抓取。
- 不支持并行工具调用、递归工具调用或多工具规划。
- 不做复杂权限模型，只在具体工具中实现首版安全边界。
- 不把工具执行器耦合到 provider 私有协议，首版以仓库内统一结构适配模型输出。

## Decisions

### 1. 新增 `internal/tools` 模块封装工具定义与执行器

建议新增如下结构：

```text
internal/tools/
  tool.go
  executor.go
  calculator.go
  time.go
  file_read.go
  web_search.go
  *_test.go
```

核心类型：
- `Tool`：包含 `Name()`、`Description()`、`Parameters()`、`Execute(ctx, args)`。
- `ToolSpec`：包含 `Name`、`Description`、`Parameters`，作为给模型看的工具定义快照。
- `ToolCall`：包含 `tool_name`、`arguments`、`reason`。
- `ToolResult`：包含 `success`、`content`、`error`、`metadata`。
- `Executor`：负责注册工具、按名称查找工具、校验入参、执行分发，并提供 `Specs()` 生成模型可见工具说明。

原因：
- 工具能力是独立横切模块，放在 `internal/tools` 便于单元测试和后续扩展。
- ChatBot 只需要依赖执行器接口，不需要理解每个工具的内部实现。

### 2. 工具调用协议采用仓库内统一 JSON 文本结构，不直接绑定某个 provider 的私有格式

本版本不使用 provider 原生 tool calling，而是通过 system prompt 中的工具说明，让模型按约定输出 JSON 文本。该方式便于学习和测试，但不能由 API 层强制模型遵守，因此需要系统侧解析和 fallback。

模型第一阶段响应可以被解析为普通文本或结构化工具调用。结构化工具调用必须包含：

```json
{
  "tool_name": "calculator",
  "arguments": {"expression": "1 + 2 * 3"},
  "reason": "需要确定性计算结果"
}
```

工具结果统一回填为结构化内容：

```json
{
  "success": true,
  "content": "7",
  "error": "",
  "metadata": {"tool_name": "calculator"}
}
```

原因：
- 当前项目是学习型 Mini ChatBot，使用统一中间结构更利于理解和测试。
- 后续如接入 OpenAI 原生 tool schema，可在 `internal/llm` 适配层转换，而不是改写工具执行器。

### 3. 启动阶段将工具说明合并进会话 system prompt

ChatBot 初始化时通过 `ToolExecutor.Specs()` 获取已注册工具定义，生成统一工具说明，并调用 `Session.AppendSystemPromptSection()` 将其追加到会话 system prompt。

运行时语义：
- 工具说明是 Agent 的全局能力说明，不在每次 `Send` 时临时追加。
- `clear`/`Reset` 只清空摘要和近期消息，工具说明仍作为 system prompt 的一部分保留。
- ChatBot 创建后再修改外部 executor，不会改变已经固化进 system prompt 的工具说明。

原因：
- 工具能力是 Agent 启动后的全局能力，而不是某一轮对话的临时上下文。
- 将工具说明并入 system prompt，更符合当前 Prompt Runtime 和 Session 的职责边界。
- 文本协议模式下，模型需要在第一次请求中看到工具清单和 JSON 格式约束，才能决定是否输出 `tool_call`。

### 4. 单轮最多执行一次工具调用

首版流程：

```text
用户输入
-> 构建上下文
-> 调用模型
-> 若模型返回 tool_call，则执行工具
-> 将 tool_result 作为受控上下文回填
-> 再次调用模型生成最终回答
-> 成功后写入 user/assistant 会话历史
```

限制：
- 每个用户输入最多执行一次工具调用。
- 工具结果回填后的第二次模型响应必须是最终文本回答。
- 如果第二次响应仍请求工具调用，系统不继续执行第二个工具，并基于已有工具结果生成兜底最终回答。

原因：
- 单工具闭环足以验证协议、执行器和结果回填。
- 递归和并行会显著增加状态管理、预算控制和失败回退复杂度，后续再引入更稳妥。
- 文本协议模式下，部分模型可能在工具结果回填后仍重复输出 `tool_call`；系统级兜底能保证用户拿到最终回答。

### 5. 四个工具的边界按风险分层处理

`calculator`：
- 只支持基础数学表达式。
- 禁止执行任意 Go 代码或脚本。
- 非法表达式、空输入和除零返回结构化错误。

`time`：
- 返回当前日期、时间、时区和 Unix 时间戳。
- 首版使用本地默认时区，允许后续扩展指定时区参数。

`file_read`：
- 只读取工作区内文本文件。
- 禁止读取 `.git/`、`.claude/`、`.codex/`、敏感配置目录和明显密钥文件。
- 限制单次读取字节数，超限时返回错误或截断策略必须明确。

`web_search`：
- 返回标题、摘要、URL、来源和时间信息。
- 结果数量必须有上限。
- 搜索失败或未配置外部依赖时返回结构化错误，Agent 不得编造结果。

### 6. `web_search` 使用最小可替换适配器

建议定义 `SearchClient` 接口：

```text
Search(ctx, query, limit) ([]SearchResult, error)
```

实现上可以先提供：
- 配置缺失时的不可用实现，稳定返回结构化错误。
- 测试用 fake client。
- 后续再接入真实搜索服务。

原因：
- 当前仓库不应为了工具调用闭环强绑定某个搜索供应商。
- 这样可以先完成协议、执行器和 ChatBot 集成测试，同时把真实外部搜索接入作为可替换实现。

## Risks / Trade-offs

- [Risk] 模型输出结构化工具调用可能不稳定。 -> Mitigation：解析失败时按普通文本处理或返回明确格式错误，并在测试中覆盖无效 JSON。
- [Risk] 工具结果回填后模型仍重复请求工具。 -> Mitigation：不继续执行第二个工具，直接基于已有 `ToolResult` 生成兜底最终回答。
- [Risk] 文本协议无法像 provider 原生 tool calling 一样强制工具阶段。 -> Mitigation：首版通过 system prompt 约束、解析器和 fallback 保底，后续评估接入原生 tool schema。
- [Risk] `file_read` 可能读取敏感文件。 -> Mitigation：限制工作区、拒绝隐藏工具目录和密钥命名文件，限制文本类型与读取大小。
- [Risk] `web_search` 依赖外部服务，测试不稳定。 -> Mitigation：抽象 `SearchClient`，单元测试使用 fake client，真实服务只做可选配置。
- [Risk] 单轮一次工具调用能力有限。 -> Mitigation：这是有意边界，先保证闭环稳定，再逐步引入多工具规划。

## Migration Plan

1. 新增 `internal/tools` 类型、`ToolSpec`、执行器和四个工具实现。
2. 在 `internal/session` 中支持启动阶段向 system prompt 追加全局能力说明。
3. 在 `internal/app/chatbot.go` 接入工具执行器、工具说明固化和单次工具回填流程。
4. 在上层解析逻辑中支持识别结构化 `tool_call` 响应。
5. 为 `web_search` 增加最小配置和可替换搜索客户端。
6. 更新示例配置、README 和演进文档，明确支持工具和暂不支持能力。
7. 运行 `go test ./...` 验证工具、执行器和 ChatBot 集成行为。

## Open Questions

- `web_search` 首个真实实现使用哪个供应商或本地桥接方式；本次设计默认通过接口隔离，先不锁定供应商。
- `file_read` 超过大小限制时应直接失败还是返回截断内容；本次建议首版直接失败，避免上下文被意外污染。
