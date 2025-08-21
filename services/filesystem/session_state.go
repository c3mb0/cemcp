package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SessionState tracks per-session usage limits.
type SessionState struct {
	ID          string
	MaxThoughts int
	mu          sync.Mutex
	Thoughts    int
}

// sessionStateKey is used for storing SessionState in context.
type sessionStateKey struct{}

// SessionStateFromContext retrieves SessionState from context if present.
func SessionStateFromContext(ctx context.Context) *SessionState {
	if v, ok := ctx.Value(sessionStateKey{}).(*SessionState); ok {
		return v
	}
	return nil
}

// sessionMiddleware attaches SessionState to each request and enforces limits.
func sessionMiddleware() server.ToolHandlerMiddleware {
	var states sync.Map
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			sid := *sessionIDFlag
			if sid == "" {
				if s := server.ClientSessionFromContext(ctx); s != nil {
					sid = s.SessionID()
				}
			}
			if sid == "" {
				sid = "anonymous"
			}

			v, _ := states.LoadOrStore(sid, &SessionState{ID: sid, MaxThoughts: *maxThoughtsFlag})
			state := v.(*SessionState)
			ctx = context.WithValue(ctx, sessionStateKey{}, state)

			state.mu.Lock()
			defer state.mu.Unlock()
			if state.MaxThoughts > 0 && state.Thoughts >= state.MaxThoughts {
				return nil, fmt.Errorf("max thoughts exceeded")
			}
			state.Thoughts++
			return next(ctx, req)
		}
	}
}
