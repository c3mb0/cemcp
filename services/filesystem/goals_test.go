package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
)

// extractGoals decodes outstanding goals from the CallToolResult meta
func extractGoals(res *mcp.CallToolResult) ([]Goal, bool) {
	if res.Meta == nil || res.Meta.AdditionalFields == nil {
		return nil, false
	}
	sessionField, ok := res.Meta.AdditionalFields["session"]
	if !ok {
		return nil, false
	}
	b, err := json.Marshal(sessionField)
	if err != nil {
		return nil, false
	}
	var state SessionState
	if err := json.Unmarshal(b, &state); err != nil {
		return nil, false
	}
	return state.Goals, true
}

func TestSessionGoalsContext(t *testing.T) {
	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: mcp.NewTool("addgoal", mcp.WithOutputSchema[AddGoalResult]()), Handler: wrapStructuredHandler(handleAddGoal())},
		server.ServerTool{Tool: mcp.NewTool("updategoal", mcp.WithOutputSchema[UpdateGoalResult]()), Handler: wrapStructuredHandler(handleUpdateGoal())},
		server.ServerTool{Tool: mcp.NewTool("noop"), Handler: wrapStructuredHandler(func(ctx context.Context, req mcp.CallToolRequest, _ struct{}) (struct{}, error) {
			return struct{}{}, nil
		})},
	)
	if err != nil {
		t.Fatalf("server start failed: %v", err)
	}
	defer srv.Close()

	// Add a goal
	_, err = srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "addgoal", Arguments: map[string]any{"description": "test goal"}},
	})
	if err != nil {
		t.Fatalf("addgoal call failed: %v", err)
	}

	// Check session context via noop tool
	res, err := srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "noop"},
	})
	if err != nil {
		t.Fatalf("noop call failed: %v", err)
	}
	goals, ok := extractGoals(res)
	if !ok || len(goals) != 1 || goals[0].Description != "test goal" || goals[0].Completed {
		t.Fatalf("unexpected goals after add: %+v", goals)
	}

	// Complete the goal
	_, err = srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "updategoal", Arguments: map[string]any{"index": 0, "completed": true}},
	})
	if err != nil {
		t.Fatalf("updategoal call failed: %v", err)
	}

	// Verify session context is empty
	res, err = srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "noop"},
	})
	if err != nil {
		t.Fatalf("noop call failed: %v", err)
	}
	goals, _ = extractGoals(res)
	if len(goals) != 0 {
		t.Fatalf("expected no outstanding goals, got %#v", goals)
	}
}
