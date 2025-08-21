package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
)

func TestWriteReadIntegration(t *testing.T) {
	root := t.TempDir()
	sessions := map[string]*SessionState{"s1": {Root: root}}
	var mu sync.RWMutex
	manager := &sessionManager{id: "s1"}
	addSession := func(h server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx = withSessionManager(ctx, manager)
			return h(ctx, req)
		}
	}
	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: mcp.NewTool("fs_write"), Handler: addSession(wrapStructuredHandler(handleWrite(sessions, &mu)))},
		server.ServerTool{Tool: mcp.NewTool("fs_read"), Handler: addSession(mcp.NewStructuredToolHandler(handleRead(sessions, &mu)))},
	)
	if err != nil {
		t.Fatalf("server start failed: %v", err)
	}
	defer srv.Close()

	_, err = srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "fs_write", Arguments: map[string]any{
			"path": "hello.txt", "content": "hello",
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

func TestWriteErrorResponse(t *testing.T) {
	root := t.TempDir()
	sessions := map[string]*SessionState{"s1": {Root: root}}
	var mu sync.RWMutex
	manager := &sessionManager{id: "s1"}
	addSession := func(h server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx = withSessionManager(ctx, manager)
			return h(ctx, req)
		}
	}
	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: mcp.NewTool("fs_write", mcp.WithOutputSchema[WriteResult]()), Handler: addSession(wrapStructuredHandler(handleWrite(sessions, &mu)))},
	)
	if err != nil {
		t.Fatalf("server start failed: %v", err)
	}
	defer srv.Close()

	res, err := srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "fs_write", Arguments: map[string]any{
			"path":     "f.txt",
			"content":  "x",
			"strategy": "bogus",
		}},
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError result")
	}
}
