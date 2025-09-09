package main

import (
	"strings"
	"testing"
)

// Helper functions for creating test parameters
func floatPtr(f float64) *float64 { return &f }
func intPtr(i int) *int          { return &i }
func stringPtr(s string) *string  { return &s }

func TestValidateArgs_NilArgs(t *testing.T) {
	err := validateArgs(nil)
	if err == nil {
		t.Fatal("Expected error for nil arguments")
	}
	if !strings.Contains(err.Error(), "arguments cannot be nil") {
		t.Errorf("Expected 'arguments cannot be nil' error, got: %v", err)
	}
}

func TestValidateArgs_EmptyAlgorithm(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "",
		Problem:   "test problem",
	}
	err := validateArgs(args)
	if err == nil {
		t.Fatal("Expected error for empty algorithm")
	}
	if !strings.Contains(err.Error(), "parameter 'algorithm' is required") {
		t.Errorf("Expected algorithm required error, got: %v", err)
	}
}

func TestValidateArgs_EmptyProblem(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "mdp",
		Problem:   "",
	}
	err := validateArgs(args)
	if err == nil {
		t.Fatal("Expected error for empty problem")
	}
	if !strings.Contains(err.Error(), "parameter 'problem' is required") {
		t.Errorf("Expected problem required error, got: %v", err)
	}
}

func TestValidateArgs_UnknownAlgorithm(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "unknown",
		Problem:   "test problem",
	}
	err := validateArgs(args)
	if err == nil {
		t.Fatal("Expected error for unknown algorithm")
	}
	expected := "parameter 'algorithm' must be one of"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected algorithm validation error, got: %v", err)
	}
}

// MDP Tests
func TestValidateArgs_MDP_Valid(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "mdp",
		Problem:   "test problem",
		MDP: &MDPParams{
			Gamma:  floatPtr(0.9),
			States: intPtr(10),
		},
	}
	err := validateArgs(args)
	if err != nil {
		t.Errorf("Expected no error for valid MDP args, got: %v", err)
	}
}

func TestValidateArgs_MDP_NilParams(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "mdp",
		Problem:   "test problem",
		MDP:       nil,
	}
	err := validateArgs(args)
	if err == nil {
		t.Fatal("Expected error for nil MDP params")
	}
	if !strings.Contains(err.Error(), "mdp parameters are required") {
		t.Errorf("Expected MDP params required error, got: %v", err)
	}
}

func TestValidateArgs_MDP_NilGamma(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "mdp",
		Problem:   "test problem",
		MDP: &MDPParams{
			Gamma:  nil,
			States: intPtr(10),
		},
	}
	err := validateArgs(args)
	if err == nil {
		t.Fatal("Expected error for nil gamma")
	}
	if !strings.Contains(err.Error(), "parameter 'gamma' is required") {
		t.Errorf("Expected gamma required error, got: %v", err)
	}
}

func TestValidateArgs_MDP_InvalidGamma(t *testing.T) {
	testCases := []struct {
		gamma       float64
		description string
	}{
		{-0.1, "negative gamma"},
		{1.1, "gamma > 1"},
		{-1.0, "gamma = -1"},
		{2.0, "gamma = 2"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			args := &StochasticArgs{
				Algorithm: "mdp",
				Problem:   "test problem",
				MDP: &MDPParams{
					Gamma:  floatPtr(tc.gamma),
					States: intPtr(10),
				},
			}
			err := validateArgs(args)
			if err == nil {
				t.Fatal("Expected error for invalid gamma")
			}
			if !strings.Contains(err.Error(), "parameter 'gamma' must be between") {
				t.Errorf("Expected gamma range error, got: %v", err)
			}
		})
	}
}

func TestValidateArgs_MDP_InvalidStates(t *testing.T) {
	testCases := []struct {
		states      int
		description string
	}{
		{0, "zero states"},
		{-1, "negative states"},
		{-100, "very negative states"},
		{10001, "too many states"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			args := &StochasticArgs{
				Algorithm: "mdp",
				Problem:   "test problem",
				MDP: &MDPParams{
					Gamma:  floatPtr(0.9),
					States: intPtr(tc.states),
				},
			}
			err := validateArgs(args)
			if err == nil {
				t.Fatalf("Expected error for states=%d", tc.states)
			}
		})
	}
}

// MCTS Tests
func TestValidateArgs_MCTS_Valid(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "mcts",
		Problem:   "test problem",
		MCTS: &MCTSParams{
			Simulations:         intPtr(1000),
			ExplorationConstant: floatPtr(1.4),
		},
	}
	err := validateArgs(args)
	if err != nil {
		t.Errorf("Expected no error for valid MCTS args, got: %v", err)
	}
}

func TestValidateArgs_MCTS_NilParams(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "mcts",
		Problem:   "test problem",
		MCTS:      nil,
	}
	err := validateArgs(args)
	if err == nil {
		t.Fatal("Expected error for nil MCTS params")
	}
	if !strings.Contains(err.Error(), "mcts parameters are required") {
		t.Errorf("Expected MCTS params required error, got: %v", err)
	}
}

func TestValidateArgs_MCTS_InvalidSimulations(t *testing.T) {
	testCases := []struct {
		simulations int
		description string
	}{
		{0, "zero simulations"},
		{-1, "negative simulations"},
		{1000001, "too many simulations"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			args := &StochasticArgs{
				Algorithm: "mcts",
				Problem:   "test problem",
				MCTS: &MCTSParams{
					Simulations:         intPtr(tc.simulations),
					ExplorationConstant: floatPtr(1.4),
				},
			}
			err := validateArgs(args)
			if err == nil {
				t.Fatalf("Expected error for simulations=%d", tc.simulations)
			}
		})
	}
}

func TestValidateArgs_MCTS_InvalidExplorationConstant(t *testing.T) {
	testCases := []struct {
		constant    float64
		description string
	}{
		{0.0, "zero exploration constant"},
		{-1.0, "negative exploration constant"},
		{10.1, "too high exploration constant"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			args := &StochasticArgs{
				Algorithm: "mcts",
				Problem:   "test problem",
				MCTS: &MCTSParams{
					Simulations:         intPtr(1000),
					ExplorationConstant: floatPtr(tc.constant),
				},
			}
			err := validateArgs(args)
			if err == nil {
				t.Fatalf("Expected error for explorationConstant=%f", tc.constant)
			}
		})
	}
}

// Bandit Tests
func TestValidateArgs_Bandit_Valid(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "bandit",
		Problem:   "test problem",
		Bandit: &BanditParams{
			Strategy: stringPtr("epsilon_greedy"),
			Epsilon:  floatPtr(0.1),
		},
	}
	err := validateArgs(args)
	if err != nil {
		t.Errorf("Expected no error for valid Bandit args, got: %v", err)
	}
}

func TestValidateArgs_Bandit_InvalidStrategy(t *testing.T) {
	testCases := []struct {
		strategy    string
		description string
	}{
		{"invalid_strategy", "unknown strategy"},
		{"", "empty strategy"},
		{" ", "whitespace strategy"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			args := &StochasticArgs{
				Algorithm: "bandit",
				Problem:   "test problem",
				Bandit: &BanditParams{
					Strategy: stringPtr(tc.strategy),
					Epsilon:  floatPtr(0.1),
				},
			}
			err := validateArgs(args)
			if err == nil {
				t.Fatalf("Expected error for strategy='%s'", tc.strategy)
			}
		})
	}
}

func TestValidateArgs_Bandit_ValidStrategies(t *testing.T) {
	strategies := []string{"epsilon_greedy", "ucb", "thompson_sampling"}
	
	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			args := &StochasticArgs{
				Algorithm: "bandit",
				Problem:   "test problem",
				Bandit: &BanditParams{
					Strategy: stringPtr(strategy),
					Epsilon:  floatPtr(0.1),
				},
			}
			err := validateArgs(args)
			if err != nil {
				t.Errorf("Expected no error for valid strategy '%s', got: %v", strategy, err)
			}
		})
	}
}

// Bayesian Tests
func TestValidateArgs_Bayesian_Valid(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "bayesian",
		Problem:   "test problem",
		Bayesian: &BayesianParams{
			AcquisitionFunction: stringPtr("ucb"),
		},
	}
	err := validateArgs(args)
	if err != nil {
		t.Errorf("Expected no error for valid Bayesian args, got: %v", err)
	}
}

func TestValidateArgs_Bayesian_ValidAcquisitionFunctions(t *testing.T) {
	functions := []string{"ucb", "ei", "pi"}
	
	for _, function := range functions {
		t.Run(function, func(t *testing.T) {
			args := &StochasticArgs{
				Algorithm: "bayesian",
				Problem:   "test problem",
				Bayesian: &BayesianParams{
					AcquisitionFunction: stringPtr(function),
				},
			}
			err := validateArgs(args)
			if err != nil {
				t.Errorf("Expected no error for valid acquisition function '%s', got: %v", function, err)
			}
		})
	}
}

// HMM Tests
func TestValidateArgs_HMM_Valid(t *testing.T) {
	args := &StochasticArgs{
		Algorithm: "hmm",
		Problem:   "test problem",
		HMM: &HMMParams{
			Algorithm: stringPtr("viterbi"),
		},
	}
	err := validateArgs(args)
	if err != nil {
		t.Errorf("Expected no error for valid HMM args, got: %v", err)
	}
}

func TestValidateArgs_HMM_ValidAlgorithms(t *testing.T) {
	algorithms := []string{"viterbi", "forward_backward", "baum_welch"}
	
	for _, algorithm := range algorithms {
		t.Run(algorithm, func(t *testing.T) {
			args := &StochasticArgs{
				Algorithm: "hmm",
				Problem:   "test problem",
				HMM: &HMMParams{
					Algorithm: stringPtr(algorithm),
				},
			}
			err := validateArgs(args)
			if err != nil {
				t.Errorf("Expected no error for valid HMM algorithm '%s', got: %v", algorithm, err)
			}
		})
	}
}

// Summary Function Tests
func TestSummaryForAlgorithm_Valid(t *testing.T) {
	testCases := []struct {
		name string
		args StochasticArgs
	}{
		{
			name: "Valid MDP",
			args: StochasticArgs{
				Algorithm: "mdp",
				MDP: &MDPParams{
					Gamma:  floatPtr(0.9),
					States: intPtr(10),
				},
			},
		},
		{
			name: "Valid MCTS",
			args: StochasticArgs{
				Algorithm: "mcts",
				MCTS: &MCTSParams{
					Simulations:         intPtr(1000),
					ExplorationConstant: floatPtr(1.4),
				},
			},
		},
		{
			name: "Valid Bandit",
			args: StochasticArgs{
				Algorithm: "bandit",
				Bandit: &BanditParams{
					Strategy: stringPtr("epsilon_greedy"),
					Epsilon:  floatPtr(0.1),
				},
			},
		},
		{
			name: "Valid Bayesian",
			args: StochasticArgs{
				Algorithm: "bayesian",
				Bayesian: &BayesianParams{
					AcquisitionFunction: stringPtr("ucb"),
				},
			},
		},
		{
			name: "Valid HMM",
			args: StochasticArgs{
				Algorithm: "hmm",
				HMM: &HMMParams{
					Algorithm: stringPtr("viterbi"),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			summary, nextSteps := summaryForAlgorithm(tc.args)
			if summary == "" {
				t.Error("Expected non-empty summary")
			}
			if nextSteps == "" {
				t.Error("Expected non-empty next steps")
			}
		})
	}
}

func TestSummaryForAlgorithm_NilSafety(t *testing.T) {
	testCases := []struct {
		name string
		args StochasticArgs
	}{
		{
			name: "MDP with nil params",
			args: StochasticArgs{Algorithm: "mdp", MDP: nil},
		},
		{
			name: "MCTS with nil params",
			args: StochasticArgs{Algorithm: "mcts", MCTS: nil},
		},
		{
			name: "Bandit with nil params",
			args: StochasticArgs{Algorithm: "bandit", Bandit: nil},
		},
		{
			name: "Bayesian with nil params",
			args: StochasticArgs{Algorithm: "bayesian", Bayesian: nil},
		},
		{
			name: "HMM with nil params",
			args: StochasticArgs{Algorithm: "hmm", HMM: nil},
		},
		{
			name: "Unknown algorithm",
			args: StochasticArgs{Algorithm: "unknown"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This should not panic
			summary, nextSteps := summaryForAlgorithm(tc.args)
			if summary == "" {
				t.Error("Expected non-empty summary for error case")
			}
			if nextSteps == "" {
				t.Error("Expected non-empty next steps for error case")
			}
		})
	}
}

// Benchmark tests
func BenchmarkValidateArgs_MDP(b *testing.B) {
	args := &StochasticArgs{
		Algorithm: "mdp",
		Problem:   "test problem",
		MDP: &MDPParams{
			Gamma:  floatPtr(0.9),
			States: intPtr(10),
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validateArgs(args)
	}
}

func BenchmarkValidateArgs_AllAlgorithms(b *testing.B) {
	testArgs := []*StochasticArgs{
		{
			Algorithm: "mdp",
			Problem:   "test",
			MDP: &MDPParams{
				Gamma:  floatPtr(0.9),
				States: intPtr(10),
			},
		},
		{
			Algorithm: "mcts",
			Problem:   "test",
			MCTS: &MCTSParams{
				Simulations:         intPtr(1000),
				ExplorationConstant: floatPtr(1.4),
			},
		},
		{
			Algorithm: "bandit",
			Problem:   "test",
			Bandit: &BanditParams{
				Strategy: stringPtr("epsilon_greedy"),
				Epsilon:  floatPtr(0.1),
			},
		},
		{
			Algorithm: "bayesian",
			Problem:   "test",
			Bayesian: &BayesianParams{
				AcquisitionFunction: stringPtr("ucb"),
			},
		},
		{
			Algorithm: "hmm",
			Problem:   "test",
			HMM: &HMMParams{
				Algorithm: stringPtr("viterbi"),
			},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, args := range testArgs {
			_ = validateArgs(args)
		}
	}
}