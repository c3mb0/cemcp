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

// startTestServer spins up the stochastic-thinking server for integration tests.
func startTestServer(t *testing.T) (*client.Client, func()) {
	t.Helper()
	srv := setupServer()

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
		tr.Close()
		cancel()
		sr.Close()
		sw.Close()
		cr.Close()
		cw.Close()
	}
	return cli, cleanup
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
	cli, cleanup := startTestServer(t)
	defer cleanup()

	ctx := context.Background()
	// Missing gamma parameter for mdp
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
