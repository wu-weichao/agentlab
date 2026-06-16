# Repository Guidelines

## 项目结构与模块组织
本仓库是一个基于 Golang 的 AI Agent 学习项目，目标是让 Agent 能力从简单到复杂逐步演进。当前仓库以 OpenSpec 规范驱动为主，核心目录如下：

- `openspec/config.yaml`：OpenSpec 全局配置。
- `openspec/specs/`：已经稳定的能力规范与行为定义。
- `openspec/changes/`：进行中的变更提案、设计说明与任务拆解。
- `openspec/changes/archive/`：已完成并归档的变更。

后续如引入 Go 代码，建议优先采用清晰分层结构，例如 `cmd/`、`internal/agent/`、`internal/tools/`、`internal/memory/`、`tests/`。`.codex/` 与 `.claude/` 仅在维护本地 Agent 工具时修改。

## 构建、测试与开发命令
当前主要工作流是规范优先，常用命令如下：

- `rg --files openspec`：快速列出规范与变更文件。
- `Get-ChildItem -Recurse openspec`：查看当前规范结构。
- `Get-Content openspec\config.yaml`：检查 OpenSpec 配置。
- `go fmt ./...`：格式化 Go 代码。
- `go test ./...`：运行全部 Go 测试。
- `go run ./cmd/basic-agent`：运行某个示例 Agent，路径按实际模块调整。

新增命令时，应在对应模块附近补充说明。

## 编码风格与命名规范
Go 代码遵循默认风格：使用 `gofmt`，缩进使用 tab，包名简短明确，导出标识符使用 PascalCase，非导出标识符使用 camelCase。文件名按职责命名，例如 `planner.go`、`tool_executor.go`、`memory_store.go`。

- 规范文档使用简洁中文描述，标题明确，避免空泛表述。OpenSpec 变更目录使用 kebab-case，例如 `openspec/changes/add-basic-reasoning-loop`。
- 学习项目，核心模块需要确保代码注释，核心执行节点需要保证日志记录

## 测试规范
优先使用 Go 原生测试框架，测试文件命名为 `*_test.go`，测试函数命名为 `TestXxx`。推荐采用表驱动测试，重点覆盖以下内容：

- 输入解析与参数校验。
- 规划逻辑与状态流转。
- 工具调用边界与错误处理。
- 多步 Agent 流程中的失败回退。

提交前至少确保新增能力具备基本可执行测试；如果暂时无法自动化验证，需要在变更说明中写清原因。

## 提交与合并请求规范
当前工作区没有可用的 `.git` 历史，因此提交信息采用简洁祈使句即可，例如 `新增基础 Agent 循环`、`补充规划器设计规范`。每个提交和 PR 应聚焦单一能力演进步骤，避免一次引入过多概念。

PR 说明建议包含：

- 本次新增的 Agent 能力或学习阶段。
- 涉及的 `openspec/` 与 Go 代码路径。
- 测试结果，或暂未补测的原因。
- 后续待办与已知限制。

## Agent 协作规则
- 默认使用中文编写文档、注释、说明与回复；如需英文，需在任务中明确说明。
- 开发顺序应遵循“先简单、后复杂”的演进原则，优先完成最小可运行版本。
- 新增能力前，先在 `openspec/changes/` 中补充提案、设计或任务，再进入实现。
- 避免过早抽象；只有当同类逻辑重复出现时再提炼公共模块。
- 若实现依赖外部模型、工具或环境变量，应在相关文档中写明接入方式与最小配置要求。
- `docs/evolution.md` 版本描述要包含能力、实现思路、可观测性、边界、下一步。
