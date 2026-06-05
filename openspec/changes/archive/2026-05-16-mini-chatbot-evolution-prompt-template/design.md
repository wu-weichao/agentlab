## Context

当前仓库已具备最小可运行的 CLI ChatBot 闭环：`CLI -> ChatBot -> Session -> LLMClient`。在 `v0.0.1` 中，`chat.system_prompt` 是一个静态字符串，配置层只负责读取它，会话层只负责把它作为第一条 `system` 消息写入内存。

`v0.0.2` 需要把 Prompt 升级为真正的运行时能力，但仍然保持“先简单、后复杂”的演进原则。这意味着第二版只解决 Prompt 模板、角色设定和参数化渲染，不提前引入动态角色切换、复杂模板语法或多 Prompt 管理。

本次变更同时带有一个明确约束：不兼容旧字段 `chat.system_prompt`。也就是说，新版本配置必须使用 `chat.prompt.*`，避免同一版本中同时维护两套 Prompt 来源。

## Goals / Non-Goals

**Goals:**
- 用结构化配置替代静态 `chat.system_prompt`。
- 支持通过模板变量生成最终 system prompt。
- 在启动阶段校验模板变量是否完整且非空。
- 保持 `CLI -> Session -> LLMClient` 主链路不变。
- 让 `clear` 后的会话恢复到同一个渲染结果。

**Non-Goals:**
- 不支持条件、循环、include 等复杂模板语法。
- 不支持运行时修改角色、语言或上下文参数。
- 不支持多模板切换、模板版本管理或用户级个性化 Prompt。
- 不引入 Tool Calling、长期记忆、RAG 或 Workflow。

## Decisions

### 1. 使用新的 `chat.prompt` 结构，直接替代 `chat.system_prompt`

配置结构调整为：

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

原因：
- 第二版的目标是建立新的 Prompt Runtime，而不是迁就旧结构。
- 取消双路径后，配置来源唯一，校验逻辑更简单。
- 避免用户混用 `chat.system_prompt` 与 `chat.prompt.template` 导致行为不确定。

备选方案：
- 保留 `chat.system_prompt` 兼容。
  缺点是会把第二版的配置校验和文档复杂化，也不符合本次明确约束，因此不采用。

### 2. Prompt 渲染发生在配置完成之后、会话创建之前

推荐链路：

```text
Config Load
  -> Validate
  -> Build Prompt Variables
  -> Render Prompt
  -> Session.New(renderedPrompt)
  -> ChatBot
```

原因：
- `session` 只负责消息维护，不应该知道模板存在。
- `chatbot` 只负责对话编排，不应该承担模板渲染逻辑。
- 渲染后的结果仍然是普通字符串，能最大化复用现有运行链路。

备选方案：
- 在 `Session` 内部传入模板并动态渲染。
  会让会话层承担配置逻辑，职责混乱，因此不采用。

### 3. 新增独立 `internal/prompt` 模块

建议新增：

```text
internal/prompt/
  template.go
  template_test.go
```

模块职责：
- 解析模板中的变量引用。
- 校验变量是否齐全。
- 将变量替换为最终 system prompt。

原因：
- Prompt 渲染是独立关注点，不应塞进 `config`、`session` 或 `app`。
- 后续若扩展更多模板变量或渲染规则，可以在该模块内演进。

备选方案：
- 将渲染逻辑全部写进 `internal/config/config.go`。
  短期代码更少，但职责会混合，后续扩展成本更高，因此不采用。

### 4. 只支持点路径变量和纯替换语义

第二版变量格式固定为：

```text
{{role.name}}
{{role.goal}}
{{role.style}}
{{context.language}}
```

原因：
- 点路径与结构化配置天然对应。
- 比环境变量风格 `${FOO}` 更适合表达 Prompt 模板。
- 纯替换语义足够支撑第二版目标，避免过早设计模板 DSL。

备选方案：
- 使用 Go `text/template`。
  虽然功能更强，但会天然暴露条件、循环等能力，超出第二版边界，因此不采用。

### 5. 缺失变量和空值都视为启动配置错误

校验规则：
- `chat.prompt.template` 为空时报错。
- 模板中引用未提供变量时报错。
- 标准变量值为空时报错。

原因：
- Prompt 渲染结果必须稳定可预测。
- 静默留空会让模型行为漂移，排查困难。

备选方案：
- 缺失变量时替换为空字符串并继续启动。
  用户体验表面上更“宽容”，但运行期风险更高，因此不采用。

## Risks / Trade-offs

- [Risk] 移除 `chat.system_prompt` 是 breaking change，已有本地配置会失效。 -> Mitigation：在 proposal、spec、README 和示例配置中明确迁移方式，只保留新结构。
- [Risk] 自定义模板变量范围过小，可能不足以表达更复杂角色设定。 -> Mitigation：第二版先固定最小变量集合，后续按真实需求扩展字段。
- [Risk] 自研最小模板替换器能力有限，后续若要支持条件逻辑需要额外演进。 -> Mitigation：第二版明确边界，只实现纯替换；若将来复杂度上升，再评估切换模板引擎。
- [Risk] Prompt 渲染新增模块和校验路径，增加启动阶段失败场景。 -> Mitigation：补齐单元测试和清晰错误信息，确保失败原因直接可读。

## Migration Plan

这是一个本地开发版本演进，不涉及线上迁移。落地步骤为：

1. 更新 `config.example.yaml` 与 README，使用新的 `chat.prompt.*` 结构。
2. 调整配置解析与校验逻辑，删除对 `chat.system_prompt` 的依赖。
3. 实现 Prompt 渲染模块并在启动阶段接入。
4. 让 `Session` 仅接收渲染后的 prompt 字符串。
5. 补充测试并运行 `go test ./...`。

回退方式：
- 如果第二版实现不稳定，可回退整个变更；由于没有线上状态和数据迁移，不涉及持久化回滚。

## Open Questions

- 第二版是否允许 `chat.prompt.context` 在后续扩展为任意 key-value，而不仅是固定字段？当前建议先固定结构。
- `history` 命令未来是否需要显示当前渲染后的 system prompt 来源摘要？当前版本不需要。
- 如果后续引入短期记忆裁剪，是否需要将 Prompt 渲染元数据保留在 session 外部对象中？当前版本不需要。
