## MODIFIED Requirements

### Requirement: Built-in CLI commands
系统必须在交互循环中支持基础命令，以便退出程序、清空上下文和查看当前历史消息。

#### Scenario: Exit command
- **WHEN** 用户输入 `exit` 或 `quit`
- **THEN** 系统必须终止交互循环并正常退出程序

#### Scenario: Clear command
- **WHEN** 用户输入 `clear`
- **THEN** 系统必须重置当前会话中的滚动摘要和近期历史消息
- **THEN** 系统必须保留当前启动阶段渲染得到的 system prompt 作为新会话起点

#### Scenario: History command
- **WHEN** 用户输入 `history`
- **THEN** 系统必须按顺序输出当前会话中的 system prompt、滚动摘要和原始消息历史
- **THEN** 输出内容必须包含每条消息的角色和文本，并能区分 `summary` 与普通对话消息
