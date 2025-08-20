package main

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Data types

type ThoughtData struct {
	Thought           string  `json:"thought"`
	ThoughtNumber     int     `json:"thoughtNumber"`
	TotalThoughts     int     `json:"totalThoughts"`
	NextThoughtNeeded bool    `json:"nextThoughtNeeded"`
	IsRevision        *bool   `json:"isRevision,omitempty"`
	RevisesThought    *int    `json:"revisesThought,omitempty"`
	BranchFromThought *int    `json:"branchFromThought,omitempty"`
	BranchID          *string `json:"branchId,omitempty"`
	NeedsMoreThoughts *bool   `json:"needsMoreThoughts,omitempty"`
}

type MentalModelData struct {
	ModelName  string   `json:"modelName"`
	Problem    string   `json:"problem"`
	Steps      []string `json:"steps"`
	Reasoning  string   `json:"reasoning"`
	Conclusion string   `json:"conclusion"`
}

type DebuggingApproachData struct {
	ApproachName string   `json:"approachName"`
	Issue        string   `json:"issue"`
	Steps        []string `json:"steps"`
	Findings     string   `json:"findings"`
	Resolution   string   `json:"resolution"`
}

// Session state

type ServerConfig struct {
	MaxThoughtsPerSession int
}

var defaultConfig = ServerConfig{MaxThoughtsPerSession: 100}

type SessionState struct {
	sessionID         string
	config            ServerConfig
	thoughts          []ThoughtData
	mentalModels      []MentalModelData
	debuggingSessions []DebuggingApproachData
}

func NewSessionState(id string, cfg ServerConfig) *SessionState {
	return &SessionState{sessionID: id, config: cfg}
}

func (s *SessionState) AddThought(t ThoughtData) bool {
	if len(s.thoughts) >= s.config.MaxThoughtsPerSession {
		return false
	}
	s.thoughts = append(s.thoughts, t)
	return true
}

func (s *SessionState) GetThoughts() []ThoughtData { return s.thoughts }
func (s *SessionState) GetRemainingThoughts() int {
	return s.config.MaxThoughtsPerSession - len(s.thoughts)
}

func (s *SessionState) RetractThought() (*ThoughtData, bool) {
	if len(s.thoughts) == 0 {
		return nil, false
	}
	idx := len(s.thoughts) - 1
	t := s.thoughts[idx]
	s.thoughts = s.thoughts[:idx]
	return &t, true
}

func (s *SessionState) AddMentalModel(m MentalModelData)   { s.mentalModels = append(s.mentalModels, m) }
func (s *SessionState) GetMentalModels() []MentalModelData { return s.mentalModels }

func (s *SessionState) AddDebuggingSession(d DebuggingApproachData) {
	s.debuggingSessions = append(s.debuggingSessions, d)
}
func (s *SessionState) GetDebuggingSessions() []DebuggingApproachData { return s.debuggingSessions }

func (s *SessionState) SessionID() string { return s.sessionID }

// Server setup and handlers

func setupServer() *server.MCPServer {
	s := server.NewMCPServer("clear-thought", "0.0.5")
	session := NewSessionState("default", defaultConfig)

	registerSequentialThinking(s, session)
	registerRetractThought(s, session)
	registerMentalModel(s, session)
	registerDebuggingApproach(s, session)

	return s
}

func registerSequentialThinking(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"sequentialthinking",
		mcp.WithDescription("Process sequential thoughts with branching, revision, and memory management capabilities"),
		mcp.WithString("thought", mcp.Required(), mcp.Description("The thought content")),
		mcp.WithNumber("thoughtNumber", mcp.Required(), mcp.Description("Current thought number in sequence")),
		mcp.WithNumber("totalThoughts", mcp.Required(), mcp.Description("Total expected thoughts in sequence")),
		mcp.WithBoolean("nextThoughtNeeded", mcp.Required(), mcp.Description("Whether the next thought is needed")),
		mcp.WithBoolean("isRevision", mcp.Description("Whether this is a revision of a previous thought")),
		mcp.WithNumber("revisesThought", mcp.Description("Which thought number this revises")),
		mcp.WithNumber("branchFromThought", mcp.Description("Which thought this branches from")),
		mcp.WithString("branchId", mcp.Description("Unique identifier for this branch")),
		mcp.WithBoolean("needsMoreThoughts", mcp.Description("Whether more thoughts are needed")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args ThoughtData
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		added := state.AddThought(args)
		all := state.GetThoughts()
		recent := lastThoughts(all, 3)
		res := map[string]any{
			"thought":           args.Thought,
			"thoughtNumber":     args.ThoughtNumber,
			"totalThoughts":     args.TotalThoughts,
			"nextThoughtNeeded": args.NextThoughtNeeded,
			"isRevision":        args.IsRevision,
			"revisesThought":    args.RevisesThought,
			"branchFromThought": args.BranchFromThought,
			"branchId":          args.BranchID,
			"needsMoreThoughts": args.NeedsMoreThoughts,
			"status":            map[bool]string{true: "success", false: "limit_reached"}[added],
			"sessionContext": map[string]any{
				"sessionId":         state.SessionID(),
				"totalThoughts":     len(all),
				"remainingThoughts": state.GetRemainingThoughts(),
				"recentThoughts":    recent,
			},
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerRetractThought(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"retractthought",
		mcp.WithDescription("Remove the most recent thought and update session totals"),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		removed, ok := state.RetractThought()
		res := map[string]any{
			"status":            map[bool]string{true: "success", false: "no_thoughts"}[ok],
			"sessionId":         state.SessionID(),
			"totalThoughts":     len(state.GetThoughts()),
			"remainingThoughts": state.GetRemainingThoughts(),
		}
		if ok {
			res["removedThoughtNumber"] = removed.ThoughtNumber
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerMentalModel(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"mentalmodel",
		mcp.WithDescription("Apply mental models to analyze problems systematically"),
		mcp.WithString("modelName", mcp.Required(), mcp.Enum("first_principles", "opportunity_cost", "error_propagation", "rubber_duck", "pareto_principle", "occams_razor")),
		mcp.WithString("problem", mcp.Required(), mcp.Description("The problem being analyzed")),
		mcp.WithArray("steps", mcp.Required(), mcp.WithStringItems()),
		mcp.WithString("reasoning", mcp.Required(), mcp.Description("Reasoning process")),
		mcp.WithString("conclusion", mcp.Required(), mcp.Description("Conclusions drawn")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args MentalModelData
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		state.AddMentalModel(args)
		all := state.GetMentalModels()
		recent := lastModels(all, 3)
		res := map[string]any{
			"modelName":     args.ModelName,
			"status":        "success",
			"hasSteps":      len(args.Steps) > 0,
			"hasConclusion": args.Conclusion != "",
			"sessionContext": map[string]any{
				"sessionId":         state.SessionID(),
				"totalMentalModels": len(all),
				"recentModels":      recent,
			},
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerDebuggingApproach(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"debuggingapproach",
		mcp.WithDescription("Apply systematic debugging approaches to identify and resolve issues"),
		mcp.WithString("approachName", mcp.Required(), mcp.Enum(
			"binary_search", "reverse_engineering", "divide_conquer", "backtracking", "cause_elimination", "program_slicing",
			"log_analysis", "static_analysis", "root_cause_analysis", "delta_debugging", "fuzzing", "incremental_testing")),
		mcp.WithString("issue", mcp.Required(), mcp.Description("Description of the issue being debugged")),
		mcp.WithArray("steps", mcp.Required(), mcp.WithStringItems()),
		mcp.WithString("findings", mcp.Required(), mcp.Description("Findings discovered during debugging")),
		mcp.WithString("resolution", mcp.Required(), mcp.Description("How the issue was resolved")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args DebuggingApproachData
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		state.AddDebuggingSession(args)
		recent := lastDebugging(state.GetDebuggingSessions(), 3)
		res := map[string]any{
			"approachName":  args.ApproachName,
			"issue":         args.Issue,
			"steps":         args.Steps,
			"findings":      args.Findings,
			"resolution":    args.Resolution,
			"status":        "success",
			"hasSteps":      len(args.Steps) > 0,
			"hasResolution": args.Resolution != "",
			"sessionContext": map[string]any{
				"sessionId":                state.SessionID(),
				"totalDebuggingApproaches": len(state.GetDebuggingSessions()),
				"recentApproaches":         recent,
			},
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

// helpers

func lastThoughts(thoughts []ThoughtData, n int) []map[string]any {
	if len(thoughts) > n {
		thoughts = thoughts[len(thoughts)-n:]
	}
	out := make([]map[string]any, 0, len(thoughts))
	for _, t := range thoughts {
		out = append(out, map[string]any{
			"thoughtNumber": t.ThoughtNumber,
			"isRevision":    t.IsRevision != nil && *t.IsRevision,
		})
	}
	return out
}

func lastModels(models []MentalModelData, n int) []map[string]any {
	if len(models) > n {
		models = models[len(models)-n:]
	}
	out := make([]map[string]any, 0, len(models))
	for _, m := range models {
		out = append(out, map[string]any{
			"modelName": m.ModelName,
			"problem":   m.Problem,
		})
	}
	return out
}

func lastDebugging(list []DebuggingApproachData, n int) []map[string]any {
	if len(list) > n {
		list = list[len(list)-n:]
	}
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, map[string]any{
			"approachName": d.ApproachName,
			"resolved":     d.Resolution != "",
		})
	}
	return out
}
