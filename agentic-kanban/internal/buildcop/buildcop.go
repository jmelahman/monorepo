// Package buildcop polls the GitHub Actions REST API and files tickets on a
// dedicated board for each workflow job whose failure rate, over a rolling
// window, exceeds a configurable threshold. When a tracked job recovers (a
// configurable streak of green runs) the ticket is auto-moved to "Fixed".
//
// The design mirrors internal/errreport (board+columns are created lazily on
// first poll) and internal/github (one ticker per process; gh-CLI token
// fallback). The poller derives ticket state entirely from the API window
// on each tick — no per-job persistence in the DB beyond the ticket and its
// fingerprint.
package buildcop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/eventbus"
	"github.com/jmelahman/kanban/internal/github"
	"github.com/jmelahman/kanban/internal/metrics"
	"github.com/jmelahman/kanban/internal/slug"
)

const (
	ColumnFailing       = "Failing"
	ColumnInvestigating = "Investigating"
	ColumnFixed         = "Fixed"
	// ColumnWontFix is a terminal, human-owned column. Once a person parks a
	// ticket here the reconcile loop leaves it untouched (see reconcile); the
	// poller never files into it.
	ColumnWontFix = "Won't fix"
)

type Poller struct {
	store    *db.Store
	bus      eventbus.Publisher
	cfg      Config
	interval time.Duration
	http     *http.Client

	mu     sync.Mutex
	boards map[string]boardCache // slug -> {boardID, columnIDs}

	// jobsCache memoizes /actions/runs/:id/jobs responses. Completed workflow
	// runs are immutable, so once cached we never refetch. Key is
	// "<repo_path>|<run_id>". Retention runs once per tick (see tick) over the
	// UNION of every board's in-window runs for a repo — multiple boards can
	// poll the same repo (e.g. an all-branches board plus a main-only board),
	// and the cache is shared per repo, so retaining against a single board's
	// window would evict the other boards' freshly cached runs and force a full
	// refetch of the jobs endpoint every tick.
	jobsMu    sync.Mutex
	jobsCache map[string][]ghJob

	// attemptCache memoizes job lists for prior (non-latest) workflow-run
	// attempts. Completed attempts are immutable, so entries are never
	// evicted — the working set is bounded by the rolling window.
	cacheMu      sync.Mutex
	attemptCache map[attemptKey][]ghJob
}

type boardCache struct {
	ID   int64
	Cols map[string]int64
}

type attemptKey struct {
	RunID   int64
	Attempt int
}

// NewPoller constructs a poller. interval defaults to 2 minutes when <=0:
// workflow runs change much more slowly than PR state, and the higher
// cadence would waste rate-limit budget without surfacing failures sooner
// than the GitHub UI does.
func NewPoller(store *db.Store, bus eventbus.Publisher, cfg Config, interval time.Duration) *Poller {
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	return &Poller{
		store:    store,
		bus:      bus,
		cfg:      cfg,
		interval: interval,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: metrics.WrapGitHubTransport(nil),
		},
		boards:       make(map[string]boardCache),
		jobsCache:    make(map[string][]ghJob),
		attemptCache: make(map[attemptKey][]ghJob),
	}
}

// jobsCacheKey is the lookup key for a (repo, run_id) pair. Workflow runs
// have repo-scoped numeric IDs, so the repo prefix prevents collisions when
// two boards poll different repos.
func jobsCacheKey(repoPath string, runID int64) string {
	return repoPath + "|" + strconv.FormatInt(runID, 10)
}

// getCachedJobs returns the cached jobs for a run, or false if not cached.
func (p *Poller) getCachedJobs(repoPath string, runID int64) ([]ghJob, bool) {
	p.jobsMu.Lock()
	defer p.jobsMu.Unlock()
	v, ok := p.jobsCache[jobsCacheKey(repoPath, runID)]
	return v, ok
}

func (p *Poller) putCachedJobs(repoPath string, runID int64, jobs []ghJob) {
	p.jobsMu.Lock()
	defer p.jobsMu.Unlock()
	p.jobsCache[jobsCacheKey(repoPath, runID)] = jobs
}

// retainJobs drops any cached jobs whose (repoPath, runID) is not in keep.
// Called at the end of each syncBoard so the cache stays sized to the window.
func (p *Poller) retainJobs(repoPath string, keep map[int64]struct{}) {
	p.jobsMu.Lock()
	defer p.jobsMu.Unlock()
	prefix := repoPath + "|"
	for k := range p.jobsCache {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		idStr := k[len(prefix):]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			delete(p.jobsCache, k)
			continue
		}
		if _, ok := keep[id]; !ok {
			delete(p.jobsCache, k)
		}
	}
}

// Start blocks until ctx is done. No-op when the config is disabled or has
// zero boards — the caller is expected to wrap the goroutine launch with
// `if cfg.Enabled` but we re-check defensively.
func (p *Poller) Start(ctx context.Context) {
	if !p.cfg.Enabled || len(p.cfg.Boards) == 0 {
		return
	}
	t := time.NewTicker(p.interval)
	defer t.Stop()
	p.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.tick(ctx)
		}
	}
}

// boardKeep is one board's contribution to the per-tick cache-retention pass:
// the repo it polls and the set of run IDs currently inside its window.
type boardKeep struct {
	repo string
	keep map[int64]struct{}
}

func (p *Poller) tick(ctx context.Context) {
	var synced []boardKeep
	// Repos with at least one board that failed to sync this tick. We skip
	// eviction for them so a transient error (e.g. a 5xx from listRuns) can't
	// drop cache entries the failed board would have kept, forcing a refetch.
	failed := make(map[string]struct{})
	for _, b := range p.cfg.Boards {
		if b.RepoPath == "" {
			log.Printf("buildcop: board %q has no repo_path, skipping", b.Name)
			continue
		}
		keep, err := p.syncBoard(ctx, b)
		if err != nil {
			log.Printf("buildcop: board %q: %v", b.Name, err)
			failed[b.RepoPath] = struct{}{}
			continue
		}
		synced = append(synced, boardKeep{repo: b.RepoPath, keep: keep})
	}
	p.retainJobsForBoards(synced, failed)
}

// retainJobsForBoards evicts cached run-jobs once per tick. Entries are kept
// when their run is in the UNION of every board's window for that repo, so two
// boards sharing a repo never evict each other (see the jobsCache comment).
// Repos in skip are left untouched this tick.
func (p *Poller) retainJobsForBoards(synced []boardKeep, skip map[string]struct{}) {
	union := make(map[string]map[int64]struct{})
	for _, bk := range synced {
		if _, bad := skip[bk.repo]; bad {
			continue
		}
		m := union[bk.repo]
		if m == nil {
			m = make(map[int64]struct{}, len(bk.keep))
			union[bk.repo] = m
		}
		for id := range bk.keep {
			m[id] = struct{}{}
		}
	}
	for repo, keep := range union {
		p.retainJobs(repo, keep)
	}
}

// syncBoard polls one board and reconciles its tickets. It returns the set of
// run IDs currently inside the board's window so the caller can retain the
// shared jobs cache against the union across all boards on the repo (eviction
// is deliberately NOT done here — see tick / retainJobsForBoards).
func (p *Poller) syncBoard(ctx context.Context, cfg BoardConfig) (map[int64]struct{}, error) {
	cache, err := p.ensureBoard(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("ensure board: %w", err)
	}
	since := time.Now().UTC().Add(-time.Duration(cfg.WindowDays) * 24 * time.Hour)
	runs, err := p.listRuns(ctx, cfg.RepoPath, cfg.Branch, since)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	// Filter to within window (the API filter is approximate when paginating)
	// and to the configured branch.
	filtered := runs[:0]
	for _, r := range runs {
		if r.CreatedAt.Before(since) {
			continue
		}
		if !cfg.MatchesAllBranches() && r.HeadBranch != cfg.Branch {
			continue
		}
		filtered = append(filtered, r)
	}
	runs = filtered

	jobsByRun := make(map[int64][]ghJob, len(runs))
	keep := make(map[int64]struct{}, len(runs))
	for _, r := range runs {
		keep[r.ID] = struct{}{}
		if cached, ok := p.getCachedJobs(cfg.RepoPath, r.ID); ok {
			jobsByRun[r.ID] = cached
			continue
		}
		jobs, err := p.listJobs(ctx, cfg.RepoPath, r.ID)
		if err != nil {
			log.Printf("buildcop: board %q: jobs for run %d: %v", cfg.Name, r.ID, err)
			continue
		}
		jobsByRun[r.ID] = jobs
		p.putCachedJobs(cfg.RepoPath, r.ID, jobs)
	}

	// For each run that succeeded on retry, fetch the prior attempts' jobs
	// so flake events can be attributed to the specific job that failed
	// then passed. Prior attempts are immutable, so fetchPriorAttemptJobs
	// memoizes results across ticks.
	priorFailedJobs := make(map[int64][]ghJob)
	for _, r := range runs {
		if r.RunAttempt <= 1 || classifyConclusion(r.Conclusion) != "success" {
			continue
		}
		for attempt := 1; attempt < r.RunAttempt; attempt++ {
			jobs, err := p.fetchPriorAttemptJobs(ctx, cfg.RepoPath, r.ID, attempt)
			if err != nil {
				log.Printf("buildcop: board %q: prior attempt jobs run %d attempt %d: %v",
					cfg.Name, r.ID, attempt, err)
				continue
			}
			for _, j := range jobs {
				if classifyConclusion(j.Conclusion) == "failure" {
					priorFailedJobs[r.ID] = append(priorFailedJobs[r.ID], j)
				}
			}
		}
	}

	stats := aggregate(runs, jobsByRun, priorFailedJobs)
	if err := p.reconcile(ctx, cfg, cache, stats); err != nil {
		return nil, err
	}
	return keep, nil
}

func (p *Poller) reconcile(ctx context.Context, cfg BoardConfig, cache boardCache, stats map[string]jobStats) error {
	failingColID := cache.Cols[ColumnFailing]
	fixedColID := cache.Cols[ColumnFixed]
	wontFixColID := cache.Cols[ColumnWontFix]
	if failingColID == 0 || fixedColID == 0 {
		return fmt.Errorf("board %q missing required columns", cfg.Name)
	}

	for _, s := range stats {
		fp := computeFingerprint(s.Workflow, s.Job)
		existing, err := p.store.FindOpenTicketByFingerprint(ctx, cache.ID, fp)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			log.Printf("buildcop: find fingerprint %s: %v", fp, err)
			continue
		}
		// "Won't fix" is a terminal manual state: once a human parks a ticket
		// there the poller leaves it untouched — no title/body refresh, no
		// re-open on continued failure, no promotion to Fixed on recovery. The
		// ticket is still open, so FindOpenTicketByFingerprint returns it and
		// we also avoid filing a duplicate for the same job.
		if existing != nil && wontFixColID != 0 && existing.ColumnID == wontFixColID {
			continue
		}
		switch {
		case shouldFile(s, cfg):
			if existing == nil {
				p.createTicket(ctx, cache.ID, failingColID, s, fp)
			} else {
				p.updateOrRestore(ctx, existing, failingColID, fixedColID, s)
			}
		case existing != nil && shouldFix(s, cfg):
			p.moveToFixed(ctx, existing, fixedColID, s)
		}
	}
	return nil
}

func (p *Poller) createTicket(ctx context.Context, boardID, colID int64, s jobStats, fp string) {
	t := &db.Ticket{
		BoardID:     boardID,
		ColumnID:    colID,
		Title:       titleFor(s),
		Slug:        slug.Make(fmt.Sprintf("buildcop-%s-%s", s.Workflow, s.Job), "build-cop"),
		Body:        bodyFor(s),
		Fingerprint: fp,
	}
	if err := p.store.CreateTicket(ctx, t); err != nil {
		log.Printf("buildcop: create ticket: %v", err)
		return
	}
	p.publish(boardID, "ticket_created", t)
}

// updateOrRestore refreshes the ticket's title/body to reflect the current
// stats. If the ticket sits in the Fixed column it is moved back to Failing
// — the job has regressed, so the user should see it re-open.
func (p *Poller) updateOrRestore(ctx context.Context, t *db.Ticket, failingColID, fixedColID int64, s jobStats) {
	t.Title = titleFor(s)
	t.Body = bodyFor(s)
	if err := p.store.UpdateTicket(ctx, t); err != nil {
		log.Printf("buildcop: update ticket %d: %v", t.ID, err)
		return
	}
	if t.ColumnID == fixedColID {
		if err := p.move(ctx, t, failingColID); err != nil {
			log.Printf("buildcop: re-open ticket %d: %v", t.ID, err)
			return
		}
		return
	}
	p.publish(t.BoardID, "ticket_updated", t)
}

func (p *Poller) moveToFixed(ctx context.Context, t *db.Ticket, fixedColID int64, s jobStats) {
	if t.ColumnID == fixedColID {
		return
	}
	t.Title = titleFor(s)
	t.Body = bodyFor(s)
	if err := p.store.UpdateTicket(ctx, t); err != nil {
		log.Printf("buildcop: update before move %d: %v", t.ID, err)
		return
	}
	if err := p.move(ctx, t, fixedColID); err != nil {
		log.Printf("buildcop: move ticket %d to Fixed: %v", t.ID, err)
	}
}

func (p *Poller) move(ctx context.Context, t *db.Ticket, colID int64) error {
	maxPos, err := p.store.MaxTicketPosition(ctx, colID)
	if err != nil {
		return err
	}
	if err := p.store.MoveTicket(ctx, t.ID, colID, maxPos+1); err != nil {
		return err
	}
	if fresh, err := p.store.GetTicket(ctx, t.ID); err == nil {
		p.publish(t.BoardID, "ticket_moved", fresh)
	}
	return nil
}

func (p *Poller) publish(boardID int64, typ string, data any) {
	if p.bus == nil {
		return
	}
	p.bus.Publish(boardID, typ, data)
}

// ensureBoard returns the cached board+column IDs, populating the cache from
// the DB (or creating fresh rows) on first call per slug. Holds the mutex
// across the DB calls so concurrent first ticks don't race to create
// duplicate boards.
func (p *Poller) ensureBoard(ctx context.Context, cfg BoardConfig) (boardCache, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cache, ok := p.boards[cfg.Slug]; ok {
		return cache, nil
	}
	cache, err := p.loadOrCreateBoard(ctx, cfg)
	if err != nil {
		return boardCache{}, err
	}
	p.boards[cfg.Slug] = cache
	return cache, nil
}

func (p *Poller) loadOrCreateBoard(ctx context.Context, cfg BoardConfig) (boardCache, error) {
	board, err := p.store.GetBoardBySlug(ctx, cfg.Slug)
	if errors.Is(err, db.ErrNotFound) {
		b := &db.Board{
			Name:       cfg.Name,
			Slug:       cfg.Slug,
			RepoPath:   cfg.RepoPath,
			BaseBranch: defaultBaseBranch(cfg.Branch),
		}
		if err := p.store.CreateBoardRaw(ctx, b); err != nil {
			return boardCache{}, fmt.Errorf("create board: %w", err)
		}
		cols := []db.Column{
			{BoardID: b.ID, Name: ColumnFailing, Position: 0},
			{BoardID: b.ID, Name: ColumnInvestigating, Position: 1},
			{BoardID: b.ID, Name: ColumnFixed, Position: 2},
			{BoardID: b.ID, Name: ColumnWontFix, Position: 3},
		}
		out := boardCache{ID: b.ID, Cols: map[string]int64{}}
		for i := range cols {
			if err := p.store.CreateColumn(ctx, &cols[i]); err != nil {
				return boardCache{}, fmt.Errorf("create column %s: %w", cols[i].Name, err)
			}
			out.Cols[cols[i].Name] = cols[i].ID
		}
		return out, nil
	}
	if err != nil {
		return boardCache{}, fmt.Errorf("lookup board: %w", err)
	}
	cols, err := p.store.ListColumns(ctx, board.ID)
	if err != nil {
		return boardCache{}, fmt.Errorf("list columns: %w", err)
	}
	out := boardCache{ID: board.ID, Cols: map[string]int64{}}
	maxPos := -1
	for _, c := range cols {
		out.Cols[c.Name] = c.ID
		if c.Position > maxPos {
			maxPos = c.Position
		}
	}
	// Backfill "Won't fix" onto boards created before it existed. Scoped here
	// rather than in the global db migration because only Build Cop boards get
	// this layout, and this runs lazily on the first poll after upgrade.
	if _, ok := out.Cols[ColumnWontFix]; !ok {
		c := db.Column{BoardID: board.ID, Name: ColumnWontFix, Position: maxPos + 1}
		if err := p.store.CreateColumn(ctx, &c); err != nil {
			return boardCache{}, fmt.Errorf("backfill column %s: %w", ColumnWontFix, err)
		}
		out.Cols[ColumnWontFix] = c.ID
	}
	return out, nil
}

// defaultBaseBranch is what the auto-created board records as its
// base_branch column. Cosmetic — the board has no sessions, so base_branch
// is informational only. We use the configured branch when scoped to a
// specific one, else "main".
func defaultBaseBranch(branch string) string {
	if branch == "" || branch == "*" {
		return "main"
	}
	return branch
}

// GitHub API plumbing -------------------------------------------------------

type ghRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	WorkflowID int64     `json:"workflow_id"`
	HeadBranch string    `json:"head_branch"`
	HeadSHA    string    `json:"head_sha"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	HTMLURL    string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	RunAttempt int       `json:"run_attempt"`
}

type ghRunsResp struct {
	WorkflowRuns []ghRun `json:"workflow_runs"`
}

type ghJob struct {
	ID         int64  `json:"id"`
	RunID      int64  `json:"run_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
}

type ghJobsResp struct {
	Jobs []ghJob `json:"jobs"`
}

func (p *Poller) listRuns(ctx context.Context, repoPath, branch string, since time.Time) ([]ghRun, error) {
	owner, repo, host, err := github.ParseGitHubRepo(repoPath)
	if err != nil {
		return nil, err
	}
	apiBase := github.APIBaseFor(host)
	tok := github.Token(host)

	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	branchParam := ""
	if branch != "" && branch != "*" {
		branchParam = "&branch=" + url.QueryEscape(branch)
	}
	// `created` uses GitHub's search-style >=DATE filter (URL-encoded as %3E%3D)
	// to keep the response bounded on busy repos. We still re-filter client-side
	// because the API matches whole days, not the exact `since` instant.
	next := fmt.Sprintf("%s/repos/%s/%s/actions/runs?status=completed&per_page=100&created=%%3E%%3D%s%s",
		apiBase, url.PathEscape(owner), url.PathEscape(repo),
		since.UTC().Format("2006-01-02"), branchParam)

	var all []ghRun
	for page := 0; page < 5 && next != ""; page++ {
		var resp ghRunsResp
		link, err := github.GetJSONPage(cctx, p.http, tok, next, &resp)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.WorkflowRuns...)
		// Stop once the page's oldest run falls before the window: subsequent
		// pages only have older runs.
		if n := len(resp.WorkflowRuns); n > 0 && resp.WorkflowRuns[n-1].CreatedAt.Before(since) {
			break
		}
		next = link
	}
	return all, nil
}

func (p *Poller) listJobs(ctx context.Context, repoPath string, runID int64) ([]ghJob, error) {
	owner, repo, host, err := github.ParseGitHubRepo(repoPath)
	if err != nil {
		return nil, err
	}
	apiBase := github.APIBaseFor(host)
	tok := github.Token(host)

	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	next := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/jobs?per_page=100",
		apiBase, url.PathEscape(owner), url.PathEscape(repo), runID)

	var all []ghJob
	for page := 0; page < 3 && next != ""; page++ {
		var resp ghJobsResp
		link, err := github.GetJSONPage(cctx, p.http, tok, next, &resp)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Jobs...)
		next = link
	}
	return all, nil
}

// fetchPriorAttemptJobs returns the jobs from a non-latest workflow-run
// attempt. Completed attempts are immutable, so results are cached in
// attemptCache and reused across ticks.
func (p *Poller) fetchPriorAttemptJobs(ctx context.Context, repoPath string, runID int64, attempt int) ([]ghJob, error) {
	key := attemptKey{RunID: runID, Attempt: attempt}
	p.cacheMu.Lock()
	if cached, ok := p.attemptCache[key]; ok {
		p.cacheMu.Unlock()
		return cached, nil
	}
	p.cacheMu.Unlock()

	owner, repo, host, err := github.ParseGitHubRepo(repoPath)
	if err != nil {
		return nil, err
	}
	apiBase := github.APIBaseFor(host)
	tok := github.Token(host)

	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	next := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/attempts/%d/jobs?per_page=100",
		apiBase, url.PathEscape(owner), url.PathEscape(repo), runID, attempt)

	var all []ghJob
	for page := 0; page < 3 && next != ""; page++ {
		var resp ghJobsResp
		link, err := github.GetJSONPage(cctx, p.http, tok, next, &resp)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Jobs...)
		next = link
	}

	p.cacheMu.Lock()
	p.attemptCache[key] = all
	p.cacheMu.Unlock()
	return all, nil
}

// Aggregation --------------------------------------------------------------

type aggKey struct {
	Workflow string
	Job      string
}

type jobStats struct {
	Workflow       string
	Job            string
	Total          int
	Successes      int
	Failures       int
	GreenStreak    int
	LastFailureURL string
	LastFailureAt  time.Time
	// FlakyRetries counts distinct commits (workflow runs) in the window
	// where this job failed in an earlier attempt but the run ultimately
	// succeeded — one flake per head_sha regardless of how many attempts
	// it took to get green.
	FlakyRetries int
	LastFlakyURL string
	LastFlakyAt  time.Time
}

// aggregate folds the runs+jobs into per-(workflow,job) statistics. Runs are
// processed newest-first so GreenStreak — number of leading consecutive
// successes before the first failure — is computed in one pass.
//
// priorFailedJobs maps a run ID to the jobs that failed in earlier attempts
// of that same run when the run ultimately succeeded on retry. Each such job
// is counted as a flake for the (workflow, job) pair — provided the job
// still exists in the successful attempt (matched by name) — and breaks the
// green streak, since a job that only passes after retry is not reliably
// green.
func aggregate(runs []ghRun, jobsByRun map[int64][]ghJob,
	priorFailedJobs map[int64][]ghJob,
) map[string]jobStats {
	sortedRuns := append([]ghRun(nil), runs...)
	sort.SliceStable(sortedRuns, func(i, j int) bool {
		return sortedRuns[i].CreatedAt.After(sortedRuns[j].CreatedAt)
	})

	type entry struct {
		stats        jobStats
		streakBroken bool
	}
	byKey := make(map[aggKey]*entry)

	for _, r := range sortedRuns {
		workflow := r.Name
		if workflow == "" {
			workflow = fmt.Sprintf("workflow %d", r.WorkflowID)
		}

		// Attribute flake events first so streakBroken is set before the
		// current attempt's success is counted. A job that only passes on
		// retry doesn't extend the leading green streak.
		//
		// Dedupe by job name within a single run: FlakyRetries counts
		// distinct commits where the job needed a retry. A run with three
		// attempts where the same job failed twice before passing is one
		// flaky incident against one head_sha, not two.
		if r.RunAttempt > 1 && classifyConclusion(r.Conclusion) == "success" {
			// A flake is failed-then-passed against the same head_sha. Require
			// the job to have SUCCEEDED on the current attempt — a job that
			// failed in both attempts (e.g. under continue-on-error, or a
			// matrix cell that stayed red while the run reported success) is
			// a real failure, not a flake. Skipped jobs on the retry also
			// don't qualify.
			currentSucceeded := make(map[string]struct{}, len(jobsByRun[r.ID]))
			for _, j := range jobsByRun[r.ID] {
				if classifyConclusion(j.Conclusion) == "success" {
					currentSucceeded[j.Name] = struct{}{}
				}
			}
			seenThisRun := make(map[string]struct{})
			for _, failedJob := range priorFailedJobs[r.ID] {
				if _, ok := currentSucceeded[failedJob.Name]; !ok {
					continue
				}
				if _, dup := seenThisRun[failedJob.Name]; dup {
					continue
				}
				seenThisRun[failedJob.Name] = struct{}{}
				k := aggKey{Workflow: workflow, Job: failedJob.Name}
				e, ok := byKey[k]
				if !ok {
					e = &entry{stats: jobStats{Workflow: workflow, Job: failedJob.Name}}
					byKey[k] = e
				}
				e.stats.FlakyRetries++
				if e.stats.LastFlakyAt.IsZero() {
					e.stats.LastFlakyAt = r.CreatedAt
					if failedJob.HTMLURL != "" {
						e.stats.LastFlakyURL = failedJob.HTMLURL
					} else {
						e.stats.LastFlakyURL = r.HTMLURL
					}
				}
				e.streakBroken = true
			}
		}

		for _, j := range jobsByRun[r.ID] {
			class := classifyConclusion(j.Conclusion)
			if class == "skip" {
				continue
			}
			k := aggKey{Workflow: workflow, Job: j.Name}
			e, ok := byKey[k]
			if !ok {
				e = &entry{stats: jobStats{Workflow: workflow, Job: j.Name}}
				byKey[k] = e
			}
			e.stats.Total++
			switch class {
			case "success":
				e.stats.Successes++
				if !e.streakBroken {
					e.stats.GreenStreak++
				}
			case "failure":
				e.stats.Failures++
				e.streakBroken = true
				if e.stats.LastFailureAt.IsZero() {
					e.stats.LastFailureAt = r.CreatedAt
					if j.HTMLURL != "" {
						e.stats.LastFailureURL = j.HTMLURL
					} else {
						e.stats.LastFailureURL = r.HTMLURL
					}
				}
			}
		}
	}
	out := make(map[string]jobStats, len(byKey))
	for k, e := range byKey {
		out[k.Workflow+":"+k.Job] = e.stats
	}
	return out
}

// classifyConclusion maps a GitHub Actions conclusion string to one of
// "success", "failure", or "skip". `cancelled` is treated as skip: GitHub
// cancels jobs both when the user clicks Cancel and when a required prior
// job failed, and we don't want either to inflate the failure count.
func classifyConclusion(c string) string {
	switch strings.ToLower(c) {
	case "success", "neutral":
		return "success"
	case "failure", "timed_out", "action_required":
		return "failure"
	}
	return "skip"
}

func computeFingerprint(workflow, job string) string {
	h := sha256.New()
	h.Write([]byte(workflow))
	h.Write([]byte{':'})
	h.Write([]byte(job))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func shouldFile(s jobStats, cfg BoardConfig) bool {
	if s.Total < cfg.MinRuns {
		return false
	}
	rate := float64(s.Failures) / float64(s.Total)
	if rate > cfg.FailureThreshold {
		return true
	}
	return s.FlakyRetries >= cfg.FlakyThreshold
}

func shouldFix(s jobStats, cfg BoardConfig) bool {
	return s.GreenStreak >= cfg.GreenStreakRequired
}

func titleFor(s jobStats) string {
	rate := 0.0
	if s.Total > 0 {
		rate = float64(s.Failures) / float64(s.Total) * 100
	}
	hasFail := s.Failures > 0
	hasFlake := s.FlakyRetries > 0
	switch {
	case hasFail && hasFlake:
		return fmt.Sprintf("%s / %s failing+flaky (%d%% over %d runs, %d retries)",
			s.Workflow, s.Job, int(math.Round(rate)), s.Total, s.FlakyRetries)
	case hasFlake && !hasFail:
		return fmt.Sprintf("%s / %s flaky (%d passes-on-retry over %d runs)",
			s.Workflow, s.Job, s.FlakyRetries, s.Total)
	}
	return fmt.Sprintf("%s / %s failing (%d%% over %d runs)",
		s.Workflow, s.Job, int(math.Round(rate)), s.Total)
}

func bodyFor(s jobStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**workflow:** `%s`\n", s.Workflow)
	fmt.Fprintf(&b, "**job:** `%s`\n", s.Job)
	fmt.Fprintf(&b, "**runs:** %d (success %d, failure %d)\n", s.Total, s.Successes, s.Failures)
	if s.Total > 0 {
		rate := float64(s.Failures) / float64(s.Total) * 100
		fmt.Fprintf(&b, "**failure rate:** %.1f%%\n", rate)
	}
	if s.GreenStreak > 0 {
		fmt.Fprintf(&b, "**green streak:** %d\n", s.GreenStreak)
	}
	if s.FlakyRetries > 0 {
		fmt.Fprintf(&b, "**flaky retries:** %d\n", s.FlakyRetries)
	}
	if !s.LastFailureAt.IsZero() {
		fmt.Fprintf(&b, "**last failure:** %s\n", s.LastFailureAt.UTC().Format(time.RFC3339))
	}
	if !s.LastFlakyAt.IsZero() {
		fmt.Fprintf(&b, "**last flaky run:** %s\n", s.LastFlakyAt.UTC().Format(time.RFC3339))
	}
	if s.LastFailureURL != "" {
		fmt.Fprintf(&b, "\n[Most recent failing job](%s)\n", s.LastFailureURL)
	}
	if s.LastFlakyURL != "" {
		fmt.Fprintf(&b, "\n[Most recent flaky job (prior attempt)](%s)\n", s.LastFlakyURL)
	}
	return b.String()
}
