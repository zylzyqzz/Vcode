package runtime

import "strings"

type CompletionInput struct {
	FinalResponse      string
	ChangedFiles       []string
	VerificationStatus string
	VerificationFresh  bool
	UnresolvedFailures int
	BoundaryViolations int
	DiffMatchesGoal    bool
	EvidenceRecorded   bool
	WritesCompleted    bool
}

type CompletionDecision struct {
	Allowed bool     `json:"allowed"`
	Outcome string   `json:"outcome"`
	Reasons []string `json:"reasons,omitempty"`
}

// EvaluateCompletion is deliberately strict. It is called by the host after
// the final tool and verification events; model text cannot bypass it.
func EvaluateCompletion(in CompletionInput) CompletionDecision {
	var reasons []string
	if strings.TrimSpace(in.FinalResponse) == "" {
		reasons = append(reasons, "missing final agent response")
	}
	if len(in.ChangedFiles) == 0 {
		reasons = append(reasons, "no files changed")
	}
	if !in.WritesCompleted {
		reasons = append(reasons, "writes are not confirmed complete")
	}
	if !strings.EqualFold(strings.TrimSpace(in.VerificationStatus), "VERIFIED") {
		reasons = append(reasons, "verification did not pass")
	}
	if !in.VerificationFresh {
		reasons = append(reasons, "verification is stale or missing after the last write")
	}
	if in.UnresolvedFailures > 0 {
		reasons = append(reasons, "unresolved tool failures remain")
	}
	if in.BoundaryViolations > 0 {
		reasons = append(reasons, "workspace boundary violations detected")
	}
	if !in.DiffMatchesGoal {
		reasons = append(reasons, "diff does not match the task goal")
	}
	if !in.EvidenceRecorded {
		reasons = append(reasons, "completion evidence was not recorded")
	}
	if len(reasons) == 0 {
		return CompletionDecision{Allowed: true, Outcome: "VERIFIED"}
	}
	outcome := "UNVERIFIED"
	if strings.EqualFold(strings.TrimSpace(in.VerificationStatus), "PARTIAL") || len(in.ChangedFiles) > 0 {
		outcome = "PARTIAL"
	}
	return CompletionDecision{Allowed: false, Outcome: outcome, Reasons: reasons}
}
