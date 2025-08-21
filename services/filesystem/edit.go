package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func formatEditResult(r EditResult) string {
	return fmt.Sprintf("path=%s replacements=%d bytes=%d sha=%s", r.Path, r.Replacements, r.Bytes, r.SHA256)
}

func handleEdit(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[EditArgs, EditResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args EditArgs) (EditResult, error) {
		state, err := getSessionState(ctx, sessions, mu)
		if err != nil {
			return EditResult{}, err
		}
		root := state.Root
		start := time.Now()
		dprintf("%s -> fs_edit path=%q regex=%v count=%d", sessionContext(ctx), args.Path, args.Regex, args.Count)
		var res EditResult
		if args.Path == "" || args.Pattern == "" {
			return res, errors.New("path and pattern required")
		}
		full, err := safeJoin(root, args.Path)
		if err != nil {
			dprintf("fs_edit error: %v", err)
			return res, err
		}
		fi, err := os.Lstat(full)
		if err != nil {
			dprintf("fs_edit error: %v", err)
			return res, err
		}
		if (fi.Mode() & os.ModeSymlink) != 0 {
			return res, fmt.Errorf("refusing to edit symlink: %s", args.Path)
		}
		if !fi.Mode().IsRegular() {
			return res, fmt.Errorf("target not a regular file: %s", args.Path)
		}

		release, err := acquireLock(full, 3*time.Second)
		if err != nil {
			dprintf("fs_edit lock error: %v", err)
			return res, err
		}
		defer release()

		b, err := os.ReadFile(full)
		if err != nil {
			dprintf("fs_edit read error: %v", err)
			return res, err
		}
		var re *regexp.Regexp
		if args.Regex {
			re, err = regexp.Compile(args.Pattern)
			if err != nil {
				return res, fmt.Errorf("invalid regex: %w", err)
			}
		}
		count := 0
		var out []byte
		if args.Regex {
			if args.Count <= 0 {
				out = re.ReplaceAll(b, []byte(args.Replace))
				count = len(re.FindAllIndex(b, -1))
			} else {
				remaining := args.Count
				out = re.ReplaceAllFunc(b, func(m []byte) []byte {
					if remaining == 0 {
						return m
					}
					remaining--
					count++
					return []byte(args.Replace)
				})
			}
		} else {
			old := string(b)
			limit := args.Count
			if limit <= 0 {
				out = []byte(strings.ReplaceAll(old, args.Pattern, args.Replace))
				if args.Pattern != "" {
					count = strings.Count(old, args.Pattern)
				}
			} else {
				out = []byte(strings.Replace(old, args.Pattern, args.Replace, limit))
				if args.Pattern != "" {
					if c := strings.Count(old, args.Pattern); c < limit {
						count = c
					} else {
						count = limit
					}
				}
			}
		}
		mode := fi.Mode() & os.ModePerm
		if mode == 0 {
			mode = 0o644
		}
		if err := atomicWrite(full, out, mode); err != nil {
			dprintf("fs_edit write error: %v", err)
			return res, err
		}
		res = EditResult{
			Path:         args.Path,
			Replacements: count,
			Bytes:        len(out),
			SHA256:       sha256sum(out),
			MetaFields: MetaFields{
				Mode:       fmt.Sprintf("%#o", mode),
				ModifiedAt: time.Now().UTC().Format(time.RFC3339),
			},
		}
		dprintf("<- fs_edit ok replacements=%d bytes=%d dur=%s", count, len(out), time.Since(start))
		return res, nil
	}
}
