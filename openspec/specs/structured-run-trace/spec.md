## Purpose

定义单次 Agent 运行的结构化结果、步骤轨迹、稳定错误分类、日志关联和数据边界。

## Requirements

### Requirement: Return a structured result for each Agent run
系统 MUST 提供单次 Agent 运行的结构化结果，包含 request ID、最终回答、按发生顺序排列的步骤、总步骤数、总耗时和稳定终止原因。

#### Scenario: Ordinary answer returns a complete result
- **WHEN** Agent 无需工具即可成功生成普通文本回答
- **THEN** 结构化结果 MUST 包含该最终回答
- **THEN** 结构化结果 MUST 包含本次运行的 request ID、非负总耗时和与步骤数组长度一致的总步骤数
- **THEN** 终止原因 MUST 为稳定的完成分类

#### Scenario: Failed run returns partial trace
- **WHEN** Agent 在已经完成部分步骤后因模型、工具协议或最大步数错误而终止
- **THEN** 结构化入口 MUST 同时返回原始 Go error 和已收集的部分运行结果
- **THEN** 运行结果 MUST 保留失败前已经发生的步骤
- **THEN** 终止原因 MUST 使用稳定分类而不是要求调用方匹配错误字符串

### Requirement: Represent runtime events as ordered typed steps
系统 MUST 使用有序 `RunStep` 表示模型调用、真实工具调用、缓存复用和最终回答，并为每个步骤提供稳定关联与状态字段。

#### Scenario: Record model and final answer steps
- **WHEN** 一次运行通过一次模型调用直接得到最终回答
- **THEN** 轨迹 MUST 先包含 `model_call` 步骤，再包含 `final_answer` 步骤
- **THEN** 每个步骤 MUST 包含从 1 开始的顺序序号、request ID、在该请求内唯一的 step ID、非负耗时和成功状态

#### Scenario: Record real tool execution
- **WHEN** 模型请求的工具调用未命中本轮 runCache 并执行真实工具
- **THEN** 对应步骤类型 MUST 为 `tool_call`
- **THEN** 步骤 MUST 包含工具名、tool call key、`cache_hit=false`、执行耗时、成功状态和稳定工具错误代码

#### Scenario: Record cached tool reuse
- **WHEN** 模型请求的工具调用命中本轮 runCache
- **THEN** 对应步骤类型 MUST 为 `cache_reuse`
- **THEN** 步骤 MUST 包含工具名、tool call key、`cache_hit=true` 和非负耗时
- **THEN** 系统 MUST NOT 把缓存复用记录成一次真实工具执行

#### Scenario: Record sequential tool flow in occurrence order
- **WHEN** 一次运行依次发生模型工具请求、工具执行、后续模型调用和最终回答
- **THEN** 轨迹 MUST 按实际发生顺序记录所有对应步骤
- **THEN** 总步骤数 MUST 等于步骤数组长度

### Requirement: Expose stable termination and error classifications
系统 MUST 为运行终止和失败步骤提供稳定机器可读分类，并保持原始错误通过 Go error 返回。

#### Scenario: Classify max steps termination
- **WHEN** 模型达到配置的最大工具循环步数后仍未返回最终文本
- **THEN** 运行 MUST 返回现有最大步数错误
- **THEN** 结构化结果的终止原因 MUST 为稳定的 `max_steps_exceeded` 分类

#### Scenario: Classify model failure
- **WHEN** 模型调用返回错误
- **THEN** 对应 `model_call` 步骤 MUST 标记失败并包含稳定模型错误分类
- **THEN** 运行结果的终止原因 MUST 表示模型失败

#### Scenario: Tool business failure can still complete
- **WHEN** ToolExecutor 返回 `success=false` 的结构化工具结果且 Runtime 能继续把该结果反馈给模型
- **THEN** 工具步骤 MUST 保留失败状态和稳定工具错误代码
- **THEN** 如果模型随后生成最终文本，运行终止原因 MUST 仍为完成分类

### Requirement: Separate tool content from diagnostics
系统 MUST 在工具步骤中分别提供工具结果正文和诊断 metadata，并避免不同步骤共享可变诊断数据。

#### Scenario: Return tool body and diagnostics separately
- **WHEN** 工具执行或缓存复用产生结构化 ToolResult
- **THEN** 对应步骤 MUST 分别暴露工具正文、成功状态、错误分类和诊断 metadata
- **THEN** 诊断 metadata MUST 包含当前步骤的 cache hit 和 tool call key 信息

#### Scenario: Cached metadata does not mutate prior steps
- **WHEN** 同一工具结果在后续步骤命中 runCache 并更新当前步骤诊断信息
- **THEN** 先前工具步骤中已经记录的 metadata MUST NOT 被后续步骤修改

### Requirement: Correlate structured trace with runtime logs
系统 MUST 让结构化轨迹和运行日志使用同一 request ID 与 step ID，同时保持既有工具循环 step 字段语义。

#### Scenario: Correlate model and tool events
- **WHEN** 系统记录模型调用、工具执行、缓存复用或最终回答日志
- **THEN** 日志 MUST 包含对应结构化步骤的 request ID 和 step ID
- **THEN** 现有工具循环 `step` 和 `max_steps` MUST 继续表示工具循环轮次

### Requirement: Keep run trace ephemeral and reasoning-safe
结构化运行轨迹 MUST 只描述当前运行，不得写入会话历史或暴露模型隐藏推理。

#### Scenario: Trace is excluded from Session
- **WHEN** 一次带工具的结构化运行成功完成
- **THEN** Session MUST 继续只写入用户消息和最终 assistant 回答
- **THEN** 模型步骤、工具步骤、cache metadata 和终止信息 MUST NOT 写入 recent messages 或 rolling summary

#### Scenario: Hidden reasoning is excluded
- **WHEN** provider 响应包含 reasoning、thinking 或其他非用户可见推理字段
- **THEN** `RunStep` 和 `RunResult` MUST NOT 暴露这些字段
- **THEN** 最终回答步骤 MUST 只包含用户可见的 assistant content

#### Scenario: Trace is not persisted automatically
- **WHEN** 结构化运行结束
- **THEN** Agent MUST NOT 自动把轨迹写入文件、数据库、指标平台或外部 tracing 系统
