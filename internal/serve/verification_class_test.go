package serve

import (
	"testing"

	"vcode/internal/verify"
)

func TestVerificationErrorClassDistinguishesUnavailableChecks(t *testing.T) {
	if got := verificationErrorClass(verify.Result{
		Status:  verify.Unverified,
		Skipped: "no supported project verification command was found",
	}); got != "verification_unavailable" {
		t.Fatalf("unavailable verifier class = %q, want verification_unavailable", got)
	}
	if got := verificationErrorClass(verify.Result{
		Status: verify.Partial,
		Failed: []string{"go test ./...: failed"},
	}); got != "verification_failed" {
		t.Fatalf("failed check class = %q, want verification_failed", got)
	}
}
