package main

import (
	"context"
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
