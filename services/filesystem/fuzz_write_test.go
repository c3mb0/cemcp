//go:build go1.18
// +build go1.18

package main

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// FuzzHandleWrite ensures arbitrary inputs do not cause panics.
func FuzzHandleWrite(f *testing.F) {
	f.Add("f.txt", []byte("seed"))
	f.Fuzz(func(t *testing.T, path string, data []byte) {
		root := t.TempDir()
		ctx, sessions, mu := testSession(root)
		h := handleWrite(sessions, mu)
		_, _ = h(ctx, mcp.CallToolRequest{}, WriteArgs{
			Path:    path,
			Content: string(data),
		})
	})
}
