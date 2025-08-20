package stochastic

import (
	"sync"
	"time"
)

// StochasticSummary represents the outcome of a stochastic algorithm run.
type StochasticSummary struct {
	// Value holds the final value produced by the algorithm.
	Value float64
	// Iterations captures how many iterations were executed.
	Iterations int
	// GeneratedAt records when the summary was created.
	GeneratedAt time.Time
}

var (
	mu        sync.Mutex
	summaries = make(map[string]StochasticSummary)
)

// SaveSummary stores a summary for the given session ID.
func SaveSummary(sessionID string, summary StochasticSummary) {
	mu.Lock()
	defer mu.Unlock()
	summaries[sessionID] = summary
}

// GetSummary retrieves the summary for the session ID, if present.
func GetSummary(sessionID string) (StochasticSummary, bool) {
	mu.Lock()
	defer mu.Unlock()
	s, ok := summaries[sessionID]
	return s, ok
}

// ClearSummaries removes all stored summaries.
func ClearSummaries() {
	mu.Lock()
	defer mu.Unlock()
	summaries = make(map[string]StochasticSummary)
}

// StochasticAlgorithm performs a simple stochastic computation and stores its summary.
func StochasticAlgorithm(sessionID string) (StochasticSummary, error) {
	summary := StochasticSummary{
		Value:       0.42,
		Iterations:  1,
		GeneratedAt: time.Now(),
	}
	SaveSummary(sessionID, summary)
	return summary, nil
}
