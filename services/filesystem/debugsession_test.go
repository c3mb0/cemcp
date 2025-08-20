package main

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func resetDebugSessions() {
	dbgMu.Lock()
	defer dbgMu.Unlock()
	dbgSessions = make(map[string]*DebugSession)
	dbgOrder = nil
}

func TestDebuggingApproachIncomplete(t *testing.T) {
	resetDebugSessions()
	h := handleDebuggingApproach()
	res, err := h(context.Background(), mcp.CallToolRequest{}, DebuggingApproachArgs{Approach: "step", Resolution: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "incomplete" {
		t.Fatalf("expected status incomplete, got %s", res.Status)
	}
	p := handlePendingDebug()
	pending, err := p(context.Background(), mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending.Sessions) != 1 {
		t.Fatalf("expected 1 pending session, got %d", len(pending.Sessions))
	}
}

func TestDebuggingApproachComplete(t *testing.T) {
	resetDebugSessions()
	h := handleDebuggingApproach()
	res, err := h(context.Background(), mcp.CallToolRequest{}, DebuggingApproachArgs{Approach: "step", Resolution: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "complete" {
		t.Fatalf("expected status complete, got %s", res.Status)
	}
	p := handlePendingDebug()
	pending, err := p(context.Background(), mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending.Sessions) != 0 {
		t.Fatalf("expected 0 pending sessions, got %d", len(pending.Sessions))
	}
}
