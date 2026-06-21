# agentlab

本仓库是一个基于 Golang 的 AI Agent 学习项目，目标是让 Agent 能力从简单到复杂逐步演进。

项目演进记录见 [docs/evolution.md](docs/evolution.md)。

当前已落地基础 CLI ChatBot，能力边界如下：
- 使用 `spf13/cobra` 提供命令行入口
- 支持 `system`、`user`、`assistant` 和内部 `summary` 语义消息
- 支持基于 OpenAI 兼容接口的 LLM 调用和 provider 原生 Tool Calling
- 仅从本地 `configs/config.yaml` 加载配置
- 支持通过 `chat.prompt.template`、`chat.prompt.role`、`chat.prompt.context` 生成 system prompt
- 启动阶段校验 Prompt 模板变量完整性
- 支持 `chat.context.*` 驱动的上下文预算控制、完整轮次裁剪和单份滚动摘要
- 支持受 `tools.max_steps` 约束的顺序多步 Tool Loop，内置 `calculator`、`time`、`file_read`、`web_search` 四个受控工具
- 支持 `exit`、`quit`、`clear`、`history` 交互命令

当前版本暂不包含文件写入、shell 命令执行、交互式授权、长期记忆、RAG、Workflow、Multi-Agent 和 Web UI。

## 运行方式

1. 复制 `configs/config.example.yaml` 为本地 `configs/config.yaml`
2. 按需修改 `configs/config.yaml`，将带 `{...}` 的占位值替换成真实配置
3. 运行：

```bash
go run ./cmd/chat run
```

可选参数：

```bash
go run ./cmd/chat run --config configs/config.yaml
```

示例配置文件见 `configs/config.example.yaml`。

### Prompt 配置示例

```yaml
chat:
  context:
    max_chars: 12000
    keep_recent_turns: 6
    summary_max_chars: 2400
    enable_rolling_summary: true
  prompt:
    template: |
      你是一个{{role.name}}。
      你的职责是：{{role.goal}}。
      默认使用{{context.language}}回答。
      回答风格：{{role.style}}。
    role:
      name: AI Agent 学习助理
      goal: 帮助用户逐步理解和构建 AI Agent 系统
      style: 简洁、准确、务实
    context:
      language: 中文
```

`chat.max_history_messages` 已移除，配置中如果仍包含该字段会在启动阶段直接报错。

### Tool Calling 边界

当前支持同一次用户请求内的顺序多步工具调用：

```yaml
tools:
  max_steps: 3
  tool_calling_mode: native
  web_search:
    enabled: false
    provider: ""
    endpoint: ""
```

`tools.max_steps` 缺失时默认值为 `3`，合法范围是 `1` 到 `10`。小于 `1` 或大于 `10` 会在启动阶段直接报错，避免模型在工具循环中无限调用或因为误配置导致成本失控。

`tools.tool_calling_mode` 支持：

- `native`：默认值。通过 OpenAI 兼容接口的原生 `tools`、assistant `tool_calls` 和 tool result 消息完成工具调用。
- `text_compat`：兼容不支持原生 Tool Calling 的 provider，继续使用 assistant content 中的 JSON 文本协议。

模式必须显式配置为上述值之一，非法值会在启动阶段报错。`native` 请求失败时不会自动切换到 `text_compat`，避免隐藏 provider 能力问题或产生额外模型请求。使用 OpenAI-compatible 第三方服务时，需要先确认其聊天补全接口支持 `tools` 和 `tool_calls`；不支持时应配置 `text_compat`。

- `calculator`：执行安全的基础数学表达式，不执行任意代码。
- `time`：返回当前日期、时间、时区和 Unix 时间戳。
- `file_read`：只读取工作区内允许范围的文本文件，拒绝 `.git/`、`.claude/`、`.codex/`、明显密钥文件、超大文件和非文本文件。
- `web_search`：通过可替换搜索客户端返回结构化搜索结果；未配置真实搜索客户端时返回结构化不可用错误，不影响 ChatBot 启动。

每一步最多执行一个工具调用；不支持并行工具调用，也不支持一次模型响应中请求多个工具。native 模式使用 provider tool call ID 关联 assistant 调用和 tool result；工具反馈只存在于本轮工作消息，不写入 `history`。

本仓库是学习项目，`logs/chat.log` 默认保留完整协议内容，便于查看 Agent 实际运行过程：

- `发送请求`：记录实际发送的完整 `request_body`，包括 messages、原生 tools、assistant tool calls 和 tool result。
- `收到响应`：记录 provider 返回的完整 `response_body`。
- `检测到工具调用`：记录完整 `tool_call_json`，包括调用 ID、工具名、参数和 reason。
- `工具执行完成`：记录完整 `tool_result_json`，包括 content、error 和 metadata。

工具循环日志同时记录 `request_id`、`tool_calling_mode`、`step`、`max_steps`、`tool_name`、`tool_call_key`、`cache_hit`、`success` 和 `error_code`。Authorization header 和 API Key 不会写入日志。

完整日志可能包含用户输入、文件内容、搜索结果和工具参数，只适合当前本地学习环境；如果后续用于生产，需要增加日志级别、开关、脱敏和截断策略。

暂不支持 `file_write`、`ask_user`、`shell_exec`、跨轮工具缓存、工具轨迹 UI 展示和复杂权限审批。

运行日志默认写入 `logs/chat.log`，不会直接输出到命令行交互界面。
