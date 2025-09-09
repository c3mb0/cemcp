package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
		defer mu.Unlock()
		
		// Check if session already exists
		if _, exists := sessions[id]; exists {
			return CreateSessionResult{}, fmt.Errorf("session %s already exists", id)
		}
		
		// Copy root from current session if available
		root := ""
		currentID := getSessionID(ctx)
		if currentID != "" && currentID != id {
			if currentState, exists := sessions[currentID]; exists {
				root = currentState.SafeRoot()
			}
		}
		
		// Create new session with proper initialization
		sessions[id] = NewSessionState(id, root)
		
		return CreateSessionResult{ID: id}, nil
	}
}

func handleSwitchSession(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[SwitchSessionArgs, SwitchSessionResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args SwitchSessionArgs) (SwitchSessionResult, error) {
		mu.RLock()
		session, exists := sessions[args.ID]
		mu.RUnlock()
		
		if !exists {
			return SwitchSessionResult{}, fmt.Errorf("session %s not found", args.ID)
		}
		
		// Update last accessed time
		if session != nil {
			session.IncRef()  // Track usage
			session.DecRef()  // But don't hold reference
		}
		
		setSessionID(ctx, args.ID)
		return SwitchSessionResult(args), nil
	}
}

func handleListSessions(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[struct{}, ListSessionsResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args struct{}) (ListSessionsResult, error) {
		mu.RLock()
		defer mu.RUnlock()
		
		ids := make([]string, 0, len(sessions))
		for id, session := range sessions {
			// Update last accessed time for each session we iterate over
			if session != nil {
				atomic.StoreInt64(&session.lastAccessed, time.Now().Unix())
			}
			ids = append(ids, id)
		}
		
		return ListSessionsResult{Sessions: ids, Active: getSessionID(ctx)}, nil
	}
}
