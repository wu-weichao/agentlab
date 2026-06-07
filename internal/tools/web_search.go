package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrSearchUnavailable 表示未配置真实搜索客户端。
var ErrSearchUnavailable = errors.New("web search is not configured")

// SearchResult 是 web_search 返回给 Agent 的单条来源感知结果。
type SearchResult struct {
	Title       string
	Snippet     string
	URL         string
	Source      string
	PublishedAt string
}

// SearchClient 抽象真实搜索供应商。
// 工具执行器只依赖该接口，避免把项目强绑定到某个外部服务。
type SearchClient interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

// UnavailableSearchClient 是未配置搜索服务时的默认实现。
// 它让 ChatBot 可以正常启动，并在真正调用 web_search 时返回结构化错误。
type UnavailableSearchClient struct{}

func (UnavailableSearchClient) Search(_ context.Context, _ string, _ int) ([]SearchResult, error) {
	return nil, ErrSearchUnavailable
}

// WebSearchTool 执行来源感知的外部搜索。
// 首版只做查询和结果格式化，不做网页抓取或事实验证。
type WebSearchTool struct {
	client       SearchClient
	defaultLimit int
}

// NewWebSearchTool 创建 web_search 工具。
func NewWebSearchTool(client SearchClient, defaultLimit int) *WebSearchTool {
	if client == nil {
		client = UnavailableSearchClient{}
	}
	if defaultLimit <= 0 {
		defaultLimit = DefaultSearchLimit
	}
	return &WebSearchTool{
		client:       client,
		defaultLimit: defaultLimit,
	}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "搜索外部信息并返回来源感知的结构化结果"
}

func (t *WebSearchTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "query",
			Type:        "string",
			Required:    true,
			Description: "搜索关键词",
		},
		{
			Name:        "limit",
			Type:        "integer",
			Required:    false,
			Description: "结果数量上限",
		},
	}
}

// Execute 调用 SearchClient，并把结果限制为可控数量后回填给模型。
func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	query, ok := requiredString(args, "query")
	if !ok {
		return errorResult("query is required", map[string]any{"code": "invalid_arguments"})
	}
	limit, ok := optionalInt(args, "limit", t.defaultLimit)
	if !ok {
		return errorResult("limit must be a positive integer", map[string]any{"code": "invalid_arguments"})
	}
	if limit > t.defaultLimit {
		// 模型可以提出 limit，但不能突破工具自身配置的上限。
		limit = t.defaultLimit
	}

	results, err := t.client.Search(ctx, query, limit)
	if err != nil {
		return errorResult(err.Error(), map[string]any{
			"code":  "search_failed",
			"query": query,
			"limit": limit,
		})
	}
	if len(results) > limit {
		results = results[:limit]
	}

	content := formatSearchResults(results)
	return successResult(content, map[string]any{
		"query":        query,
		"limit":        limit,
		"result_count": len(results),
		"results":      resultsToMetadata(results),
	})
}

func formatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "未找到搜索结果。"
	}

	var builder strings.Builder
	for i, result := range results {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(result.Title)))
		if strings.TrimSpace(result.Snippet) != "" {
			builder.WriteString("摘要: " + strings.TrimSpace(result.Snippet) + "\n")
		}
		if strings.TrimSpace(result.URL) != "" {
			builder.WriteString("URL: " + strings.TrimSpace(result.URL) + "\n")
		}
		if strings.TrimSpace(result.Source) != "" {
			builder.WriteString("来源: " + strings.TrimSpace(result.Source) + "\n")
		}
		if strings.TrimSpace(result.PublishedAt) != "" {
			builder.WriteString("时间: " + strings.TrimSpace(result.PublishedAt) + "\n")
		}
		if i < len(results)-1 {
			builder.WriteString("\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func resultsToMetadata(results []SearchResult) []map[string]any {
	metadata := make([]map[string]any, 0, len(results))
	for _, result := range results {
		metadata = append(metadata, map[string]any{
			"title":        result.Title,
			"snippet":      result.Snippet,
			"url":          result.URL,
			"source":       result.Source,
			"published_at": result.PublishedAt,
		})
	}
	return metadata
}
