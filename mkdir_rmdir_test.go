package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMkdirAndRmdir(t *testing.T) {
	root := t.TempDir()
	mk := handleMkdir(root)
	rm := handleRmdir(root)

	res, err := mk(context.Background(), mcp.CallToolRequest{}, MkdirArgs{Path: "a/b", Parents: true, Mode: "755"})
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

	_, err = rm(context.Background(), mcp.CallToolRequest{}, RmdirArgs{Path: "a", Recursive: true})
	if err != nil {
		t.Fatalf("rmdir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Fatalf("directory not removed: %v", err)
	}
}
