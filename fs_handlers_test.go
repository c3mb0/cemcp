package main

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestWrite_Base64PathAndMode(t *testing.T) {
	root := t.TempDir()
	wr := handleWrite(root)
	data := base64.StdEncoding.EncodeToString([]byte("hello"))
	res, err := wr(context.Background(), mcp.CallToolRequest{}, WriteArgs{Path: "m/sub/file.txt", Encoding: "base64", Content: data, Mode: "0640", CreateDirs: boolPtr(true)})
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

func TestPeek_BinaryBase64(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "b.bin")
	os.WriteFile(p, []byte{0, 1, 2, 3, 4, 5}, 0o644)
	pk := handlePeek(root)
	res, err := pk(context.Background(), mcp.CallToolRequest{}, PeekArgs{Path: "b.bin", Offset: 1, MaxBytes: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Encoding != string(encBase64) {
		t.Fatalf("want base64 for binary, got %s", res.Encoding)
	}
}

func boolPtr(b bool) *bool { return &b }
