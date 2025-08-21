package main

import (
	"context"
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
	mgr := &sessionManager{id: "default"}
	if err := server.ServeStdio(s, server.WithStdioContextFunc(func(ctx context.Context) context.Context {
		return withSessionManager(ctx, mgr)
	})); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		dprintf("server error: %v", err)
		os.Exit(1)
	}
}
