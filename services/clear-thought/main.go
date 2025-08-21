package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	maxFlag := flag.Int("max-thoughts", defaultConfig.MaxThoughtsPerSession, "maximum thoughts per session")
	flag.Parse()

	maxThoughts := *maxFlag
	if env := os.Getenv("CT_MAX_THOUGHTS"); env != "" && maxThoughts == defaultConfig.MaxThoughtsPerSession {
		if v, err := strconv.Atoi(env); err == nil {
			maxThoughts = v
		}
	}

	cfg := ServerConfig{MaxThoughtsPerSession: maxThoughts}

	s := setupServer(cfg)
	if err := server.ServeStdio(s); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
