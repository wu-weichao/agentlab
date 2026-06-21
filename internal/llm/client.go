package llm

import "context"

type Client interface {
	Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error)
}
