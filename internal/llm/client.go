package llm

import "context"

type Client interface {
	Chat(ctx context.Context, messages []Message) (*ChatResponse, error)
}
