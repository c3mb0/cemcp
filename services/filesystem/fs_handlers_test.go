package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestWrite_PathAndMode(t *testing.T) {
	root := t.TempDir()
	wr := handleWrite(root)
	res, err := wr(context.Background(), mcp.CallToolRequest{}, WriteArgs{Path: "m/sub/file.txt", Content: "hello", Mode: "0640"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Bytes != 5 || res.MIMEType == "" {
		t.Fatalf("unexpected write result: %+v", res)
	}
	st, err := os.Stat(filepath.Join(root, "m/sub/file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&0o777 != 0o640 {
		t.Fatalf("mode mismatch: %o", st.Mode()&0o777)
	}
}
