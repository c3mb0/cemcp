package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func readWindow(path string, offset, max int) ([]byte, int64, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, false, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, 0, false, err
	}
	sz := fi.Size()
	if offset < 0 {
		offset = 0
	}
	if int64(offset) > sz {
		return []byte{}, sz, true, nil
	}
	if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, sz, false, err
	}
	if max <= 0 {
		max = defaultPeekMaxBytes
	}
	buf := make([]byte, max)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, sz, false, err
	}
	buf = buf[:n]
	return buf, sz, int64(offset+n) >= sz, nil
}

func formatPeekResult(r PeekResult) string {
	return fmt.Sprintf("path=%s offset=%d size=%d eof=%v content=%s", r.Path, r.Offset, r.Size, r.EOF, r.Content)
}

func handlePeek(root string) mcp.StructuredToolHandlerFunc[PeekArgs, PeekResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args PeekArgs) (PeekResult, error) {
		start := time.Now()
		if args.MaxBytes <= 0 {
			args.MaxBytes = defaultPeekMaxBytes
		}
		dprintf("-> fs_peek path=%q offset=%d max_bytes=%d", args.Path, args.Offset, args.MaxBytes)
		var res PeekResult
		full, err := safeJoinResolveFinal(root, args.Path)
		if err != nil {
			dprintf("fs_peek error: %v", err)
			return res, err
		}
		chunk, sz, eof, err := readWindow(full, args.Offset, args.MaxBytes)
		if err != nil {
			dprintf("fs_peek read error: %v", err)
			return res, err
		}
		content := string(chunk)
		var mode string
		var modAt string
		if fi, statErr := os.Lstat(full); statErr == nil {
			mode = fmt.Sprintf("%#o", fi.Mode()&os.ModePerm)
			modAt = fi.ModTime().UTC().Format(time.RFC3339)
		}
		res = PeekResult{
			Path:    args.Path,
			Offset:  args.Offset,
			Size:    sz,
			EOF:     eof,
			Content: content,
			MetaFields: MetaFields{
				Mode:       mode,
				ModifiedAt: modAt,
			},
		}
		dprintf("<- fs_peek ok bytes=%d eof=%v dur=%s", len(chunk), eof, time.Since(start))
		return res, nil
	}
}
