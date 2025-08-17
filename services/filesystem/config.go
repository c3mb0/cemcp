package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Configuration constants with tunable defaults
const (
	// Size limits for operations
	maxPeekBytesForSniff = 1 << 20  // 1 MiB for MIME/encoding detection
	maxHashBytes         = 32 << 20 // 32 MiB hashing cap
	maxFileSize          = 1 << 30  // 1 GiB maximum file size

	// Default operation limits
	defaultReadMaxBytes     = 64 * 1024 // 64 KiB
	defaultPeekMaxBytes     = 4 * 1024  // 4 KiB
	defaultListMaxEntries   = 1000
	defaultGlobMaxResults   = 1000
	defaultSearchMaxResults = 100

	// Performance tuning
	defaultWorkers     = 0 // 0 = auto-detect
	maxWorkers         = 16
	fileChannelBuffer  = 64
	matchChannelBuffer = 128

	// Timeouts
	defaultLockTimeout = 3 // seconds
	staleLockAge       = 5 // minutes
)

// Command-line flags
var (
	rootDirFlag     = flag.String("root", "", "filesystem base folder (defaults to the current working directory or $FS_ROOT)")
	debugFlag       = flag.String("debug", "", "write debug logs to this file")
	compatFlag      = flag.Bool("compat", false, "return tool results as plain text instead of JSON")
	workersFlag     = flag.Int("workers", defaultWorkers, "number of worker threads (0=auto)")
	maxSizeFlag     = flag.Int64("max-size", maxFileSize, "maximum file size in bytes")
	lockTimeoutFlag = flag.Int("lock-timeout", defaultLockTimeout, "file lock timeout in seconds")
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Root        string
	Debug       string
	CompatMode  bool
	Workers     int
	MaxFileSize int64
	LockTimeout int
}

// LoadConfig loads configuration from flags and environment
func LoadConfig() (*ServerConfig, error) {
	flag.Parse()

	// Determine base folder
	root, err := getRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to determine base folder: %w", err)
	}

	// Validate base folder
	if err := validateRoot(root); err != nil {
		return nil, err
	}

	// Determine worker count
	workers := *workersFlag
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > maxWorkers {
			workers = maxWorkers
		}
	}

	config := &ServerConfig{
		Root:        root,
		Debug:       *debugFlag,
		CompatMode:  *compatFlag,
		Workers:     workers,
		MaxFileSize: *maxSizeFlag,
		LockTimeout: *lockTimeoutFlag,
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate checks configuration validity
func (c *ServerConfig) Validate() error {
	if c.Root == "" {
		return fmt.Errorf("base folder is required")
	}

	if c.Workers < 1 || c.Workers > maxWorkers {
		return fmt.Errorf("workers must be between 1 and %d", maxWorkers)
	}

	if c.MaxFileSize < 1024 {
		return fmt.Errorf("max file size must be at least 1KB")
	}

	if c.LockTimeout < 1 {
		return fmt.Errorf("lock timeout must be at least 1 second")
	}

	return nil
}

// getRoot determines the base folder from flags/env/cwd
func getRoot() (string, error) {
	var base string

	// Priority: flag > environment variable > current directory
	if *rootDirFlag != "" {
		base = mustAbs(*rootDirFlag)
	} else if env := os.Getenv("FS_ROOT"); env != "" {
		base = mustAbs(env)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		base = mustAbs(cwd)
	}

	// Resolve symlinks for security
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	} else {
		// If symlink resolution fails, verify directory exists
		if _, err := os.Stat(base); err != nil {
			return "", fmt.Errorf("base folder does not exist: %w", err)
		}
	}

	return base, nil
}

// validateRoot ensures the base folder is usable
func validateRoot(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("base folder does not exist: %s", root)
		}
		return fmt.Errorf("cannot access base folder: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("base path is not a directory: %s", root)
	}

	// Test read access
	if _, err := os.ReadDir(root); err != nil {
		return fmt.Errorf("cannot read base folder: %w", err)
	}

	// Test write access by creating a temp file
	testFile := filepath.Join(root, ".mcp-test-"+fmt.Sprintf("%d", os.Getpid()))
	if f, err := os.Create(testFile); err != nil {
		dprintf("warning: base folder may not be writable: %v", err)
	} else {
		f.Close()
		os.Remove(testFile)
	}

	return nil
}

// GetWorkerCount returns the configured number of workers for an operation
func (c *ServerConfig) GetWorkerCount(operation string) int {
	// Could be customized per operation in the future
	return c.Workers
}
