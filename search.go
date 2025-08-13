package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func formatSearchResult(r SearchResult) string {
	var b strings.Builder
	for i, m := range r.Matches {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s:%d:%s", m.Path, m.Line, m.Text)
	}
	return b.String()
}

func handleSearch(root string) mcp.StructuredToolHandlerFunc[SearchArgs, SearchResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args SearchArgs) (SearchResult, error) {
		start := time.Now()
		dprintf("-> fs_search path=%q pattern=%q regex=%v max=%d", args.Path, args.Pattern, args.Regex, args.MaxResults)
		var out SearchResult
		if args.Pattern == "" {
			return out, errors.New("pattern required")
		}
		max := args.MaxResults
		if max <= 0 {
			max = defaultSearchMaxResults
		}
		var rx *regexp.Regexp
		if args.Regex {
			r, err := regexp.Compile(args.Pattern)
			if err != nil {
				dprintf("fs_search error: %v", err)
				return out, err
			}
			rx = r
		}
		startPath := root
		if args.Path != "" {
			p, err := safeJoin(root, args.Path)
			if err != nil {
				dprintf("fs_search error: %v", err)
				return out, err
			}
			startPath = p
		}
		if _, err := os.Stat(startPath); err != nil {
			dprintf("fs_search error: %v", err)
			return out, err
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		files := make(chan string, 64)
		var walkErr error
		var walkWG sync.WaitGroup
		walkWG.Add(1)
		go func() {
			defer walkWG.Done()
			walkErr = filepath.WalkDir(startPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				if d.IsDir() {
					return nil
				}
				if d.Type()&os.ModeSymlink != 0 {
					return nil
				}
				files <- path
				return nil
			})
			close(files)
		}()

		var mu sync.Mutex
		matches := []SearchMatch{}
		workers := runtime.NumCPU()
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for path := range files {
					if ctx.Err() != nil {
						return
					}
					f, err := os.Open(path)
					if err != nil {
						continue
					}
					scanner := bufio.NewScanner(f)
					scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
					lineNo := 1
					for scanner.Scan() {
						txt := scanner.Text()
						var ok bool
						if rx != nil {
							ok = rx.MatchString(txt)
						} else {
							ok = strings.Contains(txt, args.Pattern)
						}
						if ok {
							rel, _ := filepath.Rel(root, path)
							mu.Lock()
							matches = append(matches, SearchMatch{Path: filepath.ToSlash(rel), Line: lineNo, Text: txt})
							if len(matches) >= max {
								mu.Unlock()
								cancel()
								f.Close()
								return
							}
							mu.Unlock()
						}
						lineNo++
					}
					f.Close()
					if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
						continue
					}
				}
			}()
		}
		wg.Wait()
		walkWG.Wait()
		if walkErr != nil && !errors.Is(walkErr, context.Canceled) {
			dprintf("fs_search error: %v", walkErr)
			return out, walkErr
		}
		out.Matches = matches
		dprintf("<- fs_search ok matches=%d dur=%s", len(out.Matches), time.Since(start))
		return out, nil
	}
}
