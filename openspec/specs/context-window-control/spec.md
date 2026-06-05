## ADDED Requirements

### Requirement: Build bounded context before each model request
系统必须在每次调用模型前，根据上下文预算构建受控消息列表，而不是无界传递全部会话历史。

#### Scenario: Request stays within budget
- **WHEN** 当前 system prompt、滚动摘要、近期消息和本轮用户输入的总长度未超过配置预算
- **THEN** 系统必须直接使用这些消息构建请求上下文
- **THEN** 系统不得触发历史裁剪或摘要更新

#### Scenario: Request exceeds budget
- **WHEN** 候选上下文总长度超过配置预算
- **THEN** 系统必须保持 system prompt 和当前用户输入不被裁剪
- **THEN** 系统必须进入旧消息整轮裁剪流程以缩减上下文

### Requirement: Evict old history by complete turns
系统必须按完整对话轮次裁剪旧历史，并尽量保留最近若干轮原始 `user/assistant` 对话。

#### Scenario: Preserve recent turns
- **WHEN** 系统因预算超限开始裁剪历史
- **THEN** 系统必须优先保留配置指定数量的最近完整轮次
- **THEN** 系统不得拆开单个 `user/assistant` 轮次只保留其中一部分

#### Scenario: Degrade preferred recent window when still over budget
- **WHEN** 当前上下文即使保留到配置指定的最近轮次数量仍然超出预算
- **THEN** 系统必须允许继续缩小“最近保留轮次”窗口
- **THEN** 系统仍必须至少保留最近 1 轮完整对话以及当前用户输入

#### Scenario: Evict oldest eligible turns first
- **WHEN** 仍有可裁剪的旧原始轮次且预算仍超限
- **THEN** 系统必须按从旧到新的顺序移出完整轮次
- **THEN** 被移出的轮次必须作为后续滚动摘要更新的输入

### Requirement: Maintain a single rolling summary for evicted history
系统必须仅维护一份滚动摘要，用于承载已经离开近期原文窗口的旧历史事实、约束、决策和未决问题。

#### Scenario: Create or update summary after eviction
- **WHEN** 旧原始消息因预算问题被移出近期窗口且启用了滚动摘要
- **THEN** 系统必须基于旧摘要和本次被移出的消息，通过 LLM 生成新的结构化摘要
- **THEN** 新摘要必须覆盖用户目标、已确认约束、已做决策、关键事实和未解决问题等结构化内容

#### Scenario: Re-compress oversized summary
- **WHEN** 新生成的滚动摘要长度超过配置的摘要上限
- **THEN** 系统必须通过二次摘要压缩继续缩短该摘要
- **THEN** 系统仍必须保证会话中最多只存在一份滚动摘要

#### Scenario: Re-compress existing summary when rebuild still exceeds budget
- **WHEN** 插入新的滚动摘要后，请求上下文仍然超出总预算
- **THEN** 系统必须继续压缩已有滚动摘要并重试一次上下文构建
- **THEN** 若重试成功，系统必须使用压缩后的摘要继续本轮请求

### Requirement: Fail safely when minimum context still cannot fit
系统必须在最小必保留上下文仍超出预算时返回明确错误，而不是发送不可控请求或破坏会话状态。

#### Scenario: Minimum retained context still exceeds budget
- **WHEN** system prompt、当前用户输入、单份滚动摘要和最小近期轮次已经是最小集合但仍超出预算
- **THEN** 系统必须返回明确的上下文预算错误
- **THEN** 系统不得为该轮失败请求追加 assistant 消息

### Requirement: Record context control diagnostics
系统必须记录上下文预算命中、裁剪和摘要更新的关键诊断信息，便于本地观察运行时行为。

#### Scenario: Log context management summary
- **WHEN** 系统在一次对话请求前完成上下文构建
- **THEN** 系统必须记录预算值、估算长度、是否触发裁剪、被移出的轮次数量和摘要是否更新
- **THEN** 系统不得把这些调试日志直接混入交互式聊天输出

#### Scenario: Correlate logs across one request
- **WHEN** 系统处理一次用户输入并触发上下文裁剪、摘要更新或模型调用
- **THEN** 系统必须为该次请求分配稳定的 `request_id`
- **THEN** ChatBot、摘要器和 LLM 客户端日志必须共享同一个 `request_id`
