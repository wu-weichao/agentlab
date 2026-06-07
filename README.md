# agentlab

本仓库是一个基于 Golang 的 AI Agent 学习项目，目标是让 Agent 能力从简单到复杂逐步演进。

项目演进记录见 [docs/evolution.md](docs/evolution.md)。

当前已落地基础 CLI ChatBot，能力边界如下：
- 使用 `spf13/cobra` 提供命令行入口
- 支持 `system`、`user`、`assistant` 和内部 `summary` 语义消息
- 支持基于 OpenAI 兼容接口的 LLM 调用
- 仅从本地 `configs/config.yaml` 加载配置
- 支持通过 `chat.prompt.template`、`chat.prompt.role`、`chat.prompt.context` 生成 system prompt
- 启动阶段校验 Prompt 模板变量完整性
- 支持 `chat.context.*` 驱动的上下文预算控制、完整轮次裁剪和单份滚动摘要
- 支持最小 Tool Calling 闭环，内置 `calculator`、`time`、`file_read`、`web_search` 四个受控工具
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

当前只支持单轮一次工具调用：
- `calculator`：执行安全的基础数学表达式，不执行任意代码。
- `time`：返回当前日期、时间、时区和 Unix 时间戳。
- `file_read`：只读取工作区内允许范围的文本文件，拒绝 `.git/`、`.claude/`、`.codex/`、明显密钥文件、超大文件和非文本文件。
- `web_search`：通过可替换搜索客户端返回结构化搜索结果；未配置真实搜索客户端时返回结构化不可用错误，不影响 ChatBot 启动。

暂不支持 `file_write`、`ask_user`、`shell_exec`、并行工具调用、多轮递归工具调用和复杂权限审批。

运行日志默认写入 `logs/chat.log`，不会直接输出到命令行交互界面。
