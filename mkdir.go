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
		dprintf("-> fs_mkdir path=%q parents=%v mode=%s", args.Path, args.Parents, args.Mode)
		var out MkdirResult
		full, err := safeJoin(root, args.Path)
		if err != nil {
			dprintf("fs_mkdir error: %v", err)
			return out, err
		}
		mode, err := parseMode(args.Mode)
		if err != nil {
			dprintf("fs_mkdir mode error: %v", err)
			return out, fmt.Errorf("invalid mode: %w", err)
		}
		if args.Mode == "" {
			mode = 0o755
		}
		created := false
		if fi, err := os.Lstat(full); err == nil {
			if !fi.IsDir() {
				dprintf("fs_mkdir exists but not dir")
				return out, fmt.Errorf("exists and not a directory: %s", args.Path)
			}
		} else if os.IsNotExist(err) {
			if args.Parents {
				if err := os.MkdirAll(full, mode); err != nil {
					dprintf("fs_mkdir MkdirAll error: %v", err)
					return out, err
				}
			} else {
				if err := os.Mkdir(full, mode); err != nil {
					dprintf("fs_mkdir Mkdir error: %v", err)
					return out, err
				}
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
		out = MkdirResult{
			Path:    args.Path,
			Created: created,
			MetaFields: MetaFields{
				Mode:       fmt.Sprintf("%#o", fi.Mode()&os.ModePerm),
				ModifiedAt: fi.ModTime().UTC().Format(time.RFC3339),
			},
		}
		dprintf("<- fs_mkdir ok created=%v dur=%s", created, time.Since(start))
		return out, nil
	}
}
