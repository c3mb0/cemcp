package main

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/mark3labs/mcp-go/mcp"
)

func formatGlobResult(r GlobResult) string {
	return strings.Join(r.Matches, "\n")
}

func handleGlob(root string) mcp.StructuredToolHandlerFunc[GlobArgs, GlobResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args GlobArgs) (GlobResult, error) {
		start := time.Now()
		dprintf("-> fs_glob pattern=%q max_results=%d", args.Pattern, args.MaxResults)
		var out GlobResult
		if args.Pattern == "" {
			return out, errors.New("pattern required")
		}
		if _, err := safeJoin(root, args.Pattern); err != nil {
			dprintf("fs_glob error: %v", err)
			return out, err
		}
		max := args.MaxResults
		if max <= 0 {
			max = defaultGlobMaxResults
		}
		pat := filepath.ToSlash(filepath.Clean(args.Pattern))
		if _, err := doublestar.Match(pat, ""); err != nil {
			dprintf("fs_glob error: %v", err)
			return out, err
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		paths := make(chan string, 64)
		var walkErr error
		var walkWG sync.WaitGroup
		walkWG.Add(1)
		go func() {
			defer walkWG.Done()
			walkErr = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return nil
				}
				paths <- filepath.ToSlash(rel)
				return nil
			})
			close(paths)
		}()

		var mu sync.Mutex
		matches := []string{}
		workers := runtime.NumCPU()
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for p := range paths {
					if ctx.Err() != nil {
						return
					}
					ok, err := doublestar.Match(pat, p)
					if err != nil {
						cancel()
						return
					}
					if ok {
						mu.Lock()
						if len(matches) >= max {
							mu.Unlock()
							return
						}
						matches = append(matches, filepath.ToSlash(p))
						if len(matches) >= max {
							mu.Unlock()
							cancel()
							return
						}
						mu.Unlock()
					}
				}
			}()
		}
		wg.Wait()
		walkWG.Wait()
		if walkErr != nil && !errors.Is(walkErr, context.Canceled) {
			dprintf("fs_glob error: %v", walkErr)
			return out, walkErr
		}
		out.Matches = matches
		dprintf("<- fs_glob ok matches=%d dur=%s", len(out.Matches), time.Since(start))
		return out, nil
	}
}
