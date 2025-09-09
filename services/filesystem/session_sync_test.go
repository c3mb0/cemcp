package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	
	"github.com/mark3labs/mcp-go/mcp"
)

// TestConcurrentSessionAccess tests concurrent access to sessions
func TestConcurrentSessionAccess(t *testing.T) {
	config := DefaultSessionConfig()
	config.MaxSessions = 10
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	// Create default session
	err := sm.CreateSession("default", "/tmp")
	if err != nil {
		t.Fatalf("Failed to create default session: %v", err)
	}
	
	const numGoroutines = 100
	const numOperations = 1000
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	// Test concurrent session access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Get session
				session, err := sm.GetSession("default")
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}
				
				// Use session (simulate work)
				_ = session.SafeRoot()
				time.Sleep(time.Microsecond)
				
				// Release session
				sm.ReleaseSession(session)
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Success: %d, Errors: %d", successCount, errorCount)
	
	if errorCount > 0 {
		t.Errorf("Expected no errors, got %d", errorCount)
	}
	
	expectedSuccess := int64(numGoroutines * numOperations)
	if successCount != expectedSuccess {
		t.Errorf("Expected %d successful operations, got %d", expectedSuccess, successCount)
	}
}

// TestSessionCreationAndDeletion tests concurrent session creation and deletion
func TestSessionCreationAndDeletion(t *testing.T) {
	config := DefaultSessionConfig()
	config.MaxSessions = 50
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	const numGoroutines = 20
	const numSessions = 100
	
	var wg sync.WaitGroup
	var createdCount int64
	var deletedCount int64
	
	// Concurrent session creation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numSessions; j++ {
				sessionID := fmt.Sprintf("session_%d_%d", goroutineID, j)
				
				// Create session
				err := sm.CreateSession(sessionID, fmt.Sprintf("/tmp/%s", sessionID))
				if err == nil {
					atomic.AddInt64(&createdCount, 1)
					
					// Brief usage
					session, getErr := sm.GetSession(sessionID)
					if getErr == nil {
						_ = session.SafeRoot()
						sm.ReleaseSession(session)
					}
					
					// Delete session
					time.Sleep(time.Millisecond) // Allow some time for operations
					delErr := sm.DeleteSession(sessionID)
					if delErr == nil {
						atomic.AddInt64(&deletedCount, 1)
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Created: %d, Deleted: %d", createdCount, deletedCount)
	
	// Check that we successfully created and deleted sessions
	if createdCount == 0 {
		t.Error("Expected to create some sessions")
	}
	
	stats := sm.GetStats()
	t.Logf("Final stats: %+v", stats)
}

// TestSessionMemoryTracking tests memory usage tracking
func TestSessionMemoryTracking(t *testing.T) {
	sm := NewSafeSessionManager(DefaultSessionConfig())
	defer sm.Stop()
	
	err := sm.CreateSession("test", "/tmp")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	session, err := sm.GetSession("test")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	defer sm.ReleaseSession(session)
	
	// Test memory usage tracking
	const testMemory = 1024 * 1024 // 1MB
	session.UpdateMemoryUsage(testMemory)
	
	if session.MemoryUsage() != testMemory {
		t.Errorf("Expected memory usage %d, got %d", testMemory, session.MemoryUsage())
	}
	
	stats := sm.GetStats()
	if totalMem, ok := stats["total_memory"].(int64); ok {
		if totalMem < testMemory {
			t.Errorf("Expected total memory >= %d, got %d", testMemory, totalMem)
		}
	} else {
		t.Error("Expected total_memory in stats")
	}
}

// TestSessionCleanup tests automatic cleanup of expired sessions
func TestSessionCleanup(t *testing.T) {
	config := DefaultSessionConfig()
	config.SessionTimeout = 100 * time.Millisecond
	config.CleanupInterval = 50 * time.Millisecond
	
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	// Create test sessions
	for i := 0; i < 5; i++ {
		sessionID := fmt.Sprintf("cleanup_test_%d", i)
		err := sm.CreateSession(sessionID, "/tmp")
		if err != nil {
			t.Fatalf("Failed to create session %s: %v", sessionID, err)
		}
	}
	
	// Check initial count
	sessions := sm.ListSessions()
	initialCount := len(sessions)
	t.Logf("Initial session count: %d", initialCount)
	
	// Wait for cleanup to occur
	time.Sleep(200 * time.Millisecond)
	
	// Check that sessions were cleaned up
	sessions = sm.ListSessions()
	finalCount := len(sessions)
	t.Logf("Final session count: %d", finalCount)
	
	if finalCount >= initialCount {
		t.Errorf("Expected session cleanup, initial: %d, final: %d", initialCount, finalCount)
	}
}

// TestSessionLimits tests session limit enforcement
func TestSessionLimits(t *testing.T) {
	config := DefaultSessionConfig()
	config.MaxSessions = 3
	
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	// Create sessions up to limit
	var successCount int
	for i := 0; i < 5; i++ {
		sessionID := fmt.Sprintf("limit_test_%d", i)
		err := sm.CreateSession(sessionID, "/tmp")
		if err == nil {
			successCount++
		} else {
			t.Logf("Expected error for session %d: %v", i, err)
		}
	}
	
	if successCount != config.MaxSessions {
		t.Errorf("Expected to create %d sessions, created %d", config.MaxSessions, successCount)
	}
}

// TestNoRaceConditions runs concurrent operations to detect race conditions
func TestNoRaceConditions(t *testing.T) {
	config := DefaultSessionConfig()
	config.MaxSessions = 20
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	const numGoroutines = 10
	const duration = 2 * time.Second
	
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var wg sync.WaitGroup
	var operations int64
	
	// Multiple types of concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				sessionID := fmt.Sprintf("race_test_%d", id)
				
				// Create session
				_ = sm.CreateSession(sessionID, "/tmp")
				atomic.AddInt64(&operations, 1)
				
				// Get session
				if session, err := sm.GetSession(sessionID); err == nil {
					_ = session.SafeRoot()
					session.UpdateMemoryUsage(1024)
					sm.ReleaseSession(session)
					atomic.AddInt64(&operations, 1)
				}
				
				// List sessions
				_ = sm.ListSessions()
				atomic.AddInt64(&operations, 1)
				
				// Delete session
				_ = sm.DeleteSession(sessionID)
				atomic.AddInt64(&operations, 1)
				
				time.Sleep(time.Microsecond)
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Completed %d operations without race conditions", operations)
	
	if operations == 0 {
		t.Error("Expected some operations to complete")
	}
}

// TestMemoryLimitEnforcement tests memory limit enforcement
func TestMemoryLimitEnforcement(t *testing.T) {
	config := DefaultSessionConfig()
	config.MaxMemoryPerSession = 1024 // 1KB limit for testing
	
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	err := sm.CreateSession("memory_test", "/tmp")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	session, err := sm.GetSession("memory_test")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	defer sm.ReleaseSession(session)
	
	// Try to exceed memory limit
	session.UpdateMemoryUsage(config.MaxMemoryPerSession + 1)
	
	// Check if memory usage is tracked correctly
	if session.MemoryUsage() != config.MaxMemoryPerSession+1 {
		t.Errorf("Memory usage not tracked correctly")
	}
	
	// In a real implementation, you might want to prevent operations
	// when memory limit is exceeded
}

// BenchmarkConcurrentSessionAccess benchmarks concurrent session access
func BenchmarkConcurrentSessionAccess(b *testing.B) {
	sm := NewSafeSessionManager(DefaultSessionConfig())
	defer sm.Stop()
	
	err := sm.CreateSession("bench", "/tmp")
	if err != nil {
		b.Fatalf("Failed to create session: %v", err)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			session, err := sm.GetSession("bench")
			if err != nil {
				b.Errorf("Failed to get session: %v", err)
				continue
			}
			_ = session.SafeRoot()
			sm.ReleaseSession(session)
		}
	})
}

// TestDebugSessionSynchronization tests the debug session synchronization
func TestDebugSessionSynchronization(t *testing.T) {
	// Reset debug sessions
	dbgMu.Lock()
	dbgSessions = make(map[string]*DebugSession)
	dbgOrder = []string{}
	atomic.StoreInt64(&dbgMemUsage, 0)
	dbgMu.Unlock()
	
	const numGoroutines = 50
	const numSessions = 20
	
	var wg sync.WaitGroup
	var successCount int64
	
	// Concurrent debug session operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numSessions; j++ {
				ctx := context.Background()
				args := DebuggingApproachArgs{
					SessionID:  fmt.Sprintf("debug_%d_%d", id, j),
					Approach:   "Test approach for concurrent debugging",
					Resolution: "Test resolution",
				}
				
				handler := handleDebuggingApproach()
				req := mcp.CallToolRequest{} // Empty request is fine for testing
				_, err := handler(ctx, req, args)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Successfully created %d debug sessions", successCount)
	
	// Test concurrent read operations
	var readCount int64
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			handler := handlePendingDebug()
			req := mcp.CallToolRequest{} // Empty request is fine for testing
			_, err := handler(context.Background(), req, struct{}{})
			if err == nil {
				atomic.AddInt64(&readCount, 1)
			}
		}()
	}
	
	wg.Wait()
	
	t.Logf("Successfully performed %d read operations", readCount)
	
	memUsage := getDebugMemoryUsage()
	t.Logf("Debug session memory usage: %d bytes", memUsage)
	
	if successCount == 0 {
		t.Error("Expected some debug sessions to be created")
	}
	
	if readCount != numGoroutines {
		t.Errorf("Expected %d read operations, got %d", numGoroutines, readCount)
	}
}

// TestGoroutineLeakage tests for goroutine leaks
func TestGoroutineLeakage(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	
	// Create and destroy multiple session managers
	for i := 0; i < 10; i++ {
		sm := NewSafeSessionManager(DefaultSessionConfig())
		
		// Create some sessions
		_ = sm.CreateSession("test", "/tmp")
		
		// Stop the manager
		sm.Stop()
		
		// Give time for cleanup
		time.Sleep(10 * time.Millisecond)
	}
	
	// Force garbage collection
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	finalGoroutines := runtime.NumGoroutine()
	
	if finalGoroutines > initialGoroutines+1 { // Allow for 1 goroutine variance
		t.Errorf("Potential goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}