package main

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	maxThoughts      = 100
	defaultRecentCap = 5
)

// mem holds the current session state.
var mem = struct {
	sync.RWMutex
	thoughts      []string
	mentalModels  []string
	debugSessions []string
}{}

// SessionContextArgs controls sessioncontext output limits.
type SessionContextArgs struct {
	Limit int `json:"limit"`
}

// SessionContextResult describes the sessioncontext tool output.
type SessionContextResult struct {
	ThoughtCount             int      `json:"thought_count"`
	RecentThoughts           []string `json:"recent_thoughts"`
	MentalModelCount         int      `json:"mental_model_count"`
	RecentMentalModels       []string `json:"recent_mental_models"`
	DebugSessionCount        int      `json:"debug_session_count"`
	RecentDebugSessions      []string `json:"recent_debug_sessions"`
	RemainingThoughtCapacity int      `json:"remaining_thought_capacity"`
}

func setupServer() *server.MCPServer {
	s := server.NewMCPServer("clear-thought", "0.1.0")

	tool := mcp.NewTool(
		"sessioncontext",
		mcp.WithDescription("Return counts and recent entries for thoughts, mental models, and debugging sessions, along with remaining thought capacity."),
		mcp.WithNumber("limit", mcp.Description("Maximum number of recent items to return"), mcp.DefaultNumber(defaultRecentCap), mcp.Min(1)),
		mcp.WithOutputSchema[SessionContextResult](),
	)

	s.AddTool(tool, mcp.NewStructuredToolHandler(handleSessionContext))
	return s
}

func handleSessionContext(ctx context.Context, req mcp.CallToolRequest, args SessionContextArgs) (SessionContextResult, error) {
	if args.Limit <= 0 {
		args.Limit = defaultRecentCap
	}

	mem.RLock()
	defer mem.RUnlock()

	return SessionContextResult{
		ThoughtCount:             len(mem.thoughts),
		RecentThoughts:           recent(mem.thoughts, args.Limit),
		MentalModelCount:         len(mem.mentalModels),
		RecentMentalModels:       recent(mem.mentalModels, args.Limit),
		DebugSessionCount:        len(mem.debugSessions),
		RecentDebugSessions:      recent(mem.debugSessions, args.Limit),
		RemainingThoughtCapacity: remainingCapacity(len(mem.thoughts)),
	}, nil
}

func recent(list []string, limit int) []string {
	if limit <= 0 || len(list) == 0 {
		return nil
	}
	if limit > len(list) {
		limit = len(list)
	}
	out := make([]string, limit)
	for i := 0; i < limit; i++ {
		out[i] = list[len(list)-1-i]
	}
	return out
}

func remainingCapacity(current int) int {
	cap := maxThoughts - current
	if cap < 0 {
		return 0
	}
	return cap
}
