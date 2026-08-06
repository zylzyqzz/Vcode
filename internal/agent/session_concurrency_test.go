package agent

import (
	"sync"
	"testing"

	"vcode/internal/provider"
)

// TestSessionConcurrentAddAndRead models the real hazard: the run loop appends
// messages while a frontend (serve /history, autosave) reads the log from another
// goroutine. Snapshot must copy under the lock; before it, an append racing the
// copy could tear the slice header and crash.
func TestSessionConcurrentAddAndRead(t *testing.T) {
	s := NewSession("sys")
	const (
		messageCount    = 1000
		readerCount     = 4
		readerPollCount = 500
	)

	var wg sync.WaitGroup
	// One writer mimicking the turn goroutine. Keep the workload large enough to
	// exercise lock interleaving, but bounded: Snapshot intentionally copies the
	// history, so 16 readers x 5000 iterations turns this correctness test into
	// an accidental quadratic benchmark on Windows.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < messageCount; i++ {
			s.Add(provider.Message{Role: provider.RoleUser, Content: "msg"})
		}
	}()
	// Several readers mimicking frontends polling history.
	for r := 0; r < readerCount; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < readerPollCount; i++ {
				snap := s.Snapshot()
				for _, m := range snap { // iterate the copy: must never tear
					_ = m.Content
				}
				_ = s.HasContent()
			}
		}()
	}
	wg.Wait()

	if got := len(s.Snapshot()); got != messageCount+1 { // + the system prompt
		t.Fatalf("final message count = %d, want %d", got, messageCount+1)
	}
}
