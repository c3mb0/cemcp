package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Goal represents a session goal tracked by the agent
// description: text of the objective
// completed: whether the goal has been completed
// notes: optional additional notes

type Goal struct {
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	Notes       string `json:"notes,omitempty"`
}

// SessionState holds per-session data including goals

type SessionState struct {
	mu    sync.RWMutex
	Goals []Goal `json:"goals"`
}

var sessionStates sync.Map // map sessionID -> *SessionState

func getSessionState(ctx context.Context) *SessionState {
	session := server.ClientSessionFromContext(ctx)
	if session == nil {
		return nil
	}
	sid := session.SessionID()
	state, _ := sessionStates.LoadOrStore(sid, &SessionState{})
	return state.(*SessionState)
}

// attachSessionContext adds outstanding goals to the result meta so clients are aware
func attachSessionContext(ctx context.Context, result *mcp.CallToolResult) {
	state := getSessionState(ctx)
	if state == nil {
		return
	}
	state.mu.RLock()
	var pending []Goal
	for _, g := range state.Goals {
		if !g.Completed {
			pending = append(pending, g)
		}
	}
	state.mu.RUnlock()
	sessionData := map[string]any{"goals": pending}
	if result.Meta == nil {
		result.Meta = mcp.NewMetaFromMap(map[string]any{"session": sessionData})
		return
	}
	if result.Meta.AdditionalFields == nil {
		result.Meta.AdditionalFields = make(map[string]any)
	}
	result.Meta.AdditionalFields["session"] = sessionData
}

type AddGoalArgs struct {
	Description string `json:"description"`
	Notes       string `json:"notes,omitempty"`
}

type AddGoalResult struct {
	Index int  `json:"index"`
	Goal  Goal `json:"goal"`
}

func formatAddGoalResult(r AddGoalResult) string {
	return fmt.Sprintf("goal %d added", r.Index)
}

func handleAddGoal() mcp.StructuredToolHandlerFunc[AddGoalArgs, AddGoalResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args AddGoalArgs) (AddGoalResult, error) {
		state := getSessionState(ctx)
		if state == nil {
			return AddGoalResult{}, fmt.Errorf("no active session")
		}
		goal := Goal{Description: args.Description, Notes: args.Notes}
		state.mu.Lock()
		state.Goals = append(state.Goals, goal)
		idx := len(state.Goals) - 1
		state.mu.Unlock()
		return AddGoalResult{Index: idx, Goal: goal}, nil
	}
}

type UpdateGoalArgs struct {
	Index     int     `json:"index"`
	Completed *bool   `json:"completed,omitempty"`
	Notes     *string `json:"notes,omitempty"`
}

type UpdateGoalResult struct {
	Index int  `json:"index"`
	Goal  Goal `json:"goal"`
}

func formatUpdateGoalResult(r UpdateGoalResult) string {
	return fmt.Sprintf("goal %d updated", r.Index)
}

func handleUpdateGoal() mcp.StructuredToolHandlerFunc[UpdateGoalArgs, UpdateGoalResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args UpdateGoalArgs) (UpdateGoalResult, error) {
		state := getSessionState(ctx)
		if state == nil {
			return UpdateGoalResult{}, fmt.Errorf("no active session")
		}
		state.mu.Lock()
		defer state.mu.Unlock()
		if args.Index < 0 || args.Index >= len(state.Goals) {
			return UpdateGoalResult{}, fmt.Errorf("invalid goal index")
		}
		goal := state.Goals[args.Index]
		if args.Completed != nil {
			goal.Completed = *args.Completed
		}
		if args.Notes != nil {
			goal.Notes = *args.Notes
		}
		state.Goals[args.Index] = goal
		return UpdateGoalResult{Index: args.Index, Goal: goal}, nil
	}
}
