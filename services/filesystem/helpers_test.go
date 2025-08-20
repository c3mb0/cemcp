package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestInitDebugAndDprintf(t *testing.T) {
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	// Debug disabled early return
	*debugFlag = ""
	debugEnabled = false
	debugLog = nil
	initDebug()
	if _, err := os.Stat("log"); !os.IsNotExist(err) {
		t.Fatalf("log should not exist when disabled")
	}

	// Error when creating the log file
	os.Mkdir("log", 0o755)
	*debugFlag = "log"
	initDebug()
	if debugEnabled {
		t.Fatalf("debug should not enable on error")
	}
	os.Remove("log")

	// Successful run
	initDebug()
	dprintf("hello %s", "world")
	data, err := os.ReadFile("log")
	if err != nil {
		t.Fatalf("log not created: %v", err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Fatalf("log missing content: %q", string(data))
	}
}

func TestGetRoot(t *testing.T) {
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	oldFlag := *rootDirFlag
	defer func() { *rootDirFlag = oldFlag }()
	oldEnv := os.Getenv("FS_ROOT")
	defer os.Setenv("FS_ROOT", oldEnv)

	t.Run("flag", func(t *testing.T) {
		dir := t.TempDir()
		*rootDirFlag = dir
		os.Setenv("FS_ROOT", "")
		r, err := getRoot()
		if err != nil || r != dir {
			t.Fatalf("getRoot flag failed: %q %v", r, err)
		}
	})

	t.Run("env", func(t *testing.T) {
		dir := t.TempDir()
		*rootDirFlag = ""
		os.Setenv("FS_ROOT", dir)
		r, err := getRoot()
		if err != nil || r != dir {
			t.Fatalf("getRoot env failed: %q %v", r, err)
		}
	})

	t.Run("cwd", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		*rootDirFlag = ""
		os.Setenv("FS_ROOT", "")
		r, err := getRoot()
		if err != nil || r != dir {
			t.Fatalf("getRoot cwd failed: %q %v", r, err)
		}
	})
}

func TestDetectMIMEAndIsTextExtra(t *testing.T) {
	if mt := detectMIME("a.txt", []byte("hi")); mt != "text/plain; charset=utf-8" {
		t.Fatalf("ext detect failed: %s", mt)
	}
	if mt := detectMIME("bin", []byte{0}); mt != "application/octet-stream" {
		t.Fatalf("binary detect failed: %s", mt)
	}
	if mt := detectMIME("noext", []byte("hi")); mt != "text/plain; charset=utf-8" {
		t.Fatalf("text detect failed: %s", mt)
	}
	if !isText([]byte{'a', '\n', '\r', '\t', 'b'}) {
		t.Fatalf("expected text with whitespace controls")
	}
	if isText([]byte{0, 1, 2}) {
		t.Fatalf("expected binary not text")
	}
}

func TestReadWindowVariants(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.txt")
	os.WriteFile(p, []byte("0123456789"), 0o644)
	b, _, _, err := readWindow(p, -5, 2)
	if err != nil || string(b) != "01" {
		t.Fatalf("neg offset failed: %q %v", string(b), err)
	}
	b, sz, eof, err := readWindow(p, 999, 2)
	if err != nil || len(b) != 0 || !eof || sz != 10 {
		t.Fatalf("beyond size failed: %q sz=%d eof=%v err=%v", string(b), sz, eof, err)
	}
	if _, _, _, err := readWindow(filepath.Join(root, "missing"), 0, 1); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	if err := atomicWrite(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "x" {
		t.Fatalf("atomic write failed: %v %q", err, string(data))
	}
	// Rename should fail when the target is a directory
	targetDir := filepath.Join(dir, "sub")
	os.Mkdir(targetDir, 0o755)
	if err := atomicWrite(targetDir, []byte("x"), 0o600); err == nil {
		t.Fatalf("expected error writing to dir")
	}
}

type fakeFileInfo struct{ mode os.FileMode }

func (f fakeFileInfo) Name() string       { return "" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

func TestKindOf(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "f")
	os.WriteFile(f, []byte(""), 0o644)
	fi, _ := os.Lstat(f)
	if k := kindOf(fi); k != "file" {
		t.Fatalf("want file, got %s", k)
	}
	d := filepath.Join(root, "d")
	os.Mkdir(d, 0o755)
	fi, _ = os.Lstat(d)
	if k := kindOf(fi); k != "dir" {
		t.Fatalf("want dir, got %s", k)
	}
	if err := os.Symlink("f", filepath.Join(root, "l")); err == nil {
		fi, _ = os.Lstat(filepath.Join(root, "l"))
		if k := kindOf(fi); k != "symlink" {
			t.Fatalf("want symlink, got %s", k)
		}
	}
	pipe := kindOf(fakeFileInfo{mode: os.ModeNamedPipe})
	if pipe != "pipe" {
		t.Fatalf("want pipe, got %s", pipe)
	}
	other := kindOf(fakeFileInfo{mode: os.ModeIrregular})
	if other != "other" {
		t.Fatalf("want other, got %s", other)
	}
}

func TestAcquireLock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	release, err := acquireLock(p, time.Second)
	if err != nil {
		t.Fatalf("acquireLock failed: %v", err)
	}
	defer release()
	_, err = acquireLock(p, 100*time.Millisecond)
	if err == nil {
		t.Fatalf("expected lock timeout")
	}
}

func TestHandleReadAndPeekVariants(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "b.bin")
	os.WriteFile(p, []byte("hello"), 0o644)
	rd := handleRead(root)
	res, err := rd(context.Background(), mcp.CallToolRequest{}, ReadArgs{Path: "b.bin"})
	if err != nil || res.Content != "hello" {
		t.Fatalf("read failed: %+v %v", res, err)
	}
	if _, err := rd(context.Background(), mcp.CallToolRequest{}, ReadArgs{Path: "../bad"}); err == nil {
		t.Fatalf("expected path error")
	}
	pk := handlePeek(root)
	res2, err := pk(context.Background(), mcp.CallToolRequest{}, PeekArgs{Path: "b.bin", Offset: -2, MaxBytes: 2})
	if err != nil || res2.Content != "he" {
		t.Fatalf("peek neg offset failed: %+v %v", res2, err)
	}
	if _, err := pk(context.Background(), mcp.CallToolRequest{}, PeekArgs{Path: "../bad"}); err == nil {
		t.Fatalf("expected peek path error")
	}
}

func TestHandleWriteErrors(t *testing.T) {
	root := t.TempDir()
	wr := handleWrite(root)
	if _, err := wr(context.Background(), mcp.CallToolRequest{}, WriteArgs{Path: "a.txt", Strategy: "bogus", Content: "x"}); err == nil {
		t.Fatalf("expected strategy error")
	}
	// append to directory should error
	os.Mkdir(filepath.Join(root, "adir"), 0o755)
	if _, err := wr(context.Background(), mcp.CallToolRequest{}, WriteArgs{Path: "adir", Content: "x", Strategy: strategyAppend}); err == nil {
		t.Fatalf("expected append dir error")
	}

	// prepare file for replace_range tests
	os.WriteFile(filepath.Join(root, "r.txt"), []byte("abcd"), 0o644)
	s, e := 3, 2 // invalid range (end < start)
	if _, err := wr(context.Background(), mcp.CallToolRequest{}, WriteArgs{Path: "r.txt", Content: "x", Strategy: strategyReplaceRange, Start: &s, End: &e}); err == nil {
		t.Fatalf("expected invalid range error")
	}
	s = 0
	if _, err := wr(context.Background(), mcp.CallToolRequest{}, WriteArgs{Path: "r.txt", Content: "x", Strategy: strategyReplaceRange, Start: &s}); err == nil {
		t.Fatalf("expected missing end error")
	}
}

func TestHandleEditError(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "e.txt")
	os.WriteFile(p, []byte("data"), 0o644)
	ed := handleEdit(root)
	if _, err := ed(context.Background(), mcp.CallToolRequest{}, EditArgs{Path: "e.txt", Pattern: "(", Replace: "x", Regex: true}); err == nil {
		t.Fatalf("expected regex error")
	}
	if _, err := ed(context.Background(), mcp.CallToolRequest{}, EditArgs{Path: "missing.txt", Pattern: "x", Replace: "y"}); err == nil {
		t.Fatalf("expected missing file error")
	}
}

func TestHandleListVariants(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte(""), 0o644)
	hl := handleList(root)
	res, err := hl(context.Background(), mcp.CallToolRequest{}, ListArgs{Path: ".", MaxEntries: 1})
	if err != nil || len(res.Entries) != 1 {
		t.Fatalf("non-recursive max failed: %+v %v", res, err)
	}
	res, err = hl(context.Background(), mcp.CallToolRequest{}, ListArgs{Path: "a.txt"})
	if err != nil || len(res.Entries) != 1 || res.Entries[0].Name != "a.txt" {
		t.Fatalf("file path list failed: %+v %v", res, err)
	}
	if _, err = hl(context.Background(), mcp.CallToolRequest{}, ListArgs{Path: "missing"}); err == nil {
		t.Fatalf("expected missing error")
	}
}

func TestHandleGlobErrors(t *testing.T) {
	root := t.TempDir()
	gb := handleGlob(root)
	if _, err := gb(context.Background(), mcp.CallToolRequest{}, GlobArgs{Pattern: ""}); err == nil {
		t.Fatalf("expected pattern error")
	}
	if _, err := gb(context.Background(), mcp.CallToolRequest{}, GlobArgs{Pattern: "["}); err == nil {
		t.Fatalf("expected invalid pattern error")
	}
	if _, err := gb(context.Background(), mcp.CallToolRequest{}, GlobArgs{Pattern: "../*"}); err == nil {
		t.Fatalf("expected join error")
	}
}

func TestHandleGlobMaxResults(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), []byte(""), 0o644)
	mustWrite(t, filepath.Join(root, "b.txt"), []byte(""), 0o644)
	mustWrite(t, filepath.Join(root, "c.txt"), []byte(""), 0o644)
	gb := handleGlob(root)
	res, err := gb(context.Background(), mcp.CallToolRequest{}, GlobArgs{Pattern: "*.txt", MaxResults: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(res.Matches))
	}
}
