package main

import (
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
	ctx, sessions, mu := testSession(root)
	h := wrapTextHandler(handleRead(sessions, mu), formatReadResult)

	// Attempt to read path outside the root to force an error.
	res, err := h(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"path": "../outside"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected error result, got %v", res)
	}
}

// Test that wrapTextHandler returns an error result when argument binding fails.
func TestWrapTextHandlerBindingError(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	h := wrapTextHandler(handleRead(sessions, mu), formatReadResult)

	// Provide invalid argument type to trigger binding error.
	res, err := h(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"path": 123}},
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
	ctx, sessions, mu := testSession(root)
	h := wrapStructuredHandler(handleRead(sessions, mu))
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": "f.txt"}}}
	res, err := h(ctx, req)
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

// Test that wrapStructuredHandler returns an error result when argument binding fails.
func TestWrapStructuredHandlerBindingError(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	h := wrapStructuredHandler(handleRead(sessions, mu))
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": 123}}}
	res, err := h(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected error result, got %v", res)
	}
}
