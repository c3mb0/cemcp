package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// readFileWithHash reads file content and computes hash using single file handle
func readFileWithHash(path string, contentLimit, fileSize int64) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			dprintf("warning: failed to close file %s: %v", path, closeErr)
		}
	}()

	// Reserve memory for content reading
	if err := reserveMemory(contentLimit); err != nil {
		// Fallback to streaming if memory is limited
		return readFileStreamingFallback(f, path, contentLimit, fileSize)
	}
	defer releaseMemory(contentLimit)

	var content []byte
	var sha string

	if fileSize <= maxHashBytes {
		// Small file: read content and compute hash simultaneously using TeeReader
		hasher := sha256.New()
		limitedReader := io.LimitReader(f, contentLimit)
		teeReader := io.TeeReader(limitedReader, hasher)

		content, err = io.ReadAll(teeReader)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read file content: %w", err)
		}

		sha = fmt.Sprintf("%x", hasher.Sum(nil))
	} else {
		// Large file: read content only, skip hash
		limitedReader := io.LimitReader(f, contentLimit)
		content, err = io.ReadAll(limitedReader)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read file content: %w", err)
		}
		dprintf("fs_read: skip sha256 (size %d > cap %d)", fileSize, maxHashBytes)
	}

	return content, sha, nil
}

// readFileStreamingFallback handles file reading when memory is constrained
func readFileStreamingFallback(f *os.File, path string, contentLimit, fileSize int64) ([]byte, string, error) {
	dprintf("fs_read: using streaming fallback for %s due to memory constraints", path)
	
	// Use smaller buffer to reduce memory pressure
	const streamBufferSize = 8192
	buffer := make([]byte, streamBufferSize)
	var content []byte
	var hasher hash.Hash
	var shouldHash bool
	
	if fileSize <= maxHashBytes {
		hasher = sha256.New()
		shouldHash = true
	}
	
	var totalRead int64
	for totalRead < contentLimit {
		toRead := streamBufferSize
		if contentLimit-totalRead < int64(streamBufferSize) {
			toRead = int(contentLimit - totalRead)
		}
		
		n, err := f.Read(buffer[:toRead])
		if n > 0 {
			content = append(content, buffer[:n]...)
			if shouldHash {
				hasher.Write(buffer[:n])
			}
			totalRead += int64(n)
		}
		
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to read file chunk: %w", err)
		}
	}
	
	var sha string
	if shouldHash {
		sha = fmt.Sprintf("%x", hasher.Sum(nil))
	}
	
	return content, sha, nil
}

// sha256sumStream computes SHA256 with streaming to avoid memory issues
//nolint:unused // Utility function kept for future use
func sha256sumStream(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			dprintf("warning: failed to close file %s: %v", path, closeErr)
		}
	}()

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

// atomicWrite performs atomic file write with enhanced error handling and resource management
func atomicWrite(target string, data []byte, mode os.FileMode) error {
	// Check available disk space (approximate)
	if err := checkDiskSpace(target, int64(len(data))); err != nil {
		return fmt.Errorf("insufficient disk space: %w", err)
	}

	// Reserve memory for the operation
	dataSize := int64(len(data))
	if err := reserveMemory(dataSize); err != nil {
		// Try streaming write for large files when memory is constrained
		return atomicWriteStreaming(target, data, mode)
	}
	defer releaseMemory(dataSize)

	// Acquire file handle resource
	if err := globalFileHandlePool.Acquire(); err != nil {
		dprintf("warning: file handle pool exhausted, proceeding without pooling")
	} else {
		defer globalFileHandlePool.Release()
	}

	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".mcpfs-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Register temp file for cleanup tracking
	globalTempFileManager.RegisterTempFile(tmpName)
	
	// Ensure cleanup on any error
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
			globalTempFileManager.UnregisterTempFile(tmpName)
		}
	}()

	// Write data with error handling
	if _, err := tmp.Write(data); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			dprintf("warning: failed to close temp file during error cleanup: %v", closeErr)
		}
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk before rename
	if err := tmp.Sync(); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			dprintf("warning: failed to close temp file during error cleanup: %v", closeErr)
		}
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Set permissions
	if err := tmp.Chmod(mode); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			dprintf("warning: failed to close temp file during error cleanup: %v", closeErr)
		}
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

	// Unregister temp file after successful completion
	globalTempFileManager.UnregisterTempFile(tmpName)
	success = true
	return nil
}

// atomicWriteStreaming performs streaming write for large files when memory is constrained
func atomicWriteStreaming(target string, data []byte, mode os.FileMode) error {
	dprintf("using streaming write for %s due to memory constraints", target)
	
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".mcpfs-stream-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for streaming: %w", err)
	}
	tmpName := tmp.Name()

	globalTempFileManager.RegisterTempFile(tmpName)
	
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
			globalTempFileManager.UnregisterTempFile(tmpName)
		}
	}()

	// Write in chunks to minimize memory usage
	const chunkSize = 64 * 1024 // 64KB chunks
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		
		if _, err := tmp.Write(data[i:end]); err != nil {
			if closeErr := tmp.Close(); closeErr != nil {
				dprintf("warning: failed to close streaming temp file during error: %v", closeErr)
			}
			return fmt.Errorf("failed to write chunk to temp file: %w", err)
		}
	}

	if err := tmp.Sync(); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			dprintf("warning: failed to close streaming temp file during sync error: %v", closeErr)
		}
		return fmt.Errorf("failed to sync streaming temp file: %w", err)
	}

	if err := tmp.Chmod(mode); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			dprintf("warning: failed to close streaming temp file during chmod error: %v", closeErr)
		}
		return fmt.Errorf("failed to set permissions on streaming temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close streaming temp file: %w", err)
	}

	if err := os.Rename(tmpName, target); err != nil {
		if runtime.GOOS == "windows" {
			if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("failed to remove target for Windows streaming rename: %w", removeErr)
			}
			if err := os.Rename(tmpName, target); err != nil {
				return fmt.Errorf("failed to streaming rename on Windows: %w", err)
			}
		} else {
			return fmt.Errorf("failed to streaming rename temp file: %w", err)
		}
	}

	globalTempFileManager.UnregisterTempFile(tmpName)
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

// getLanguageFromExtension maps file extensions to language identifiers for syntax highlighting
func getLanguageFromExtension(filePath string) string {
	if filePath == "" {
		return ""
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// Comprehensive file extension to language mapping
	extensionMap := map[string]string{
		// Web technologies
		".js":   "js",
		".jsx":  "jsx", 
		".ts":   "ts",
		".tsx":  "tsx",
		".vue":  "vue",
		".html": "html",
		".htm":  "html",
		".css":  "css",
		".scss": "scss",
		".sass": "sass",
		".less": "less",
		
		// Programming languages
		".go":   "go",
		".py":   "py",
		".pyw":  "py",
		".java": "java",
		".kt":   "kotlin",
		".kts":  "kotlin",
		".rs":   "rust",
		".rb":   "ruby",
		".php":  "php",
		".cpp":  "cpp",
		".cxx":  "cpp",
		".cc":   "cpp",
		".c":    "c",
		".h":    "c",
		".hpp":  "cpp",
		".cs":   "csharp",
		".swift": "swift",
		".m":    "objc",
		".mm":   "objc",
		
		// Scripting languages
		".sh":   "bash",
		".bash": "bash",
		".zsh":  "zsh",
		".fish": "fish",
		".ps1":  "powershell",
		".bat":  "batch",
		".cmd":  "batch",
		
		// Data formats
		".json": "json",
		".yaml": "yaml",
		".yml":  "yaml",
		".xml":  "xml",
		".toml": "toml",
		".ini":  "ini",
		".cfg":  "ini",
		".conf": "ini",
		
		// Markup languages
		".md":   "md",
		".mdx":  "mdx",
		".rst":  "rst",
		".tex":  "latex",
		
		// Database
		".sql":  "sql",
		
		// Docker
		".dockerfile": "dockerfile",
		
		// Other formats
		".r":    "r",
		".pl":   "perl",
		".lua":  "lua",
		".dart": "dart",
		".scala": "scala",
		".clj":  "clojure",
		".ex":   "elixir",
		".exs":  "elixir",
		".erl":  "erlang",
		".hrl":  "erlang",
		".hs":   "haskell",
		".ml":   "ocaml",
		".elm":  "elm",
		".f90":  "fortran",
		".f95":  "fortran",
		".vb":   "vbnet",
		".pas":  "pascal",
		".asm":  "assembly",
	}
	
	if lang, exists := extensionMap[ext]; exists {
		return lang
	}
	
	// Special handling for files without extensions or specific names
	baseName := strings.ToLower(filepath.Base(filePath))
	switch baseName {
	case "dockerfile":
		return "dockerfile"
	case "makefile":
		return "makefile"
	case "rakefile":
		return "ruby"
	case "gemfile":
		return "ruby"
	case "vagrantfile":
		return "ruby"
	}
	
	return ""
}

// Resource management structures and functions

// MemoryLimiter tracks and limits memory usage
type MemoryLimiter struct {
	maxMemory     int64
	_ int64 // currentUsage - unused but kept for future use
	reservations  int64
	mu            sync.RWMutex
}

// Global memory limiter instance
var globalMemoryLimiter = &MemoryLimiter{
	maxMemory: 512 * 1024 * 1024, // 512MB default limit
}

// InitializeResourceLimits sets up resource limits based on system capabilities
func InitializeResourceLimits() {
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	
	// Set memory limit to 25% of available system memory or 1GB, whichever is smaller
	systemMemory := int64(memStats.Sys)
	maxMemory := systemMemory / 4
	if maxMemory > 1024*1024*1024 {
		maxMemory = 1024 * 1024 * 1024 // Cap at 1GB
	}
	if maxMemory < 128*1024*1024 {
		maxMemory = 128 * 1024 * 1024 // Minimum 128MB
	}
	
	globalMemoryLimiter.maxMemory = maxMemory
	dprintf("initialized memory limiter: max=%d bytes", maxMemory)
}

// reserveMemory attempts to reserve memory for an operation
func reserveMemory(size int64) error {
	globalMemoryLimiter.mu.Lock()
	defer globalMemoryLimiter.mu.Unlock()
	
	if globalMemoryLimiter.reservations + size > globalMemoryLimiter.maxMemory {
		return fmt.Errorf("memory reservation would exceed limit: requested=%d, available=%d", 
			size, globalMemoryLimiter.maxMemory - globalMemoryLimiter.reservations)
	}
	
	globalMemoryLimiter.reservations += size
	return nil
}

// releaseMemory releases previously reserved memory
func releaseMemory(size int64) {
	globalMemoryLimiter.mu.Lock()
	defer globalMemoryLimiter.mu.Unlock()
	
	if globalMemoryLimiter.reservations >= size {
		globalMemoryLimiter.reservations -= size
	} else {
		globalMemoryLimiter.reservations = 0
	}
}

// getMemoryUsageStats returns current memory usage statistics
func getMemoryUsageStats() (reserved, max int64) {
	globalMemoryLimiter.mu.RLock()
	defer globalMemoryLimiter.mu.RUnlock()
	return globalMemoryLimiter.reservations, globalMemoryLimiter.maxMemory
}

// ResourcePool manages finite resources like file handles
type ResourcePool struct {
	name       string
	maxSize    int
	semaphore  chan struct{}
	active     int64
	total      int64
	failed     int64
}

// NewResourcePool creates a new resource pool
func NewResourcePool(name string, maxSize int) *ResourcePool {
	return &ResourcePool{
		name:      name,
		maxSize:   maxSize,
		semaphore: make(chan struct{}, maxSize),
	}
}

// Acquire attempts to acquire a resource from the pool
func (rp *ResourcePool) Acquire() error {
	select {
	case rp.semaphore <- struct{}{}:
		atomic.AddInt64(&rp.active, 1)
		atomic.AddInt64(&rp.total, 1)
		return nil
	default:
		atomic.AddInt64(&rp.failed, 1)
		return fmt.Errorf("resource pool '%s' exhausted (max: %d)", rp.name, rp.maxSize)
	}
}

// Release returns a resource to the pool
func (rp *ResourcePool) Release() {
	select {
	case <-rp.semaphore:
		atomic.AddInt64(&rp.active, -1)
	default:
		dprintf("warning: attempted to release resource from empty pool '%s'", rp.name)
	}
}

// Stats returns usage statistics for the pool
func (rp *ResourcePool) Stats() (active, total, failed int64) {
	return atomic.LoadInt64(&rp.active), atomic.LoadInt64(&rp.total), atomic.LoadInt64(&rp.failed)
}

// Global file handle pool
var globalFileHandlePool = NewResourcePool("file_handles", 1000)

// TempFileManager manages temporary files with automatic cleanup
type TempFileManager struct {
	files  map[string]time.Time
	mu     sync.RWMutex
	ticker *time.Ticker
	done   chan bool
}

// Global temp file manager instance
var globalTempFileManager = &TempFileManager{
	files: make(map[string]time.Time),
	done:  make(chan bool),
}

// InitializeTempFileManager starts the temporary file cleanup process
func InitializeTempFileManager() {
	globalTempFileManager.ticker = time.NewTicker(30 * time.Second)
	go globalTempFileManager.cleanupLoop()
	dprintf("initialized temporary file manager")
}

// cleanupLoop runs periodic cleanup of stale temporary files
func (tfm *TempFileManager) cleanupLoop() {
	for {
		select {
		case <-tfm.ticker.C:
			tfm.cleanupStale()
		case <-tfm.done:
			tfm.ticker.Stop()
			return
		}
	}
}

// RegisterTempFile registers a temporary file for cleanup tracking
func (tfm *TempFileManager) RegisterTempFile(path string) {
	tfm.mu.Lock()
	defer tfm.mu.Unlock()
	tfm.files[path] = time.Now()
}

// UnregisterTempFile removes a file from cleanup tracking (successful cleanup)
func (tfm *TempFileManager) UnregisterTempFile(path string) {
	tfm.mu.Lock()
	defer tfm.mu.Unlock()
	delete(tfm.files, path)
}

// cleanupStale removes temporary files older than 1 minute
func (tfm *TempFileManager) cleanupStale() {
	tfm.mu.RLock()
	var staleFiles []string
	now := time.Now()
	
	for path, created := range tfm.files {
		if now.Sub(created) > time.Minute {
			staleFiles = append(staleFiles, path)
		}
	}
	tfm.mu.RUnlock()
	
	if len(staleFiles) == 0 {
		return
	}
	
	tfm.mu.Lock()
	defer tfm.mu.Unlock()
	
	cleanedCount := 0
	for _, path := range staleFiles {
		if err := os.Remove(path); err != nil {
			if !os.IsNotExist(err) {
				dprintf("warning: failed to cleanup stale temp file %s: %v", path, err)
			}
		} else {
			cleanedCount++
		}
		delete(tfm.files, path)
	}
	
	if cleanedCount > 0 {
		dprintf("cleaned up %d stale temporary files", cleanedCount)
	}
}

// StopTempFileManager gracefully shuts down the temp file manager
func StopTempFileManager() {
	close(globalTempFileManager.done)
}

// ResourceHealthCheck provides system resource health information
type ResourceHealthCheck struct {
	MemoryUsage      ResourceMetric `json:"memory_usage"`
	FileHandleUsage  ResourceMetric `json:"file_handle_usage"`
	TempFileCount    int            `json:"temp_file_count"`
	SystemHealth     string         `json:"system_health"`
}

// ResourceMetric represents usage statistics for a resource
type ResourceMetric struct {
	Current   int64   `json:"current"`
	Max       int64   `json:"max"`
	UsagePercent float64 `json:"usage_percent"`
	Status    string  `json:"status"`
}

// GetResourceHealthCheck returns current resource health status
func GetResourceHealthCheck() ResourceHealthCheck {
	// Memory usage
	memReserved, memMax := getMemoryUsageStats()
	memPercent := float64(memReserved) / float64(memMax) * 100
	memStatus := "healthy"
	if memPercent > 80 {
		memStatus = "warning"
	} else if memPercent > 95 {
		memStatus = "critical"
	}

	// File handle usage
	fhActive, _, fhFailed := globalFileHandlePool.Stats()
	fhPercent := float64(fhActive) / float64(globalFileHandlePool.maxSize) * 100
	fhStatus := "healthy"
	if fhPercent > 80 {
		fhStatus = "warning"
	} else if fhPercent > 95 || fhFailed > 0 {
		fhStatus = "critical"
	}

	// Temp files
	globalTempFileManager.mu.RLock()
	tempFileCount := len(globalTempFileManager.files)
	globalTempFileManager.mu.RUnlock()

	// Overall system health
	systemHealth := "healthy"
	if memStatus == "critical" || fhStatus == "critical" || tempFileCount > 100 {
		systemHealth = "critical"
	} else if memStatus == "warning" || fhStatus == "warning" || tempFileCount > 50 {
		systemHealth = "warning"
	}

	return ResourceHealthCheck{
		MemoryUsage: ResourceMetric{
			Current:      memReserved,
			Max:          memMax,
			UsagePercent: memPercent,
			Status:       memStatus,
		},
		FileHandleUsage: ResourceMetric{
			Current:      fhActive,
			Max:          int64(globalFileHandlePool.maxSize),
			UsagePercent: fhPercent,
			Status:       fhStatus,
		},
		TempFileCount: tempFileCount,
		SystemHealth:  systemHealth,
	}
}

// IsResourceHealthy returns true if system resources are healthy for operations
func IsResourceHealthy() bool {
	health := GetResourceHealthCheck()
	return health.SystemHealth != "critical"
}

// gracefullyDegradeOnResourceExhaustion attempts to free resources when under pressure
//nolint:unused // Resource management function kept for future use
func gracefullyDegradeOnResourceExhaustion() {
	health := GetResourceHealthCheck()
	
	if health.SystemHealth == "critical" || health.SystemHealth == "warning" {
		dprintf("resource pressure detected (health=%s), attempting cleanup", health.SystemHealth)
		
		// Force cleanup of stale temp files
		globalTempFileManager.cleanupStale()
		
		// Force garbage collection
		runtime.GC()
		
		// Log resource usage after cleanup
		healthAfter := GetResourceHealthCheck()
		dprintf("resource usage after cleanup: memory=%.1f%%, file_handles=%.1f%%, temp_files=%d", 
			healthAfter.MemoryUsage.UsagePercent, 
			healthAfter.FileHandleUsage.UsagePercent,
			healthAfter.TempFileCount)
	}
}

// withResourceManagement wraps operations with resource monitoring and recovery
//nolint:unused // Resource management wrapper kept for future use
func withResourceManagement(operation func() error) error {
	// Check health before operation
	if !IsResourceHealthy() {
		gracefullyDegradeOnResourceExhaustion()
		
		// Check again after cleanup attempt
		if !IsResourceHealthy() {
			return fmt.Errorf("system resources critically low, operation declined for system stability")
		}
	}
	
	// Execute operation
	err := operation()
	
	// Clean up if resources are under pressure after operation
	if health := GetResourceHealthCheck(); health.SystemHealth != "healthy" {
		gracefullyDegradeOnResourceExhaustion()
	}
	
	return err
}
