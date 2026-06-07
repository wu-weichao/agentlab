package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileReadReadsAllowedTextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := NewFileReadTool(dir, 1024)
	result := tool.Execute(context.Background(), map[string]any{"path": "notes.md"})
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Error)
	}
	if result.Content != "hello\n" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if result.Metadata["read_bytes"] != 6 {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestFileReadRejectsOutsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := NewFileReadTool(dir, 1024)
	result := tool.Execute(context.Background(), map[string]any{"path": outside})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Metadata["code"] != "path_not_allowed" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestFileReadRejectsBlockedDirectory(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := NewFileReadTool(dir, 1024)
	result := tool.Execute(context.Background(), map[string]any{"path": ".git/config"})
	if result.Success {
		t.Fatal("expected failure")
	}
}

func TestFileReadRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "large.txt"), []byte("123456"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := NewFileReadTool(dir, 3)
	result := tool.Execute(context.Background(), map[string]any{"path": "large.txt"})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Metadata["code"] != "file_too_large" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestFileReadRejectsNonTextFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{0xff, 0x00, 0x01}, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := NewFileReadTool(dir, 1024)
	result := tool.Execute(context.Background(), map[string]any{"path": "bin.dat"})
	if result.Success {
		t.Fatal("expected failure")
	}
}
