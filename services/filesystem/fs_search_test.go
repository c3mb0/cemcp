package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestSearchBasic(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), []byte("hello world\nbye\n"), 0o644)
	mustWrite(t, filepath.Join(root, "dir", "b.txt"), []byte("world line\nfoo\n"), 0o644)

	ctx, sessions, mu := testSession(root)
	search := handleSearch(sessions, mu)
	res, err := search(ctx, mcp.CallToolRequest{}, SearchArgs{Pattern: "world"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("want 2 matches, got %d", len(res.Matches))
	}
}

func TestSearchRegexAndLimit(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "c.txt"), []byte("cat\ncar\ncap\n"), 0o644)

	ctx, sessions, mu := testSession(root)
	search := handleSearch(sessions, mu)
	res, err := search(ctx, mcp.CallToolRequest{}, SearchArgs{Pattern: "ca.", Regex: true, MaxResults: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("limit failed, got %d", len(res.Matches))
	}
}
func TestSearchNoPattern(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	search := handleSearch(sessions, mu)
	_, err := search(ctx, mcp.CallToolRequest{}, SearchArgs{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchRegexError(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	search := handleSearch(sessions, mu)
	_, err := search(ctx, mcp.CallToolRequest{}, SearchArgs{Pattern: "[", Regex: true})
	if err == nil {
		t.Fatal("expected regex compile error")
	}
}

func TestSearchStartPathAndOutsideRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "dir", "f.txt"), []byte("inside"), 0o644)
	mustWrite(t, filepath.Join(root, "g.txt"), []byte("outside"), 0o644)
	ctx, sessions, mu := testSession(root)
	search := handleSearch(sessions, mu)
	res, err := search(ctx, mcp.CallToolRequest{}, SearchArgs{Pattern: "i", Path: "dir"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || !strings.Contains(res.Matches[0].Path, "dir") {
		t.Fatalf("unexpected matches: %+v", res.Matches)
	}
	// path escaping root should error
	_, err = search(ctx, mcp.CallToolRequest{}, SearchArgs{Pattern: "x", Path: ".."})
	if err == nil {
		t.Fatal("expected path error")
	}
	_, err = search(ctx, mcp.CallToolRequest{}, SearchArgs{Pattern: "x", Path: "missing"})
	if err == nil {
		t.Fatal("expected missing path error")
	}
}

func TestSearchSymlinkAndErrorIgnored(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "target.txt"), []byte("hi"), 0o644)
	os.Symlink(filepath.Join(root, "target.txt"), filepath.Join(root, "link.txt"))
	os.Mkdir(filepath.Join(root, "blocked"), 0o000)
	ctx, sessions, mu := testSession(root)
	search := handleSearch(sessions, mu)
	_, err := search(ctx, mcp.CallToolRequest{}, SearchArgs{Pattern: "hi"})
	if err != nil {
		t.Fatal(err)
	}
}
