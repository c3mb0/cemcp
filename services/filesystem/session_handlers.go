package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func handleCreateSession(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[CreateSessionArgs, CreateSessionResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args CreateSessionArgs) (CreateSessionResult, error) {
		id := args.ID
		if id == "" {
			id = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		mu.Lock()
		if _, exists := sessions[id]; exists {
			mu.Unlock()
			return CreateSessionResult{}, fmt.Errorf("session %s exists", id)
		}
		// Copy root from current session if available
		root := ""
		if state, err := getSessionState(ctx, sessions, mu); err == nil {
			root = state.Root
		}
		sessions[id] = &SessionState{Root: root}
		mu.Unlock()
		return CreateSessionResult{ID: id}, nil
	}
}

func handleSwitchSession(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[SwitchSessionArgs, SwitchSessionResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args SwitchSessionArgs) (SwitchSessionResult, error) {
		mu.RLock()
		_, ok := sessions[args.ID]
		mu.RUnlock()
		if !ok {
			return SwitchSessionResult{}, fmt.Errorf("session %s not found", args.ID)
		}
		setSessionID(ctx, args.ID)
		return SwitchSessionResult{ID: args.ID}, nil
	}
}

func handleListSessions(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[struct{}, ListSessionsResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args struct{}) (ListSessionsResult, error) {
		mu.RLock()
		ids := make([]string, 0, len(sessions))
		for id := range sessions {
			ids = append(ids, id)
		}
		mu.RUnlock()
		return ListSessionsResult{Sessions: ids, Active: getSessionID(ctx)}, nil
	}
}
