// Package watch polls watched repos' mirror clones and deploys new branch
// tips — the trigger for repos that push somewhere the server can fetch
// from but can't run a post-commit hook or deliver a webhook. It is a thin
// adapter over the build queue: fetch, list branch heads, request a deploy
// for any matching tip that has none.
package watch

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/local-preview/internal/build"
	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
)

// DefaultInterval is how often watched repos are polled unless configured
// otherwise.
const DefaultInterval = time.Minute

// Watcher periodically fetches every watched repo and deploys branch tips
// that have no deploy yet.
type Watcher struct {
	db       *db.Store
	git      *gitrepo.Manager
	queue    *build.Queue
	interval time.Duration
	kick     chan struct{}

	// mu serializes PollAll; the poll loop is its only production caller,
	// but Kick-adjacent callers and tests may invoke it directly.
	mu sync.Mutex
	// lastPass caches each repo's state as of its last completed
	// reconcile. A poll whose fetch moved no refs reuses it to skip the
	// go-git graph walks (see pollRepo) — those walks, repeated every
	// interval against unchanged mirrors, otherwise dominate the server's
	// idle memory and CPU.
	lastPass map[int64]passState
}

// passState is what a quiet poll needs to make the same decisions the last
// full pass made: the branch tips (identical while refs are unchanged) and
// the newest deploy row id seen (eviction inputs can only change without a
// ref change when a deploy row is created).
type passState struct {
	tips        []gitrepo.BranchTip
	maxDeployID int64
}

// New returns a Watcher polling at interval. A non-positive interval
// disables it (Start and Kick become no-ops).
func New(database *db.Store, git *gitrepo.Manager, queue *build.Queue, interval time.Duration) *Watcher {
	return &Watcher{
		db:       database,
		git:      git,
		queue:    queue,
		interval: interval,
		kick:     make(chan struct{}, 1),
		lastPass: map[int64]passState{},
	}
}

// Start launches the poll loop in a goroutine; it polls once immediately,
// then on every interval tick (or sooner after a Kick).
func (w *Watcher) Start(ctx context.Context) {
	if w.interval <= 0 {
		return
	}
	go func() {
		w.PollAll(ctx)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-w.kick:
			}
			w.PollAll(ctx)
		}
	}()
}

// Kick requests an out-of-band poll — called after watch settings change so
// a newly watched repo doesn't wait a full interval. Never blocks.
func (w *Watcher) Kick() {
	if w == nil || w.interval <= 0 {
		return
	}
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// PollAll polls every watched repo once. Per-repo failures are logged, not
// returned: one unreachable origin must not stall the others.
func (w *Watcher) PollAll(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	repos, err := w.db.ListRepos()
	if err != nil {
		log.Printf("watch: list repos: %v", err)
		return
	}
	polled := make(map[int64]bool, len(repos))
	for _, repo := range repos {
		// Repos still cloning (or whose clone failed) have no mirror to
		// fetch; the cloner kicks a poll when one turns ready.
		if !repo.Watch || repo.Status != db.RepoReady || ctx.Err() != nil {
			continue
		}
		polled[repo.ID] = true
		if err := w.pollRepo(ctx, repo); err != nil && ctx.Err() == nil {
			log.Printf("watch: %s: %v", repo.Name, err)
		}
	}
	// Unwatched or deleted repos start over with a full pass if they return.
	for id := range w.lastPass {
		if !polled[id] {
			delete(w.lastPass, id)
		}
	}
}

// pollRepo fetches one repo, requests a deploy for every matching branch tip
// that doesn't have one, then evicts deploys whose commit fell out of the
// repo's branch history. Tips are deployed by sha, so RequestDeploy resolves
// them without a second fetch.
//
// A fetch that moved no refs leaves the tips and reachability exactly as
// the last completed pass saw them, so the poll reconciles from the cached
// tips with database checks alone and walks the graph for eviction only if
// a deploy row appeared in the meantime — the decisions are identical,
// without re-reading an unchanged repo every interval.
func (w *Watcher) pollRepo(ctx context.Context, repo db.Repo) error {
	gr := w.git.Open(repo.Name)
	changed, err := gr.Fetch(ctx)
	if err != nil {
		return err
	}
	if prev, ok := w.lastPass[repo.ID]; ok && !changed {
		maxID, err := w.db.LatestDeployID(repo.ID)
		if err != nil {
			return err
		}
		return w.reconcile(ctx, repo, prev.tips, maxID > prev.maxDeployID)
	}
	tips, err := gr.BranchTips(ctx)
	if err != nil {
		return err
	}
	return w.reconcile(ctx, repo, tips, true)
}

// reconcile requests deploys for matching tips that lack one and, when
// evict is set, evicts deploys unreachable from the tips. On success the
// pass state is cached for quiet polls.
//
// A repo that has just started being watched is baselined instead: its
// current tips are recorded and none of them deploy. "Has no deploy row" is
// otherwise true of every branch a repo has ever had, so the first poll
// would deploy the entire branch list at once.
func (w *Watcher) reconcile(ctx context.Context, repo db.Repo, tips []gitrepo.BranchTip, evict bool) error {
	if !repo.WatchBaselined {
		// Every branch, not just the matched ones: widening the globs later
		// must not deploy tips that were already there.
		shas := make([]string, len(tips))
		for i, tip := range tips {
			shas[i] = tip.SHA
		}
		if err := w.db.SetWatchBaseline(repo.ID, shas); err != nil {
			return err
		}
		log.Printf("watch: %s: baselined %d branch tip(s); deploying changes from here", repo.Name, len(tips))
		repo.WatchBaselined = true
	}
	baseline, err := w.db.WatchBaseline(repo.ID)
	if err != nil {
		return err
	}
	patterns := SplitPatterns(repo.WatchBranches)
	for _, tip := range tips {
		if !MatchBranch(patterns, tip.Branch) || baseline[tip.SHA] {
			continue
		}
		if _, err := w.db.GetDeployBySHA(repo.ID, tip.SHA); err == nil {
			continue
		} else if !errors.Is(err, db.ErrNotFound) {
			return err
		}
		// The poller has no triggering identity — created_by stays empty.
		if _, err := w.queue.RequestDeploy(ctx, repo.Name, tip.SHA, false, ""); err != nil {
			log.Printf("watch: deploy %s@%s: %v", repo.Name, tip.Branch, err)
			continue
		}
		log.Printf("watch: deploying %s@%s (%.7s)", repo.Name, tip.Branch, tip.SHA)
	}
	if evict {
		// Reachability is judged against every surviving tip — not just the
		// watched ones — so a commit still living on an unwatched branch
		// keeps its preview.
		tipSHAs := make([]string, len(tips))
		for i, tip := range tips {
			tipSHAs[i] = tip.SHA
		}
		if len(baseline) > 0 {
			if err := w.db.PruneWatchBaseline(repo.ID, tipSHAs); err != nil {
				return err
			}
		}
		n, err := w.queue.EvictUnreachable(ctx, repo, tipSHAs)
		if err != nil {
			return err
		}
		if n > 0 {
			log.Printf("watch: %s: evicted %d deploy(s) for deleted branches", repo.Name, n)
		}
	}
	maxID, err := w.db.LatestDeployID(repo.ID)
	if err != nil {
		return err
	}
	w.lastPass[repo.ID] = passState{tips: tips, maxDeployID: maxID}
	return nil
}

// SplitPatterns splits a comma-separated branch pattern list, dropping
// empty entries.
func SplitPatterns(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ValidatePatterns canonicalizes a comma-separated branch pattern list (glob
// syntax; see matchGlob). A `!` prefix negates a pattern (see MatchBranch).
// Empty input is valid and means "all branches".
func ValidatePatterns(s string) (string, error) {
	parts := SplitPatterns(s)
	for _, p := range parts {
		glob := strings.TrimPrefix(p, "!")
		if glob == "" { // bare "!"
			return "", fmt.Errorf("invalid branch pattern %q", p)
		}
		for _, seg := range strings.Split(glob, "/") {
			if seg == "**" {
				continue
			}
			if _, err := path.Match(seg, "x"); err != nil {
				return "", fmt.Errorf("invalid branch pattern %q", p)
			}
		}
	}
	return strings.Join(parts, ","), nil
}

// MatchBranch reports whether branch should be built under patterns. A
// `!`-prefixed pattern excludes, and any exclude match wins regardless of the
// includes. Non-`!` patterns form an allowlist that applies only when at
// least one is present: with no include pattern (an empty list, or excludes
// only) every branch matches except those excluded.
func MatchBranch(patterns []string, branch string) bool {
	hasInclude, included := false, false
	for _, p := range patterns {
		if neg, ok := strings.CutPrefix(p, "!"); ok {
			if matchGlob(neg, branch) {
				return false // exclude wins, regardless of includes
			}
			continue
		}
		hasInclude = true
		if matchGlob(p, branch) {
			included = true
		}
	}
	return !hasInclude || included
}

// matchGlob reports whether branch matches pattern. It extends path.Match
// with a `**` segment that spans separators — matching zero or more
// `/`-separated segments — so `gh-readonly-queue/**` covers the multi-segment
// merge-queue refs (`gh-readonly-queue/<base>/pr-<n>-<sha>`) a plain `*`
// can't. A single `*` still stops at `/`.
func matchGlob(pattern, branch string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(branch, "/"))
}

// matchSegments matches the `/`-split pattern against the `/`-split branch. A
// `**` pattern segment consumes zero or more branch segments; every other
// segment matches exactly one, via path.Match.
func matchSegments(pat, name []string) bool {
	if len(pat) == 0 {
		return len(name) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(name); i++ {
			if matchSegments(pat[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	if ok, _ := path.Match(pat[0], name[0]); !ok {
		return false
	}
	return matchSegments(pat[1:], name[1:])
}
