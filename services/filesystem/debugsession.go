package main

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

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
	dbgMu       sync.RWMutex
	dbgSessions = make(map[string]*DebugSession)
	dbgOrder    []string
	dbgMemUsage int64 // Track memory usage atomically
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
		// Use write lock for modifications
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

		// Clean up old sessions if we're at capacity
		if _, exists := dbgSessions[id]; !exists {
			if len(dbgOrder) >= maxDebugSessions {
				// Remove oldest session
				oldest := dbgOrder[0]
				dbgOrder = dbgOrder[1:]
				if oldSession, ok := dbgSessions[oldest]; ok {
					// Update memory usage tracking
					sessionSize := int64(len(oldSession.Approach) + len(oldSession.Resolution) + len(oldSession.ID) + len(oldSession.Status))
					atomic.AddInt64(&dbgMemUsage, -sessionSize)
				}
				delete(dbgSessions, oldest)
			}
			dbgOrder = append(dbgOrder, id)
		}

		// Create new session
		newSession := &DebugSession{
			ID:         id,
			Approach:   args.Approach,
			Resolution: args.Resolution,
			Status:     status,
		}
		
		// Track memory usage
		sessionSize := int64(len(newSession.Approach) + len(newSession.Resolution) + len(newSession.ID) + len(newSession.Status))
		if oldSession, exists := dbgSessions[id]; exists {
			// Remove old session memory usage
			oldSize := int64(len(oldSession.Approach) + len(oldSession.Resolution) + len(oldSession.ID) + len(oldSession.Status))
			atomic.AddInt64(&dbgMemUsage, -oldSize)
		}
		atomic.AddInt64(&dbgMemUsage, sessionSize)
		
		dbgSessions[id] = newSession

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
		// Use read lock for read-only operations
		dbgMu.RLock()
		defer dbgMu.RUnlock()
		
		res := PendingDebugResult{}
		for _, id := range dbgOrder {
			if s, ok := dbgSessions[id]; ok && s.Status != "complete" {
				// Copy session to avoid race conditions
				sessionCopy := *s
				res.Sessions = append(res.Sessions, sessionCopy)
			}
		}
		return res, nil
	}
}

// getDebugMemoryUsage returns the current memory usage of debug sessions.
func getDebugMemoryUsage() int64 {
	return atomic.LoadInt64(&dbgMemUsage)
}
