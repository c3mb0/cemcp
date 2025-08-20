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

type MDPParams struct {
	Gamma  *float64 `json:"gamma"`
	States *int     `json:"states"`
}

type MCTSParams struct {
	Simulations         *int     `json:"simulations"`
	ExplorationConstant *float64 `json:"explorationConstant"`
}

type BanditParams struct {
	Strategy *string  `json:"strategy"`
	Epsilon  *float64 `json:"epsilon"`
}

type BayesianParams struct {
	AcquisitionFunction *string `json:"acquisitionFunction"`
}

type HMMParams struct {
	Algorithm *string `json:"algorithm"`
}

type StochasticArgs struct {
	Algorithm string          `json:"algorithm"`
	Problem   string          `json:"problem"`
	MDP       *MDPParams      `json:"mdp,omitempty"`
	MCTS      *MCTSParams     `json:"mcts,omitempty"`
	Bandit    *BanditParams   `json:"bandit,omitempty"`
	Bayesian  *BayesianParams `json:"bayesian,omitempty"`
	HMM       *HMMParams      `json:"hmm,omitempty"`
	Result    string          `json:"result,omitempty"`
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
		mcp.WithObject("mdp"),
		mcp.WithObject("mcts"),
		mcp.WithObject("bandit"),
		mcp.WithObject("bayesian"),
		mcp.WithObject("hmm"),
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

		if err := validateArgs(&args); err != nil {
			errResp := map[string]any{"error": err.Error(), "status": "failed"}
			b, _ := json.MarshalIndent(errResp, "", "  ")
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		fmt.Fprintln(os.Stderr, formatOutput(args))
		summary, nextSteps := summaryForAlgorithm(args)
		res := map[string]any{
			"algorithm": args.Algorithm,
			"status":    "success",
			"summary":   summary,
			"hasResult": args.Result != "",
			"nextSteps": nextSteps,
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		out := mcp.NewToolResultText(string(b))
		return out, nil
	})

	return s
}

func summaryForAlgorithm(a StochasticArgs) (string, string) {
	switch a.Algorithm {
	case "mdp":
		gamma := *a.MDP.Gamma
		states := *a.MDP.States
		return fmt.Sprintf("Optimized policy over %v states with discount factor %v", states, gamma), "Evaluate the derived policy on new states to verify performance"
	case "mcts":
		sims := *a.MCTS.Simulations
		c := *a.MCTS.ExplorationConstant
		return fmt.Sprintf("Explored %v paths with exploration constant %v", sims, c), "Run additional simulations or adjust the exploration constant for deeper search"
	case "bandit":
		strategy := *a.Bandit.Strategy
		eps := *a.Bandit.Epsilon
		return fmt.Sprintf("Selected optimal arm with %s strategy (ε=%v)", strategy, eps), "Collect reward feedback and refine exploration parameters"
	case "bayesian":
		acq := *a.Bayesian.AcquisitionFunction
		return fmt.Sprintf("Optimized objective with %s acquisition", acq), "Consider more iterations or alternative acquisition functions"
	case "hmm":
		alg := *a.HMM.Algorithm
		return fmt.Sprintf("Inferred hidden states using %s algorithm", alg), "Analyze inferred states or tune model parameters"
	default:
		return "", ""
	}
}

func validateArgs(a *StochasticArgs) error {
	switch a.Algorithm {
	case "mdp":
		if a.MDP == nil {
			return fmt.Errorf("mdp parameters are required")
		}
		var missing []string
		if a.MDP.Gamma == nil {
			missing = append(missing, "gamma")
		}
		if a.MDP.States == nil {
			missing = append(missing, "states")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing parameters: %s", strings.Join(missing, ", "))
		}
	case "mcts":
		if a.MCTS == nil {
			return fmt.Errorf("mcts parameters are required")
		}
		var missing []string
		if a.MCTS.Simulations == nil {
			missing = append(missing, "simulations")
		}
		if a.MCTS.ExplorationConstant == nil {
			missing = append(missing, "explorationConstant")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing parameters: %s", strings.Join(missing, ", "))
		}
	case "bandit":
		if a.Bandit == nil {
			return fmt.Errorf("bandit parameters are required")
		}
		var missing []string
		if a.Bandit.Strategy == nil {
			missing = append(missing, "strategy")
		}
		if a.Bandit.Epsilon == nil {
			missing = append(missing, "epsilon")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing parameters: %s", strings.Join(missing, ", "))
		}
	case "bayesian":
		if a.Bayesian == nil {
			return fmt.Errorf("bayesian parameters are required")
		}
		if a.Bayesian.AcquisitionFunction == nil {
			return fmt.Errorf("missing parameters: acquisitionFunction")
		}
	case "hmm":
		if a.HMM == nil {
			return fmt.Errorf("hmm parameters are required")
		}
		if a.HMM.Algorithm == nil {
			return fmt.Errorf("missing parameters: algorithm")
		}
	default:
		return fmt.Errorf("unknown algorithm: %s", a.Algorithm)
	}
	return nil
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
	params := map[string]any{}
	switch a.Algorithm {
	case "mdp":
		if a.MDP != nil {
			if a.MDP.Gamma != nil {
				params["gamma"] = *a.MDP.Gamma
			}
			if a.MDP.States != nil {
				params["states"] = *a.MDP.States
			}
		}
	case "mcts":
		if a.MCTS != nil {
			if a.MCTS.Simulations != nil {
				params["simulations"] = *a.MCTS.Simulations
			}
			if a.MCTS.ExplorationConstant != nil {
				params["explorationConstant"] = *a.MCTS.ExplorationConstant
			}
		}
	case "bandit":
		if a.Bandit != nil {
			if a.Bandit.Strategy != nil {
				params["strategy"] = *a.Bandit.Strategy
			}
			if a.Bandit.Epsilon != nil {
				params["epsilon"] = *a.Bandit.Epsilon
			}
		}
	case "bayesian":
		if a.Bayesian != nil {
			if a.Bayesian.AcquisitionFunction != nil {
				params["acquisitionFunction"] = *a.Bayesian.AcquisitionFunction
			}
		}
	case "hmm":
		if a.HMM != nil {
			if a.HMM.Algorithm != nil {
				params["algorithm"] = *a.HMM.Algorithm
			}
		}
	}
	for k, v := range params {
		fmt.Fprintf(b, "│ • %s: %v\n", k, v)
	}
	if a.Result != "" {
		fmt.Fprintf(b, "├%s┤\n", border)
		fmt.Fprintf(b, "│ Result: %s\n", a.Result)
	}
	fmt.Fprintf(b, "└%s┘", border)
	return b.String()
}
