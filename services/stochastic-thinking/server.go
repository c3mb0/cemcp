package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type StochasticArgs struct {
	Algorithm  string         `json:"algorithm"`
	Problem    string         `json:"problem"`
	Parameters map[string]any `json:"parameters"`
	Result     string         `json:"result,omitempty"`
}

func setupServer() *server.MCPServer {
	s := server.NewMCPServer("stochastic-thinking", "0.1.0")

	tool := mcp.NewTool(
		"stochasticalgorithm",
		mcp.WithDescription(`A tool for applying stochastic algorithms to decision-making problems.
Supports various algorithms including:
- Markov Decision Processes (MDPs): Optimize policies over long sequences of decisions
- Monte Carlo Tree Search (MCTS): Simulate future action sequences for large decision spaces
- Multi-Armed Bandit: Balance exploration vs exploitation in action selection
- Bayesian Optimization: Optimize decisions with probabilistic inference
- Hidden Markov Models (HMMs): Infer latent states affecting decision outcomes`),
		mcp.WithString("algorithm", mcp.Required(), mcp.Enum("mdp", "mcts", "bandit", "bayesian", "hmm")),
		mcp.WithString("problem", mcp.Required()),
		mcp.WithObject("parameters", mcp.Required()),
		mcp.WithString("result"),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args StochasticArgs
		if err := req.BindArguments(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		fmt.Fprintln(os.Stderr, formatOutput(args))
		summary, missing, unknown := summaryForAlgorithm(args)
		status := "success"
		if len(missing) > 0 || len(unknown) > 0 {
			status = "failed"
		}
		res := map[string]any{
			"algorithm":         args.Algorithm,
			"status":            status,
			"summary":           summary,
			"missingParameters": missing,
			"unknownParameters": unknown,
			"hasResult":         args.Result != "",
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		out := mcp.NewToolResultText(string(b))
		if status == "failed" {
			out.IsError = true
		}
		return out, nil
	})

	return s
}

func summaryForAlgorithm(a StochasticArgs) (string, []string, []string) {
	expected := map[string][]string{
		"mdp":      []string{"gamma", "states"},
		"mcts":     []string{"simulations", "explorationConstant"},
		"bandit":   []string{"strategy", "epsilon"},
		"bayesian": []string{"acquisitionFunction"},
		"hmm":      []string{"algorithm"},
	}
	required := expected[a.Algorithm]
	var missing, unknown []string

	for _, k := range required {
		if _, ok := a.Parameters[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range a.Parameters {
		if !contains(required, k) {
			unknown = append(unknown, k)
		}
	}
	if len(missing) > 0 || len(unknown) > 0 {
		return "", missing, unknown
	}

	switch a.Algorithm {
	case "mdp":
		gamma := getNumber(a.Parameters["gamma"], 0.9)
		states := getNumber(a.Parameters["states"], 0)
		return fmt.Sprintf("Optimized policy over %v states with discount factor %v", states, gamma), nil, nil
	case "mcts":
		sims := getNumber(a.Parameters["simulations"], 1000)
		c := getNumber(a.Parameters["explorationConstant"], 1.4)
		return fmt.Sprintf("Explored %v paths with exploration constant %v", sims, c), nil, nil
	case "bandit":
		strategy := getString(a.Parameters["strategy"], "epsilon-greedy")
		eps := getNumber(a.Parameters["epsilon"], 0.1)
		return fmt.Sprintf("Selected optimal arm with %s strategy (ε=%v)", strategy, eps), nil, nil
	case "bayesian":
		acq := getString(a.Parameters["acquisitionFunction"], "expected improvement")
		return fmt.Sprintf("Optimized objective with %s acquisition", acq), nil, nil
	case "hmm":
		alg := getString(a.Parameters["algorithm"], "forward-backward")
		return fmt.Sprintf("Inferred hidden states using %s algorithm", alg), nil, nil
	default:
		return "", nil, nil
	}
}

func contains(arr []string, v string) bool {
	for _, s := range arr {
		if s == v {
			return true
		}
	}
	return false
}

func getNumber(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return def
	}
}

func getString(v any, def string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func formatOutput(a StochasticArgs) string {
	border := strings.Repeat("─", 40)
	b := &strings.Builder{}
	fmt.Fprintf(b, "┌%s┐\n", border)
	fmt.Fprintf(b, "│ 🎲 Algorithm: %s\n", a.Algorithm)
	fmt.Fprintf(b, "├%s┤\n", border)
	fmt.Fprintf(b, "│ Problem: %s\n", a.Problem)
	fmt.Fprintf(b, "├%s┤\n", border)
	fmt.Fprintf(b, "│ Parameters:\n")
	for k, v := range a.Parameters {
		fmt.Fprintf(b, "│ • %s: %v\n", k, v)
	}
	if a.Result != "" {
		fmt.Fprintf(b, "├%s┤\n", border)
		fmt.Fprintf(b, "│ Result: %s\n", a.Result)
	}
	fmt.Fprintf(b, "└%s┘", border)
	return b.String()
}
