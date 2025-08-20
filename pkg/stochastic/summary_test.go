package stochastic

import "testing"

func TestStochasticAlgorithmStoresSummary(t *testing.T) {
	ClearSummaries()
	summary, err := StochasticAlgorithm("session1")
	if err != nil {
		t.Fatalf("algorithm error: %v", err)
	}
	got, ok := GetSummary("session1")
	if !ok {
		t.Fatalf("summary not stored")
	}
	if got != summary {
		t.Fatalf("stored summary mismatch: got %#v want %#v", got, summary)
	}
}

func TestClearSummaries(t *testing.T) {
	ClearSummaries()
	_, _ = StochasticAlgorithm("s")
	ClearSummaries()
	if _, ok := GetSummary("s"); ok {
		t.Fatalf("expected store to be empty")
	}
}
