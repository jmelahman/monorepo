package build

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/fleet"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// TestStartResumesWithoutBlocking: a backlog larger than the work buffer must
// not wedge Start — the caller goes on to serve HTTP, and a server that never
// listens can't be told to cancel the backlog either.
func TestStartResumesWithoutBlocking(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	repo, err := database.CreateRepo("demo", "/src", "/bare", db.RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	files := store.New(
		filepath.Join(root, "artifacts"),
		filepath.Join(root, "state"),
		filepath.Join(root, "tmp"),
	)
	super := supervise.New(database, files, filepath.Join(root, "logs"))
	t.Cleanup(super.StopAll)
	q := NewQueue(database, gitrepo.NewManager(filepath.Join(root, "repos")), files, super, filepath.Join(root, "logs"), nil)

	for i := range cap(q.work) + 1 {
		if _, err := database.CreateDeploy(repo.ID, fmt.Sprintf("%040x", i), db.DeployMeta{}); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan struct{})
	// No workers: nothing drains the channel, so a blocking resume never
	// returns.
	go func() {
		q.Start(ctx, 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Start blocked on a backlog larger than the work buffer")
	}
}

// startWithRetry is what makes scale-from-zero work for pushes: the failed
// pre-warm registered the demand that launches a worker, so it must retry
// ErrNoWorker until that worker registers — and give up immediately on any
// other error (a worker existed; the start itself is broken).
func TestStartWithRetry(t *testing.T) {
	ctx := context.Background()

	// ErrNoWorker (wrapped, as StartDeploy wraps it) retries until success.
	calls := 0
	err := startWithRetry(ctx, 5, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("start backend: %w", fleet.ErrNoWorker)
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("retry until worker: err=%v calls=%d, want nil after 3", err, calls)
	}

	// Any other failure is terminal on the first attempt.
	calls = 0
	boom := fmt.Errorf("start backend: exec format error")
	if err := startWithRetry(ctx, 5, time.Millisecond, func() error { calls++; return boom }); err != boom || calls != 1 {
		t.Fatalf("non-ErrNoWorker: err=%v calls=%d, want boom after 1", err, calls)
	}

	// A fleet that never gets a worker exhausts attempts and reports it.
	calls = 0
	err = startWithRetry(ctx, 3, time.Millisecond, func() error {
		calls++
		return fleet.ErrNoWorker
	})
	if !errors.Is(err, fleet.ErrNoWorker) || calls != 3 {
		t.Fatalf("exhausted: err=%v calls=%d, want ErrNoWorker after 3", err, calls)
	}

	// Cancellation stops the loop between attempts.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	calls = 0
	err = startWithRetry(cctx, 10, time.Hour, func() error { calls++; return fleet.ErrNoWorker })
	if !errors.Is(err, fleet.ErrNoWorker) || calls != 1 {
		t.Fatalf("cancelled: err=%v calls=%d, want ErrNoWorker after 1", err, calls)
	}
}
