package main

import (
	"context"
	"sync"
)

// testSession creates a context with a default session and returns the session map and mutex.
func testSession(root string) (context.Context, map[string]*SessionState, *sync.RWMutex) {
	sessions := map[string]*SessionState{"s1": {Root: root}}
	var mu sync.RWMutex
	ctx := withSessionManager(context.Background(), &sessionManager{id: "s1"})
	return ctx, sessions, &mu
}
