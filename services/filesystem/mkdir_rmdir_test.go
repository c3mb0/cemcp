package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMkdirAndRmdir(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	mk := handleMkdir(sessions, mu)
	rm := handleRmdir(sessions, mu)

	res, err := mk(ctx, mcp.CallToolRequest{}, MkdirArgs{Path: "a/b", Mode: "755"})
	if err != nil || !res.Created {
		t.Fatalf("mkdir failed: %+v err=%v", res, err)
	}
	info, err := os.Stat(filepath.Join(root, "a", "b"))
	if err != nil || !info.IsDir() {
		t.Fatalf("directory not created: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "a", "b", "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err = rm(ctx, mcp.CallToolRequest{}, RmdirArgs{Path: "a", Recursive: true})
	if err != nil {
		t.Fatalf("rmdir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Fatalf("directory not removed: %v", err)
	}
}

func TestMkdirBraceExpansion(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	mk := handleMkdir(sessions, mu)
	pattern := "internal/agents/{dev,test,automation,security,uat}"
	res, err := mk(ctx, mcp.CallToolRequest{}, MkdirArgs{Path: pattern})
	if err != nil {
		t.Fatalf("mkdir brace expansion failed: %v", err)
	}
	if !res.Created {
		t.Fatalf("expected Created=true, got %+v", res)
	}
	dirs := []string{"dev", "test", "automation", "security", "uat"}
	for _, d := range dirs {
		p := filepath.Join(root, "internal", "agents", d)
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			t.Fatalf("directory %s not created: %v", d, err)
		}
	}
}

func TestMkdirIdempotent(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	mk := handleMkdir(sessions, mu)

	// First call - should create directory
	res1, err := mk(ctx, mcp.CallToolRequest{}, MkdirArgs{Path: "testdir", Mode: "755"})
	if err != nil {
		t.Fatalf("first mkdir failed: %v", err)
	}
	if !res1.Created {
		t.Fatalf("expected Created=true on first call, got %+v", res1)
	}

	// Second call - should succeed but not create (idempotent)
	res2, err := mk(ctx, mcp.CallToolRequest{}, MkdirArgs{Path: "testdir", Mode: "755"})
	if err != nil {
		t.Fatalf("second mkdir failed: %v", err)
	}
	if res2.Created {
		t.Fatalf("expected Created=false on second call (already exists), got %+v", res2)
	}

	// Verify directory still exists
	info, err := os.Stat(filepath.Join(root, "testdir"))
	if err != nil || !info.IsDir() {
		t.Fatalf("directory not found after idempotent calls: %v", err)
	}
}

func TestRmdirIdempotent(t *testing.T) {
	root := t.TempDir()
	ctx, sessions, mu := testSession(root)
	mk := handleMkdir(sessions, mu)
	rm := handleRmdir(sessions, mu)

	// Create a directory first
	_, err := mk(ctx, mcp.CallToolRequest{}, MkdirArgs{Path: "testdir", Mode: "755"})
	if err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// First rmdir - should remove directory
	res1, err := rm(ctx, mcp.CallToolRequest{}, RmdirArgs{Path: "testdir", Recursive: false})
	if err != nil {
		t.Fatalf("first rmdir failed: %v", err)
	}
	if !res1.Removed {
		t.Fatalf("expected Removed=true on first call, got %+v", res1)
	}

	// Second rmdir - should succeed but not remove anything (idempotent)
	res2, err := rm(ctx, mcp.CallToolRequest{}, RmdirArgs{Path: "testdir", Recursive: false})
	if err != nil {
		t.Fatalf("second rmdir failed (should be idempotent): %v", err)
	}
	if res2.Removed {
		t.Fatalf("expected Removed=false on second call (already removed), got %+v", res2)
	}

	// Verify directory doesn't exist
	if _, err := os.Stat(filepath.Join(root, "testdir")); !os.IsNotExist(err) {
		t.Fatalf("directory still exists after removal: %v", err)
	}
}
