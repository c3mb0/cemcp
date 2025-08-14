package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// detectMIME determines MIME type from filename and content sample
func detectMIME(name string, sample []byte) string {
	if ext := filepath.Ext(name); ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt
		}
	}
	if isText(sample) {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

// isText performs enhanced text detection with UTF-8 validation
func isText(b []byte) bool {
	if len(b) == 0 {
		return true
	}

	// Check for null bytes (strong indicator of binary)
	for _, c := range b {
		if c == 0 {
			return false
		}
	}

	// Validate UTF-8 encoding
	if !utf8.Valid(b) {
		return false
	}

	// Count control characters vs printable
	controlCount := 0
	totalCount := len(b)

	for _, c := range b {
		// Allow common whitespace
		if c == 9 || c == 10 || c == 13 {
			continue
		}
		// Count other control characters
		if c < 32 || c == 0x7f {
			controlCount++
		}
	}

	// If more than 30% control characters, likely binary
	if float64(controlCount)/float64(totalCount) > 0.3 {
		return false
	}

	return true
}

// sha256sumStream computes SHA256 with streaming to avoid memory issues
func sha256sumStream(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.CopyN(h, f, maxHashBytes); err != nil && err != io.EOF {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// sha256sum computes SHA256 of byte slice
func sha256sum(b []byte) string {
	s := sha256.Sum256(b)
	return fmt.Sprintf("%x", s[:])
}

// ensureParent creates parent directories with proper error handling
func ensureParent(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}
	return nil
}

// parseMode parses file mode with validation
func parseMode(s string) (os.FileMode, error) {
	if s == "" {
		return 0o644, nil
	}

	// Normalize format
	if !strings.HasPrefix(s, "0") {
		s = "0" + s
	}

	u, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode format: %w", err)
	}

	// Validate reasonable permissions
	if u > 0o777 {
		return 0, fmt.Errorf("mode exceeds maximum permissions (0777): %#o", u)
	}

	return os.FileMode(u), nil
}

// atomicWrite performs atomic file write with enhanced error handling
func atomicWrite(target string, data []byte, mode os.FileMode) error {
	// Check available disk space (approximate)
	if err := checkDiskSpace(target, int64(len(data))); err != nil {
		return fmt.Errorf("insufficient disk space: %w", err)
	}

	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".mcpfs-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Ensure cleanup on any error
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	// Write data
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk before rename
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Set permissions
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpName, target); err != nil {
		// Windows fallback: remove target first
		if runtime.GOOS == "windows" {
			if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("failed to remove target for Windows rename: %w", removeErr)
			}
			if err := os.Rename(tmpName, target); err != nil {
				return fmt.Errorf("failed to rename on Windows: %w", err)
			}
		} else {
			return fmt.Errorf("failed to rename temp file: %w", err)
		}
	}

	success = true
	return nil
}

// acquireLock creates an advisory lock with improved stale detection
func acquireLock(path string, timeout time.Duration) (release func(), err error) {
	lock := path + ".lock"
	deadline := time.Now().Add(timeout)

	// Use exponential backoff
	wait := 10 * time.Millisecond
	maxWait := 500 * time.Millisecond

	for {
		// Try to create lock file
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			// Write PID and timestamp for debugging
			_, _ = fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), time.Now().Unix())
			_ = f.Close()

			return func() {
				_ = os.Remove(lock)
			}, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("failed to create lock file: %w", err)
		}

		// Check for stale lock
		if info, statErr := os.Stat(lock); statErr == nil {
			age := time.Since(info.ModTime())
			// Reduced stale timeout to 5 minutes
			if age > 5*time.Minute {
				dprintf("removing stale lock (age=%v): %s", age, lock)
				_ = os.Remove(lock)
				continue
			}
		}

		// Check timeout
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("lock acquisition timeout after %v: %s", timeout, path)
		}

		// Exponential backoff
		time.Sleep(wait)
		wait *= 2
		if wait > maxWait {
			wait = maxWait
		}
	}
}

// kindOf returns the file type as a string
func kindOf(fi os.FileInfo) string {
	m := fi.Mode()
	switch {
	case m.IsRegular():
		return "file"
	case m.IsDir():
		return "dir"
	case m&os.ModeSymlink != 0:
		return "symlink"
	case m&os.ModeNamedPipe != 0:
		return "pipe"
	case m&os.ModeSocket != 0:
		return "socket"
	case m&os.ModeDevice != 0:
		return "device"
	default:
		return "other"
	}
}

// checkDiskSpace verifies approximate available disk space
func checkDiskSpace(path string, needed int64) error {
	// This is a simplified check - proper implementation would use syscalls
	// For now, just check if we're not trying to write something huge
	const maxFileSize = 1 << 30 // 1GB limit per file
	if needed > maxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed (%d)", needed, maxFileSize)
	}
	return nil
}
