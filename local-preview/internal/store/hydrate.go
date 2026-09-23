package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// hydrateTimeout bounds a single durable-tier fetch+extract. It is generous
// because a backend artifact can be ~1.5 GB (an extracted uv venv), but it must
// exist: the fetch runs detached from any one caller's deadline, so without its
// own bound a wedged download would hang the shared singleflight forever.
const hydrateTimeout = 15 * time.Minute

// ErrNotInTier is returned by Hydrate when the durable tier has no object for
// the requested (repo, side, hash). It is distinct from "no tier configured"
// so a serve path can tell a reconcile gap (live deploy, artifact never
// persisted) from a plain single-node instance with no tier at all.
var ErrNotInTier = errors.New("artifact not present in durable tier")

// ArtifactTier is the durable, content-addressed artifact store the local
// on-disk cache is hydrated from and persisted to. Implemented by
// *s3store.Tier; an interface so tests can substitute a fake and so store
// stays free of any cloud dependency. side is "fe", "be", or "dl".
//
// It lives in store — not build — because the serving path
// (proxy → supervise.Manager) never passes through build yet must be able to
// bring an evicted artifact back on demand. build persists through the same
// interface.
type ArtifactTier interface {
	// Save uploads srcDir's contents under the (repo, side, hash) key,
	// idempotently.
	Save(ctx context.Context, repo, side, hash, srcDir string) error
	// Open returns a reader over the artifact's decompressed tar bytes, or
	// found=false when absent. want is the regular-file content byte count
	// recorded at Save time (0 when the object predates the metadata): the
	// caller compares it against what extraction actually wrote — the two
	// sides of the integrity check counting the same quantity. Close reports
	// transport/decompression errors.
	Open(ctx context.Context, repo, side, hash string) (rc io.ReadCloser, want int64, found bool, err error)
}

// SetArtifactTier attaches the durable tier. nil (the default) leaves local
// disk authoritative — Hydrate then only ever reports "already resident" or
// "no tier". Set once at startup, before any serve or build.
func (s *Store) SetArtifactTier(t ArtifactTier) { s.tier = t }

// ArtifactTier returns the configured durable tier, or nil. Callers that
// persist (build) or report durable usage read it here rather than holding
// their own copy, so there is a single source of truth.
func (s *Store) ArtifactTier() ArtifactTier { return s.tier }

// Hydrate makes an artifact side resident on local disk by fetching it from
// the durable tier and publishing it through the same atomic rename a build
// uses. It is the inverse of eviction: local disk is a cache, and this refills
// it.
//
// Returns nil when the side is already resident or was successfully hydrated.
// Returns an error otherwise: no tier configured, the tier lacks the object
// (ErrNotInTier), or extraction/verification/publish failed. On the build path
// the caller ignores the error and rebuilds; on the serve path it distinguishes
// "not resident, hydrate and serve" from "genuinely gone".
//
// Concurrent hydrates of the same content-address are deduplicated with
// singleflight, so two simultaneous serve requests fetch the object once.
func (s *Store) Hydrate(ctx context.Context, repo, side, hash string) error {
	if hash == "" {
		return fmt.Errorf("hydrate %s/%s: empty hash", repo, side)
	}
	if s.hasSide(repo, side, hash) {
		return nil
	}
	if s.tier == nil {
		return fmt.Errorf("hydrate %s %s/%s: no durable tier configured", repo, side, hash)
	}
	key := repo + ":hydrate:" + side + ":" + hash
	// DoChan, not Do: a caller that gives up (a proxy request whose short
	// deadline elapses) must be able to bail and render a "loading" page while
	// the shared fetch keeps running for the next request — a joined waiter's
	// cancellation must never abort the winner's download. The fetch therefore
	// runs on a context detached from any one caller's cancellation, with its
	// own generous timeout so it is still bounded.
	ch := s.sf.DoChan(key, func() (any, error) {
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), hydrateTimeout)
		defer cancel()
		// A sibling may have landed it while we waited on the singleflight.
		if s.hasSide(repo, side, hash) {
			return nil, nil
		}
		rc, want, found, err := s.tier.Open(fctx, repo, side, hash)
		if err != nil {
			return nil, fmt.Errorf("hydrate %s %s/%s: open: %w", repo, side, hash, err)
		}
		if !found {
			return nil, fmt.Errorf("hydrate %s %s/%s: %w", repo, side, hash, ErrNotInTier)
		}
		dir, cleanup, err := s.NewScratchDir("hydrate")
		if err != nil {
			rc.Close()
			return nil, fmt.Errorf("hydrate %s %s/%s: scratch: %w", repo, side, hash, err)
		}
		defer cleanup()
		// Backend trees are executed payloads whose symlinks may legitimately
		// point outside the tree (resolved inside their run container); every
		// other side keeps the strict policy. See ExtractTarPayload.
		var extracted int64
		var extractErr error
		if side == "be" {
			extracted, extractErr = s.ExtractTarPayload(rc, dir)
		} else {
			extracted, extractErr = s.ExtractTar(rc, dir)
		}
		closeErr := rc.Close()
		if extractErr != nil {
			return nil, fmt.Errorf("hydrate %s %s/%s: extract: %w", repo, side, hash, extractErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("hydrate %s %s/%s: close: %w", repo, side, hash, closeErr)
		}
		// Integrity: the content bytes extraction wrote must equal what Save
		// recorded — a truncated object must never publish under a
		// content-addressed key, where skip-if-exists would make it permanent.
		if want > 0 && extracted != want {
			return nil, fmt.Errorf("hydrate %s %s/%s: integrity check failed: extracted %d content bytes, expected %d", repo, side, hash, extracted, want)
		}
		if err := s.publishSide(repo, side, hash, dir); err != nil {
			return nil, fmt.Errorf("hydrate %s %s/%s: publish: %w", repo, side, hash, err)
		}
		s.NoteAccess(repo, side, hash)
		return nil, nil
	})
	select {
	case res := <-ch:
		return res.Err
	case <-ctx.Done():
		// Caller gave up; the fetch continues in the background for the next one.
		return ctx.Err()
	}
}

// hasSide reports whether an artifact side is resident on local disk.
func (s *Store) hasSide(repo, side, hash string) bool {
	switch side {
	case "fe":
		return s.HasFrontend(repo, hash)
	case "be":
		return s.HasBackend(repo, hash)
	case "dl":
		return s.HasArtifact(repo, hash)
	}
	return false
}

// publishSide lands a hydrated side's extracted tree via the same atomic
// publish a build uses. overwrite is false: hydrate only fills a missing
// artifact, never displaces a resident one.
func (s *Store) publishSide(repo, side, hash, srcDir string) error {
	switch side {
	case "fe":
		return s.PublishFrontend(repo, hash, srcDir, false)
	case "be":
		return s.PublishBackend(repo, hash, srcDir, false)
	case "dl":
		return s.PublishArtifactDir(repo, hash, srcDir, false)
	}
	return fmt.Errorf("hydrate: unknown side %q", side)
}
