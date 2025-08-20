package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func formatMkdirResult(r MkdirResult) string {
	return fmt.Sprintf("path=%s created=%v mode=%s modified_at=%s", r.Path, r.Created, r.Mode, r.ModifiedAt)
}

func handleMkdir(root string) mcp.StructuredToolHandlerFunc[MkdirArgs, MkdirResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args MkdirArgs) (MkdirResult, error) {
		start := time.Now()
		dprintf("-> fs_mkdir path=%q mode=%s", args.Path, args.Mode)
		var out MkdirResult
		paths := expandBraces(args.Path)
		mode, err := parseMode(args.Mode)
		if err != nil {
			dprintf("fs_mkdir mode error: %v", err)
			return out, fmt.Errorf("invalid mode: %w", err)
		}
		if args.Mode == "" {
			mode = 0o755
		}
		anyCreated := false
		var firstFi os.FileInfo
		for i, p := range paths {
			full, err := safeJoin(root, p)
			if err != nil {
				dprintf("fs_mkdir error: %v", err)
				return out, err
			}
			created := false
			if fi, err := os.Lstat(full); err == nil {
				if !fi.IsDir() {
					dprintf("fs_mkdir exists but not dir")
					return out, fmt.Errorf("exists and not a directory: %s", p)
				}
			} else if os.IsNotExist(err) {
				if err := os.MkdirAll(full, mode); err != nil {
					dprintf("fs_mkdir MkdirAll error: %v", err)
					return out, err
				}
				created = true
			} else {
				dprintf("fs_mkdir lstat error: %v", err)
				return out, err
			}
			fi, err := os.Lstat(full)
			if err != nil {
				dprintf("fs_mkdir stat error: %v", err)
				return out, err
			}
			if i == 0 {
				firstFi = fi
			}
			anyCreated = anyCreated || created
		}
		if firstFi != nil {
			out = MkdirResult{
				Path:    args.Path,
				Created: anyCreated,
				MetaFields: MetaFields{
					Mode:       fmt.Sprintf("%#o", firstFi.Mode()&os.ModePerm),
					ModifiedAt: firstFi.ModTime().UTC().Format(time.RFC3339),
				},
			}
		} else {
			out = MkdirResult{Path: args.Path, Created: anyCreated}
		}
		dprintf("<- fs_mkdir ok created=%v dur=%s", anyCreated, time.Since(start))
		return out, nil
	}
}
