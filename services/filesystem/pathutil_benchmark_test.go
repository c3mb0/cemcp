package main

import "testing"

func BenchmarkSafeJoin(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < b.N; i++ {
		if _, err := safeJoin(root, "a/b/c.txt"); err != nil {
			b.Fatal(err)
		}
	}
}
