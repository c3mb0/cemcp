package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestTrimUnderRootHandlesSlashRoot(t *testing.T) {
	if got := trimUnderRoot("/", "/etc/hosts"); got != "etc/hosts" {
		t.Fatalf("trimUnderRoot failed: %q", got)
	}
}

func TestReadSkipsHugeHash(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "huge.bin")
	if err := os.WriteFile(p, make([]byte, maxHashBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	h := handleRead(root)
	res, err := h(context.Background(), mcp.CallToolRequest{}, ReadArgs{Path: "huge.bin", MaxBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if res.SHA256 != "" {
		t.Fatalf("expected empty SHA256, got %q", res.SHA256)
	}
	if !res.Truncated {
		t.Fatalf("expected truncated content")
	}
}

func TestWriteCreateDirsDefaultFalse(t *testing.T) {
	root := t.TempDir()
	h := handleWrite(root)
	_, err := h(context.Background(), mcp.CallToolRequest{}, WriteArgs{
		Path:     "nested/dir/file.txt",
		Encoding: string(encText),
		Content:  "hi",
	})
	if err == nil {
		t.Fatalf("expected error when creating dirs not opted in")
	}
}

func TestOverwritePreservesModeWhenEmpty(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "f.txt")
	if err := os.WriteFile(p, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := handleWrite(root)
	if _, err := h(context.Background(), mcp.CallToolRequest{}, WriteArgs{
		Path:     "f.txt",
		Encoding: string(encText),
		Content:  "v2",
	}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode() & os.ModePerm; got != 0o600 {
		t.Fatalf("expected mode 0600, got %#o", got)
	}
}

func TestOverwriteChangesModeWhenProvided(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "f2.txt")
	if err := os.WriteFile(p, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := handleWrite(root)
	if _, err := h(context.Background(), mcp.CallToolRequest{}, WriteArgs{
		Path:     "f2.txt",
		Encoding: string(encText),
		Content:  "v2",
		Mode:     "0644",
	}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode() & os.ModePerm; got != 0o644 {
		t.Fatalf("expected mode 0644, got %#o", got)
	}
}

func TestEditRegexCountConsistency(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "t.txt")
	if err := os.WriteFile(p, []byte("a a a"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := handleEdit(root)
	res, err := h(context.Background(), mcp.CallToolRequest{}, EditArgs{
		Path:    "t.txt",
		Pattern: "a",
		Replace: "b",
		Regex:   true,
		Count:   0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Replacements != 3 {
		t.Fatalf("expected 3 replacements, got %d", res.Replacements)
	}
}

func TestEditRegexBackrefAll(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "t.txt")
	if err := os.WriteFile(p, []byte("x=1; x=2;"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := handleEdit(root)
	res, err := h(context.Background(), mcp.CallToolRequest{}, EditArgs{
		Path:    "t.txt",
		Pattern: `x=(\d)`,
		Replace: `y=$1`,
		Regex:   true,
		Count:   0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Replacements != 2 {
		t.Fatalf("expected 2 replacements, got %d", res.Replacements)
	}
	b, _ := os.ReadFile(p)
	if !regexp.MustCompile(`y=1; y=2;`).Match(b) {
		t.Fatalf("unexpected content: %q", string(b))
	}
}

func TestSearchLongLine(t *testing.T) {
	root := t.TempDir()
	long := make([]byte, 200000)
	for i := range long {
		long[i] = 'x'
	}
	copy(long[:6], []byte("hello!"))
	if err := os.WriteFile(filepath.Join(root, "big.txt"), long, 0o644); err != nil {
		t.Fatal(err)
	}
	h := handleSearch(root)
	res, err := h(context.Background(), mcp.CallToolRequest{}, SearchArgs{Pattern: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(res.Matches))
	}
}

func TestLockStale(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.txt")
	_ = os.WriteFile(p+".lock", []byte("123\n"), 0o644)
	old := time.Now().Add(-11 * time.Minute)
	_ = os.Chtimes(p+".lock", old, old)
	release, err := acquireLock(p, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	release()
}
