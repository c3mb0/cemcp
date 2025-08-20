package stochastic

import "fmt"

type Algorithm string

const (
	AlgorithmMonteCarlo Algorithm = "monte_carlo"
	AlgorithmRandomWalk Algorithm = "random_walk"
)

type MonteCarloParams struct {
	Iterations int `json:"iterations"`
}

type RandomWalkParams struct {
	Steps int `json:"steps"`
}

type Request struct {
	Algorithm  Algorithm         `json:"algorithm"`
	MonteCarlo *MonteCarloParams `json:"monteCarlo,omitempty"`
	RandomWalk *RandomWalkParams `json:"randomWalk,omitempty"`
}

type Response struct {
	Summary   string `json:"summary"`
	NextSteps string `json:"nextSteps"`
}

func Run(req Request) (Response, error) {
	switch req.Algorithm {
	case AlgorithmMonteCarlo:
		if req.MonteCarlo == nil {
			return Response{}, fmt.Errorf("missing parameters for %s", req.Algorithm)
		}
		summary, err := summaryForAlgorithm(req.Algorithm, *req.MonteCarlo)
		if err != nil {
			return Response{}, err
		}
		return Response{Summary: summary, NextSteps: "Consider adjusting iterations or trying another algorithm."}, nil
	case AlgorithmRandomWalk:
		if req.RandomWalk == nil {
			return Response{}, fmt.Errorf("missing parameters for %s", req.Algorithm)
		}
		summary, err := summaryForAlgorithm(req.Algorithm, *req.RandomWalk)
		if err != nil {
			return Response{}, err
		}
		return Response{Summary: summary, NextSteps: "Try exploring a different path or modifying step count."}, nil
	default:
		return Response{}, fmt.Errorf("unsupported algorithm %s", req.Algorithm)
	}
}

func summaryForAlgorithm(algo Algorithm, params any) (string, error) {
	return fmt.Sprintf("%s algorithm executed", algo), nil
}
