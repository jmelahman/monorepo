package build

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jmelahman/local-preview/internal/s3store"
	"github.com/jmelahman/local-preview/internal/store"
)

const (
	// persistWorkers upload artifacts to the durable tier concurrently.
	persistWorkers = 4
	// persistQueueSize bounds buffered uploads; a full queue drops the oldest
	// enqueue (best-effort — a reconcile pass is the correctness net).
	persistQueueSize = 256
	// persistTimeout bounds a single artifact upload.
	persistTimeout = 10 * time.Minute
)

// persistJob names a published artifact side to upload. The source directory
// is resolved from the store at upload time (the stable published path), not
// carried here.
type persistJob struct {
	repo string
	side string // "fe", "be", "dl"
	hash string
}

// SetArtifactTier attaches the durable artifact tier. nil (the default)
// disables persist and hydrate. Call before Start. The tier is owned by the
// store — the serving path hydrates through it without passing through build —
// so this also hands it to the store; the queue keeps a copy only for the
// persist pool's hot-path nil checks.
func (q *Queue) SetArtifactTier(t store.ArtifactTier) {
	q.tier = t
	q.files.SetArtifactTier(t)
}

// startPersist launches the upload worker pool when a tier is configured.
func (q *Queue) startPersist() {
	if q.tier == nil {
		return
	}
	q.persistJobs = make(chan persistJob, persistQueueSize)
	q.persistQuit = make(chan struct{})
	for range persistWorkers {
		q.persistWG.Add(1)
		go q.persistWorker()
	}
}

// Stop drains in-flight and buffered artifact uploads, then returns. Call it
// during graceful shutdown, after the build workers' context is cancelled so
// no new uploads are being enqueued. Safe to call when no tier is configured.
func (q *Queue) Stop() {
	if q.persistQuit == nil {
		return
	}
	close(q.persistQuit)
	q.persistWG.Wait()
}

func (q *Queue) persistWorker() {
	defer q.persistWG.Done()
	for {
		select {
		case job := <-q.persistJobs:
			q.runPersist(job)
		case <-q.persistQuit:
			// Drain what's buffered so a normal shutdown doesn't drop a
			// just-built artifact, then exit.
			for {
				select {
				case job := <-q.persistJobs:
					q.runPersist(job)
				default:
					return
				}
			}
		}
	}
}

// enqueuePersist schedules an upload of a published artifact side. No-op when
// no tier is configured. Non-blocking: a full queue logs and drops (the upload
// will be recovered by a rebuild, or a future reconcile pass).
func (q *Queue) enqueuePersist(repo, side, hash string) {
	if q.tier == nil || q.persistJobs == nil || hash == "" {
		return
	}
	select {
	case q.persistJobs <- persistJob{repo: repo, side: side, hash: hash}:
	default:
		log.Printf("build: persist queue full, dropping %s %s/%s", repo, side, hash)
	}
}

// runPersist is the async pool's wrapper: it uploads one side and logs a
// failure (the enqueue is best-effort — reconcile is the correctness net).
func (q *Queue) runPersist(job persistJob) {
	if err := q.persistSide(job.repo, job.side, job.hash); err != nil {
		log.Printf("build: persist %s %s/%s: %v", job.repo, job.side, job.hash, err)
	}
}

// persistSide uploads one artifact side to the durable tier synchronously and
// returns the outcome, so a caller that must not proceed until the side is
// durable (see ensureDurable) can wait and act on failure. It uses a fresh
// timeout context, not the build context, so a build finishing (or the server
// draining) doesn't abort an in-flight upload mid-stream. A source directory
// that GC deleted out from under the upload is a benign nil — nothing to
// persist. Save is skip-if-exists, so a side the async pool already uploaded
// costs one HeadObject.
func (q *Queue) persistSide(repo, side, hash string) error {
	if q.tier == nil || hash == "" {
		return nil
	}
	dir := q.artifactDir(repo, side, hash)
	if dir == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
	defer cancel()
	if err := q.tier.Save(ctx, repo, side, hash, dir); err != nil {
		if errors.Is(err, s3store.ErrSourceGone) {
			return nil // retention won the race; nothing to persist
		}
		return err
	}
	return nil
}

// ensureDurable blocks until a deploy's frontend and backend are in the
// durable tier, so "ready" can mean servable by any node. A worker hydrates
// only from the tier, so auto-start (and an on-demand serve, which also gates
// on ready) would otherwise race the async persist and fail with "not present
// in durable tier" until the upload happened to finish. No-op without a tier:
// on a single node local disk is the only tier and the serving node already
// holds the files.
func (q *Queue) ensureDurable(repo, feHash, beHash string) error {
	if q.tier == nil {
		return nil
	}
	for _, s := range []struct{ side, hash string }{{"fe", feHash}, {"be", beHash}} {
		if err := q.persistSide(repo, s.side, s.hash); err != nil {
			return fmt.Errorf("%s: %w", s.side, err)
		}
	}
	return nil
}

// hydrate makes an artifact side present locally by fetching it from the
// durable tier, so a build can be skipped. No-op when no tier is configured.
// Best-effort on the build path: a failure (including the tier simply not
// having the object) falls through to a rebuild, so only genuine errors —
// not a plain miss — are logged. The heavy lifting, singleflighted against the
// same fetch the serving path uses, lives in the store.
func (q *Queue) hydrate(ctx context.Context, repo, side, hash string) {
	if q.tier == nil {
		return
	}
	if err := q.files.Hydrate(ctx, repo, side, hash); err != nil && !errors.Is(err, store.ErrNotInTier) {
		log.Printf("build: %v", err)
	}
}

func (q *Queue) artifactDir(repo, side, hash string) string {
	switch side {
	case "fe":
		return q.files.FrontendDir(repo, hash)
	case "be":
		return q.files.BackendDir(repo, hash)
	case "dl":
		return q.files.ArtifactDir(repo, hash)
	}
	return ""
}
