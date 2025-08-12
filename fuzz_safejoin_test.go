//go:build go1.18
// +build go1.18

package main

import (
	"testing"
)

// FuzzSafeJoin tries to find path traversal or panic cases.
func FuzzSafeJoin(f *testing.F) {
	root := f.TempDir()
	seeds := []string{"a.txt", "./a.txt", "../a", "..//..//etc/passwd", "/etc/passwd", "dir/../a"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		_, _ = safeJoin(root, p)
	})
}
