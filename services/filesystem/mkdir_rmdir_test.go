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
