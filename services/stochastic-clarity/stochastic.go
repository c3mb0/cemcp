package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/c3mb0/cemcp/pkg/stochastic"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ErrorCategory defines the general category of errors for stochastic service
type ErrorCategory string

const (
	CategoryValidation ErrorCategory = "VALIDATION"
	CategoryOperation  ErrorCategory = "OPERATION"
	CategorySystem     ErrorCategory = "SYSTEM"
)

// ErrorSeverity defines the severity level of errors for stochastic service
type ErrorSeverity string

const (
	SeverityError    ErrorSeverity = "ERROR"
	SeverityWarning  ErrorSeverity = "WARNING"
	SeverityCritical ErrorSeverity = "CRITICAL"
)

// StandardErrorResponse provides standardized error reporting for stochastic service
type StandardErrorResponse struct {
	Message     string                 `json:"error"`              // Human-readable error message
	Code        string                 `json:"code"`               // Machine-readable error code
	Category    ErrorCategory          `json:"category"`           // Error category for grouping
	Severity    ErrorSeverity          `json:"severity"`           // Error severity level
	Operation   string                 `json:"operation,omitempty"`// Operation that failed
	Timestamp   string                 `json:"timestamp"`          // ISO 8601 timestamp
	Algorithm   string                 `json:"algorithm,omitempty"`// Algorithm being used
	Context     map[string]interface{} `json:"context,omitempty"`  // Additional context
	Recovery    *RecoveryHint          `json:"recovery,omitempty"` // Recovery suggestions
	Status      string                 `json:"status"`             // Always "failed" for errors
}

// RecoveryHint provides suggestions for error recovery in stochastic service
type RecoveryHint struct {
	Suggestion string   `json:"suggestion"`          // Human-readable suggestion
	Actions    []string `json:"actions,omitempty"`   // Specific actions to take
	Examples   []string `json:"examples,omitempty"`  // Example corrections
}

// NewStochasticError creates a standardized error response for stochastic service
func NewStochasticError(code string, category ErrorCategory, severity ErrorSeverity, message string, algorithm string) StandardErrorResponse {
	return StandardErrorResponse{
		Message:   message,
		Code:      code,
		Category:  category,
		Severity:  severity,
		Algorithm: algorithm,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Status:    "failed",
	}
}

// NewValidationError creates a validation error response for stochastic service
func NewValidationError(field, message string, examples []string, algorithm string) StandardErrorResponse {
	return StandardErrorResponse{
		Message:   fmt.Sprintf("validation failed for %s: %s", field, message),
		Code:      fmt.Sprintf("VALIDATION_%s", strings.ToUpper(field)),
		Category:  CategoryValidation,
		Severity:  SeverityError,
		Algorithm: algorithm,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Status:    "failed",
		Context: map[string]interface{}{
			"field": field,
		},
		Recovery: &RecoveryHint{
			Suggestion: fmt.Sprintf("Fix the %s parameter", field),
			Examples:   examples,
		},
	}
}

// convertToStandardizedError converts validation errors to standardized format
func convertToStandardizedError(err error, algorithm string) StandardErrorResponse {
	errMsg := err.Error()
	
	// Handle specific validation error patterns
	switch {
	case strings.Contains(errMsg, "parameter") && strings.Contains(errMsg, "required"):
		// Extract parameter name from error message
		parts := strings.Split(errMsg, "'")
		field := "unknown"
		if len(parts) >= 2 {
			field = parts[1]
		}
		
		examples := getExamplesForField(field, algorithm)
		return NewValidationError(field, "parameter is required", examples, algorithm)
		
	case strings.Contains(errMsg, "must be between"):
		// Range validation error
		parts := strings.Split(errMsg, "'")
		field := "unknown"
		if len(parts) >= 2 {
			field = parts[1]
		}
		
		examples := getExamplesForField(field, algorithm)
		return NewValidationError(field, "parameter value out of valid range", examples, algorithm)
		
	case strings.Contains(errMsg, "must be one of"):
		// Enum validation error
		parts := strings.Split(errMsg, "'")
		field := "unknown" 
		if len(parts) >= 2 {
			field = parts[1]
		}
		
		examples := getExamplesForField(field, algorithm)
		return NewValidationError(field, "parameter value not in allowed list", examples, algorithm)
		
	default:
		// Generic error
		return NewStochasticError("VALIDATION_GENERAL", CategoryValidation, SeverityError, errMsg, algorithm)
	}
}

// getExamplesForField returns appropriate examples for validation errors
func getExamplesForField(field, algorithm string) []string {
	switch field {
	case "gamma":
		return []string{`"gamma": 0.9`, `"gamma": 0.95`}
	case "states":
		return []string{`"states": 4`, `"states": 10`}
	case "simulations":
		return []string{`"simulations": 1000`, `"simulations": 5000`}
	case "explorationConstant":
		return []string{`"explorationConstant": 1.4`, `"explorationConstant": 2.0`}
	case "strategy":
		return []string{`"strategy": "epsilon_greedy"`, `"strategy": "ucb"`}
	case "epsilon":
		return []string{`"epsilon": 0.1`, `"epsilon": 0.05`}
	case "acquisitionFunction":
		return []string{`"acquisitionFunction": "ucb"`, `"acquisitionFunction": "ei"`}
	case "algorithm":
		if algorithm == "hmm" {
			return []string{`"algorithm": "viterbi"`, `"algorithm": "forward_backward"`}
		}
		return []string{`"algorithm": "mdp"`, `"algorithm": "mcts"`}
	default:
		return []string{}
	}
}

// Error implements the error interface for StandardErrorResponse
func (e StandardErrorResponse) Error() string {
	return e.Message
}

// Types and specs for stochastic algorithms

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

type AlgorithmSpec struct {
	Required    []string       `json:"required"`
	Optional    map[string]any `json:"optional"`
	Description string         `json:"description"`
}

func algorithmSpecs() map[string]AlgorithmSpec {
	return map[string]AlgorithmSpec{
		"mdp": {
			Required:    []string{"gamma", "states"},
			Optional:    map[string]any{},
			Description: "Optimize policies over long sequences of decisions",
		},
		"mcts": {
			Required:    []string{"simulations", "explorationConstant"},
			Optional:    map[string]any{},
			Description: "Simulate future action sequences for large decision spaces",
		},
		"bandit": {
			Required:    []string{"strategy", "epsilon"},
			Optional:    map[string]any{},
			Description: "Balance exploration vs exploitation in action selection",
		},
		"bayesian": {
			Required:    []string{"acquisitionFunction"},
			Optional:    map[string]any{},
			Description: "Optimize decisions with probabilistic inference",
		},
		"hmm": {
			Required:    []string{"algorithm"},
			Optional:    map[string]any{},
			Description: "Infer latent states affecting decision outcomes",
		},
	}
}

func registerStochasticTools(srv *server.MCPServer) {
	specTool := mcp.NewTool(
		"algorithmspec",
		mcp.WithDescription("List available stochastic algorithms and their parameters"),
	)
	srv.AddTool(specTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		b, marshalErr := json.MarshalIndent(algorithmSpecs(), "", "  ")
		if marshalErr != nil {
			b = []byte(`{"error": "failed to serialize algorithm specs", "status": "failed"}`)
		}
		return mcp.NewToolResultText(string(b)), nil
	})

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

	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args StochasticArgs
		if err := req.BindArguments(&args); err != nil {
			errResp := NewStochasticError("OPERATION_BIND_ARGUMENTS", CategoryOperation, SeverityError, err.Error(), args.Algorithm)
			errResp.Recovery = &RecoveryHint{
				Suggestion: "Check request parameters and format",
				Actions:    []string{"Validate JSON structure", "Check parameter names", "Ensure required fields are present"},
			}
			b, marshalErr := json.MarshalIndent(errResp, "", "  ")
			if marshalErr != nil {
				b = []byte(fmt.Sprintf(`{"error": "%s", "status": "failed"}`, "marshal error"))
			}
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		if err := validateArgs(&args); err != nil {
			errResp := convertToStandardizedError(err, args.Algorithm)
			b, marshalErr := json.MarshalIndent(errResp, "", "  ")
			if marshalErr != nil {
				b = []byte(fmt.Sprintf(`{"error": "%s", "status": "failed"}`, "marshal error"))
			}
			out := mcp.NewToolResultText(string(b))
			out.IsError = true
			return out, nil
		}

		fmt.Fprintln(os.Stderr, formatOutput(args))
		summary, nextSteps := summaryForAlgorithm(args)

		sessionID := ""
		if req.Params.Meta != nil {
			if v, ok := req.Params.Meta.AdditionalFields["sessionId"].(string); ok {
				sessionID = v
			}
		}
		if sessionID != "" {
			_ = stochastic.WriteSummary(sessionID, stochastic.StochasticSummary{
				Algorithm: args.Algorithm,
				Summary:   summary,
				NextSteps: nextSteps,
			})
		}

		res := map[string]any{
			"algorithm": args.Algorithm,
			"status":    "success",
			"summary":   summary,
			"hasResult": args.Result != "",
			"nextSteps": nextSteps,
			"hint":      "Run again with `result` populated to verify outcomes.",
		}
		b, marshalErr := json.MarshalIndent(res, "", "  ")
		if marshalErr != nil {
			b = []byte(`{"error": "failed to serialize response", "status": "failed"}`)
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	examplesTool := mcp.NewTool(
		"stochasticexamples",
		mcp.WithDescription("Sample requests for each stochastic algorithm"),
	)
	srv.AddTool(examplesTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		examples := []map[string]any{
			{
				"algorithm": "mdp",
				"problem":   "navigate grid",
				"mdp": map[string]any{
					"gamma":  0.9,
					"states": 4,
				},
			},
			{
				"algorithm": "mcts",
				"problem":   "choose move",
				"mcts": map[string]any{
					"simulations":         1000,
					"explorationConstant": 1.4,
				},
			},
			{
				"algorithm": "bandit",
				"problem":   "select ad",
				"bandit": map[string]any{
					"strategy": "epsilon_greedy",
					"epsilon":  0.1,
				},
			},
			{
				"algorithm": "bayesian",
				"problem":   "tune parameter",
				"bayesian": map[string]any{
					"acquisitionFunction": "ucb",
				},
			},
			{
				"algorithm": "hmm",
				"problem":   "infer weather",
				"hmm": map[string]any{
					"algorithm": "viterbi",
				},
			},
		}
		b, marshalErr := json.MarshalIndent(examples, "", "  ")
		if marshalErr != nil {
			b = []byte(`{"error": "failed to serialize examples", "status": "failed"}`)
		}
		return mcp.NewToolResultText(string(b)), nil
	})
}

func summaryForAlgorithm(a StochasticArgs) (string, string) {
	switch a.Algorithm {
	case "mdp":
		if a.MDP == nil || a.MDP.Gamma == nil || a.MDP.States == nil {
			return "MDP algorithm with incomplete parameters", "Provide valid gamma and states parameters"
		}
		gamma := *a.MDP.Gamma
		states := *a.MDP.States
		return fmt.Sprintf("Optimized policy over %v states with discount factor %v", states, gamma), "Evaluate the derived policy on new states to verify performance"
	case "mcts":
		if a.MCTS == nil || a.MCTS.Simulations == nil || a.MCTS.ExplorationConstant == nil {
			return "MCTS algorithm with incomplete parameters", "Provide valid simulations and explorationConstant parameters"
		}
		sims := *a.MCTS.Simulations
		c := *a.MCTS.ExplorationConstant
		return fmt.Sprintf("Explored %v paths with exploration constant %v", sims, c), "Run additional simulations or adjust the exploration constant for deeper search"
	case "bandit":
		if a.Bandit == nil || a.Bandit.Strategy == nil || a.Bandit.Epsilon == nil {
			return "Bandit algorithm with incomplete parameters", "Provide valid strategy and epsilon parameters"
		}
		strategy := *a.Bandit.Strategy
		eps := *a.Bandit.Epsilon
		return fmt.Sprintf("Selected optimal arm with %s strategy (ε=%v)", strategy, eps), "Collect reward feedback and refine exploration parameters"
	case "bayesian":
		if a.Bayesian == nil || a.Bayesian.AcquisitionFunction == nil {
			return "Bayesian optimization with incomplete parameters", "Provide valid acquisitionFunction parameter"
		}
		acq := *a.Bayesian.AcquisitionFunction
		return fmt.Sprintf("Optimized objective with %s acquisition", acq), "Consider more iterations or alternative acquisition functions"
	case "hmm":
		if a.HMM == nil || a.HMM.Algorithm == nil {
			return "HMM algorithm with incomplete parameters", "Provide valid algorithm parameter"
		}
		alg := *a.HMM.Algorithm
		return fmt.Sprintf("Inferred hidden states using %s algorithm", alg), "Analyze inferred states or tune model parameters"
	default:
		return fmt.Sprintf("Unknown algorithm: %s", a.Algorithm), "Use a supported algorithm: mdp, mcts, bandit, bayesian, or hmm"
	}
}

// Helper functions for validation
func validateNumericRange(value *float64, name string, min, max float64) error {
	if value == nil {
		return fmt.Errorf("parameter '%s' is required but was not provided", name)
	}
	if *value < min || *value > max {
		return fmt.Errorf("parameter '%s' must be between %.2f and %.2f, got %.2f", name, min, max, *value)
	}
	return nil
}

func validatePositiveInt(value *int, name string) error {
	if value == nil {
		return fmt.Errorf("parameter '%s' is required but was not provided", name)
	}
	if *value <= 0 {
		return fmt.Errorf("parameter '%s' must be a positive integer, got %d", name, *value)
	}
	return nil
}

func validateStringEnum(value *string, name string, allowedValues []string) error {
	if value == nil {
		return fmt.Errorf("parameter '%s' is required but was not provided", name)
	}
	if strings.TrimSpace(*value) == "" {
		return fmt.Errorf("parameter '%s' cannot be empty", name)
	}
	for _, allowed := range allowedValues {
		if *value == allowed {
			return nil
		}
	}
	return fmt.Errorf("parameter '%s' must be one of %v, got '%s'", name, allowedValues, *value)
}

func validateArgs(a *StochasticArgs) error {
	if a == nil {
		return fmt.Errorf("arguments cannot be nil")
	}

	// Validate algorithm is provided and not empty
	if strings.TrimSpace(a.Algorithm) == "" {
		return fmt.Errorf("parameter 'algorithm' is required but was not provided")
	}

	// Validate problem is provided and not empty
	if strings.TrimSpace(a.Problem) == "" {
		return fmt.Errorf("parameter 'problem' is required but was not provided")
	}

	switch a.Algorithm {
	case "mdp":
		if a.MDP == nil {
			return fmt.Errorf("mdp parameters are required for algorithm 'mdp'")
		}
		// Validate gamma: must be between 0 and 1 (discount factor)
		if err := validateNumericRange(a.MDP.Gamma, "gamma", 0.0, 1.0); err != nil {
			return err
		}
		// Validate states: must be positive integer
		if err := validatePositiveInt(a.MDP.States, "states"); err != nil {
			return err
		}
		// Additional constraint: states should be reasonable (not too large)
		if *a.MDP.States > 10000 {
			return fmt.Errorf("parameter 'states' exceeds maximum allowed value of 10000, got %d", *a.MDP.States)
		}

	case "mcts":
		if a.MCTS == nil {
			return fmt.Errorf("mcts parameters are required for algorithm 'mcts'")
		}
		// Validate simulations: must be positive integer
		if err := validatePositiveInt(a.MCTS.Simulations, "simulations"); err != nil {
			return err
		}
		// Validate explorationConstant: must be positive
		if a.MCTS.ExplorationConstant == nil {
			return fmt.Errorf("parameter 'explorationConstant' is required but was not provided")
		}
		if *a.MCTS.ExplorationConstant <= 0 {
			return fmt.Errorf("parameter 'explorationConstant' must be positive, got %.2f", *a.MCTS.ExplorationConstant)
		}
		// Additional constraints for reasonable values
		if *a.MCTS.Simulations > 1000000 {
			return fmt.Errorf("parameter 'simulations' exceeds maximum allowed value of 1000000, got %d", *a.MCTS.Simulations)
		}
		if *a.MCTS.ExplorationConstant > 10.0 {
			return fmt.Errorf("parameter 'explorationConstant' exceeds maximum allowed value of 10.0, got %.2f", *a.MCTS.ExplorationConstant)
		}

	case "bandit":
		if a.Bandit == nil {
			return fmt.Errorf("bandit parameters are required for algorithm 'bandit'")
		}
		// Validate strategy: must be from allowed set
		allowedStrategies := []string{"epsilon_greedy", "ucb", "thompson_sampling"}
		if err := validateStringEnum(a.Bandit.Strategy, "strategy", allowedStrategies); err != nil {
			return err
		}
		// Validate epsilon: must be between 0 and 1
		if err := validateNumericRange(a.Bandit.Epsilon, "epsilon", 0.0, 1.0); err != nil {
			return err
		}

	case "bayesian":
		if a.Bayesian == nil {
			return fmt.Errorf("bayesian parameters are required for algorithm 'bayesian'")
		}
		// Validate acquisitionFunction: must be from allowed set
		allowedAcqFunctions := []string{"ucb", "ei", "pi"}
		if err := validateStringEnum(a.Bayesian.AcquisitionFunction, "acquisitionFunction", allowedAcqFunctions); err != nil {
			return err
		}

	case "hmm":
		if a.HMM == nil {
			return fmt.Errorf("hmm parameters are required for algorithm 'hmm'")
		}
		// Validate algorithm: must be from allowed set
		allowedAlgorithms := []string{"viterbi", "forward_backward", "baum_welch"}
		if err := validateStringEnum(a.HMM.Algorithm, "algorithm", allowedAlgorithms); err != nil {
			return err
		}

	default:
		allowedAlgorithms := []string{"mdp", "mcts", "bandit", "bayesian", "hmm"}
		return fmt.Errorf("parameter 'algorithm' must be one of %v, got '%s'", allowedAlgorithms, a.Algorithm)
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
