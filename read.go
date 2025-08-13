package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func formatReadResult(r ReadResult) string {
	return fmt.Sprintf("path=%s size=%d mime=%s sha=%s truncated=%v content=%s", r.Path, r.Size, r.MIMEType, r.SHA256, r.Truncated, r.Content)
}

func handleRead(root string) mcp.StructuredToolHandlerFunc[ReadArgs, ReadResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args ReadArgs) (ReadResult, error) {
		start := time.Now()
		dprintf("-> fs_read path=%q max_bytes=%d", args.Path, args.MaxBytes)
		var res ReadResult
		full, err := safeJoinResolveFinal(root, args.Path)
		if err != nil {
			dprintf("fs_read error: %v", err)
			return res, err
		}
		fi, err := os.Stat(full)
		if err != nil {
			dprintf("fs_read stat error: %v", err)
			return res, err
		}
		limit := args.MaxBytes
		if limit <= 0 {
			limit = defaultReadMaxBytes
		}
		f, err := os.Open(full)
		if err != nil {
			dprintf("fs_read open error: %v", err)
			return res, err
		}
		defer f.Close()
		r := io.LimitReader(f, int64(limit))
		buf, err := io.ReadAll(r)
		if err != nil {
			dprintf("fs_read read error: %v", err)
			return res, err
		}
		trunc := fi.Size() > int64(len(buf))

		sha := ""
		if fi.Size() <= maxHashBytes {
			hf, err := os.Open(full)
			if err == nil {
				h := sha256.New()
				if _, err := io.Copy(h, hf); err == nil {
					sha = fmt.Sprintf("%x", h.Sum(nil))
				}
				hf.Close()
			}
		} else {
			dprintf("fs_read: skip sha256 (size %d > cap %d)", fi.Size(), maxHashBytes)
		}

		content := string(buf)

		res = ReadResult{
			Path:      args.Path,
			Size:      fi.Size(),
			MIMEType:  detectMIME(full, buf),
			SHA256:    sha,
			Content:   content,
			Truncated: trunc,
			MetaFields: MetaFields{
				Mode:       fmt.Sprintf("%#o", fi.Mode()&os.ModePerm),
				ModifiedAt: fi.ModTime().UTC().Format(time.RFC3339),
			},
		}
		dprintf("<- fs_read ok size=%d truncated=%v dur=%s", len(buf), trunc, time.Since(start))
		return res, nil
	}
}
