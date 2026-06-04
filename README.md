# agentlab

本仓库是一个基于 Golang 的 AI Agent 学习项目，目标是让 Agent 能力从简单到复杂逐步演进。

项目演进记录见 [docs/evolution.md](docs/evolution.md)。

当前已落地基础 CLI ChatBot，能力边界如下：
- 使用 `spf13/cobra` 提供命令行入口
- 支持 `system`、`user`、`assistant` 三类消息
- 支持基于 OpenAI 兼容接口的 LLM 调用
- 仅从本地 `configs/config.yaml` 加载配置
- 支持通过 `chat.prompt.template`、`chat.prompt.role`、`chat.prompt.context` 生成 system prompt
- 启动阶段校验 Prompt 模板变量完整性
- 支持 `exit`、`quit`、`clear`、`history` 交互命令

当前版本暂不包含 Tool Calling、RAG、长期记忆、Workflow、Multi-Agent 和 Web UI。

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
  max_history_messages: 20
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

运行日志默认写入 `logs/chat.log`，不会直接输出到命令行交互界面。
