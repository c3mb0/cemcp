package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// SearchConfig holds search configuration
type SearchConfig struct {
	Workers    int
	ScanBuffer int
}

// DefaultSearchConfig returns optimized search configuration
func DefaultSearchConfig() SearchConfig {
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8 // Cap workers to prevent resource exhaustion
	}
	return SearchConfig{
		Workers:    workers,
		ScanBuffer: 64 * 1024, // 64KB initial buffer
	}
}

func formatSearchResult(r SearchResult) string {
	var b strings.Builder
	for i, m := range r.Matches {
		if i > 0 {
			b.WriteByte('\n')
		}
		// Truncate long lines for display
		text := m.Text
		if len(text) > 200 {
			text = text[:197] + "..."
		}
		fmt.Fprintf(&b, "%s:%d:%s", m.Path, m.Line, text)
	}
	return b.String()
}

func handleSearch(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[SearchArgs, SearchResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args SearchArgs) (SearchResult, error) {
		state, err := getSessionState(ctx, sessions, mu)
		if err != nil {
			return SearchResult{}, err
		}
		root := state.Root
		start := time.Now()
		dprintf("%s -> fs_search path=%q pattern=%q regex=%v max=%d", sessionContext(ctx), args.Path, args.Pattern, args.Regex, args.MaxResults)

		var out SearchResult
		if args.Pattern == "" {
			return out, newOpError("search", args.Path, ErrPatternRequired)
		}

		max := args.MaxResults
		if max <= 0 {
			max = defaultSearchMaxResults
		}

		// Compile regex if needed
		var rx *regexp.Regexp
		if args.Regex {
			r, err := regexp.Compile(args.Pattern)
			if err != nil {
				return out, newOpError("search", args.Path, ErrInvalidRegex, err.Error())
			}
			rx = r
		}

		// Determine start path
		startPath := root
		if args.Path != "" {
			p, err := safeJoin(root, args.Path)
			if err != nil {
				return out, newOpError("search", args.Path, err)
			}
			startPath = p
		}

		// Verify path exists
		if _, err := os.Stat(startPath); err != nil {
			return out, newOpError("search", startPath, ErrPathNotFound)
		}

		// Set up search
		config := DefaultSearchConfig()
		matches, stats, err := performSearch(ctx, startPath, root, args.Pattern, rx, max, config)
		if err != nil {
			return out, err
		}

		out.Matches = matches
		out.Statistics = map[string]interface{}{
			"files_scanned": stats.filesScanned,
			"bytes_read":    stats.bytesRead,
			"duration_ms":   time.Since(start).Milliseconds(),
		}

		dprintf("<- fs_search ok matches=%d files=%d bytes=%d dur=%s",
			len(out.Matches), stats.filesScanned, stats.bytesRead, time.Since(start))
		return out, nil
	}
}

type searchStats struct {
	filesScanned int64
	bytesRead    int64
}

func performSearch(ctx context.Context, startPath, root, pattern string, rx *regexp.Regexp, max int, config SearchConfig) ([]SearchMatch, *searchStats, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channel for files to process
	files := make(chan string, 64)

	// Stats tracking
	stats := &searchStats{}

	// Walk filesystem in separate goroutine
	var walkErr error
	var walkWG sync.WaitGroup
	walkWG.Add(1)
	go func() {
		defer walkWG.Done()
		defer close(files)

		walkErr = filepath.WalkDir(startPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				dprintf("walk error at %s: %v", path, err)
				return nil // Continue walking
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Skip directories and symlinks
			if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
				return nil
			}

			// Skip files that are likely binary based on extension
			if isBinaryExtension(filepath.Ext(path)) {
				return nil
			}

			// Get file info for size check
			info, err := d.Info()
			if err != nil {
				return nil
			}

			// Skip huge files (>100MB)
			if info.Size() > 100<<20 {
				dprintf("skipping large file: %s (%d bytes)", path, info.Size())
				return nil
			}

			files <- path
			return nil
		})
	}()

	// Process files with worker pool
	var mu sync.Mutex
	matches := []SearchMatch{}
	matchCount := int32(0)

	var wg sync.WaitGroup
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for path := range files {
				if ctx.Err() != nil {
					return
				}

				// Check if we've hit the limit
				if atomic.LoadInt32(&matchCount) >= int32(max) {
					return
				}

				fileMatches, bytesRead := searchFile(path, pattern, rx, root, max-int(atomic.LoadInt32(&matchCount)), config)

				// Update stats
				atomic.AddInt64(&stats.filesScanned, 1)
				atomic.AddInt64(&stats.bytesRead, bytesRead)

				if len(fileMatches) > 0 {
					mu.Lock()
					// Double-check limit under lock
					if len(matches) < max {
						remaining := max - len(matches)
						if len(fileMatches) > remaining {
							fileMatches = fileMatches[:remaining]
						}
						matches = append(matches, fileMatches...)
						atomic.StoreInt32(&matchCount, int32(len(matches)))

						// Cancel if we've hit the limit
						if len(matches) >= max {
							cancel()
						}
					}
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	walkWG.Wait()

	if walkErr != nil && ctx.Err() == nil {
		return matches, stats, walkErr
	}

	return matches, stats, nil
}

func searchFile(path, pattern string, rx *regexp.Regexp, root string, maxMatches int, config SearchConfig) ([]SearchMatch, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0
	}
	defer f.Close()

	var matches []SearchMatch
	var bytesRead int64

	reader := bufio.NewReaderSize(f, config.ScanBuffer)

	lineNo := 1
	for len(matches) < maxMatches {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			dprintf("read error in %s: %v", path, err)
			break
		}

		if len(line) == 0 && err == io.EOF {
			break
		}

		bytesRead += int64(len(line))
		line = strings.TrimRight(line, "\n")

		// Check for match
		var found bool
		if rx != nil {
			found = rx.MatchString(line)
		} else {
			found = strings.Contains(line, pattern)
		}

		if found {
			rel, _ := filepath.Rel(root, path)

			// Truncate very long lines
			displayText := line
			if len(displayText) > 500 {
				displayText = displayText[:497] + "..."
			}

			matches = append(matches, SearchMatch{
				Path: filepath.ToSlash(rel),
				Line: lineNo,
				Text: displayText,
			})
		}

		lineNo++

		// Bail out if line number gets suspiciously high (likely binary file)
		if lineNo > 1000000 {
			dprintf("stopping search in %s: too many lines", path)
			break
		}

		if err == io.EOF {
			break
		}
	}

	return matches, bytesRead
}

// isBinaryExtension checks if file extension suggests binary content
func isBinaryExtension(ext string) bool {
	ext = strings.ToLower(ext)
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".pdf": true, ".doc": true, ".docx": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true,
		".bin": true, ".dat": true, ".db": true,
		".pyc": true, ".pyo": true, ".class": true,
		".o": true, ".a": true, ".lib": true,
	}
	return binaryExts[ext]
}
