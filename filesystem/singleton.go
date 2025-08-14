package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ensureSingleInstance ensures only one instance of the server runs
func ensureSingleInstance() (func(), error) {
	pidFile := filepath.Join(os.TempDir(), "fs-mcp-go.pid")
	exePath, _ := os.Executable()
	execName := filepath.Base(exePath)

	// Try to read existing PID file
	if b, err := os.ReadFile(pidFile); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(b)), ":", 3)
		if len(parts) >= 2 && parts[1] == execName {
			if oldPid, err := strconv.Atoi(parts[0]); err == nil {
				// Check if process is actually running
				if isProcessRunning(oldPid) {
					dprintf("found running instance with PID %d", oldPid)

					// Try graceful shutdown first
					if err := signalProcess(oldPid, syscall.SIGTERM); err == nil {
						// Wait briefly for graceful shutdown
						for i := 0; i < 10; i++ {
							time.Sleep(100 * time.Millisecond)
							if !isProcessRunning(oldPid) {
								break
							}
						}
					}

					// Force kill if still running
					if isProcessRunning(oldPid) {
						dprintf("force killing old instance PID %d", oldPid)
						_ = signalProcess(oldPid, syscall.SIGKILL)
						time.Sleep(100 * time.Millisecond)
					}
				} else {
					dprintf("stale PID file found for non-running process %d", oldPid)
				}
			}
		}
	}

	// Write new PID file with timestamp
	content := fmt.Sprintf("%d:%s:%d", os.Getpid(), execName, time.Now().Unix())
	if err := os.WriteFile(pidFile, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write PID file: %w", err)
	}

	cleanup := func() {
		// Only remove if it's still our PID
		if b, err := os.ReadFile(pidFile); err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(b)), ":", 3)
			if len(parts) >= 1 {
				if pid, _ := strconv.Atoi(parts[0]); pid == os.Getpid() {
					_ = os.Remove(pidFile)
				}
			}
		}
	}

	return cleanup, nil
}

// isProcessRunning checks if a process with given PID is running
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, sending signal 0 checks if process exists
	if runtime.GOOS != "windows" {
		err := process.Signal(syscall.Signal(0))
		return err == nil
	}

	// On Windows, this is more complex
	// For now, assume process exists if we can find it
	return true
}

// signalProcess sends a signal to a process (Unix-like systems)
func signalProcess(pid int, sig syscall.Signal) error {
	if runtime.GOOS == "windows" {
		// On Windows, just try to kill
		process, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return process.Kill()
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Signal(sig)
}
