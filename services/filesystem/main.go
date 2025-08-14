package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	flag.Parse()
	cleanup, err := ensureSingleInstance()
	if err != nil {
		panic(err)
	}
	defer cleanup()
	initDebug()
	root, err := getRoot()
	if err != nil {
		panic(err)
	}
	dprintf("server start root=%q debug=%v", root, debugEnabled)

	s := setupServer(root)
	if err := server.ServeStdio(s); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		dprintf("server error: %v", err)
		os.Exit(1)
	}
}
