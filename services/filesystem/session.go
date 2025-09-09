package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SessionState holds data for a single session with thread-safe management.
type SessionState struct {
	Root         string
	mu           sync.RWMutex     // Per-session lock
	id           string           // Session ID
	createdAt    time.Time        // Creation timestamp
	lastAccessed int64            // Last access timestamp (atomic)
	refCount     int64            // Reference counter for safe cleanup (atomic)
	memoryUsage  int64            // Memory usage in bytes (atomic)
}

// NewSessionState creates a new session with proper initialization.
func NewSessionState(id, root string) *SessionState {
	now := time.Now()
	return &SessionState{
		Root:         root,
		id:           id,
		createdAt:    now,
		lastAccessed: now.Unix(),
		refCount:     1,
		memoryUsage:  0,
	}
}

// IncRef atomically increments the reference count.
func (s *SessionState) IncRef() {
	atomic.AddInt64(&s.refCount, 1)
	atomic.StoreInt64(&s.lastAccessed, time.Now().Unix())
}

// DecRef atomically decrements the reference count.
func (s *SessionState) DecRef() int64 {
	return atomic.AddInt64(&s.refCount, -1)
}

// RefCount returns the current reference count.
func (s *SessionState) RefCount() int64 {
	return atomic.LoadInt64(&s.refCount)
}

// UpdateMemoryUsage atomically updates the memory usage.
func (s *SessionState) UpdateMemoryUsage(bytes int64) {
	atomic.StoreInt64(&s.memoryUsage, bytes)
}

// MemoryUsage returns the current memory usage.
func (s *SessionState) MemoryUsage() int64 {
	return atomic.LoadInt64(&s.memoryUsage)
}

// LastAccessed returns the last accessed timestamp.
func (s *SessionState) LastAccessed() time.Time {
	timestamp := atomic.LoadInt64(&s.lastAccessed)
	return time.Unix(timestamp, 0)
}

// IsExpired checks if the session has expired.
func (s *SessionState) IsExpired(timeout time.Duration) bool {
	return time.Since(s.LastAccessed()) > timeout
}

// SafeRoot returns the root path with read lock.
func (s *SessionState) SafeRoot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Root
}

// SetRoot sets the root path with write lock.
func (s *SessionState) SetRoot(root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Root = root
	atomic.StoreInt64(&s.lastAccessed, time.Now().Unix())
}

// sessionManager keeps track of the active session ID per connection.
type sessionManager struct {
	mu sync.RWMutex
	id string
}

// SessionConfig holds configuration for session management.
type SessionConfig struct {
	MaxSessions      int           // Maximum number of concurrent sessions
	SessionTimeout   time.Duration // Session timeout duration  
	CleanupInterval  time.Duration // Cleanup interval for expired sessions
	MaxMemoryPerSession int64      // Maximum memory per session in bytes
}

// DefaultSessionConfig returns default session configuration.
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		MaxSessions:         100,
		SessionTimeout:      30 * time.Minute,
		CleanupInterval:     5 * time.Minute,
		MaxMemoryPerSession: 50 * 1024 * 1024, // 50MB
	}
}

// SafeSessionManager provides thread-safe session management with cleanup.
type SafeSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionState
	config   *SessionConfig
	stopCh   chan struct{}
	once     sync.Once
}

// NewSafeSessionManager creates a new thread-safe session manager.
func NewSafeSessionManager(config *SessionConfig) *SafeSessionManager {
	if config == nil {
		config = DefaultSessionConfig()
	}
	
	sm := &SafeSessionManager{
		sessions: make(map[string]*SessionState),
		config:   config,
		stopCh:   make(chan struct{}),
	}
	
	// Start background cleanup goroutine
	go sm.cleanupWorker()
	
	return sm
}

// CreateSession creates a new session with the given ID and root.
func (sm *SafeSessionManager) CreateSession(id, root string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Check if session already exists
	if _, exists := sm.sessions[id]; exists {
		return fmt.Errorf("session %s already exists", id)
	}
	
	// Check session limit
	if len(sm.sessions) >= sm.config.MaxSessions {
		return fmt.Errorf("maximum number of sessions (%d) exceeded", sm.config.MaxSessions)
	}
	
	// Create new session
	session := NewSessionState(id, root)
	sm.sessions[id] = session
	
	return nil
}

// GetSession retrieves a session by ID with proper reference counting.
func (sm *SafeSessionManager) GetSession(id string) (*SessionState, error) {
	sm.mu.RLock()
	session, exists := sm.sessions[id]
	sm.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("session %s not found", id)
	}
	
	// Increment reference count
	session.IncRef()
	return session, nil
}

// ReleaseSession decrements the reference count for a session.
func (sm *SafeSessionManager) ReleaseSession(session *SessionState) {
	if session != nil {
		session.DecRef()
	}
}

// DeleteSession removes a session if it has no active references.
func (sm *SafeSessionManager) DeleteSession(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	session, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session %s not found", id)
	}
	
	// Check if session has active references
	if session.RefCount() > 0 {
		return fmt.Errorf("session %s has active references, cannot delete", id)
	}
	
	delete(sm.sessions, id)
	return nil
}

// ListSessions returns a list of all session IDs.
func (sm *SafeSessionManager) ListSessions() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	ids := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		ids = append(ids, id)
	}
	return ids
}

// GetStats returns session statistics.
func (sm *SafeSessionManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	totalMemory := int64(0)
	activeSessions := 0
	
	for _, session := range sm.sessions {
		totalMemory += session.MemoryUsage()
		if session.RefCount() > 0 {
			activeSessions++
		}
	}
	
	return map[string]interface{}{
		"total_sessions":  len(sm.sessions),
		"active_sessions": activeSessions,
		"total_memory":    totalMemory,
		"max_sessions":    sm.config.MaxSessions,
	}
}

// cleanupWorker runs in the background to clean up expired sessions.
func (sm *SafeSessionManager) cleanupWorker() {
	ticker := time.NewTicker(sm.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions removes expired sessions with no references.
func (sm *SafeSessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	now := time.Now()
	toDelete := make([]string, 0)
	
	for id, session := range sm.sessions {
		// Skip default session
		if id == "default" {
			continue
		}
		
		// Check if session is expired and has no references (excluding initial ref count of 1)
		if session.IsExpired(sm.config.SessionTimeout) && session.RefCount() <= 1 {
			toDelete = append(toDelete, id)
		}
	}
	
	// Delete expired sessions
	for _, id := range toDelete {
		delete(sm.sessions, id)
	}
	
	if len(toDelete) > 0 {
		fmt.Printf("Cleaned up %d expired sessions at %v\n", len(toDelete), now)
	}
}

// Stop shuts down the session manager.
func (sm *SafeSessionManager) Stop() {
	sm.once.Do(func() {
		close(sm.stopCh)
	})
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
// This function is kept for backward compatibility but should be replaced with SafeSessionManager.
func getSessionState(ctx context.Context, sessions map[string]*SessionState, mu *sync.RWMutex) (*SessionState, error) {
	id := getSessionID(ctx)
	mu.RLock()
	state, ok := sessions[id]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown session %s", id)
	}
	// Update last accessed time atomically
	if state != nil {
		atomic.StoreInt64(&state.lastAccessed, time.Now().Unix())
	}
	return state, nil
}

// getSessionStateSafe retrieves the SessionState using the SafeSessionManager.
//nolint:unused // Utility function kept for future use
func getSessionStateSafe(ctx context.Context, manager *SafeSessionManager) (*SessionState, error) {
	id := getSessionID(ctx)
	if id == "" {
		id = "default" // Fallback to default session
	}
	
	session, err := manager.GetSession(id)
	if err != nil {
		return nil, fmt.Errorf("unknown session %s: %w", id, err)
	}
	
	return session, nil
}
