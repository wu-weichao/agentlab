package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"agentlab/internal/app"
	"agentlab/internal/config"
	"agentlab/internal/llm"
	"agentlab/internal/prompt"
	"agentlab/internal/session"

	"github.com/spf13/cobra"
)

const defaultLogPath = "logs/chat.log"

// main 是命令行程序入口，负责初始化日志并执行 Cobra 命令树。
func main() {
	if err := setupLogging(defaultLogPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: setup logging failed: %v\n", err)
		os.Exit(1)
	}

	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// newRootCommand 创建根命令，并挂载全局配置参数和子命令。
func newRootCommand() *cobra.Command {
	var configPath string

	rootCmd := &cobra.Command{
		Use:   "chat",
		Short: "AI Agent Lab ChatBot",
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "configs/config.yaml", "path to config file")
	rootCmd.AddCommand(newChatCommand(&configPath))

	return rootCmd
}

// newChatCommand 创建交互式聊天命令。
func newChatCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start an interactive chat session",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Printf("[cli] 启动聊天命令 config=%s", *configPath)

			cfg, err := config.Load(*configPath)
			if err != nil {
				return err
			}
			log.Printf(
				"[cli] 配置加载成功 provider=%s model=%s base_url=%s context_max_chars=%d keep_recent_turns=%d summary_max_chars=%d rolling_summary=%t",
				cfg.LLM.Provider,
				cfg.LLM.Model,
				cfg.LLM.BaseURL,
				cfg.Chat.Context.MaxChars,
				cfg.Chat.Context.KeepRecentTurns,
				cfg.Chat.Context.SummaryMaxChars,
				cfg.Chat.Context.EnableRollingSummary,
			)
			log.Printf("[cli] 开始渲染 prompt template variables=%d", len(cfg.Chat.Prompt.VariableMap()))

			renderedPrompt, err := prompt.Render(cfg.Chat.Prompt.Template, cfg.Chat.Prompt.VariableMap())
			if err != nil {
				log.Printf("[cli] prompt 渲染失败: %v", err)
				return fmt.Errorf("render prompt template: %w", err)
			}
			log.Printf("[cli] prompt 渲染成功 length=%d", len(renderedPrompt))

			client, err := buildClient(cfg)
			if err != nil {
				return err
			}

			sess := session.New(renderedPrompt)
			bot := app.NewChatBot(client, sess, cfg.Chat.Context)

			fmt.Println("AI Agent Lab ChatBot")
			fmt.Println("Type 'exit' or 'quit' to leave.")

			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Print("You: ")
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return err
					}
					fmt.Println()
					return nil
				}

				input := strings.TrimSpace(scanner.Text())
				if input == "" {
					continue
				}

				switch strings.ToLower(input) {
				case "exit", "quit":
					log.Printf("[cli] 收到退出命令")
					return nil
				case "clear":
					sess.Reset()
					log.Printf("[cli] 会话已清空")
					fmt.Println("Bot: session cleared")
					continue
				case "history":
					history := sess.History()
					log.Printf("[cli] 输出历史消息 count=%d", len(history))
					printHistory(history)
					continue
				}

				reply, err := bot.Send(context.Background(), input)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Bot error: %v\n", err)
					continue
				}

				fmt.Printf("Bot: %s\n", reply)
			}
		},
	}
}

// buildClient 根据配置创建具体的模型客户端实现。
func buildClient(cfg *config.Config) (llm.Client, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.LLM.Provider)) {
	case "openai":
		log.Printf("[cli] 初始化 OpenAI 兼容客户端")
		return llm.NewOpenAIClient(llm.OpenAIConfig{
			BaseURL:     cfg.LLM.BaseURL,
			APIKey:      cfg.LLM.APIKey,
			Model:       cfg.LLM.Model,
			Temperature: cfg.LLM.Temperature,
		}), nil
	default:
		return nil, errors.New("unsupported llm provider")
	}
}

// printHistory 按顺序打印当前会话中的所有消息。
func printHistory(messages []llm.Message) {
	for _, message := range messages {
		fmt.Printf("%s: %s\n", message.Role, message.Content)
	}
}

// setupLogging 将默认日志输出重定向到本地日志文件，避免打断终端交互。
func setupLogging(logPath string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	log.SetOutput(file)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.Printf("[bootstrap] 日志初始化完成 path=%s", logPath)
	return nil
}
