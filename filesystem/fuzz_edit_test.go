// file: fuzz_edit_test.go
//go:build go1.18
// +build go1.18

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// FuzzEdit ensures regex/text replacement never panics and handles invalid regexes.
// IMPORTANT: Do not call any *F methods from the fuzz target body; use t.* inside the callback.
func FuzzEdit(f *testing.F) {
	// Seeds
	f.Add("foo bar baz", "ba.", "XX", true, 1) // regex with dot
	f.Add("aaaaa", "a", "b", false, 0)         // text replace all
	f.Add("hello", "(unclosed", "x", true, 0)  // invalid regex (exercise error path)

	f.Fuzz(func(t *testing.T, content, pattern, repl string, regex bool, count int) {
		root := t.TempDir() // use t.* within the target body
		p := filepath.Join(root, "e.txt")
		_ = os.WriteFile(p, []byte(content), 0o644)
		h := handleEdit(root)
		_, _ = h(context.Background(), mcp.CallToolRequest{}, EditArgs{Path: "e.txt", Pattern: pattern, Replace: repl, Regex: regex, Count: count})
	})
}
