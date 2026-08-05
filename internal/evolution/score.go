package evolution

import "fmt"

func CalculateScore(completion, verification, quality, efficiency float64, hardGate bool) (Score, error) {
	values := []float64{completion, verification, quality, efficiency}
	for _, value := range values {
		if value < 0 || value > 100 {
			return Score{}, fmt.Errorf("score component must be between 0 and 100")
		}
	}
	s := Score{Completion: completion, Verify: verification, Quality: quality, Efficiency: efficiency, HardGate: hardGate}
	s.Total = completion*0.4 + verification*0.3 + quality*0.2 + efficiency*0.1
	if !hardGate {
		s.Reason = "hard gate failed"
	}
	return s, nil
}

func AcceptCandidate(reference, candidate Score) bool {
	return candidate.HardGate && candidate.Total > reference.Total
}
