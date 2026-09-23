// Package reconcile guarantees that every live deploy's build artifacts exist —
// and verify — in the durable tier. It is the correctness gate a serve-only
// worker depends on: such a node can hydrate an artifact but cannot rebuild
// one, so no live deploy's artifact may be missing from the bucket before a
// second node exists.
//
// Gaps it closes: artifacts built before the tier was enabled, cache-hit builds
// that skipped both the build and the persist, and hard crashes that dropped an
// in-flight upload (the shutdown drain only covers clean exits). It verifies
// integrity, not mere existence — a StatObject existence check would not catch a
// truncated object, and the tier's skip-if-exists means a bad object is never
// repaired on its own. The uncompressed-size and file-count metadata the tier
// records at Save time is exactly what makes that check possible.
package reconcile

import (
	"context"
	"fmt"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/s3store"
	"github.com/jmelahman/local-preview/internal/store"
)

// Tier is the durable-tier surface the pass needs beyond store's hydrate path:
// integrity metadata (Stat), upload (Save), and repair (Delete). Satisfied by
// *s3store.Tier.
type Tier interface {
	Stat(ctx context.Context, repo, side, hash string) (s3store.ObjectInfo, bool, error)
	Save(ctx context.Context, repo, side, hash, srcDir string) error
	Delete(ctx context.Context, repo, side, hash string) error
}

// Report summarizes one pass.
type Report struct {
	// Checked is the number of distinct content-addresses examined.
	Checked int `json:"checked"`
	// AlreadyOK were present and verified in the tier.
	AlreadyOK int `json:"already_ok"`
	// Persisted were absent from the tier but resident locally, so re-uploaded.
	Persisted int `json:"persisted"`
	// Repaired were present but failed the integrity check, and were deleted
	// then re-uploaded from a resident local copy.
	Repaired int `json:"repaired"`
	// Gaps were absent-or-bad in the tier AND not resident locally — nothing
	// this pass can do; only a rebuild recovers them. This is the number that
	// must be zero before a serve-only worker is safe.
	Gaps int `json:"gaps"`
	// GapKeys names the gaps ("repo/side/hash") for logging.
	GapKeys []string `json:"gap_keys,omitempty"`
	// Errors is the count of sides that could not be checked or fixed due to a
	// transient tier error; the next pass retries them.
	Errors int `json:"errors"`
}

// sideRef is one artifact side of one deploy.
type sideRef struct {
	repo, side, hash string
}

func (r sideRef) key() string { return r.repo + "/" + r.side + "/" + r.hash }

// Reconciler runs the pass over the live deploy set.
type Reconciler struct {
	db    *db.Store
	files *store.Store
	tier  Tier
}

// New wires a reconciler. tier must be non-nil (the pass is meaningless
// without a durable tier).
func New(database *db.Store, files *store.Store, tier Tier) *Reconciler {
	return &Reconciler{db: database, files: files, tier: tier}
}

// Reconcile runs one full pass over every live deploy's artifacts.
func (r *Reconciler) Reconcile(ctx context.Context) (Report, error) {
	refs, err := r.collectLive()
	if err != nil {
		return Report{}, err
	}
	return r.reconcile(ctx, refs), nil
}

// collectLive gathers the deduplicated artifact sides of every ready deploy.
// Evicted deploys are history (their artifacts are reclaimable by design);
// unbuilt deploys have no artifacts yet.
func (r *Reconciler) collectLive() ([]sideRef, error) {
	rows, err := r.db.ListDeploys(db.DeployFilter{})
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var out []sideRef
	add := func(repo, side, hash string) {
		if hash == "" {
			return
		}
		ref := sideRef{repo, side, hash}
		if _, ok := seen[ref.key()]; ok {
			return
		}
		seen[ref.key()] = struct{}{}
		out = append(out, ref)
	}
	for _, row := range rows {
		if row.Status != db.DeployReady {
			continue
		}
		add(row.RepoName, "fe", row.FeHash)
		add(row.RepoName, "be", row.BeHash)
		for _, art := range row.Artifacts {
			add(row.RepoName, "dl", art.Hash)
		}
	}
	return out, nil
}

// reconcile is the pure pass over an explicit set of sides — the db-free core,
// so it is testable with a real store and a fake tier.
func (r *Reconciler) reconcile(ctx context.Context, refs []sideRef) Report {
	rep := Report{Checked: len(refs)}
	for _, ref := range refs {
		if ctx.Err() != nil {
			break
		}
		info, found, err := r.tier.Stat(ctx, ref.repo, ref.side, ref.hash)
		if err != nil {
			rep.Errors++
			continue
		}
		dir, _, files, resident := r.files.ResidentSide(ref.repo, ref.side, ref.hash)

		if found && r.consistent(info, files, resident) {
			rep.AlreadyOK++
			continue
		}

		// The object is missing or fails its integrity check. Only a resident
		// local copy lets us fix it; otherwise it is an unrecoverable gap.
		if !resident {
			rep.Gaps++
			rep.GapKeys = append(rep.GapKeys, ref.key())
			continue
		}
		if found {
			// Present but bad: skip-if-exists means Save alone won't replace it,
			// so drop it first.
			if err := r.tier.Delete(ctx, ref.repo, ref.side, ref.hash); err != nil {
				rep.Errors++
				continue
			}
		}
		if err := r.tier.Save(ctx, ref.repo, ref.side, ref.hash, dir); err != nil {
			rep.Errors++
			continue
		}
		if found {
			rep.Repaired++
		} else {
			rep.Persisted++
		}
	}
	return rep
}

// consistent reports whether a stored object's recorded metadata passes the
// integrity check. The metadata must be non-degenerate (a size and a file
// count were recorded); when a local copy is resident, the recorded file count
// must also match it — the cheap signal that catches a truncated object without
// downloading it.
func (r *Reconciler) consistent(info s3store.ObjectInfo, localFiles int, resident bool) bool {
	if info.UncompressedSize <= 0 || info.FileCount <= 0 {
		return false
	}
	if resident && info.FileCount != int64(localFiles) {
		return false
	}
	return true
}

// String renders a one-line summary for logs.
func (rep Report) String() string {
	return fmt.Sprintf("checked=%d ok=%d persisted=%d repaired=%d gaps=%d errors=%d",
		rep.Checked, rep.AlreadyOK, rep.Persisted, rep.Repaired, rep.Gaps, rep.Errors)
}
