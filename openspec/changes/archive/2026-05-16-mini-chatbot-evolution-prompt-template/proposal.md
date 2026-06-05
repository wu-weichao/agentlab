## Why

`v0.0.1` 仍然把 system prompt 作为静态字符串配置，无法结构化表达角色、目标、语言和风格，也缺少模板变量完整性校验。进入 `v0.0.2` 后，需要把 Prompt 升级为可配置、可渲染、可校验的运行时能力，为后续记忆、工具调用和工作流演进打基础。

## What Changes

- 新增 Prompt 模板能力，支持通过 `chat.prompt.template` 定义 system prompt 模板。
- 新增结构化角色设定与上下文参数配置，支持 `chat.prompt.role` 与 `chat.prompt.context`。
- 新增启动阶段的 Prompt 模板变量校验与渲染逻辑，缺失变量或空值时启动失败。
- **BREAKING** 移除 `chat.system_prompt` 旧配置字段，不提供向后兼容。
- 调整会话初始化行为，改为使用渲染后的 system prompt 创建新会话并在 `clear` 后恢复同一结果。
- 补充 Prompt 渲染、配置校验与会话恢复的测试和文档。

## Capabilities

### New Capabilities
- `prompt-template-runtime`: 定义 Prompt 模板、结构化变量、模板渲染与变量校验行为。

### Modified Capabilities
- `llm-provider-configuration`: 聊天运行时配置从静态 `chat.system_prompt` 调整为结构化 `chat.prompt` 配置，并移除旧字段兼容。
- `chat-session-runtime`: 会话初始化和重置行为改为使用渲染后的 system prompt，而不是静态配置字符串。

## Impact

- 受影响代码包括 `internal/config/`、`internal/session/`、`internal/app/`、`cmd/chat/`、`configs/`、`README.md` 和新增的 `internal/prompt/`。
- 需要调整 `configs/config.example.yaml` 的配置结构。
- 需要补充模板渲染、变量校验、会话初始化和 `clear` 行为相关测试。
