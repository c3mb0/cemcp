package sequential

import (
	"context"

	"github.com/c3mb0/cemcp/pkg/stochastic"
)

// SessionContext wraps a base context and optionally includes a stochastic summary.
type SessionContext struct {
	context.Context
	Summary *stochastic.StochasticSummary
}

// registerSequentialThinking attaches a stored stochastic summary, if any, to the session context.
func registerSequentialThinking(ctx context.Context, sessionID string) *SessionContext {
	sc := &SessionContext{Context: ctx}
	if summary, ok := stochastic.GetSummary(sessionID); ok {
		sc.Summary = &summary
	}
	return sc
}
