package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func mustWrite(t *testing.T, p string, b []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, b, mode); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func makeSymlink(t *testing.T, target, link string) error {
	t.Helper()
	// Windows often requires admin privileges for symlinks.
	if runtime.GOOS == "windows" {
		return os.ErrPermission
	}
	return os.Symlink(target, link)
}

func TestSafeJoin(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "dir", "file.txt")
	mustWrite(t, inside, []byte("hi"), 0o644)

	// Normal join
	p, err := safeJoin(root, "dir/file.txt")
	if err != nil || p != inside {
		t.Fatalf("safeJoin failed: %v %q", err, p)
	}

	// A traversal that normalizes back inside the root should be accepted
	tricky := filepath.ToSlash("../" + filepath.Base(root) + "/dir/file.txt")
	if _, err := safeJoin(root, tricky); err != nil {
		t.Fatalf("safeJoin rejected normalized path: %v", err)
	}

	// Absolute outside should be rejected
	if _, err := safeJoin(root, "/etc/passwd"); err == nil {
		t.Fatalf("safeJoin allowed absolute escape")
	}

	// File:// URI with a percent-encoded space
	u := "file://" + strings.ReplaceAll(filepath.ToSlash(filepath.Join(root, "dir", "file space.txt")), " ", "%20")
	mustWrite(t, filepath.Join(root, "dir", "file space.txt"), []byte("z"), 0o644)
	if _, err := safeJoin(root, u); err != nil {
		t.Fatalf("safeJoin file:// failed: %v", err)
	}
}

func TestSafeJoinResolveFinal(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "file.txt")
	mustWrite(t, inside, []byte("x"), 0o644)
	if err := makeSymlink(t, inside, filepath.Join(root, "link.txt")); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skip("symlinks not supported")
		}
		t.Fatalf("symlink: %v", err)
	}
	p, err := safeJoinResolveFinal(root, "link.txt")
	if err != nil || p != inside {
		t.Fatalf("resolve final inside failed: %v %q", err, p)
	}

	outside := filepath.Join(root, "..", "escape.txt")
	mustWrite(t, outside, []byte("o"), 0o644)
	if err := makeSymlink(t, outside, filepath.Join(root, "badlink")); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skip("symlinks not supported")
		}
		t.Fatalf("symlink: %v", err)
	}
	if _, err := safeJoinResolveFinal(root, "badlink"); err == nil {
		t.Fatalf("expected error for symlink outside base folder")
	}
}

func TestReadWindow(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.txt")
	mustWrite(t, p, []byte("0123456789"), 0o644)
	b, sz, eof, err := readWindow(p, 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "3456" || sz != 10 || eof {
		t.Fatalf("got %q sz=%d eof=%v", string(b), sz, eof)
	}
	b, _, eof, err = readWindow(p, 9, 10)
	if err != nil || string(b) != "9" || !eof {
		t.Fatalf("tail read failed: %q eof=%v err=%v", b, eof, err)
	}
}

func TestParseMode(t *testing.T) {
	m, err := parseMode("")
	if err != nil || m != 0o644 {
		t.Fatalf("default mode wrong: %v %o", err, m)
	}
	m, err = parseMode("644")
	if err != nil || m != 0o644 {
		t.Fatalf("parse 644: %v %o", err, m)
	}
	m, err = parseMode("0755")
	if err != nil || m != 0o755 {
		t.Fatalf("parse 0755: %v %o", err, m)
	}
	if _, err = parseMode("xyz"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAtomicWriteAndLock(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "x.txt")
	if err := atomicWrite(p, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "a" {
		t.Fatalf("atomicWrite wrong content: %q err=%v", b, err)
	}
	if err := atomicWrite(p, []byte("b"), 0o644); err != nil {
		t.Fatalf("atomicWrite overwrite failed: %v", err)
	}
	b, err = os.ReadFile(p)
	if err != nil || string(b) != "b" {
		t.Fatalf("overwrite wrong content: %q err=%v", b, err)
	}

	rel, err := acquireLock(p, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer rel() // release the first lock after testing contention
		_, err := acquireLock(p, 300*time.Millisecond)
		if err == nil {
			t.Errorf("expected timeout, got nil")
		}
	}()
	<-done
}

func TestDetectMIMEAndIsText(t *testing.T) {
	if mt := detectMIME("x.txt", []byte("abc")); !strings.HasPrefix(mt, "text/") {
		t.Fatalf("want text, got %s", mt)
	}
	if mt := detectMIME("x.bin", []byte{0x00, 0x01}); mt != "application/octet-stream" {
		t.Fatalf("want octet-stream, got %s", mt)
	}
}

func TestHandleWriteStrategies(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	// Overwrite create
	wr := handleWrite(sessions, mu)
	res, err := wr(ctx, mcp.CallToolRequest{}, WriteArgs{Path: "a.txt", Content: "A"})
	if err != nil || !res.Created || res.Bytes != 1 {
		t.Fatalf("overwrite create failed: %+v err=%v", res, err)
	}
	// No clobber
	_, err = wr(ctx, mcp.CallToolRequest{}, WriteArgs{Path: "a.txt", Content: "B", Strategy: strategyNoClobber})
	if err == nil {
		t.Fatalf("no_clobber should error if exists")
	}
	// Append
	res, err = wr(ctx, mcp.CallToolRequest{}, WriteArgs{Path: "a.txt", Content: "C", Strategy: strategyAppend})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(b) != "AC" {
		t.Fatalf("append wrong: %q", string(b))
	}
	// Prepend
	res, err = wr(ctx, mcp.CallToolRequest{}, WriteArgs{Path: "a.txt", Content: "Z", Strategy: strategyPrepend})
	if err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(root, "a.txt"))
	if string(b) != "ZAC" {
		t.Fatalf("prepend wrong: %q", string(b))
	}
	// Replace range
	s, e := 1, 2
	res, err = wr(ctx, mcp.CallToolRequest{}, WriteArgs{Path: "a.txt", Content: "XY", Strategy: strategyReplaceRange, Start: &s, End: &e})
	if err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(root, "a.txt"))
	if string(b) != "ZXYC" {
		t.Fatalf("replace_range wrong: %q", string(b))
	}
}

func TestHandleWritePrependCreates(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	wr := handleWrite(sessions, mu)
	res, err := wr(ctx, mcp.CallToolRequest{}, WriteArgs{Path: "new.txt", Content: "X", Strategy: strategyPrepend})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "new.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "X" {
		t.Fatalf("prepend create wrong: %q", string(b))
	}
	if !res.Created {
		t.Fatalf("expected created true")
	}
}

func TestHandleReadAndPeek(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "b.txt"), []byte("hello world"), 0o644)
	ctx, sessions, mu := testSession(root)
	rd := handleRead(sessions, mu)
	res, err := rd(ctx, mcp.CallToolRequest{}, ReadArgs{Path: "b.txt", MaxBytes: 5})
	if err != nil || !res.Truncated || res.Content != "hello" {
		t.Fatalf("read wrong: %+v err=%v", res, err)
	}
	pk := handlePeek(sessions, mu)
	pres, err := pk(ctx, mcp.CallToolRequest{}, PeekArgs{Path: "b.txt", Offset: 6, MaxBytes: 5})
	if err != nil || pres.Content != "world" || !pres.EOF {
		t.Fatalf("peek wrong: %+v err=%v", pres, err)
	}
}

func TestHandleEdit_TextAndRegex(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "e.txt")
	mustWrite(t, p, []byte("one two two three"), 0o644)
	ctx, sessions, mu := testSession(root)
	ed := handleEdit(sessions, mu)
	// text, limit 1
	res, err := ed(ctx, mcp.CallToolRequest{}, EditArgs{Path: "e.txt", Pattern: "two", Replace: "2", Count: 1})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("text edit failed: %+v err=%v", res, err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "one 2 two three" {
		t.Fatalf("text replace wrong: %q", string(b))
	}
	// regex, replace all
	res, err = ed(ctx, mcp.CallToolRequest{}, EditArgs{Path: "e.txt", Pattern: "t[a-z]+", Replace: "X", Regex: true})
	if err != nil || res.Replacements != 2 {
		t.Fatalf("regex edit failed: %+v err=%v", res, err)
	}
	b, _ = os.ReadFile(p)
	if !strings.Contains(string(b), "one 2 X X") {
		t.Fatalf("regex replace wrong: %q", string(b))
	}
}

func TestHandleListAndGlob(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "d", "x.txt"), []byte(""), 0o644)
	mustWrite(t, filepath.Join(root, "d", "y.bin"), []byte{0}, 0o644)
	ctx, sessions, mu := testSession(root)
	ls := handleList(sessions, mu)
	res, err := ls(ctx, mcp.CallToolRequest{}, ListArgs{Path: ".", Recursive: true, MaxEntries: 10})
	if err != nil || len(res.Entries) < 2 {
		t.Fatalf("list failed: %d err=%v", len(res.Entries), err)
	}
	gb := handleGlob(sessions, mu)
	gres, err := gb(ctx, mcp.CallToolRequest{}, GlobArgs{Pattern: "d/*.txt"})
	if err != nil || len(gres.Matches) != 1 || gres.Matches[0] != "d/x.txt" {
		t.Fatalf("glob wrong: %+v err=%v", gres, err)
	}
}

func TestHandleGlobRecursive(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a", "b", "c.txt"), []byte(""), 0o644)
	ctx, sessions, mu := testSession(root)
	gb := handleGlob(sessions, mu)
	res, err := gb(ctx, mcp.CallToolRequest{}, GlobArgs{Pattern: "**/*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || res.Matches[0] != "a/b/c.txt" {
		t.Fatalf("recursive glob failed: %+v", res.Matches)
	}
}

func TestHandleRead_DefaultLimit(t *testing.T) {
	root := t.TempDir()
	big := strings.Repeat("a", defaultReadMaxBytes+100)
	mustWrite(t, filepath.Join(root, "big.txt"), []byte(big), 0o644)
	ctx, sessions, mu := testSession(root)
	rd := handleRead(sessions, mu)
	res, err := rd(ctx, mcp.CallToolRequest{}, ReadArgs{Path: "big.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || len(res.Content) != defaultReadMaxBytes {
		t.Fatalf("expected truncation to %d bytes: %+v", defaultReadMaxBytes, res)
	}
}
