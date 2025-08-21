package main

import (
	"context"
	"fmt"
	"sync"
)

// SessionState holds data for a single session.
type SessionState struct {
	Root string
}

// sessionManager keeps track of the active session ID per connection.
type sessionManager struct {
	mu sync.RWMutex
	id string
}

type sessionManagerKey struct{}

func withSessionManager(ctx context.Context, m *sessionManager) context.Context {
	return context.WithValue(ctx, sessionManagerKey{}, m)
}

func getSessionID(ctx context.Context) string {
	if m, ok := ctx.Value(sessionManagerKey{}).(*sessionManager); ok {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return m.id
	}
	return ""
}

func setSessionID(ctx context.Context, id string) {
	if m, ok := ctx.Value(sessionManagerKey{}).(*sessionManager); ok {
		m.mu.Lock()
		m.id = id
		m.mu.Unlock()
	}
}

func sessionContext(ctx context.Context) string {
	id := getSessionID(ctx)
	if id == "" {
		id = "unknown"
	}
	return fmt.Sprintf("session=%s", id)
}

// getSessionState retrieves the SessionState for the current session ID.
func getSessionState(ctx context.Context, sessions map[string]*SessionState, mu *sync.RWMutex) (*SessionState, error) {
	id := getSessionID(ctx)
	mu.RLock()
	state, ok := sessions[id]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown session %s", id)
	}
	return state, nil
}
