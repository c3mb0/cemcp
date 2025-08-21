package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/c3mb0/cemcp/pkg/stochastic"
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

type Goal struct {
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	Notes       string `json:"notes,omitempty"`
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
	goals             []Goal
	branches          map[string]*int
	summaries         []string
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

func (s *SessionState) SummarizeThoughts(n int) string {
	if n <= 0 || len(s.thoughts) == 0 {
		return ""
	}
	if n > len(s.thoughts) {
		n = len(s.thoughts)
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = s.thoughts[i].Thought
	}
	summary := strings.Join(parts, " ")
	s.summaries = append(s.summaries, summary)
	s.thoughts = append([]ThoughtData(nil), s.thoughts[n:]...)
	return summary
}

func (s *SessionState) AddThought(t ThoughtData) (bool, string) {
	if len(s.thoughts) >= s.config.MaxThoughtsPerSession {
		return false, ""
	}
	s.thoughts = append(s.thoughts, t)
	var summary string
	threshold := int(float64(s.config.MaxThoughtsPerSession) * 0.8)
	if len(s.thoughts) >= threshold {
		summary = s.SummarizeThoughts(len(s.thoughts) / 2)
	}
	return true, summary
}

func (s *SessionState) GetThoughts() []ThoughtData { return s.thoughts }
func (s *SessionState) GetRemainingThoughts() int {
	return s.config.MaxThoughtsPerSession - len(s.thoughts)
}

func (s *SessionState) RetractThought() (*ThoughtData, bool) {
	n := len(s.thoughts)
	if n == 0 {
		return nil, false
	}
	t := s.thoughts[n-1]
	s.thoughts = s.thoughts[:n-1]
	return &t, true
}

func (s *SessionState) AddMentalModel(m MentalModelData)   { s.mentalModels = append(s.mentalModels, m) }
func (s *SessionState) GetMentalModels() []MentalModelData { return s.mentalModels }

func (s *SessionState) AddDebuggingSession(d DebuggingApproachData) {
	s.debuggingSessions = append(s.debuggingSessions, d)
}
func (s *SessionState) GetDebuggingSessions() []DebuggingApproachData { return s.debuggingSessions }

func (s *SessionState) AddGoal(g Goal) { s.goals = append(s.goals, g) }
func (s *SessionState) UpdateGoal(index int, completed *bool, notes *string) (*Goal, bool) {
	if index < 0 || index >= len(s.goals) {
		return nil, false
	}
	if completed != nil {
		s.goals[index].Completed = *completed
	}
	if notes != nil {
		s.goals[index].Notes = *notes
	}
	return &s.goals[index], true
}
func (s *SessionState) GetGoals() []Goal { return s.goals }
func (s *SessionState) GetOutstandingGoals() []Goal {
	out := make([]Goal, 0)
	for _, g := range s.goals {
		if !g.Completed {
			out = append(out, g)
		}
	}
	return out
}

func (s *SessionState) SessionID() string { return s.sessionID }

func (s *SessionState) UpdateThought(num int, text string) (*ThoughtData, bool) {
	for i := range s.thoughts {
		if s.thoughts[i].ThoughtNumber == num {
			s.thoughts[i].Thought = text
			return &s.thoughts[i], true
		}
	}
	return nil, false
}

func (s *SessionState) TrimThoughts(keepLast int) (removed, remaining int) {
	if keepLast < 0 {
		keepLast = 0
	}
	total := len(s.thoughts)
	if keepLast >= total {
		return 0, total
	}
	removed = total - keepLast
	s.thoughts = append([]ThoughtData(nil), s.thoughts[total-keepLast:]...)
	return removed, len(s.thoughts)
}

func (s *SessionState) Reset() {
	s.thoughts = nil
	s.mentalModels = nil
	s.debuggingSessions = nil
	s.goals = nil
	s.branches = make(map[string]*int)
	s.summaries = nil
}

// Server setup and handlers

func setupServer() *server.MCPServer {
	s := server.NewMCPServer("stochastic-clarity", "0.1.0")
	session := NewSessionState("default", defaultConfig)

	registerSequentialThinking(s, session)
	registerUpdateThought(s, session)
	registerRetractThought(s, session)
	registerGetBranch(s, session)
	registerMentalModel(s, session)
	registerDebuggingApproach(s, session)
	registerAddGoal(s, session)
	registerUpdateGoal(s, session)
	registerGetThoughts(s, session)
	registerGetMentalModels(s, session)
	registerGetDebuggingSessions(s, session)
	registerResetSession(s, session)
	registerTrimSession(s, session)
	registerSessionContext(s, session)
	registerSearchContext(s, session)
	registerStochasticClarityExamples(s)
	registerStochasticTools(s)

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
		expectedThoughtNumber := len(state.GetThoughts()) + 1
		if args.ThoughtNumber != expectedThoughtNumber {
			warnResp := map[string]any{
				"error":                 fmt.Sprintf("thoughtNumber must be %d but got %d", expectedThoughtNumber, args.ThoughtNumber),
				"expectedThoughtNumber": expectedThoughtNumber,
				"status":                "out_of_order",
			}
			b, _ := json.MarshalIndent(warnResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			return out, nil
		}
		if args.IsRevision != nil && args.RevisesThought == nil {
			errResp := map[string]any{"error": "revisesThought is required when isRevision is set", "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}
		if args.BranchFromThought != nil && args.BranchID == nil {
			errResp := map[string]any{"error": "branchId is required when branchFromThought is provided", "status": "failed"}
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

		added, summary := state.AddThought(args)
		all := state.GetThoughts()
		recent := lastThoughts(all, 3)
		sessionCtx := map[string]any{
			"sessionId":         state.SessionID(),
			"totalThoughts":     len(all),
			"remainingThoughts": state.GetRemainingThoughts(),
			"recentThoughts":    recent,
			"totalGoals":        len(state.GetGoals()),
			"outstandingGoals":  state.GetOutstandingGoals(),
		}
		if summary != "" {
			sessionCtx["summary"] = summary
		}
		if ss, err := stochastic.ReadSummary(state.SessionID()); err == nil {
			sessionCtx["stochasticSummary"] = ss
		}
		res := map[string]any{
			"thought":               args.Thought,
			"thoughtNumber":         args.ThoughtNumber,
			"expectedThoughtNumber": expectedThoughtNumber,
			"totalThoughts":         args.TotalThoughts,
			"nextThoughtNeeded":     args.NextThoughtNeeded,
			"isRevision":            args.IsRevision,
			"revisesThought":        args.RevisesThought,
			"branchFromThought":     args.BranchFromThought,
			"branchId":              args.BranchID,
			"needsMoreThoughts":     args.NeedsMoreThoughts,
			"status":                map[bool]string{true: "success", false: "limit_reached"}[added],
			"sessionContext":        sessionCtx,
			"hint":                  fmt.Sprintf("Submit thought #%d if continuing.", expectedThoughtNumber),
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
				"sessionId":        state.SessionID(),
				"updatedThought":   updated,
				"totalGoals":       len(state.GetGoals()),
				"outstandingGoals": state.GetOutstandingGoals(),
			},
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerRetractThought(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"retractthought",
		mcp.WithDescription("Remove the most recent thought"),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		t, ok := state.RetractThought()
		if !ok {
			errResp := map[string]any{"error": "no thoughts to retract", "status": "empty"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		res := map[string]any{
			"retractedThought":  t,
			"totalThoughts":     len(state.GetThoughts()),
			"remainingThoughts": state.GetRemainingThoughts(),
			"status":            "success",
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
				"totalGoals":        len(state.GetGoals()),
				"outstandingGoals":  state.GetOutstandingGoals(),
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
				"totalGoals":               len(state.GetGoals()),
				"outstandingGoals":         state.GetOutstandingGoals(),
			},
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerAddGoal(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"addgoal",
		mcp.WithDescription("Add a goal to the session"),
		mcp.WithString("description", mcp.Required(), mcp.Description("Goal description")),
		mcp.WithString("notes", mcp.Description("Optional notes for the goal")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Description string `json:"description"`
			Notes       string `json:"notes,omitempty"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		g := Goal{Description: args.Description, Notes: args.Notes}
		state.AddGoal(g)
		res := map[string]any{
			"status": "success",
			"goal":   g,
			"sessionContext": map[string]any{
				"sessionId":        state.SessionID(),
				"totalGoals":       len(state.GetGoals()),
				"outstandingGoals": state.GetOutstandingGoals(),
			},
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerUpdateGoal(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"updategoal",
		mcp.WithDescription("Update goal completion status or notes"),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("Goal index")),
		mcp.WithBoolean("completed", mcp.Description("Mark goal as completed")),
		mcp.WithString("notes", mcp.Description("Updated notes")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Index     int     `json:"index"`
			Completed *bool   `json:"completed,omitempty"`
			Notes     *string `json:"notes,omitempty"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		g, ok := state.UpdateGoal(args.Index, args.Completed, args.Notes)
		if !ok {
			errResp := map[string]any{"error": "goal not found", "status": "not_found"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		res := map[string]any{
			"status": "success",
			"goal":   g,
			"sessionContext": map[string]any{
				"sessionId":        state.SessionID(),
				"totalGoals":       len(state.GetGoals()),
				"outstandingGoals": state.GetOutstandingGoals(),
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

func registerSessionContext(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"sessioncontext",
		mcp.WithDescription("Summarize session status with counts and recent entries for thoughts, mental models, and debugging sessions"),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		thoughts := state.GetThoughts()
		models := state.GetMentalModels()
		debug := state.GetDebuggingSessions()
		res := map[string]any{
			"sessionId":               state.SessionID(),
			"totalThoughts":           len(thoughts),
			"remainingThoughts":       state.GetRemainingThoughts(),
			"recentThoughts":          lastThoughts(thoughts, 3),
			"totalMentalModels":       len(models),
			"recentMentalModels":      lastModels(models, 3),
			"totalDebuggingSessions":  len(debug),
			"recentDebuggingSessions": lastDebugging(debug, 3),
			"totalGoals":              len(state.GetGoals()),
			"outstandingGoals":        state.GetOutstandingGoals(),
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerSearchContext(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"searchcontext",
		mcp.WithDescription("Search thoughts, mental models, and debugging sessions"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Substring or regex to match")),
		mcp.WithNumber("offset", mcp.Description("Starting index for paginated results")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Query  string `json:"query"`
			Offset *int   `json:"offset"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		match := func(s string) bool { return false }
		if re, err := regexp.Compile(args.Query); err == nil {
			match = func(s string) bool { return re.MatchString(s) }
		} else {
			match = func(s string) bool { return strings.Contains(s, args.Query) }
		}

		results := make([]map[string]any, 0)

		for i, t := range state.GetThoughts() {
			if match(t.Thought) {
				results = append(results, map[string]any{
					"type":  "thought",
					"index": i,
					"data":  t,
				})
			}
		}

		for i, m := range state.GetMentalModels() {
			text := strings.Join(append([]string{m.ModelName, m.Problem, m.Reasoning, m.Conclusion}, m.Steps...), " ")
			if match(text) {
				results = append(results, map[string]any{
					"type":  "mentalModel",
					"index": i,
					"data":  m,
				})
			}
		}

		for i, d := range state.GetDebuggingSessions() {
			text := strings.Join(append([]string{d.ApproachName, d.Issue, d.Findings, d.Resolution}, d.Steps...), " ")
			if match(text) {
				results = append(results, map[string]any{
					"type":  "debuggingSession",
					"index": i,
					"data":  d,
				})
			}
		}

		off := 0
		if args.Offset != nil && *args.Offset > 0 {
			off = *args.Offset
		}
		if off > len(results) {
			off = len(results)
		}
		limit := 20
		end := off + limit
		if end > len(results) {
			end = len(results)
		}
		items := results[off:end]
		var nextOffset *int
		if end < len(results) {
			n := end
			nextOffset = &n
		}

		res := map[string]any{
			"total":      len(results),
			"offset":     off,
			"limit":      limit,
			"results":    items,
			"nextOffset": nextOffset,
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerStochasticClarityExamples(srv *server.MCPServer) {
	tool := mcp.NewTool(
		"stochasticclarityexamples",
		mcp.WithDescription("Sample requests for sequentialthinking, mentalmodel, debuggingapproach, and stochasticalgorithm"),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		examples := []map[string]any{
			{
				"tool": "sequentialthinking",
				"args": map[string]any{
					"thought":           "outline solution",
					"thoughtNumber":     1,
					"totalThoughts":     2,
					"nextThoughtNeeded": true,
				},
			},
			{
				"tool": "mentalmodel",
				"args": map[string]any{
					"modelName":  "first_principles",
					"problem":    "reduce load time",
					"steps":      []string{"break into parts", "optimize each"},
					"reasoning":  "start from basics",
					"conclusion": "cache assets",
				},
			},
			{
				"tool": "debuggingapproach",
				"args": map[string]any{
					"approachName": "binary_search",
					"issue":        "crash on launch",
					"steps":        []string{"split code", "test halves"},
					"findings":     "bad init sequence",
					"resolution":   "fix order",
				},
			},
			{
				"tool": "stochasticalgorithm",
				"args": map[string]any{
					"algorithm": "mdp",
					"problem":   "navigate grid",
					"mdp": map[string]any{
						"gamma":  0.9,
						"states": 4,
					},
				},
			},
		}
		b, _ := json.MarshalIndent(examples, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerResetSession(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"resetsession",
		mcp.WithDescription("Clear all stored thoughts, mental models, and debugging sessions, resetting the session to its initial state"),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.Reset()
		res := map[string]any{
			"status":            "reset",
			"remainingThoughts": state.GetRemainingThoughts(),
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerTrimSession(srv *server.MCPServer, state *SessionState) {
	tool := mcp.NewTool(
		"trimsession",
		mcp.WithDescription("Trim stored thoughts keeping only the most recent ones"),
		mcp.WithNumber("keepLast", mcp.Required(), mcp.Description("Number of recent thoughts to keep")),
	)

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			KeepLast int `json:"keepLast"`
		}
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		removed, remaining := state.TrimThoughts(args.KeepLast)
		res := map[string]any{
			"removed":   removed,
			"remaining": remaining,
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
