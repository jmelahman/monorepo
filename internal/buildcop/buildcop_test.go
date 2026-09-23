package buildcop

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/kanbantoml"
	"github.com/jmelahman/kanban/internal/slug"
)

func toBoardSection(name, repoPath, branch string, threshold *float64, minRuns, streak, window, flaky *int) kanbantoml.BuildCopBoard {
	out := kanbantoml.BuildCopBoard{}
	if name != "" {
		out.Name = &name
	}
	if repoPath != "" {
		out.RepoPath = &repoPath
	}
	if branch != "" {
		out.Branch = &branch
	}
	out.FailureThreshold = threshold
	out.MinRuns = minRuns
	out.GreenStreakRequired = streak
	out.WindowDays = window
	out.FlakyThreshold = flaky
	return out
}

func TestComputeFingerprintStability(t *testing.T) {
	a := computeFingerprint("CI", "lint")
	b := computeFingerprint("CI", "lint")
	if a != b {
		t.Errorf("same inputs produced different fingerprints: %s vs %s", a, b)
	}
	if len(a) != 16 {
		t.Errorf("fingerprint length = %d, want 16", len(a))
	}
}

func TestComputeFingerprintDistinct(t *testing.T) {
	cases := [][2]string{
		{"CI", "lint"},
		{"CI", "test"},
		{"Build", "lint"},
		{"CI:Lint", ""}, // pathological: ':' in workflow
		{"CI", ":Lint"},
	}
	seen := map[string]string{}
	for _, c := range cases {
		fp := computeFingerprint(c[0], c[1])
		key := c[0] + "|" + c[1]
		if dup, ok := seen[fp]; ok && dup != key {
			t.Errorf("collision: %s and %s both -> %s", dup, key, fp)
		}
		seen[fp] = key
	}
}

func TestClassifyConclusion(t *testing.T) {
	cases := map[string]string{
		"success":         "success",
		"SUCCESS":         "success",
		"neutral":         "success",
		"failure":         "failure",
		"timed_out":       "failure",
		"action_required": "failure",
		"cancelled":       "skip",
		"skipped":         "skip",
		"":                "skip",
		"unknown":         "skip",
	}
	for in, want := range cases {
		if got := classifyConclusion(in); got != want {
			t.Errorf("classifyConclusion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShouldFile(t *testing.T) {
	cfg := BoardConfig{FailureThreshold: 0.10, MinRuns: 5, FlakyThreshold: 3}
	cases := []struct {
		name string
		s    jobStats
		want bool
	}{
		{name: "below min_runs even with all failures", s: jobStats{Total: 4, Failures: 4}, want: false},
		{name: "min_runs but under threshold", s: jobStats{Total: 10, Failures: 1}, want: false},        // 10% (not strictly greater)
		{name: "min_runs and above threshold", s: jobStats{Total: 10, Failures: 2}, want: true},         // 20%
		{name: "at min_runs boundary, above threshold", s: jobStats{Total: 5, Failures: 1}, want: true}, // 20%
		{name: "zero total", s: jobStats{Total: 0, Failures: 0}, want: false},
	}
	for _, c := range cases {
		if got := shouldFile(c.s, cfg); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestShouldFileFlaky(t *testing.T) {
	cfg := BoardConfig{FailureThreshold: 0.10, MinRuns: 5, FlakyThreshold: 3}
	cases := []struct {
		name string
		s    jobStats
		want bool
	}{
		{name: "flakes meet threshold, failure rate below", s: jobStats{Total: 10, Failures: 0, FlakyRetries: 3}, want: true},
		{name: "flakes above threshold", s: jobStats{Total: 20, Failures: 1, FlakyRetries: 5}, want: true},
		{name: "flakes below threshold, failure rate below", s: jobStats{Total: 10, Failures: 0, FlakyRetries: 2}, want: false},
		{name: "flakes below threshold but Total below min_runs", s: jobStats{Total: 4, Failures: 0, FlakyRetries: 3}, want: false},
	}
	for _, c := range cases {
		if got := shouldFile(c.s, cfg); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestShouldFix(t *testing.T) {
	cfg := BoardConfig{GreenStreakRequired: 10}
	cases := []struct {
		name string
		s    jobStats
		want bool
	}{
		{name: "no streak", s: jobStats{GreenStreak: 0}, want: false},
		{name: "short streak", s: jobStats{GreenStreak: 9}, want: false},
		{name: "exact streak", s: jobStats{GreenStreak: 10}, want: true},
		{name: "long streak", s: jobStats{GreenStreak: 50}, want: true},
	}
	for _, c := range cases {
		if got := shouldFix(c.s, cfg); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// runAt returns a ghRun at minute m of an arbitrary base time. Newer m =
// newer run. Tests sort entirely by CreatedAt, so the exact base date is
// irrelevant.
func runAt(id int64, workflow string, m int) ghRun {
	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	return ghRun{
		ID:        id,
		Name:      workflow,
		CreatedAt: base.Add(time.Duration(m) * time.Minute),
		HTMLURL:   "https://example.com/runs/x",
	}
}

func jobAt(name, conclusion, htmlURL string) ghJob {
	return ghJob{Name: name, Conclusion: conclusion, HTMLURL: htmlURL}
}

func TestAggregateGreenStreak(t *testing.T) {
	// Five runs of "CI" newest→oldest with the "lint" job:
	//   run5 success, run4 success, run3 failure, run2 success, run1 success.
	// GreenStreak should be 2 (leading two successes only), total 5,
	// failures 1, successes 4.
	runs := []ghRun{
		runAt(1, "CI", 1),
		runAt(2, "CI", 2),
		runAt(3, "CI", 3),
		runAt(4, "CI", 4),
		runAt(5, "CI", 5),
	}
	jobsByRun := map[int64][]ghJob{
		1: {jobAt("lint", "success", "")},
		2: {jobAt("lint", "success", "")},
		3: {jobAt("lint", "failure", "https://example.com/jobs/3")},
		4: {jobAt("lint", "success", "")},
		5: {jobAt("lint", "success", "")},
	}
	stats := aggregate(runs, jobsByRun, nil)
	s, ok := stats["CI:lint"]
	if !ok {
		t.Fatalf("missing CI:lint, got %v", stats)
	}
	if s.Total != 5 || s.Successes != 4 || s.Failures != 1 {
		t.Errorf("counts: total=%d, successes=%d, failures=%d", s.Total, s.Successes, s.Failures)
	}
	if s.GreenStreak != 2 {
		t.Errorf("GreenStreak = %d, want 2", s.GreenStreak)
	}
	if s.LastFailureURL != "https://example.com/jobs/3" {
		t.Errorf("LastFailureURL = %q", s.LastFailureURL)
	}
}

func TestAggregateSplitsByWorkflowAndJob(t *testing.T) {
	runs := []ghRun{runAt(1, "CI", 1), runAt(2, "Build", 2)}
	jobsByRun := map[int64][]ghJob{
		1: {jobAt("lint", "success", ""), jobAt("test", "failure", "")},
		2: {jobAt("lint", "failure", "")},
	}
	stats := aggregate(runs, jobsByRun, nil)
	if len(stats) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(stats), keysOf(stats))
	}
	for _, key := range []string{"CI:lint", "CI:test", "Build:lint"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("missing %q in %v", key, keysOf(stats))
		}
	}
}

func TestAggregateSkipsCancelled(t *testing.T) {
	runs := []ghRun{runAt(1, "CI", 1), runAt(2, "CI", 2)}
	jobsByRun := map[int64][]ghJob{
		1: {jobAt("lint", "cancelled", "")},
		2: {jobAt("lint", "success", "")},
	}
	stats := aggregate(runs, jobsByRun, nil)
	s := stats["CI:lint"]
	if s.Total != 1 {
		t.Errorf("Total = %d, want 1 (cancelled skipped)", s.Total)
	}
	if s.GreenStreak != 1 {
		t.Errorf("GreenStreak = %d, want 1", s.GreenStreak)
	}
}

func TestAggregateAllFailures(t *testing.T) {
	runs := []ghRun{runAt(1, "CI", 1), runAt(2, "CI", 2)}
	jobsByRun := map[int64][]ghJob{
		1: {jobAt("lint", "failure", "https://example.com/old")},
		2: {jobAt("lint", "failure", "https://example.com/new")},
	}
	stats := aggregate(runs, jobsByRun, nil)
	s := stats["CI:lint"]
	if s.GreenStreak != 0 {
		t.Errorf("GreenStreak = %d, want 0", s.GreenStreak)
	}
	if s.LastFailureURL != "https://example.com/new" {
		t.Errorf("LastFailureURL = %q, want newest run's URL", s.LastFailureURL)
	}
}

func TestTitleFor(t *testing.T) {
	s := jobStats{Workflow: "CI", Job: "lint", Total: 20, Failures: 3}
	got := titleFor(s)
	if !strings.Contains(got, "CI / lint") {
		t.Errorf("title missing workflow/job: %q", got)
	}
	if !strings.Contains(got, "15%") {
		t.Errorf("title missing rate: %q", got)
	}
	if !strings.Contains(got, "20 runs") {
		t.Errorf("title missing count: %q", got)
	}
}

func TestTitleForFlaky(t *testing.T) {
	// Flaky only (no failures on latest attempts).
	only := jobStats{Workflow: "CI", Job: "lint", Total: 20, Failures: 0, FlakyRetries: 4}
	got := titleFor(only)
	if !strings.Contains(got, "flaky") {
		t.Errorf("flaky-only title should mention flaky: %q", got)
	}
	if strings.Contains(got, "failing+flaky") {
		t.Errorf("flaky-only title should not read as failing+flaky: %q", got)
	}
	if !strings.Contains(got, "4 passes-on-retry") {
		t.Errorf("flaky-only title should count retries: %q", got)
	}

	// Failing + flaky.
	both := jobStats{Workflow: "CI", Job: "lint", Total: 20, Failures: 3, FlakyRetries: 2}
	got = titleFor(both)
	if !strings.Contains(got, "failing+flaky") {
		t.Errorf("failing+flaky title missing composite marker: %q", got)
	}
	if !strings.Contains(got, "15%") {
		t.Errorf("failing+flaky title missing rate: %q", got)
	}
	if !strings.Contains(got, "2 retries") {
		t.Errorf("failing+flaky title missing retry count: %q", got)
	}
}

// runAttempt returns a run with a specific attempt number and conclusion.
func runAttempt(id int64, workflow string, m, attempt int, conclusion string) ghRun {
	r := runAt(id, workflow, m)
	r.RunAttempt = attempt
	r.Conclusion = conclusion
	return r
}

func TestAggregateFlaky(t *testing.T) {
	// One run that succeeded on attempt 2; on attempt 1 the "lint" job
	// failed. Expect FlakyRetries=1, Failures=0, GreenStreak=0 (flake breaks
	// green streak), LastFlakyURL matching the failed prior-attempt job.
	runs := []ghRun{runAttempt(10, "CI", 1, 2, "success")}
	jobsByRun := map[int64][]ghJob{
		10: {jobAt("lint", "success", "")},
	}
	prior := map[int64][]ghJob{
		10: {jobAt("lint", "failure", "https://example.com/jobs/prior")},
	}
	stats := aggregate(runs, jobsByRun, prior)
	s, ok := stats["CI:lint"]
	if !ok {
		t.Fatalf("missing CI:lint, got %v", keysOf(stats))
	}
	if s.FlakyRetries != 1 {
		t.Errorf("FlakyRetries = %d, want 1", s.FlakyRetries)
	}
	if s.Failures != 0 {
		t.Errorf("Failures = %d, want 0 (flake should not increment Failures)", s.Failures)
	}
	if s.GreenStreak != 0 {
		t.Errorf("GreenStreak = %d, want 0 (flake breaks streak)", s.GreenStreak)
	}
	if s.LastFlakyURL != "https://example.com/jobs/prior" {
		t.Errorf("LastFlakyURL = %q", s.LastFlakyURL)
	}
	if s.LastFlakyAt.IsZero() {
		t.Errorf("LastFlakyAt not set")
	}
}

func TestAggregateFlakyDedupePerRun(t *testing.T) {
	// One run, attempt 3 succeeded; attempts 1 and 2 both failed "lint".
	// That's a single flaky commit — FlakyRetries should be 1, not 2.
	runs := []ghRun{runAttempt(12, "CI", 1, 3, "success")}
	jobsByRun := map[int64][]ghJob{
		12: {jobAt("lint", "success", "")},
	}
	prior := map[int64][]ghJob{
		12: {
			jobAt("lint", "failure", "https://example.com/jobs/attempt1"),
			jobAt("lint", "failure", "https://example.com/jobs/attempt2"),
		},
	}
	stats := aggregate(runs, jobsByRun, prior)
	s := stats["CI:lint"]
	if s.FlakyRetries != 1 {
		t.Errorf("FlakyRetries = %d, want 1 (dedupe by job name per run)", s.FlakyRetries)
	}
}

func TestAggregateFlakyJobRemoved(t *testing.T) {
	// Prior attempt failed a job that no longer exists in the current
	// attempt (renamed or removed between attempts). Do not credit a flake.
	runs := []ghRun{runAttempt(11, "CI", 1, 2, "success")}
	jobsByRun := map[int64][]ghJob{
		11: {jobAt("test", "success", "")},
	}
	prior := map[int64][]ghJob{
		11: {jobAt("lint", "failure", "https://example.com/jobs/gone")},
	}
	stats := aggregate(runs, jobsByRun, prior)
	if s, ok := stats["CI:lint"]; ok {
		t.Errorf("removed job should not appear, got %+v", s)
	}
	s := stats["CI:test"]
	if s.FlakyRetries != 0 {
		t.Errorf("current job with no matching prior failure should have 0 flakes, got %d", s.FlakyRetries)
	}
}

func TestAggregateFlakyRequiresCurrentAttemptSuccess(t *testing.T) {
	// Workflow run reports success (e.g. via continue-on-error), but the
	// specific job failed in both attempts. That's a real failure, not a
	// flake — FlakyRetries must not be credited.
	runs := []ghRun{runAttempt(13, "CI", 1, 2, "success")}
	jobsByRun := map[int64][]ghJob{
		// lint kept failing on the retry; only accessory job succeeded.
		13: {
			jobAt("lint", "failure", "https://example.com/jobs/latest-fail"),
			jobAt("notify", "success", ""),
		},
	}
	prior := map[int64][]ghJob{
		13: {jobAt("lint", "failure", "https://example.com/jobs/attempt1")},
	}
	stats := aggregate(runs, jobsByRun, prior)
	s := stats["CI:lint"]
	if s.FlakyRetries != 0 {
		t.Errorf("FlakyRetries = %d, want 0 (job failed on retry too)", s.FlakyRetries)
	}
	if s.Failures != 1 {
		t.Errorf("Failures = %d, want 1 (latest-attempt failure still counted)", s.Failures)
	}
}

func TestAggregateFlakyIgnoresSkippedOnRetry(t *testing.T) {
	// Prior attempt failed the job; the retry skipped it (e.g. a conditional
	// dependency changed). Not a pass-on-retry — do not credit a flake.
	runs := []ghRun{runAttempt(14, "CI", 1, 2, "success")}
	jobsByRun := map[int64][]ghJob{
		14: {jobAt("lint", "skipped", "")},
	}
	prior := map[int64][]ghJob{
		14: {jobAt("lint", "failure", "https://example.com/jobs/attempt1")},
	}
	stats := aggregate(runs, jobsByRun, prior)
	if s, ok := stats["CI:lint"]; ok && s.FlakyRetries != 0 {
		t.Errorf("FlakyRetries = %d, want 0 (job skipped on retry)", s.FlakyRetries)
	}
}

func TestResolveBoardDefaults(t *testing.T) {
	got := resolveBoard(toBoardSection("", "", "", nil, nil, nil, nil, nil))
	if got.FailureThreshold != 0.10 {
		t.Errorf("FailureThreshold default = %v", got.FailureThreshold)
	}
	if got.MinRuns != 5 {
		t.Errorf("MinRuns default = %d", got.MinRuns)
	}
	if got.GreenStreakRequired != 10 {
		t.Errorf("GreenStreakRequired default = %d", got.GreenStreakRequired)
	}
	if got.WindowDays != 7 {
		t.Errorf("WindowDays default = %d", got.WindowDays)
	}
	if got.FlakyThreshold != 3 {
		t.Errorf("FlakyThreshold default = %d", got.FlakyThreshold)
	}
	if !got.MatchesAllBranches() {
		t.Errorf("empty branch should match all")
	}
}

func TestResolveBoardNameFallback(t *testing.T) {
	master := "master"
	got := resolveBoard(toBoardSection("", "", master, nil, nil, nil, nil, nil))
	if !strings.Contains(got.Name, "master") {
		t.Errorf("name should derive from branch: %q", got.Name)
	}
	if got.Slug == "" {
		t.Errorf("slug should be derived from name")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Build Cop: master": "build-cop-master",
		"  Hello   World  ": "hello-world",
		"":                  "build-cop",
		"@@@":               "build-cop",
		"Build Cop / all":   "build-cop-all",
	}
	for in, want := range cases {
		if got := slug.Make(in, "build-cop"); got != want {
			t.Errorf("slug.Make(%q) = %q, want %q", in, got, want)
		}
	}
}

func keysOf(m map[string]jobStats) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// newStore opens a process-local in-memory store for DB-backed reconcile tests.
func newStore(t *testing.T) *db.Store {
	t.Helper()
	s, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func testBoardCfg() BoardConfig {
	return BoardConfig{
		Name:                "Build Cop: test",
		Slug:                "build-cop-test",
		RepoPath:            "/tmp/repo",
		FailureThreshold:    0.10,
		MinRuns:             5,
		GreenStreakRequired: 10,
		WindowDays:          7,
	}
}

// TestEnsureBoardColumnLayout pins the column set for a freshly created Build
// Cop board: Failing/Investigating/Fixed/Won't fix at positions 0..3.
func TestEnsureBoardColumnLayout(t *testing.T) {
	s := newStore(t)
	p := NewPoller(s, nil, Config{}, 0)
	cache, err := p.ensureBoard(context.Background(), testBoardCfg())
	if err != nil {
		t.Fatalf("ensureBoard: %v", err)
	}
	cols, err := s.ListColumns(context.Background(), cache.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	want := []struct {
		name string
		pos  int
	}{
		{ColumnFailing, 0},
		{ColumnInvestigating, 1},
		{ColumnFixed, 2},
		{ColumnWontFix, 3},
	}
	if len(cols) != len(want) {
		t.Fatalf("got %d columns, want %d: %+v", len(cols), len(want), cols)
	}
	for i, w := range want {
		if cols[i].Name != w.name || cols[i].Position != w.pos {
			t.Errorf("column %d = (%q, %d), want (%q, %d)", i, cols[i].Name, cols[i].Position, w.name, w.pos)
		}
	}
	if cache.Cols[ColumnWontFix] == 0 {
		t.Errorf("board cache missing %q", ColumnWontFix)
	}
}

// TestLoadOrCreateBoardBackfillsWontFix covers boards created before the
// "Won't fix" column existed: loading them adds it at the next position, and
// the operation is idempotent.
func TestLoadOrCreateBoardBackfillsWontFix(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	cfg := testBoardCfg()

	// Seed a legacy three-column board, as older versions would have created.
	b := &db.Board{Name: cfg.Name, Slug: cfg.Slug, RepoPath: cfg.RepoPath, BaseBranch: "main"}
	if err := s.CreateBoardRaw(ctx, b); err != nil {
		t.Fatalf("CreateBoardRaw: %v", err)
	}
	for i, name := range []string{ColumnFailing, ColumnInvestigating, ColumnFixed} {
		c := db.Column{BoardID: b.ID, Name: name, Position: i}
		if err := s.CreateColumn(ctx, &c); err != nil {
			t.Fatalf("CreateColumn %q: %v", name, err)
		}
	}

	p := NewPoller(s, nil, Config{}, 0)
	cache, err := p.loadOrCreateBoard(ctx, cfg)
	if err != nil {
		t.Fatalf("loadOrCreateBoard: %v", err)
	}
	if cache.Cols[ColumnWontFix] == 0 {
		t.Fatalf("%q not backfilled into cache", ColumnWontFix)
	}
	wf, err := s.FindColumnByName(ctx, b.ID, ColumnWontFix)
	if err != nil {
		t.Fatalf("FindColumnByName: %v", err)
	}
	if wf.Position != 3 {
		t.Errorf("%q backfilled at position %d, want 3", ColumnWontFix, wf.Position)
	}

	// Backfill must be idempotent — a second load must not add a duplicate.
	if _, err := p.loadOrCreateBoard(ctx, cfg); err != nil {
		t.Fatalf("loadOrCreateBoard (2nd): %v", err)
	}
	cols, err := s.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	if len(cols) != 4 {
		t.Errorf("after re-load got %d columns, want 4: %+v", len(cols), cols)
	}
}

// TestReconcileLeavesWontFixTickets is the core sticky-state guarantee: a
// ticket parked in "Won't fix" is never touched by reconcile, whether the job
// is still failing hard or has fully recovered.
func TestReconcileLeavesWontFixTickets(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	cfg := testBoardCfg()
	p := NewPoller(s, nil, Config{}, 0)
	cache, err := p.ensureBoard(ctx, cfg)
	if err != nil {
		t.Fatalf("ensureBoard: %v", err)
	}

	const wf, job = "CI", "lint"
	fp := computeFingerprint(wf, job)
	wontFixCol := cache.Cols[ColumnWontFix]
	parked := &db.Ticket{
		BoardID:     cache.ID,
		ColumnID:    wontFixCol,
		Title:       "parked title",
		Slug:        "parked",
		Body:        "parked body",
		Fingerprint: fp,
	}
	if err := s.CreateTicket(ctx, parked); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	key := wf + ":" + job
	cases := map[string]map[string]jobStats{
		"still failing": {key: {Workflow: wf, Job: job, Total: 20, Failures: 20}},
		"fully recovered": {key: {Workflow: wf, Job: job, Total: 20, Successes: 20,
			GreenStreak: cfg.GreenStreakRequired + 5}},
	}
	for name, stats := range cases {
		if err := p.reconcile(ctx, cfg, cache, stats); err != nil {
			t.Fatalf("%s: reconcile: %v", name, err)
		}
		got, err := s.GetTicket(ctx, parked.ID)
		if err != nil {
			t.Fatalf("%s: GetTicket: %v", name, err)
		}
		if got.ColumnID != wontFixCol {
			t.Errorf("%s: ticket moved to column %d, want %d (%q)", name, got.ColumnID, wontFixCol, ColumnWontFix)
		}
		if got.Title != parked.Title || got.Body != parked.Body {
			t.Errorf("%s: ticket churned: title=%q body=%q", name, got.Title, got.Body)
		}
	}
}
