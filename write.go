package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func formatWriteResult(r WriteResult) string {
	return fmt.Sprintf("path=%s action=%s bytes=%d created=%v mime=%s sha=%s", r.Path, r.Action, r.Bytes, r.Created, r.MIMEType, r.SHA256)
}

func handleWrite(root string) mcp.StructuredToolHandlerFunc[WriteArgs, WriteResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args WriteArgs) (WriteResult, error) {
		start := time.Now()
		dprintf("-> fs_write path=%q strategy=%q encoding=%q bytes=%d", args.Path, args.Strategy, args.Encoding, len(args.Content))
		var res WriteResult
		if args.Encoding == "" {
			dprintf("fs_write error: encoding required")
			return res, errors.New("encoding is required: text|base64")
		}
		full, err := safeJoin(root, args.Path)
		if err != nil {
			dprintf("fs_write error: %v", err)
			return res, err
		}
		if args.CreateDirs == nil {
			b := false
			args.CreateDirs = &b
		}
		if *args.CreateDirs {
			if err := ensureParent(full); err != nil {
				dprintf("fs_write error: %v", err)
				return res, err
			}
		}
		mode, err := parseMode(args.Mode)
		if err != nil {
			dprintf("fs_write error: %v", err)
			return res, fmt.Errorf("invalid mode: %w", err)
		}
		modeProvided := args.Mode != ""
		var data []byte
		if encodingKind(args.Encoding) == encBase64 {
			b, err := base64.StdEncoding.DecodeString(args.Content)
			if err != nil {
				dprintf("fs_write error: %v", err)
				return res, fmt.Errorf("invalid base64 content: %w", err)
			}
			data = b
		} else {
			data = []byte(args.Content)
		}
		st := args.Strategy
		if st == "" {
			st = strategyOverwrite
		}

		preFi, preErr := os.Lstat(full)
		if preErr == nil && (preFi.Mode()&os.ModeSymlink) != 0 {
			dprintf("fs_write error: target is symlink")
			return res, fmt.Errorf("refusing to write to symlink: %s", args.Path)
		}
		if preErr == nil && preFi.IsDir() && (st == strategyOverwrite || st == strategyNoClobber) {
			return res, fmt.Errorf("target is a directory: %s", args.Path)
		}
		if preErr == nil && !modeProvided {
			if pm := preFi.Mode() & os.ModePerm; pm != 0 {
				mode = pm
			} else {
				mode = 0o644
			}
		}

		release, err := acquireLock(full, 3*time.Second)
		if err != nil {
			dprintf("fs_write lock error: %v", err)
			return res, err
		}
		defer release()

		created := false
		action := string(st)

		switch st {
		case strategyNoClobber:
			if preErr == nil {
				dprintf("fs_write noclobber exists")
				return res, fmt.Errorf("exists: %s", args.Path)
			}
			if err := atomicWrite(full, data, mode); err != nil {
				dprintf("fs_write error: %v", err)
				return res, err
			}
			created = true

		case strategyOverwrite:
			if errors.Is(preErr, os.ErrNotExist) {
				created = true
			}
			if err := atomicWrite(full, data, mode); err != nil {
				dprintf("fs_write error: %v", err)
				return res, err
			}

		case strategyAppend:
			if preErr == nil && !preFi.Mode().IsRegular() {
				return res, fmt.Errorf("append target not a regular file: %s", args.Path)
			}
			if errors.Is(preErr, os.ErrNotExist) {
				created = true
			}
			f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_APPEND, mode)
			if err != nil {
				dprintf("fs_write error: %v", err)
				return res, err
			}
			defer f.Close()
			n, err := f.Write(data)
			if err != nil {
				dprintf("fs_write error: %v", err)
				return res, err
			}
			data = data[:n]

		case strategyPrepend:
			if preErr == nil && !preFi.Mode().IsRegular() {
				return res, fmt.Errorf("prepend target not a regular file: %s", args.Path)
			}
			var old []byte
			if preErr == nil {
				old, err = os.ReadFile(full)
				if err != nil {
					return res, err
				}
			} else if errors.Is(preErr, os.ErrNotExist) {
				created = true
			}
			buf := append([]byte{}, data...)
			buf = append(buf, old...)
			if err := atomicWrite(full, buf, mode); err != nil {
				dprintf("fs_write error: %v", err)
				return res, err
			}
			data = buf

		case strategyReplaceRange:
			if preErr != nil {
				dprintf("fs_write error: %v", preErr)
				return res, fmt.Errorf("replace_range requires existing file: %w", preErr)
			}
			if !preFi.Mode().IsRegular() {
				return res, fmt.Errorf("replace_range target not a regular file: %s", args.Path)
			}
			old, err := os.ReadFile(full)
			if err != nil {
				dprintf("fs_write error: %v", err)
				return res, err
			}
			if args.Start == nil || args.End == nil {
				return res, errors.New("start and end required for replace_range")
			}
			s, e := *args.Start, *args.End
			if s < 0 || e < s || e > len(old) {
				return res, fmt.Errorf("invalid range [%d,%d)", s, e)
			}
			buf := append([]byte{}, old[:s]...)
			buf = append(buf, data...)
			buf = append(buf, old[e:]...)
			if err := atomicWrite(full, buf, mode); err != nil {
				dprintf("fs_write error: %v", err)
				return res, err
			}
			data = buf

		default:
			return res, fmt.Errorf("unknown strategy: %s", st)
		}

		final := data
		if b, err := os.ReadFile(full); err == nil {
			final = b
		}
		mt := detectMIME(full, final)
		fi, statErr := os.Lstat(full)
		modAt := time.Now().UTC().Format(time.RFC3339)
		modeStr := ""
		if fi != nil && statErr == nil {
			modAt = fi.ModTime().UTC().Format(time.RFC3339)
			modeStr = fmt.Sprintf("%#o", fi.Mode()&os.ModePerm)
		}
		sha := ""
		if len(final) <= int(maxHashBytes) {
			sha = sha256sum(final)
		} else {
			dprintf("fs_write: skip sha256 (size %d > cap %d)", len(final), maxHashBytes)
		}
		res = WriteResult{
			Path:     args.Path,
			Action:   action,
			Bytes:    len(final),
			Created:  created,
			MIMEType: mt,
			SHA256:   sha,
			MetaFields: MetaFields{
				Mode:       modeStr,
				ModifiedAt: modAt,
			},
		}
		dprintf("<- fs_write ok created=%v bytes=%d dur=%s", created, len(final), time.Since(start))
		return res, nil
	}
}
