package requestctx

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

type requestIDKey struct{}

var requestCounter atomic.Uint64

// WithNewRequestID 返回带有新 request_id 的上下文和 request_id 本身。
func WithNewRequestID(ctx context.Context) (context.Context, string) {
	id := fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), requestCounter.Add(1))
	return context.WithValue(ctx, requestIDKey{}, id), id
}

// FromContext 提取 request_id；如果不存在则返回空字符串。
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
