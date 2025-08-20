package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Test that wrapTextHandler propagates errors correctly when compat mode is enabled.
func TestCompatWrapTextHandlerPropagatesErrors(t *testing.T) {
	orig := *compatFlag
	*compatFlag = true
	t.Cleanup(func() { *compatFlag = orig })

	root := t.TempDir()
	h := wrapTextHandler(handleRead(root), formatReadResult)

	// Attempt to read path outside the root to force an error.
	res, err := h(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"path": "../outside"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected error result, got %v", res)
	}
}

func TestStructuredHandlerOmitsTextContent(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "f.txt")
	if err := os.WriteFile(p, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := wrapStructuredHandler(handleRead(root))
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": "f.txt"}}}
	res, err := h(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StructuredContent == nil {
		t.Fatalf("expected structured content")
	}
	if len(res.Content) != 0 {
		t.Fatalf("expected no text content, got %v", res.Content)
	}
}
