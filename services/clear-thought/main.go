package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	sessionFlag := flag.String("session", "", "session identifier")
	maxFlag := flag.Int("max-thoughts", defaultConfig.MaxThoughtsPerSession, "maximum thoughts per session")
	flag.Parse()

	sessionID := *sessionFlag
	if sessionID == "" {
		sessionID = os.Getenv("CT_SESSION_ID")
		if sessionID == "" {
			sessionID = "default"
		}
	}

	maxThoughts := *maxFlag
	if env := os.Getenv("CT_MAX_THOUGHTS"); env != "" && maxThoughts == defaultConfig.MaxThoughtsPerSession {
		if v, err := strconv.Atoi(env); err == nil {
			maxThoughts = v
		}
	}

	cfg := ServerConfig{MaxThoughtsPerSession: maxThoughts}

	s := setupServer(sessionID, cfg)
	if err := server.ServeStdio(s); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
