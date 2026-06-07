package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FileReadTool 读取工作区内允许范围的文本文件。
// 首版只做读取，不做写入或授权交互；路径和大小限制都在工具内完成。
type FileReadTool struct {
	workspaceRoot string
	maxBytes      int64
}

// NewFileReadTool 创建 file_read 工具，并固定工作区根目录与单次读取大小上限。
func NewFileReadTool(workspaceRoot string, maxBytes int64) *FileReadTool {
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = "."
	}
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		absRoot = workspaceRoot
	}
	if maxBytes <= 0 {
		maxBytes = DefaultFileReadMaxBytes
	}
	return &FileReadTool{
		workspaceRoot: filepath.Clean(absRoot),
		maxBytes:      maxBytes,
	}
}

func (t *FileReadTool) Name() string {
	return "file_read"
}

func (t *FileReadTool) Description() string {
	return "读取工作区内允许范围的文本文件"
}

func (t *FileReadTool) Parameters() []Parameter {
	return []Parameter{{
		Name:        "path",
		Type:        "string",
		Required:    true,
		Description: "工作区内文本文件路径",
	}}
}

// Execute 在安全边界校验通过后读取文本文件内容。
func (t *FileReadTool) Execute(_ context.Context, args map[string]any) ToolResult {
	path, ok := requiredString(args, "path")
	if !ok {
		return errorResult("path is required", map[string]any{"code": "invalid_arguments"})
	}

	resolved, err := t.resolveAllowedPath(path)
	if err != nil {
		return errorResult(err.Error(), map[string]any{"code": "path_not_allowed"})
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return errorResult("stat file: "+err.Error(), map[string]any{
			"code": "file_not_found",
			"path": resolved,
		})
	}
	if info.IsDir() {
		return errorResult("path is a directory", map[string]any{"code": "not_file", "path": resolved})
	}
	if info.Size() > t.maxBytes {
		return errorResult("file exceeds read limit", map[string]any{
			"code":       "file_too_large",
			"path":       resolved,
			"bytes":      info.Size(),
			"max_bytes":  t.maxBytes,
			"file_name":  info.Name(),
			"read_bytes": 0,
		})
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return errorResult("read file: "+err.Error(), map[string]any{"code": "read_failed", "path": resolved})
	}
	if !isText(data) {
		return errorResult("file is not valid text", map[string]any{
			"code": "non_text_file",
			"path": resolved,
		})
	}

	return successResult(string(data), map[string]any{
		"path":       resolved,
		"bytes":      len(data),
		"max_bytes":  t.maxBytes,
		"file_name":  info.Name(),
		"read_bytes": len(data),
	})
}

func (t *FileReadTool) resolveAllowedPath(path string) (string, error) {
	// 先把相对路径绑定到 workspaceRoot，再用 filepath.Rel 判断是否逃逸工作区。
	// 这种方式同时覆盖 ../ 路径和绝对路径输入。
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		cleaned = filepath.Join(t.workspaceRoot, cleaned)
	}
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return "", err
	}
	absPath = filepath.Clean(absPath)

	rel, err := filepath.Rel(t.workspaceRoot, absPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path is outside workspace")
	}
	if hasBlockedPathPart(rel) || looksSensitiveFile(filepath.Base(rel)) {
		return "", fmt.Errorf("path is not allowed")
	}
	return absPath, nil
}

func hasBlockedPathPart(rel string) bool {
	// 拒绝 Agent 工具目录和版本库目录，避免模型通过 file_read 读取内部控制信息。
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		switch strings.ToLower(part) {
		case ".git", ".claude", ".codex":
			return true
		}
	}
	return false
}

func looksSensitiveFile(name string) bool {
	lower := strings.ToLower(name)
	if lower == ".env" || strings.HasPrefix(lower, ".env.") {
		return true
	}
	sensitiveTokens := []string{"secret", "token", "credential", "private_key", "apikey", "api_key"}
	for _, token := range sensitiveTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func isText(data []byte) bool {
	// 首版只接受 UTF-8 文本；NUL 字节通常意味着二进制文件，应避免注入上下文。
	if len(data) == 0 {
		return true
	}
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return utf8.Valid(data)
}
