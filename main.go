package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ---- Config ----
var rootDirFlag = flag.String("root", "", "filesystem root (defaults to CWD or $FS_ROOT)")
var debugFlag = flag.Bool("debug", false, "enable debug logging to ./log")

// ---- Debug logging ----

var (
	debugEnabled bool
	debugMu      sync.Mutex
	debugLog     *log.Logger
)

// Resource caps to guard against excessive CPU or memory use
const (
	maxPeekBytesForSniff = 1 << 20  // 1 MiB for MIME/encoding detection
	maxHashBytes         = 32 << 20 // 32 MiB hashing cap
)

func initDebug() {
	if !*debugFlag {
		return
	}
	f, err := os.Create("log") // hardcoded per request; truncate each run
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		return
	}
	debugEnabled = true
	debugLog = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
}

func dprintf(format string, args ...any) {
	if !debugEnabled || debugLog == nil {
		return
	}
	debugMu.Lock()
	defer debugMu.Unlock()
	debugLog.Printf(format, args...)
}

// ---- Types ----

type writeStrategy string

const (
	strategyOverwrite    writeStrategy = "overwrite"
	strategyNoClobber    writeStrategy = "no_clobber"
	strategyAppend       writeStrategy = "append"
	strategyPrepend      writeStrategy = "prepend"
	strategyReplaceRange writeStrategy = "replace_range"
)

type encodingKind string

const (
	encText   encodingKind = "text"
	encBase64 encodingKind = "base64"
)

const (
	defaultReadMaxBytes     = 64 * 1024
	defaultPeekMaxBytes     = 4 * 1024
	defaultListMaxEntries   = 1000
	defaultGlobMaxResults   = 1000
	defaultSearchMaxResults = 100
)

// ---- Helpers ----

func mustAbs(p string) string {
	ap, err := filepath.Abs(p)
	if err != nil {
		panic(err)
	}
	return ap
}

func getRoot() (string, error) {
	var base string
	if *rootDirFlag != "" {
		base = mustAbs(*rootDirFlag)
	} else if env := os.Getenv("FS_ROOT"); env != "" {
		base = mustAbs(env)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		base = mustAbs(cwd)
	}
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}
	return base, nil
}

// safeJoin ensures target is within root; resolves parent to avoid symlink escapes
// NOTE: This version validates the parent path but does NOT resolve the final path.
// For read operations where following symlinks could escape the root, use safeJoinResolveFinal.
func safeJoin(root, reqPath string) (string, error) {
	if reqPath == "" {
		return "", errors.New("path is required")
	}
	if strings.HasPrefix(reqPath, "file://") {
		u, err := url.Parse(reqPath)
		if err != nil {
			return "", fmt.Errorf("invalid file URI: %w", err)
		}
		if unesc, err := url.PathUnescape(u.Path); err == nil && unesc != "" {
			reqPath = unesc
		} else {
			reqPath = u.Path
		}
	}
	clean := filepath.Clean(reqPath)
	rootAbs := mustAbs(root)
	rootResolved := rootAbs
	if r2, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootResolved = r2
	}
	if filepath.IsAbs(clean) {
		finalAbs := mustAbs(clean)
		if !strings.HasPrefix(finalAbs+string(os.PathSeparator), rootResolved+string(os.PathSeparator)) && finalAbs != rootResolved {
			return "", fmt.Errorf("refusing to access outside root: %s", reqPath)
		}
		return finalAbs, nil
	}
	dir, base := filepath.Split(clean)
	parent := filepath.Join(rootAbs, dir)
	parentResolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		parentResolved = mustAbs(parent)
	}
	final := filepath.Join(parentResolved, base)
	finalAbs := mustAbs(final)
	if !strings.HasPrefix(finalAbs+string(os.PathSeparator), rootResolved+string(os.PathSeparator)) && finalAbs != rootResolved {
		return "", fmt.Errorf("refusing to access outside root: %s", reqPath)
	}
	return finalAbs, nil
}

// safeJoinResolveFinal resolves the final target (follows the last symlink) and ensures it stays under root.
// This prevents read/peek from traversing a symlink inside the root that points outside the root.
func safeJoinResolveFinal(root, reqPath string) (string, error) {
	p, err := safeJoin(root, reqPath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		// If the file doesn't exist yet (e.g., during write no_clobber), return p;
		// callers that need to forbid symlinks should still Lstat and check.
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return p, nil
	}
	rootResolved := mustAbs(root)
	if r2, err := filepath.EvalSymlinks(rootResolved); err == nil {
		rootResolved = r2
	}
	resolvedAbs := mustAbs(resolved)
	if !strings.HasPrefix(resolvedAbs+string(os.PathSeparator), rootResolved+string(os.PathSeparator)) && resolvedAbs != rootResolved {
		return "", fmt.Errorf("refusing to access symlink outside root: %s", reqPath)
	}
	return resolvedAbs, nil
}

func detectMIME(name string, sample []byte) string {
	if ext := filepath.Ext(name); ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt
		}
	}
	if isText(sample) {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

func isText(b []byte) bool {
	for _, c := range b {
		if c == 9 || c == 10 || c == 13 {
			continue
		}
		if c < 32 || c == 0x7f {
			return false
		}
	}
	return true
}

func sha256sum(b []byte) string {
	s := sha256.Sum256(b)
	return fmt.Sprintf("%x", s[:])
}

func ensureParent(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0o755)
}

// trimUnderRoot returns p relative to root without a leading slash.
// It normalizes separators and handles the case where root is "/".
func trimUnderRoot(root, p string) string {
	r := mustAbs(root)
	r = strings.TrimSuffix(r, string(os.PathSeparator))
	prefix := r + string(os.PathSeparator)
	return strings.TrimPrefix(p, prefix)
}

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

func parseMode(s string) (os.FileMode, error) {
	if s == "" {
		return 0o644, nil
	}
	if !strings.HasPrefix(s, "0") {
		s = "0" + s
	}
	u, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(u), nil
}

// atomicWrite writes to a temp file then renames over target.
func atomicWrite(target string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".mcpfs-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		if runtime.GOOS == "windows" {
			if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
				return removeErr
			}
			return os.Rename(tmpName, target)
		}
		return err
	}
	return nil
}

// acquireLock creates a best-effort advisory lock using a sibling .lock file.
// The operation respects context cancellation and evicts lock files older than
// ten minutes.
func acquireLock(ctx context.Context, path string, timeout time.Duration) (release func(), err error) {
	lock := path + ".lock"
	deadline := time.Now().Add(timeout)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(lock) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if info, statErr := os.Stat(lock); statErr == nil {
			if time.Since(info.ModTime()) > 10*time.Minute {
				_ = os.Remove(lock)
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("lock timeout: %s", path)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// ---- Schemas ----

type MetaFields struct {
	Mode       string `json:"mode,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

type ReadArgs struct {
	Path     string `json:"path"`
	Encoding string `json:"encoding,omitempty"` // text|base64 (auto if empty)
	MaxBytes int    `json:"max_bytes,omitempty"`
}

type ReadResult struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	MIMEType  string `json:"mime_type"`
	SHA256    string `json:"sha256"`
	Encoding  string `json:"encoding"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	MetaFields
}

type PeekArgs struct {
	Path     string `json:"path"`
	Offset   int    `json:"offset,omitempty"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

type PeekResult struct {
	Path     string `json:"path"`
	Offset   int    `json:"offset"`
	Size     int64  `json:"size"`
	EOF      bool   `json:"eof"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	MetaFields
}

type WriteArgs struct {
	Path       string        `json:"path"`
	Encoding   string        `json:"encoding"` // text|base64
	Content    string        `json:"content"`
	Strategy   writeStrategy `json:"strategy,omitempty"`
	CreateDirs *bool         `json:"create_dirs,omitempty"`
	Mode       string        `json:"mode,omitempty"` // e.g. 0644
	Start      *int          `json:"start,omitempty"`
	End        *int          `json:"end,omitempty"`
}

type WriteResult struct {
	Path     string `json:"path"`
	Action   string `json:"action"`
	Bytes    int    `json:"bytes"`
	Created  bool   `json:"created"`
	MIMEType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	MetaFields
}

type EditArgs struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
	Regex   bool   `json:"regex,omitempty"`
	Count   int    `json:"count,omitempty"`
}

type EditResult struct {
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
	Bytes        int    `json:"bytes"`
	SHA256       string `json:"sha256"`
	MetaFields
}

type ListArgs struct {
	Path       string `json:"path"`
	Recursive  bool   `json:"recursive,omitempty"`
	MaxEntries int    `json:"max_entries,omitempty"`
}

type ListEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Size       int64  `json:"size"`
	Mode       string `json:"mode"`
	ModifiedAt string `json:"modified_at"`
}

type ListResult struct {
	Entries []ListEntry `json:"entries"`
}

type GlobArgs struct {
	Pattern    string `json:"pattern"`
	MaxResults int    `json:"max_results,omitempty"`
}

type GlobResult struct {
	Matches []string `json:"matches"`
}

type SearchArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Regex      bool   `json:"regex,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type SearchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type SearchResult struct {
	Matches []SearchMatch `json:"matches"`
}

func kindOf(fi os.FileInfo) string {
	m := fi.Mode()
	if m.IsRegular() {
		return "file"
	}
	if m.IsDir() {
		return "dir"
	}
	if (m & os.ModeSymlink) != 0 {
		return "symlink"
	}
	return "other"
}

// ---- Handlers ----

func handleRead(root string) mcp.StructuredToolHandlerFunc[ReadArgs, ReadResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args ReadArgs) (ReadResult, error) {
		start := time.Now()
		dprintf("-> fs_read path=%q encoding=%q max_bytes=%d", args.Path, args.Encoding, args.MaxBytes)
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

		enc := args.Encoding
		if enc == "" {
			sample := buf
			if len(sample) > maxPeekBytesForSniff {
				sample = sample[:maxPeekBytesForSniff]
			}
			if isText(sample) {
				enc = string(encText)
			} else {
				enc = string(encBase64)
			}
		}
		var content string
		if encodingKind(enc) == encBase64 {
			content = base64.StdEncoding.EncodeToString(buf)
		} else {
			content = string(buf)
		}

		res = ReadResult{
			Path:      args.Path,
			Size:      fi.Size(),
			MIMEType:  detectMIME(full, buf),
			SHA256:    sha,
			Encoding:  enc,
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
		enc := string(encText)
		content := string(chunk)
		if !isText(chunk) {
			enc = string(encBase64)
			content = base64.StdEncoding.EncodeToString(chunk)
		}
		var mode string
		var modAt string
		if fi, statErr := os.Lstat(full); statErr == nil {
			mode = fmt.Sprintf("%#o", fi.Mode()&os.ModePerm)
			modAt = fi.ModTime().UTC().Format(time.RFC3339)
		}
		res = PeekResult{
			Path:     args.Path,
			Offset:   args.Offset,
			Size:     sz,
			EOF:      eof,
			Encoding: enc,
			Content:  content,
			MetaFields: MetaFields{
				Mode:       mode,
				ModifiedAt: modAt,
			},
		}
		dprintf("<- fs_peek ok bytes=%d eof=%v dur=%s", len(chunk), eof, time.Since(start))
		return res, nil
	}
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
		// Do not create parent directories unless explicitly requested.
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

		// Pre-stat & symlink/dir guards
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

		release, err := acquireLock(ctx, full, 3*time.Second)
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

func handleEdit(root string) mcp.StructuredToolHandlerFunc[EditArgs, EditResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args EditArgs) (EditResult, error) {
		start := time.Now()
		dprintf("-> fs_edit path=%q regex=%v count=%d", args.Path, args.Regex, args.Count)
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

		release, err := acquireLock(ctx, full, 3*time.Second)
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

func handleSearch(root string) mcp.StructuredToolHandlerFunc[SearchArgs, SearchResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args SearchArgs) (SearchResult, error) {
		start := time.Now()
		dprintf("-> fs_search path=%q pattern=%q regex=%v max=%d", args.Path, args.Pattern, args.Regex, args.MaxResults)
		var out SearchResult
		if args.Pattern == "" {
			return out, errors.New("pattern required")
		}
		max := args.MaxResults
		if max <= 0 {
			max = defaultSearchMaxResults
		}
		var rx *regexp.Regexp
		if args.Regex {
			r, err := regexp.Compile(args.Pattern)
			if err != nil {
				dprintf("fs_search error: %v", err)
				return out, err
			}
			rx = r
		}
		startPath := root
		if args.Path != "" {
			p, err := safeJoin(root, args.Path)
			if err != nil {
				dprintf("fs_search error: %v", err)
				return out, err
			}
			startPath = p
		}
		if _, err := os.Stat(startPath); err != nil {
			dprintf("fs_search error: %v", err)
			return out, err
		}
		errStop := errors.New("stop")
		var matches []SearchMatch
		_ = filepath.WalkDir(startPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
			lineNo := 1
			for scanner.Scan() {
				txt := scanner.Text()
				var ok bool
				if rx != nil {
					ok = rx.MatchString(txt)
				} else {
					ok = strings.Contains(txt, args.Pattern)
				}
				if ok {
					rel, _ := filepath.Rel(root, path)
					matches = append(matches, SearchMatch{Path: filepath.ToSlash(rel), Line: lineNo, Text: txt})
					if len(matches) >= max {
						return errStop
					}
				}
				lineNo++
			}
			if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
				return nil
			}
			return nil
		})
		out.Matches = matches
		dprintf("<- fs_search ok matches=%d dur=%s", len(out.Matches), time.Since(start))
		return out, nil
	}
}

func handleGlob(root string) mcp.StructuredToolHandlerFunc[GlobArgs, GlobResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args GlobArgs) (GlobResult, error) {
		start := time.Now()
		dprintf("-> fs_glob pattern=%q max_results=%d", args.Pattern, args.MaxResults)
		var out GlobResult
		if args.Pattern == "" {
			return out, errors.New("pattern required")
		}
		full, err := safeJoin(root, args.Pattern)
		if err != nil {
			dprintf("fs_glob error: %v", err)
			return out, err
		}
		max := args.MaxResults
		if max <= 0 {
			max = defaultGlobMaxResults
		}
		matches, err := filepath.Glob(full)
		if err != nil {
			dprintf("fs_glob error: %v", err)
			return out, err
		}
		for _, m := range matches {
			out.Matches = append(out.Matches, filepath.ToSlash(trimUnderRoot(root, m)))
			if len(out.Matches) >= max {
				break
			}
		}
		dprintf("<- fs_glob ok matches=%d dur=%s", len(out.Matches), time.Since(start))
		return out, nil
	}
}

// ---- main ----

func main() {
	flag.Parse()
	initDebug()
	root, err := getRoot()
	if err != nil {
		panic(err)
	}
	dprintf("server start root=%q debug=%v", root, debugEnabled)

	s := server.NewMCPServer("fs-mcp-go", "0.1.0")

	readTool := mcp.NewTool(
		"fs_read",
		mcp.WithDescription("Read a file up to a byte limit. Auto-detects encoding when omitted."),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path or file:// URI under root")),
		mcp.WithString("encoding", mcp.Enum(string(encText), string(encBase64)), mcp.Description("Force text or base64. If empty, the server detects.")),
		mcp.WithNumber("max_bytes", mcp.Min(1), mcp.Description("Maximum bytes to return (default 64 KiB)")),
		mcp.WithOutputSchema[ReadResult](),
	)
	s.AddTool(readTool, mcp.NewStructuredToolHandler(handleRead(root)))

	peekTool := mcp.NewTool(
		"fs_peek",
		mcp.WithDescription("Read a window of a file without loading it all"),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path to read")),
		mcp.WithNumber("offset", mcp.Min(0), mcp.Description("Byte offset to start from (default 0)")),
		mcp.WithNumber("max_bytes", mcp.Min(1), mcp.Description("Window size in bytes (default 4 KiB)")),
		mcp.WithOutputSchema[PeekResult](),
	)
	s.AddTool(peekTool, mcp.NewStructuredToolHandler(handlePeek(root)))

	writeTool := mcp.NewTool(
		"fs_write",
		mcp.WithDescription("Create or modify a file using a chosen strategy"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Target file path")),
		mcp.WithString("encoding", mcp.Required(), mcp.Enum(string(encText), string(encBase64)), mcp.Description("Encoding of content: text or base64")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Data to write")),
		mcp.WithString("strategy", mcp.Enum(string(strategyOverwrite), string(strategyNoClobber), string(strategyAppend), string(strategyPrepend), string(strategyReplaceRange)), mcp.Description("Write behavior; defaults to overwrite")),
		mcp.WithBoolean("create_dirs", mcp.Description("Create parent directories when needed (default false)")),
		mcp.WithString("mode", mcp.Pattern("^0?[0-7]{3,4}$"), mcp.Description("File mode in octal. Omit to preserve existing")),
		mcp.WithNumber("start", mcp.Min(0), mcp.Description("Start byte for replace_range")),
		mcp.WithNumber("end", mcp.Min(0), mcp.Description("End byte (exclusive) for replace_range")),
		mcp.WithOutputSchema[WriteResult](),
	)
	s.AddTool(writeTool, mcp.NewStructuredToolHandler(handleWrite(root)))

	editTool := mcp.NewTool(
		"fs_edit",
		mcp.WithDescription("Search and replace text in a file"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Target text file")),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Substring or regex to match")),
		mcp.WithString("replace", mcp.Required(), mcp.Description("Replacement text; $1 etc. in regex mode")),
		mcp.WithBoolean("regex", mcp.Description("Treat pattern as regular expression")),
		mcp.WithNumber("count", mcp.Min(0), mcp.Description("If >0, maximum replacements; 0 means replace all")),
		mcp.WithOutputSchema[EditResult](),
	)
	s.AddTool(editTool, mcp.NewStructuredToolHandler(handleEdit(root)))

	listTool := mcp.NewTool(
		"fs_list",
		mcp.WithDescription("List directory contents"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory to list")),
		mcp.WithBoolean("recursive", mcp.Description("Recurse into subdirectories")),
		mcp.WithNumber("max_entries", mcp.Min(1), mcp.Description("Maximum entries to return (default 1000)")),
		mcp.WithOutputSchema[ListResult](),
	)
	s.AddTool(listTool, mcp.NewStructuredToolHandler(handleList(root)))

	searchTool := mcp.NewTool(
		"fs_search",
		mcp.WithDescription("Search files for text"),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Substring or regex to find")),
		mcp.WithString("path", mcp.Description("Optional start directory; defaults to root")),
		mcp.WithBoolean("regex", mcp.Description("Interpret pattern as regular expression")),
		mcp.WithNumber("max_results", mcp.Min(1), mcp.Description("Maximum matches to return (default 100)")),
		mcp.WithOutputSchema[SearchResult](),
	)
	s.AddTool(searchTool, mcp.NewStructuredToolHandler(handleSearch(root)))

	globTool := mcp.NewTool(
		"fs_glob",
		mcp.WithDescription("Glob for pathnames under the root"),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Glob pattern")),
		mcp.WithNumber("max_results", mcp.Min(1), mcp.Description("Maximum matches to return (default 1000)")),
		mcp.WithOutputSchema[GlobResult](),
	)
	s.AddTool(globTool, mcp.NewStructuredToolHandler(handleGlob(root)))

	if err := server.ServeStdio(s); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		dprintf("server error: %v", err)
		os.Exit(1)
	}
}
