package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := setupServer()
	if err := server.ServeStdio(s); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
