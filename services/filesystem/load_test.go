package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestHighLoadSessionManagement tests session management under high load
func TestHighLoadSessionManagement(t *testing.T) {
	config := DefaultSessionConfig()
	config.MaxSessions = 150  // Allow more than the required 100
	config.SessionTimeout = 5 * time.Minute
	config.CleanupInterval = 1 * time.Minute
	
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	const (
		numGoroutines = 100
		operationsPerGoroutine = 1000
		targetSessions = 100
	)
	
	// Track metrics
	var (
		totalOperations   int64
		successfulOps     int64
		failedOps         int64
		maxMemoryUsed     int64
		sessionsCreated   int64
	)
	
	start := time.Now()
	
	var wg sync.WaitGroup
	
	// Create initial sessions
	for i := 0; i < targetSessions; i++ {
		sessionID := fmt.Sprintf("load_test_%d", i)
		err := sm.CreateSession(sessionID, fmt.Sprintf("/tmp/session_%d", i))
		if err == nil {
			atomic.AddInt64(&sessionsCreated, 1)
		}
	}
	
	t.Logf("Created %d initial sessions", sessionsCreated)
	
	// High-load concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for op := 0; op < operationsPerGoroutine; op++ {
				atomic.AddInt64(&totalOperations, 1)
				
				sessionID := fmt.Sprintf("load_test_%d", workerID%targetSessions)
				
				// Get session
				session, err := sm.GetSession(sessionID)
				if err != nil {
					atomic.AddInt64(&failedOps, 1)
					continue
				}
				
				// Simulate work on session
				_ = session.SafeRoot()
				
				// Update memory usage to simulate real work
				memUsage := int64((workerID + op) % 1024) // Simulate variable memory usage
				session.UpdateMemoryUsage(memUsage)
				
				// Track maximum memory
				if memUsage > atomic.LoadInt64(&maxMemoryUsed) {
					atomic.StoreInt64(&maxMemoryUsed, memUsage)
				}
				
				// Release session
				sm.ReleaseSession(session)
				atomic.AddInt64(&successfulOps, 1)
				
				// Small delay to prevent overwhelming the system
				if op%100 == 0 {
					time.Sleep(time.Microsecond)
				}
			}
		}(i)
	}
	
	// Monitor progress
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			ops := atomic.LoadInt64(&totalOperations)
			success := atomic.LoadInt64(&successfulOps)
			failed := atomic.LoadInt64(&failedOps)
			
			stats := sm.GetStats()
			t.Logf("Progress: %d ops (%d success, %d failed), Sessions: %v", ops, success, failed, stats)
			
			if ops >= int64(numGoroutines*operationsPerGoroutine) {
				return
			}
		}
	}()
	
	wg.Wait()
	duration := time.Since(start)
	
	// Final metrics
	finalStats := sm.GetStats()
	totalOps := atomic.LoadInt64(&totalOperations)
	successOps := atomic.LoadInt64(&successfulOps)
	failedOpsCount := atomic.LoadInt64(&failedOps)
	maxMem := atomic.LoadInt64(&maxMemoryUsed)
	
	t.Logf("Load test completed in %v", duration)
	t.Logf("Total operations: %d", totalOps)
	t.Logf("Successful operations: %d (%.2f%%)", successOps, float64(successOps)/float64(totalOps)*100)
	t.Logf("Failed operations: %d (%.2f%%)", failedOpsCount, float64(failedOpsCount)/float64(totalOps)*100)
	t.Logf("Operations per second: %.2f", float64(totalOps)/duration.Seconds())
	t.Logf("Max memory per session: %d bytes", maxMem)
	t.Logf("Final session stats: %+v", finalStats)
	
	// Verify requirements
	expected := int64(numGoroutines * operationsPerGoroutine)
	if totalOps != expected {
		t.Errorf("Expected %d total operations, got %d", expected, totalOps)
	}
	
	// Check that we can handle 100+ sessions
	if totalSessions, ok := finalStats["total_sessions"].(int); ok {
		if totalSessions < targetSessions {
			t.Errorf("Expected at least %d sessions, got %d", targetSessions, totalSessions)
		}
	}
	
	// Check memory usage is reasonable (under 50MB per session)
	const maxMemoryPerSession = 50 * 1024 * 1024 // 50MB
	if maxMem > maxMemoryPerSession {
		t.Errorf("Memory usage per session exceeded limit: %d > %d", maxMem, maxMemoryPerSession)
	}
	
	// Check success rate is high (> 95%)
	successRate := float64(successOps) / float64(totalOps)
	if successRate < 0.95 {
		t.Errorf("Success rate too low: %.2f%% < 95%%", successRate*100)
	}
	
	// Check operations per second is reasonable (> 10,000 ops/sec)
	opsPerSecond := float64(totalOps) / duration.Seconds()
	if opsPerSecond < 10000 {
		t.Errorf("Operations per second too low: %.2f < 10000", opsPerSecond)
	}
}

// TestMemoryPressureHandling tests behavior under memory pressure
func TestMemoryPressureHandling(t *testing.T) {
	config := DefaultSessionConfig()
	config.MaxSessions = 20
	config.MaxMemoryPerSession = 1024 * 1024 // 1MB limit
	
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	// Create sessions and gradually increase memory usage
	const numSessions = 10
	
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("memory_test_%d", i)
		err := sm.CreateSession(sessionID, "/tmp")
		if err != nil {
			t.Fatalf("Failed to create session %s: %v", sessionID, err)
		}
	}
	
	var totalMemoryUsed int64
	
	// Simulate memory pressure
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("memory_test_%d", i)
		
		session, err := sm.GetSession(sessionID)
		if err != nil {
			t.Errorf("Failed to get session %s: %v", sessionID, err)
			continue
		}
		
		// Simulate increasing memory usage
		memUsage := int64(i+1) * 100 * 1024 // 100KB * (i+1)
		session.UpdateMemoryUsage(memUsage)
		atomic.AddInt64(&totalMemoryUsed, memUsage)
		
		sm.ReleaseSession(session)
	}
	
	stats := sm.GetStats()
	
	t.Logf("Total memory used: %d bytes", totalMemoryUsed)
	t.Logf("Stats under memory pressure: %+v", stats)
	
	// Verify memory tracking works correctly
	if totalMem, ok := stats["total_memory"].(int64); ok {
		if totalMem != totalMemoryUsed {
			t.Errorf("Expected total memory %d, got %d", totalMemoryUsed, totalMem)
		}
	}
}

// TestSessionConcurrencyLimits tests behavior at concurrency limits
func TestSessionConcurrencyLimits(t *testing.T) {
	config := DefaultSessionConfig()
	config.MaxSessions = 5 // Low limit for testing
	
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	const numGoroutines = 10
	
	var wg sync.WaitGroup
	var successfulCreations int64
	var rejectedCreations int64
	
	// Try to create more sessions than the limit concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			sessionID := fmt.Sprintf("limit_test_%d", id)
			err := sm.CreateSession(sessionID, "/tmp")
			if err == nil {
				atomic.AddInt64(&successfulCreations, 1)
			} else {
				atomic.AddInt64(&rejectedCreations, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	successful := atomic.LoadInt64(&successfulCreations)
	rejected := atomic.LoadInt64(&rejectedCreations)
	
	t.Logf("Successful creations: %d", successful)
	t.Logf("Rejected creations: %d", rejected)
	
	// Should only allow up to MaxSessions
	if successful != int64(config.MaxSessions) {
		t.Errorf("Expected %d successful creations, got %d", config.MaxSessions, successful)
	}
	
	if rejected != int64(numGoroutines-config.MaxSessions) {
		t.Errorf("Expected %d rejections, got %d", numGoroutines-config.MaxSessions, rejected)
	}
}

// TestLongRunningStabilityTest tests system stability over time
func TestLongRunningStabilityTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running stability test in short mode")
	}
	
	config := DefaultSessionConfig()
	config.MaxSessions = 50
	config.SessionTimeout = 1 * time.Second
	config.CleanupInterval = 500 * time.Millisecond
	
	sm := NewSafeSessionManager(config)
	defer sm.Stop()
	
	const testDuration = 10 * time.Second
	const numWorkers = 20
	
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()
	
	var wg sync.WaitGroup
	var operations int64
	var memoryLeaks int64
	
	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			sessionCounter := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				sessionID := fmt.Sprintf("stability_%d_%d", workerID, sessionCounter)
				sessionCounter++
				
				// Create session
				err := sm.CreateSession(sessionID, "/tmp")
				if err == nil {
					// Use session briefly
					if session, getErr := sm.GetSession(sessionID); getErr == nil {
						session.UpdateMemoryUsage(1024)
						sm.ReleaseSession(session)
					}
					
					// Let it expire naturally or delete it randomly
					if sessionCounter%3 == 0 {
						_ = sm.DeleteSession(sessionID)
					}
				}
				
				atomic.AddInt64(&operations, 1)
				
				// Brief pause
				time.Sleep(time.Millisecond)
			}
		}(i)
	}
	
	// Monitor for memory leaks and stability
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		
		var lastMemory runtime.MemStats
		runtime.ReadMemStats(&lastMemory)
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				
				// Check for significant memory growth
				if m.Alloc > lastMemory.Alloc*2 {
					atomic.AddInt64(&memoryLeaks, 1)
				}
				
				stats := sm.GetStats()
				t.Logf("Stability check - Operations: %d, Stats: %+v, Memory: %dKB",
					atomic.LoadInt64(&operations), stats, m.Alloc/1024)
				
				lastMemory = m
			}
		}
	}()
	
	wg.Wait()
	
	finalOperations := atomic.LoadInt64(&operations)
	leaks := atomic.LoadInt64(&memoryLeaks)
	
	t.Logf("Stability test completed: %d operations, %d potential memory leaks", finalOperations, leaks)
	
	if finalOperations == 0 {
		t.Error("Expected some operations to complete")
	}
	
	if leaks > 2 { // Allow for some variance
		t.Errorf("Too many potential memory leaks detected: %d", leaks)
	}
	
	// Final cleanup check
	time.Sleep(2 * config.CleanupInterval)
	finalStats := sm.GetStats()
	t.Logf("Final stats after cleanup: %+v", finalStats)
}