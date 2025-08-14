package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func mustAbs(p string) string {
	ap, err := filepath.Abs(p)
	if err != nil {
		panic(err)
	}
	return ap
}

// safeJoin joins root and reqPath while keeping the result within root.
// It validates the parent path but does not resolve the final element.
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

// safeJoinResolveFinal follows the last path element and ensures the target
// stays within root. It guards read/peek from symlinks that jump outside.
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

// trimUnderRoot returns p relative to root without a leading slash.
// It normalizes separators and handles the case where root is "/".
func trimUnderRoot(root, p string) string {
	r := mustAbs(root)
	r = strings.TrimSuffix(r, string(os.PathSeparator))
	pAbs := mustAbs(p)
	if pAbs == r {
		return ""
	}
	prefix := r + string(os.PathSeparator)
	return strings.TrimPrefix(pAbs, prefix)
}
