package taskgraph

import (
	"context"
	"strings"
	"testing"
)

func TestWorkspaceLockBlocksUntilContextCancellation(t *testing.T) {
	root := t.TempDir()
	release, err := acquireWorkspaceLock(context.Background(), root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = acquireWorkspaceLock(ctx, root, true)
	if err == nil || !strings.Contains(err.Error(), "workspace lock") {
		t.Fatalf("err=%v, want cancellation while waiting for lock", err)
	}
}
