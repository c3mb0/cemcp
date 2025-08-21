package stochastic

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// StochasticSummary captures the outcome of a stochastic algorithm run.
type StochasticSummary struct {
	Algorithm string `json:"algorithm"`
	Summary   string `json:"summary"`
	NextSteps string `json:"nextSteps"`
}

func filePath(sessionID string) string {
	return filepath.Join(os.TempDir(), "stochastic_"+sessionID+".json")
}

// WriteSummary persists a summary for the given session ID.
func WriteSummary(sessionID string, s StochasticSummary) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath(sessionID), b, 0o644)
}

// ReadSummary retrieves a summary for the given session ID.
func ReadSummary(sessionID string) (*StochasticSummary, error) {
	b, err := os.ReadFile(filePath(sessionID))
	if err != nil {
		return nil, err
	}
	var s StochasticSummary
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
