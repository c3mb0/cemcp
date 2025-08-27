package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// startTestServer creates a stochastic-clarity server with the provided config.
func startTestServer(t *testing.T, cfg ServerConfig) (*client.Client, *SessionState, func()) {
	t.Helper()

	srv := server.NewMCPServer("stochastic-clarity-test", "test")
	state := NewSessionState("test", cfg)
	registerSequentialThinking(srv, state)
	registerMentalModel(srv, state)
	registerDebuggingApproach(srv, state)
	registerStochasticTools(srv)

	sr, cw := io.Pipe()
	cr, sw := io.Pipe()

	stdio := server.NewStdioServer(srv)
	ctx, cancel := context.WithCancel(context.Background())
	go stdio.Listen(ctx, sr, sw)

	tr := transport.NewIO(cr, cw, io.NopCloser(&bytes.Buffer{}))
	if err := tr.Start(ctx); err != nil {
		t.Fatalf("transport start: %v", err)
	}
	cli := client.NewClient(tr)
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION}}); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	cleanup := func() {
		// First cancel the context to stop the server
		cancel()
		// Then close transport
		tr.Close()
		// Finally close pipes
		sr.Close()
		sw.Close()
		cr.Close()
		cw.Close()
	}
	return cli, state, cleanup
}

func TestSequentialThinkingEnforcesLimit(t *testing.T) {
	cli, state, cleanup := startTestServer(t, ServerConfig{MaxThoughtsPerSession: 1})
	defer cleanup()

	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		res, err := cli.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "sequentialthinking",
				Arguments: map[string]any{
					"thought":           fmt.Sprintf("t%v", i),
					"thoughtNumber":     i,
					"totalThoughts":     3,
					"nextThoughtNeeded": true,
				},
			},
		})
		if err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
		text := res.Content[0].(mcp.TextContent).Text
		var body struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(text), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		wantStatus := "success"
		if i == 2 {
			wantStatus = "limit_reached"
		}
		if body.Status != wantStatus {
			t.Fatalf("call %d status = %s want %s", i, body.Status, wantStatus)
		}
		if got := len(state.GetThoughts()); got != 1 {
			t.Fatalf("thought count after call %d = %d want 1", i, got)
		}
	}
}

func TestMentalModelUpdatesState(t *testing.T) {
	cli, state, cleanup := startTestServer(t, defaultConfig)
	defer cleanup()

	ctx := context.Background()
	res, err := cli.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "mentalmodel",
			Arguments: map[string]any{
				"modelName":  "first_principles",
				"problem":    "p",
				"steps":      []string{"s1", "s2"},
				"reasoning":  "r",
				"conclusion": "c",
			},
		},
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if len(state.GetMentalModels()) != 1 {
		t.Fatalf("expected 1 model in state")
	}
	text := res.Content[0].(mcp.TextContent).Text
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Status != "success" {
		t.Fatalf("unexpected status %s", body.Status)
	}
}

func TestDebuggingApproachUpdatesState(t *testing.T) {
	cli, state, cleanup := startTestServer(t, defaultConfig)
	defer cleanup()

	ctx := context.Background()
	res, err := cli.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "debuggingapproach",
			Arguments: map[string]any{
				"approachName": "binary_search",
				"issue":        "bug",
				"steps":        []string{"s1"},
				"findings":     "f",
				"resolution":   "r",
			},
		},
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if len(state.GetDebuggingSessions()) != 1 {
		t.Fatalf("expected 1 debugging session")
	}
	text := res.Content[0].(mcp.TextContent).Text
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Status != "success" {
		t.Fatalf("unexpected status %s", body.Status)
	}
}

func TestSummaryForAlgorithm(t *testing.T) {
	gamma := 0.9
	states := 4
	sims := 100
	ec := 1.4
	eps := 0.1
	strategy := "epsilon_greedy"
	acq := "ucb"
	alg := "viterbi"

	tests := []struct {
		name    string
		args    StochasticArgs
		summary string
		next    string
	}{
		{"mdp", StochasticArgs{Algorithm: "mdp", MDP: &MDPParams{Gamma: &gamma, States: &states}}, fmt.Sprintf("Optimized policy over %v states with discount factor %v", states, gamma), "Evaluate the derived policy on new states to verify performance"},
		{"mcts", StochasticArgs{Algorithm: "mcts", MCTS: &MCTSParams{Simulations: &sims, ExplorationConstant: &ec}}, fmt.Sprintf("Explored %v paths with exploration constant %v", sims, ec), "Run additional simulations or adjust the exploration constant for deeper search"},
		{"bandit", StochasticArgs{Algorithm: "bandit", Bandit: &BanditParams{Strategy: &strategy, Epsilon: &eps}}, fmt.Sprintf("Selected optimal arm with %s strategy (ε=%v)", strategy, eps), "Collect reward feedback and refine exploration parameters"},
		{"bayesian", StochasticArgs{Algorithm: "bayesian", Bayesian: &BayesianParams{AcquisitionFunction: &acq}}, fmt.Sprintf("Optimized objective with %s acquisition", acq), "Consider more iterations or alternative acquisition functions"},
		{"hmm", StochasticArgs{Algorithm: "hmm", HMM: &HMMParams{Algorithm: &alg}}, fmt.Sprintf("Inferred hidden states using %s algorithm", alg), "Analyze inferred states or tune model parameters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, next := summaryForAlgorithm(tt.args)
			if got != tt.summary || next != tt.next {
				t.Fatalf("summaryForAlgorithm() = %q, %q want %q, %q", got, next, tt.summary, tt.next)
			}
		})
	}
}

func TestStochasticAlgorithmMissingParams(t *testing.T) {
	cli, _, cleanup := startTestServer(t, defaultConfig)
	defer cleanup()

	ctx := context.Background()
	res, err := cli.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "stochasticalgorithm",
			Arguments: map[string]any{
				"algorithm": "mdp",
				"problem":   "navigate",
				"mdp": map[string]any{
					"states": 4,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error result")
	}
	if len(res.Content) == 0 {
		t.Fatalf("expected content")
	}
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(text.Text), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Fatalf("expected error message in body")
	}
}

func TestValidateArgsMissing(t *testing.T) {
	states := 3
	if err := validateArgs(&StochasticArgs{Algorithm: "mdp", MDP: &MDPParams{States: &states}}); err == nil {
		t.Fatalf("expected error for missing gamma")
	}
}
