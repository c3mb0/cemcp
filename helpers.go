package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

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
// It evicts lock files older than ten minutes.
func acquireLock(path string, timeout time.Duration) (release func(), err error) {
	lock := path + ".lock"
	deadline := time.Now().Add(timeout)
	for {
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
		time.Sleep(50 * time.Millisecond)
	}
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
