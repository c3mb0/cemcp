package stochastic

import "testing"

func TestRunMissingParams(t *testing.T) {
	_, err := Run(Request{Algorithm: AlgorithmMonteCarlo})
	if err == nil {
		t.Fatalf("expected error for missing params")
	}
}

func TestRunSuccess(t *testing.T) {
	resp, err := Run(Request{
		Algorithm:  AlgorithmMonteCarlo,
		MonteCarlo: &MonteCarloParams{Iterations: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.NextSteps == "" {
		t.Fatalf("expected next steps")
	}
}
