package main

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

const maxDebugSessions = 20

// DebugSession represents a recorded debugging attempt.
type DebugSession struct {
	ID         string `json:"id"`
	Approach   string `json:"approach"`
	Resolution string `json:"resolution"`
	Status     string `json:"status"`
}

var (
	dbgMu       sync.Mutex
	dbgSessions = make(map[string]*DebugSession)
	dbgOrder    []string
)

// DebuggingApproachArgs are inputs for the debuggingapproach tool.
type DebuggingApproachArgs struct {
	SessionID  string `json:"session"`
	Approach   string `json:"approach"`
	Resolution string `json:"resolution"`
}

// DebuggingApproachResult is returned from the debuggingapproach tool.
type DebuggingApproachResult struct {
	SessionID string `json:"session"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// handleDebuggingApproach records a debugging session and marks it complete or incomplete.
func handleDebuggingApproach() mcp.StructuredToolHandlerFunc[DebuggingApproachArgs, DebuggingApproachResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args DebuggingApproachArgs) (DebuggingApproachResult, error) {
		dbgMu.Lock()
		defer dbgMu.Unlock()

		id := args.SessionID
		if id == "" {
			id = uuid.NewString()
		}

		status := "complete"
		msg := "Debugging session recorded."
		if strings.TrimSpace(args.Resolution) == "" {
			status = "incomplete"
			msg = "Resolution is empty; add more steps before finishing."
		}

		if _, ok := dbgSessions[id]; !ok {
			if len(dbgOrder) >= maxDebugSessions {
				oldest := dbgOrder[0]
				dbgOrder = dbgOrder[1:]
				delete(dbgSessions, oldest)
			}
			dbgOrder = append(dbgOrder, id)
		}

		dbgSessions[id] = &DebugSession{
			ID:         id,
			Approach:   args.Approach,
			Resolution: args.Resolution,
			Status:     status,
		}

		return DebuggingApproachResult{SessionID: id, Status: status, Message: msg}, nil
	}
}

// PendingDebugResult lists unresolved debugging sessions.
type PendingDebugResult struct {
	Sessions []DebugSession `json:"sessions"`
}

// handlePendingDebug returns sessions that are incomplete.
func handlePendingDebug() mcp.StructuredToolHandlerFunc[struct{}, PendingDebugResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, _ struct{}) (PendingDebugResult, error) {
		dbgMu.Lock()
		defer dbgMu.Unlock()
		res := PendingDebugResult{}
		for _, id := range dbgOrder {
			if s, ok := dbgSessions[id]; ok && s.Status != "complete" {
				res.Sessions = append(res.Sessions, *s)
			}
		}
		return res, nil
	}
}
