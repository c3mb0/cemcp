package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func formatReadResult(r ReadResult) string {
	return fmt.Sprintf("path=%s size=%d mime=%s sha=%s truncated=%v content=%s", r.Path, r.Size, r.MIMEType, r.SHA256, r.Truncated, r.Content)
}

func handleRead(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[ReadArgs, ReadResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args ReadArgs) (ReadResult, error) {
		state, err := getSessionState(ctx, sessions, mu)
		if err != nil {
			return ReadResult{}, err
		}
		root := state.Root
		start := time.Now()
		dprintf("%s -> fs_read path=%q max_bytes=%d", sessionContext(ctx), args.Path, args.MaxBytes)
		var res ReadResult
		
		// Validate required arguments
		if args.Path == "" {
			errResp := NewValidationError("path", "path is required", []string{`"path": "/path/to/file.txt"`})
			errResp.Operation = "read"
			return res, errResp
		}
		
		full, err := safeJoinResolveFinal(root, args.Path)
		if err != nil {
			dprintf("fs_read error: %v", err)
			return res, WrapError(err, "read", args.Path)
		}
		fi, err := os.Stat(full)
		if err != nil {
			dprintf("fs_read stat error: %v", err)
			return res, WrapError(err, "read", args.Path)
		}
		limit := args.MaxBytes
		if limit <= 0 {
			limit = defaultReadMaxBytes
		}
		
		// Use single file handle for both content reading and hash computation
		content, sha, err := readFileWithHash(full, int64(limit), fi.Size())
		if err != nil {
			dprintf("fs_read error: %v", err)
			return res, WrapError(err, "read", args.Path)
		}
		
		trunc := fi.Size() > int64(len(content))

		contentStr := string(content)
		
		// Detect MIME type once and reuse the result
		mimeType := detectMIME(full, content)
		formattedMarkdown := formatAsFencedMarkdown(contentStr, mimeType, full)

		res = ReadResult{
			Path:      args.Path,
			Size:      fi.Size(),
			MIMEType:  mimeType,
			SHA256:    sha,
			Content:   contentStr,
			Truncated: trunc,
			FormattedMarkdown: formattedMarkdown,
			MetaFields: MetaFields{
				Mode:       fmt.Sprintf("%#o", fi.Mode()&os.ModePerm),
				ModifiedAt: fi.ModTime().UTC().Format(time.RFC3339),
			},
		}
		dprintf("<- fs_read ok size=%d truncated=%v dur=%s", len(content), trunc, time.Since(start))
		return res, nil
	}
}
