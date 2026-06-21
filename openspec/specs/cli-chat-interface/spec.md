## Purpose

定义命令行聊天入口、持续输入循环、退出与历史管理命令、错误处理方式以及用户可见输出和本地日志之间的行为边界。

## Requirements

### Requirement: Interactive CLI chat loop
系统 MUST 基于 `spf13/cobra` 提供命令行聊天入口，使用户可以持续输入文本并接收模型回复，直到显式退出。

#### Scenario: Start interactive session
- **WHEN** 用户通过 Cobra 注册的聊天命令启动程序
- **THEN** 系统必须显示 ChatBot 标题和退出提示
- **THEN** 系统必须进入可持续读取用户输入的交互循环

#### Scenario: Build CLI command tree
- **WHEN** 程序初始化命令行入口
- **THEN** 系统必须使用 `spf13/cobra` 定义根命令和聊天相关命令
- **THEN** 聊天命令必须可作为统一 CLI 入口的一部分被执行

#### Scenario: Send message and display reply
- **WHEN** 用户输入非空普通文本
- **THEN** 系统必须将输入交给聊天运行时处理
- **THEN** 系统必须将模型返回内容输出给用户

### Requirement: Built-in CLI commands
系统 MUST 在交互循环中支持基础命令，以便退出程序、清空上下文和查看当前历史消息。

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

### Requirement: CLI logs do not interrupt chat output
系统 MUST 将调试和请求日志写入本地日志文件，而不是直接混入命令行对话输出。

#### Scenario: Write logs to local file
- **WHEN** 用户启动聊天命令
- **THEN** 系统必须初始化本地日志文件用于接收标准日志输出
- **THEN** 终端交互界面不得直接显示调试日志内容
