package sequential

import (
	"context"
	"testing"

	"github.com/c3mb0/cemcp/pkg/stochastic"
)

func TestRegisterSequentialThinkingAttachesSummary(t *testing.T) {
	stochastic.ClearSummaries()
	summary, _ := stochastic.StochasticAlgorithm("abc")
	sc := registerSequentialThinking(context.Background(), "abc")
	if sc.Summary == nil {
		t.Fatalf("expected summary attached")
	}
	if *sc.Summary != summary {
		t.Fatalf("unexpected summary: got %#v want %#v", *sc.Summary, summary)
	}
}

func TestRegisterSequentialThinkingNoSummary(t *testing.T) {
	stochastic.ClearSummaries()
	sc := registerSequentialThinking(context.Background(), "missing")
	if sc.Summary != nil {
		t.Fatalf("expected nil summary")
	}
}
