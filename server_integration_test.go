package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
)

func TestWriteReadIntegration(t *testing.T) {
	root := t.TempDir()
	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: mcp.NewTool("fs_write"), Handler: mcp.NewStructuredToolHandler(handleWrite(root))},
		server.ServerTool{Tool: mcp.NewTool("fs_read"), Handler: mcp.NewStructuredToolHandler(handleRead(root))},
	)
	if err != nil {
		t.Fatalf("server start failed: %v", err)
	}
	defer srv.Close()

	_, err = srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "fs_write", Arguments: map[string]any{
			"path": "hello.txt", "encoding": string(encText), "content": "hello",
		}},
	})
	if err != nil {
		t.Fatalf("write call failed: %v", err)
	}

	res, err := srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "fs_read", Arguments: map[string]any{
			"path": "hello.txt",
		}},
	})
	if err != nil {
		t.Fatalf("read call failed: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected one content entry, got %d", len(res.Content))
	}
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content")
	}
	var rr ReadResult
	if err := json.Unmarshal([]byte(text.Text), &rr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rr.Content != "hello" {
		t.Fatalf("expected content hello, got %q", rr.Content)
	}
}
