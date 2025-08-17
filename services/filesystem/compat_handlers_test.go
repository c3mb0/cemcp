package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	if err == nil {
		t.Fatalf("expected error, got nil (res=%v)", res)
	}
	if res != nil {
		t.Fatalf("expected nil result on error, got %v", res)
	}
}

// Test that wrapStructuredHandler propagates errors and returns no result.
func TestStructuredHandlerPropagatesErrors(t *testing.T) {
	root := t.TempDir()
	h := wrapStructuredHandler(handleRead(root))

	// Attempt to read path outside the root to force an error.
	res, err := h(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"path": "../outside"}},
	})
	if err == nil {
		t.Fatalf("expected error, got nil (res=%v)", res)
	}
	if res != nil {
		t.Fatalf("expected nil result on error, got %v", res)
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
	if res.Content == nil {
		t.Fatalf("expected empty content slice")
	}
	if len(res.Content) != 0 {
		t.Fatalf("expected no text content, got %v", res.Content)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\"content\":[]") {
		t.Fatalf("expected JSON content array to be empty, got %s", data)
	}
}
