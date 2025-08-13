package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchFileHandlesLongLines(t *testing.T) {
	config := DefaultSearchConfig()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "long.txt")
	longSize := 1 << 20
	longLine := strings.Repeat("a", longSize) + "needle\n"
	if err := os.WriteFile(tmpFile, []byte(longLine), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	matches, bytesRead := searchFile(tmpFile, "needle", nil, tmpDir, 10, config)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if bytesRead == 0 {
		t.Fatalf("expected bytesRead > 0")
	}
}
