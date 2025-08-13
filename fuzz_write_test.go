//go:build go1.18
// +build go1.18

package main

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// FuzzHandleWrite ensures arbitrary inputs don't trigger panics.
func FuzzHandleWrite(f *testing.F) {
	f.Add("f.txt", []byte("seed"), false)
	f.Fuzz(func(t *testing.T, path string, data []byte, useBase64 bool) {
		root := t.TempDir()
		h := handleWrite(root)
		enc := string(encText)
		content := string(data)
		if useBase64 {
			enc = string(encBase64)
			content = base64.StdEncoding.EncodeToString(data)
		}
		_, _ = h(context.Background(), mcp.CallToolRequest{}, WriteArgs{
			Path:       path,
			Encoding:   enc,
			Content:    content,
			CreateDirs: boolPtr(true),
		})
	})
}
