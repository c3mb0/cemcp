//go:build go1.18
// +build go1.18

package main

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// FuzzHandleWrite ensures arbitrary inputs do not cause panics.
func FuzzHandleWrite(f *testing.F) {
	f.Add("f.txt", []byte("seed"))
	f.Fuzz(func(t *testing.T, path string, data []byte) {
		root := t.TempDir()
		h := handleWrite(root)
		_, _ = h(context.Background(), mcp.CallToolRequest{}, WriteArgs{
			Path:    path,
			Content: string(data),
		})
	})
}
