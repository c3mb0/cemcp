package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func formatRmdirResult(r RmdirResult) string {
	return fmt.Sprintf("path=%s removed=%v", r.Path, r.Removed)
}

func handleRmdir(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[RmdirArgs, RmdirResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args RmdirArgs) (RmdirResult, error) {
		state, err := getSessionState(ctx, sessions, mu)
		if err != nil {
			return RmdirResult{}, err
		}
		root := state.Root
		start := time.Now()
		dprintf("%s -> fs_rmdir path=%q recursive=%v", sessionContext(ctx), args.Path, args.Recursive)
		var out RmdirResult
		full, err := safeJoin(root, args.Path)
		if err != nil {
			dprintf("fs_rmdir error: %v", err)
			return out, err
		}
		fi, err := os.Lstat(full)
		if err != nil {
			dprintf("fs_rmdir lstat error: %v", err)
			return out, err
		}
		if !fi.IsDir() {
			dprintf("fs_rmdir not a directory")
			return out, fmt.Errorf("not a directory: %s", args.Path)
		}
		if args.Recursive {
			if err := os.RemoveAll(full); err != nil {
				dprintf("fs_rmdir RemoveAll error: %v", err)
				return out, err
			}
		} else {
			if err := os.Remove(full); err != nil {
				dprintf("fs_rmdir Remove error: %v", err)
				return out, err
			}
		}
		out = RmdirResult{Path: args.Path, Removed: true}
		dprintf("<- fs_rmdir ok removed=true dur=%s", time.Since(start))
		return out, nil
	}
}
