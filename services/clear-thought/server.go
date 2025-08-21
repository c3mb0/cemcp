package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

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
	branches          map[string]*int
}

func NewSessionState(id string, cfg ServerConfig) *SessionState {
	return &SessionState{sessionID: id, config: cfg, branches: make(map[string]*int)}
}

func (s *SessionState) RegisterBranch(id string, from *int) error {
	if existing, ok := s.branches[id]; ok {
		switch {
		case existing == nil && from != nil:
			return fmt.Errorf("branchId collision for %s", id)
		case existing != nil && from == nil:
			return fmt.Errorf("branchId collision for %s", id)
		case existing != nil && from != nil && *existing != *from:
			return fmt.Errorf("branchId collision for %s", id)
		}
	} else {
		if from != nil {
			v := *from
			s.branches[id] = &v
		} else {
			s.branches[id] = nil
		}
	}
	return nil
}

func (s *SessionState) AddThought(t ThoughtData) bool {
	if len(s.thoughts) >= s.config.MaxThoughtsPerSession {
		return false
	}
	s.thoughts = append(s.thoughts, t)
	return true
}

func (s *SessionState) GetThoughts() []ThoughtData { return s.thoughts }
func (s *SessionState) GetRemainingCapacity() int {
	return s.config.MaxThoughtsPerSession - len(s.thoughts)
}

func (s *SessionState) AddMentalModel(m MentalModelData)   { s.mentalModels = append(s.mentalModels, m) }
func (s *SessionState) GetMentalModels() []MentalModelData { return s.mentalModels }

func (s *SessionState) AddDebuggingSession(d DebuggingApproachData) {
	s.debuggingSessions = append(s.debuggingSessions, d)
}
func (s *SessionState) GetDebuggingSessions() []DebuggingApproachData { return s.debuggingSessions }

func (s *SessionState) SessionID() string { return s.sessionID }

func (s *SessionState) Reset() {
	id := s.sessionID
	cfg := s.config
	*s = *NewSessionState(id, cfg)
}

func (s *SessionState) UpdateThought(num int, text string) (*ThoughtData, bool) {
	for i := range s.thoughts {
		if s.thoughts[i].ThoughtNumber == num {
			s.thoughts[i].Thought = text
			return &s.thoughts[i], true
		}
	}
	return nil, false
}

// Server setup and handlers

func setupServer() *server.MCPServer {
	s := server.NewMCPServer("clear-thought", "0.0.5")
	session := NewSessionState("default", defaultConfig)

	registerSequentialThinking(s, session)
	registerUpdateThought(s, session)
	registerGetBranch(s, session)
	registerMentalModel(s, session)
	registerDebuggingApproach(s, session)
	registerGetThoughts(s, session)
	registerGetMentalModels(s, session)
	registerGetDebuggingSessions(s, session)
	registerResetSession(s, session)

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
		if args.BranchID != nil {
			if err := state.RegisterBranch(*args.BranchID, args.BranchFromThought); err != nil {
				errResp := map[string]any{"error": err.Error(), "status": "failed"}
				b, _ := json.MarshalIndent(errResp, "", "  ")
				out := mcp.NewToolResultText(string(b))
				out.IsError = true
				return out, nil
			}
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
				"remainingCapacity": state.GetRemainingCapacity(),
				"recentThoughts":    recent,
			},
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerUpdateThought(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"updatethought",
		mcp.WithDescription("Update an existing thought by its number"),
		mcp.WithNumber("thoughtNumber", mcp.Required(), mcp.Description("Number of the thought to update")),
		mcp.WithString("thought", mcp.Required(), mcp.Description("Updated thought content")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			ThoughtNumber int    `json:"thoughtNumber"`
			Thought       string `json:"thought"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		updated, ok := state.UpdateThought(args.ThoughtNumber, args.Thought)
		if !ok {
			errResp := map[string]any{"error": fmt.Sprintf("thought %d not found", args.ThoughtNumber), "status": "not_found"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		res := map[string]any{
			"thoughtNumber": args.ThoughtNumber,
			"thought":       updated.Thought,
			"updated":       true,
			"status":        "success",
			"sessionContext": map[string]any{
				"sessionId":      state.SessionID(),
				"updatedThought": updated,
			},
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerGetBranch(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"getbranch",
		mcp.WithDescription("Retrieve the sequence of thoughts for a given branch"),
		mcp.WithString("branchId", mcp.Required(), mcp.Description("Branch identifier")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			BranchID string `json:"branchId"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		history := branchHistory(state.GetThoughts(), args.BranchID)
		seq := make([]map[string]any, 0, len(history))
		mergePoints := make([]int, 0)
		for _, t := range history {
			item := map[string]any{
				"thoughtNumber": t.ThoughtNumber,
				"thought":       t.Thought,
			}
			if t.BranchFromThought != nil {
				mergePoints = append(mergePoints, *t.BranchFromThought)
				item["mergeFromThought"] = *t.BranchFromThought
			}
			seq = append(seq, item)
		}
		res := map[string]any{
			"branchId":    args.BranchID,
			"thoughts":    seq,
			"mergePoints": mergePoints,
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

func registerGetThoughts(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"getthoughts",
		mcp.WithDescription("Retrieve stored thoughts with optional pagination"),
		mcp.WithNumber("offset", mcp.Description("Starting index")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of thoughts to return")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Offset *int `json:"offset"`
			Limit  *int `json:"limit"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		all := state.GetThoughts()
		off := 0
		if args.Offset != nil && *args.Offset > 0 {
			off = *args.Offset
		}
		if off > len(all) {
			off = len(all)
		}
		lim := len(all) - off
		if args.Limit != nil && *args.Limit >= 0 && *args.Limit < lim {
			lim = *args.Limit
		}
		items := all[off : off+lim]

		res := map[string]any{
			"total":    len(all),
			"offset":   off,
			"limit":    lim,
			"thoughts": items,
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerGetMentalModels(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"getmentalmodels",
		mcp.WithDescription("Retrieve stored mental models with optional pagination"),
		mcp.WithNumber("offset", mcp.Description("Starting index")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of models to return")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Offset *int `json:"offset"`
			Limit  *int `json:"limit"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		all := state.GetMentalModels()
		off := 0
		if args.Offset != nil && *args.Offset > 0 {
			off = *args.Offset
		}
		if off > len(all) {
			off = len(all)
		}
		lim := len(all) - off
		if args.Limit != nil && *args.Limit >= 0 && *args.Limit < lim {
			lim = *args.Limit
		}
		items := all[off : off+lim]

		res := map[string]any{
			"total":        len(all),
			"offset":       off,
			"limit":        lim,
			"mentalModels": items,
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerGetDebuggingSessions(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"getdebuggingsessions",
		mcp.WithDescription("Retrieve stored debugging sessions with optional pagination"),
		mcp.WithNumber("offset", mcp.Description("Starting index")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of sessions to return")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Offset *int `json:"offset"`
			Limit  *int `json:"limit"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		all := state.GetDebuggingSessions()
		off := 0
		if args.Offset != nil && *args.Offset > 0 {
			off = *args.Offset
		}
		if off > len(all) {
			off = len(all)
		}
		lim := len(all) - off
		if args.Limit != nil && *args.Limit >= 0 && *args.Limit < lim {
			lim = *args.Limit
		}
		items := all[off : off+lim]

		res := map[string]any{
			"total":             len(all),
			"offset":            off,
			"limit":             lim,
			"debuggingSessions": items,
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerResetSession(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"resetsession",
		mcp.WithDescription("Clear all stored thoughts, mental models, and debugging sessions"),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.Reset()
		res := map[string]any{
			"status": "success",
			"sessionContext": map[string]any{
				"sessionId":         state.SessionID(),
				"remainingCapacity": state.GetRemainingCapacity(),
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

func groupThoughtsByBranchID(thoughts []ThoughtData) map[string][]ThoughtData {
	groups := make(map[string][]ThoughtData)
	for _, t := range thoughts {
		if t.BranchID == nil {
			continue
		}
		id := *t.BranchID
		groups[id] = append(groups[id], t)
	}
	return groups
}

func branchHistory(thoughts []ThoughtData, branchID string) []ThoughtData {
	groups := groupThoughtsByBranchID(thoughts)
	branch := groups[branchID]
	sort.Slice(branch, func(i, j int) bool { return branch[i].ThoughtNumber < branch[j].ThoughtNumber })
	return branch
}
