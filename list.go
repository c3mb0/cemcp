package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func formatListResult(r ListResult) string {
	var b strings.Builder
	for i, e := range r.Entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s %s %s %d %s %s", e.Path, e.Name, e.Kind, e.Size, e.Mode, e.ModifiedAt)
	}
	return b.String()
}

func handleList(root string) mcp.StructuredToolHandlerFunc[ListArgs, ListResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args ListArgs) (ListResult, error) {
		start := time.Now()
		dprintf("-> fs_list path=%q recursive=%v max_entries=%d", args.Path, args.Recursive, args.MaxEntries)
		var out ListResult
		base, err := safeJoinResolveFinal(root, args.Path)
		if err != nil {
			dprintf("fs_list error: %v", err)
			return out, err
		}
		max := args.MaxEntries
		if max <= 0 {
			max = defaultListMaxEntries
		}
		count := 0
		add := func(path string, fi os.FileInfo) {
			if count >= max {
				return
			}
			out.Entries = append(out.Entries, ListEntry{
				Path:       filepath.ToSlash(trimUnderRoot(root, path)),
				Name:       fi.Name(),
				Kind:       kindOf(fi),
				Size:       fi.Size(),
				Mode:       fmt.Sprintf("%#o", fi.Mode()&os.ModePerm),
				ModifiedAt: fi.ModTime().UTC().Format(time.RFC3339),
			})
			count++
		}
		fi, err := os.Stat(base)
		if err != nil {
			dprintf("fs_list stat error: %v", err)
			return out, err
		}
		if fi.IsDir() {
			if !args.Recursive {
				ents, err := os.ReadDir(base)
				if err != nil {
					dprintf("fs_list readdir error: %v", err)
					return out, err
				}
				for _, e := range ents {
					select {
					case <-ctx.Done():
						return out, ctx.Err()
					default:
					}
					info, err := e.Info()
					if err != nil {
						continue
					}
					add(filepath.Join(base, e.Name()), info)
				}
			} else {
				err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					add(path, info)
					if count >= max {
						return io.EOF
					}
					return nil
				})
				if err != nil && !errors.Is(err, io.EOF) {
					dprintf("fs_list walk error: %v", err)
					return out, err
				}
			}
		} else {
			add(base, fi)
		}
		dprintf("<- fs_list ok entries=%d dur=%s", len(out.Entries), time.Since(start))
		return out, nil
	}
}
