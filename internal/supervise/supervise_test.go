package supervise

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/manifest"
	"github.com/jmelahman/local-preview/internal/store"
)

// TestMain doubles as the supervised child: when re-executed with
// --helper-server (or --helper-crash) the test binary acts as a tiny
// backend, so tests need no compiled fixture.
func TestMain(m *testing.M) {
	if i := slices.Index(os.Args, "--helper-server"); i >= 0 && i+2 < len(os.Args) {
		// --needs-marker stands in for a backend that cannot run without the
		// external effect its init created (the per-preview database): the
		// process refuses to start once that effect is gone.
		if j := slices.Index(os.Args, "--needs-marker"); j >= 0 && j+1 < len(os.Args) {
			if _, err := os.Stat(os.Args[j+1]); err != nil {
				fmt.Fprintln(os.Stderr, "helper: init effect missing:", err)
				os.Exit(4)
			}
		}
		runHelperServer(os.Args[i+1], os.Args[i+2])
		return
	}
	if slices.Contains(os.Args, "--helper-crash") {
		fmt.Fprintln(os.Stderr, "helper crashing on purpose")
		os.Exit(3)
	}
	if i := slices.Index(os.Args, "--helper-init"); i >= 0 && i+2 < len(os.Args) {
		os.Exit(runHelperInit(os.Args[i+1], os.Args[i+2]))
	}
	os.Exit(m.Run())
}

// runHelperInit appends one marker per invocation to <stateDir>/init-runs so
// tests can count executions, then behaves per mode: "ok" succeeds, "fail"
// always exits nonzero, "fail-once" fails only the first invocation, "sleep"
// hangs to trip the init timeout, and "marker" writes <stateDir>/init-marker
// — the external effect a --needs-marker run command depends on.
func runHelperInit(stateDir, mode string) int {
	runsFile := filepath.Join(stateDir, "init-runs")
	prev, _ := os.ReadFile(runsFile)
	os.WriteFile(runsFile, append(prev, 'x'), 0o644)
	switch mode {
	case "marker":
		os.WriteFile(markerPath(stateDir), []byte("x"), 0o644)
	case "fail":
		return 3
	case "fail-once":
		if len(prev) == 0 {
			return 3
		}
	case "sleep":
		time.Sleep(30 * time.Second)
	}
	return 0
}

func runHelperServer(portStr, stateDir string) {
	countFile := filepath.Join(stateDir, "count")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		n := 0
		if b, err := os.ReadFile(countFile); err == nil {
			n, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		n++
		os.WriteFile(countFile, []byte(strconv.Itoa(n)), 0o644)
		fmt.Fprintf(w, "%d", n)
	})
	http.ListenAndServe("127.0.0.1:"+portStr, mux) //nolint:errcheck
}

type fixture struct {
	m      *Manager
	db     *db.Store
	files  *store.Store
	repoID int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	root := t.TempDir()
	files := store.New(
		filepath.Join(root, "artifacts"),
		filepath.Join(root, "state"),
		filepath.Join(root, "tmp"),
	)
	m := New(database, files, filepath.Join(root, "logs"))
	m.healthInterval = 25 * time.Millisecond
	t.Cleanup(m.StopAll)
	repo, err := database.CreateRepo("demo", "/src", "/bare", db.RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{m: m, db: database, files: files, repoID: repo.ID}
}

func testExe(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

// provision publishes an (empty) backend artifact dir, a fresh state dir,
// and the backend_artifacts row with the given run argv.
func (f *fixture) provision(t *testing.T, beHash string, argv []string) {
	t.Helper()
	f.provisionIdle(t, beHash, argv, 0)
}

// provisionIdle is provision with an explicit idle_timeout.
func (f *fixture) provisionIdle(t *testing.T, beHash string, argv []string, idle time.Duration) {
	t.Helper()
	f.provisionCfg(t, beHash, manifest.Backend{
		Run:          argv,
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
		IdleTimeout:  manifest.Duration(idle),
	})
}

// provisionCfg is provision with full control over the manifest section.
func (f *fixture) provisionCfg(t *testing.T, beHash string, cfg manifest.Backend) {
	t.Helper()
	f.provisionRunCfg(t, beHash, backendRunConfig{Backend: cfg})
}

// provisionRunCfg is provision with full control over the stored run config.
func (f *fixture) provisionRunCfg(t *testing.T, beHash string, cfg backendRunConfig) {
	t.Helper()
	scratch, _, err := f.files.NewScratchDir("be")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.files.PublishBackend("demo", beHash, scratch, false); err != nil {
		t.Fatal(err)
	}
	if err := f.files.InitFreshStateDir("demo", beHash); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.CreateBackendArtifact(db.BackendArtifact{
		RepoID: f.repoID, BeHash: beHash,
		StateDir:  f.files.StateDirPath("demo", beHash),
		RunConfig: string(raw),
	}); err != nil {
		t.Fatal(err)
	}
}

// provisionFrontend publishes an (empty) process-mode frontend artifact and
// its frontend_artifacts row with the given run argv.
func (f *fixture) provisionFrontend(t *testing.T, feHash string, argv []string) {
	t.Helper()
	scratch, _, err := f.files.NewScratchDir("fe")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.files.PublishFrontend("demo", feHash, scratch, false); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest.Frontend{
		Run:          argv,
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.CreateFrontendArtifact(db.FrontendArtifact{
		RepoID: f.repoID, FeHash: feHash, RunConfig: string(raw),
	}); err != nil {
		t.Fatal(err)
	}
}

// initRuns counts how many times the init helper ran against the state dir.
func (f *fixture) initRuns(t *testing.T, beHash string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.files.StateDirPath("demo", beHash), "init-runs"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	return len(b)
}

func initArgv(t *testing.T, mode string) []string {
	return []string{testExe(t), "--helper-init", "{state_dir}", mode}
}

// markerPath is the file a "marker" init creates and a --needs-marker run
// command requires — the test's stand-in for an init effect that lives
// outside this system and can be deleted behind its back.
func markerPath(stateDir string) string { return filepath.Join(stateDir, "init-marker") }

// markerServerArgv is serverArgv that refuses to start without that effect.
func markerServerArgv(t *testing.T) []string {
	return append(serverArgv(t), "--needs-marker", filepath.Join("{state_dir}", "init-marker"))
}

func serverArgv(t *testing.T) []string {
	return []string{testExe(t), "--helper-server", "{port}", "{state_dir}"}
}

func get(t *testing.T, port int, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func TestEnsureRunningReuseAndStop(t *testing.T) {
	f := newFixture(t)
	f.provision(t, "be-run", serverArgv(t))
	ctx := context.Background()

	port, err := f.m.EnsureRunning(ctx, BackendKey(f.repoID, "be-run"), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, port, "/api/health"); code != 200 {
		t.Fatalf("health = %d", code)
	}
	if got := f.m.Status(BackendKey(f.repoID, "be-run")); got != "running" {
		t.Fatalf("Status = %q", got)
	}

	again, err := f.m.EnsureRunning(ctx, BackendKey(f.repoID, "be-run"), "demo")
	if err != nil || again != port {
		t.Fatalf("second EnsureRunning = %d, %v; want same port %d", again, err, port)
	}

	recs, err := f.db.ListProcessRecords()
	if err != nil || len(recs) != 1 || recs[0].Port != port {
		t.Fatalf("process records = %+v, %v", recs, err)
	}

	f.m.Stop(BackendKey(f.repoID, "be-run"), "test")
	if got := f.m.Status(BackendKey(f.repoID, "be-run")); got != "idle" {
		t.Fatalf("Status after stop = %q", got)
	}
	recs, _ = f.db.ListProcessRecords()
	if len(recs) != 0 {
		t.Fatalf("records after stop = %+v", recs)
	}
}

// TestStartDeployWarmStartsBothSides: StartDeploy brings up the deploy's
// backend and process-mode frontend without any request; a static frontend
// (no frontend_artifacts row) is skipped rather than attempted.
func TestStartDeployWarmStartsBothSides(t *testing.T) {
	f := newFixture(t)
	f.provision(t, "be-warm", serverArgv(t))
	f.provisionFrontend(t, "fe-warm", []string{testExe(t), "--helper-server", "{port}", "."})

	row := db.DeployRow{RepoName: "demo"}
	row.RepoID = f.repoID
	row.BeHash = "be-warm"
	row.FeHash = "fe-warm"
	if err := f.m.StartDeploy(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	if got := f.m.Status(BackendKey(f.repoID, "be-warm")); got != "running" {
		t.Fatalf("backend Status = %q, want running", got)
	}
	if got := f.m.Status(FrontendKey(f.repoID, "fe-warm", "be-warm")); got != "running" {
		t.Fatalf("frontend Status = %q, want running", got)
	}

	// Static frontend: FeHash set but no artifact row → only the backend runs.
	static := db.DeployRow{RepoName: "demo"}
	static.RepoID = f.repoID
	static.BeHash = "be-warm"
	static.FeHash = "fe-static"
	if err := f.m.StartDeploy(context.Background(), static); err != nil {
		t.Fatal(err)
	}
	if got := f.m.Status(FrontendKey(f.repoID, "fe-static", "be-warm")); got != "idle" {
		t.Fatalf("static frontend Status = %q, want idle", got)
	}
}

// TestStopAllRefusesNewStarts: once shutdown began, a straggling start
// trigger must not spawn a child the orchestrator would leak.
func TestStopAllRefusesNewStarts(t *testing.T) {
	f := newFixture(t)
	f.provision(t, "be-late", serverArgv(t))
	f.m.StopAll()
	if _, err := f.m.EnsureRunning(context.Background(), BackendKey(f.repoID, "be-late"), "demo"); err == nil {
		t.Fatal("EnsureRunning after StopAll succeeded; want refusal")
	}
	if got := f.m.Status(BackendKey(f.repoID, "be-late")); got != "idle" {
		t.Fatalf("Status = %q, want idle", got)
	}
}

// TestOutOfBandKillRecovers: a healthy process dying on its own reads
// "crashed" — an idle deploy and a dead one must not look alike — and the
// next start supersedes that record.
func TestOutOfBandKillRecovers(t *testing.T) {
	f := newFixture(t)
	f.provision(t, "be-kill", serverArgv(t))
	ctx := context.Background()
	k := BackendKey(f.repoID, "be-kill")

	port, err := f.m.EnsureRunning(ctx, k, "demo")
	if err != nil {
		t.Fatal(err)
	}
	recs, _ := f.db.ListProcessRecords()
	if len(recs) != 1 {
		t.Fatalf("records = %+v", recs)
	}
	syscall.Kill(-recs[0].PGID, syscall.SIGKILL)

	// The reaper notices and clears state; a new EnsureRunning restarts.
	f.waitStatus(t, k, "crashed")
	fail, ok := f.m.LastFailure(k)
	if !ok || fail.Detail == "" || fail.At.IsZero() {
		t.Fatalf("LastFailure = %+v, %v; want the exit detail", fail, ok)
	}
	newPort, err := f.m.EnsureRunning(ctx, k, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, newPort, "/api/health"); code != 200 {
		t.Fatalf("health after restart = %d", code)
	}
	if got := f.m.Status(k); got != "running" {
		t.Fatalf("Status after restart = %q, want running", got)
	}
	if _, ok := f.m.LastFailure(k); ok {
		t.Fatal("restart did not clear the recorded failure")
	}
	_ = port
}

// TestStopClearsCrash: stopping a crashed key acknowledges it — the deploy
// goes back to reading "idle" rather than a crash nobody can clear.
func TestStopClearsCrash(t *testing.T) {
	f := newFixture(t)
	f.provision(t, "be-ack", []string{testExe(t), "--helper-crash"})
	k := BackendKey(f.repoID, "be-ack")

	if _, err := f.m.EnsureRunning(context.Background(), k, "demo"); err == nil {
		t.Fatal("EnsureRunning succeeded; want a crash")
	}
	if got := f.m.Status(k); got != "crashed" {
		t.Fatalf("Status = %q, want crashed", got)
	}
	f.m.Stop(k, "test")
	if got := f.m.Status(k); got != "idle" {
		t.Fatalf("Status after stop = %q, want idle", got)
	}
}

func TestInstantCrashSurfaces(t *testing.T) {
	f := newFixture(t)
	f.provision(t, "be-crash", []string{testExe(t), "--helper-crash"})
	k := BackendKey(f.repoID, "be-crash")

	_, err := f.m.EnsureRunning(context.Background(), k, "demo")
	if err == nil || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("err = %v, want exit error", err)
	}
	// A start that never reached healthy is a crash too: nothing serves the
	// preview, so "idle" would be a lie.
	if got := f.m.Status(k); got != "crashed" {
		t.Fatalf("Status = %q, want crashed", got)
	}
	// The exit status beats "never went healthy" as an explanation.
	if fail, ok := f.m.LastFailure(k); !ok || !strings.Contains(fail.Detail, "exit status 3") {
		t.Fatalf("LastFailure = %+v, %v; want the helper's exit status", fail, ok)
	}
}

func TestInitRunsOncePerArtifact(t *testing.T) {
	f := newFixture(t)
	f.provisionCfg(t, "be-init", manifest.Backend{
		Init:         [][]string{initArgv(t, "ok")},
		Run:          serverArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	ctx := context.Background()
	k := BackendKey(f.repoID, "be-init")

	if _, err := f.m.EnsureRunning(ctx, k, "demo"); err != nil {
		t.Fatal(err)
	}
	if runs := f.initRuns(t, "be-init"); runs != 1 {
		t.Fatalf("init runs after first start = %d, want 1", runs)
	}
	art, err := f.db.GetBackendArtifact(f.repoID, "be-init")
	if err != nil || art.InitDoneAt == "" {
		t.Fatalf("artifact after init = %+v, %v", art, err)
	}

	// A cold start of the same artifact must skip init.
	f.m.Stop(k, "test")
	port, err := f.m.EnsureRunning(ctx, k, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, port, "/api/health"); code != 200 {
		t.Fatalf("health after cold start = %d", code)
	}
	if runs := f.initRuns(t, "be-init"); runs != 1 {
		t.Fatalf("init runs after cold start = %d, want still 1", runs)
	}
}

// publishFiles publishes only the artifact *files* — the worker case, where
// files hydrate from the tier but no DB row exists.
func (f *fixture) publishFiles(t *testing.T, beHash string) {
	t.Helper()
	scratch, _, err := f.files.NewScratchDir("be")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.files.PublishBackend("demo", beHash, scratch, false); err != nil {
		t.Fatal(err)
	}
}

func wireCfg(t *testing.T, cfg manifest.Backend) WireSpec {
	t.Helper()
	raw, err := json.Marshal(backendRunConfig{Backend: cfg})
	if err != nil {
		t.Fatal(err)
	}
	return WireSpec{RunConfig: string(raw)}
}

// TestWireSpecServesWithoutDBRow: a Manager whose DB has no artifact rows (a
// worker) serves from an offered wire spec, recomputing the state dir against
// its own store root.
func TestWireSpecServesWithoutDBRow(t *testing.T) {
	f := newFixture(t)
	const beHash = "be-wire"
	f.publishFiles(t, beHash)
	k := BackendKey(f.repoID, beHash)
	f.m.OfferWireSpec(k, wireCfg(t, manifest.Backend{
		Run:          serverArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	}))

	port, err := f.m.EnsureRunning(context.Background(), k, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, port, "/api/health"); code != 200 {
		t.Fatalf("health = %d", code)
	}
	if !f.files.HasStateDir("demo", beHash) {
		t.Fatal("wire-served start did not create a node-local state dir")
	}
}

// TestWireSpecInitSticky: the control node never learns an init ran on this
// node, so it re-sends InitDone=false with every ensure — a re-offer must not
// make the next cold start re-run init.
func TestWireSpecInitSticky(t *testing.T) {
	f := newFixture(t)
	const beHash = "be-wire-init"
	f.publishFiles(t, beHash)
	k := BackendKey(f.repoID, beHash)
	spec := wireCfg(t, manifest.Backend{
		Init:         [][]string{initArgv(t, "ok")},
		Run:          serverArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	ctx := context.Background()

	f.m.OfferWireSpec(k, spec)
	if _, err := f.m.EnsureRunning(ctx, k, "demo"); err != nil {
		t.Fatal(err)
	}
	if runs := f.initRuns(t, beHash); runs != 1 {
		t.Fatalf("init runs after first start = %d, want 1", runs)
	}

	// Cold start after a fresh offer with InitDone=false (what control sends).
	f.m.Stop(k, "test")
	f.m.OfferWireSpec(k, spec)
	if _, err := f.m.EnsureRunning(ctx, k, "demo"); err != nil {
		t.Fatal(err)
	}
	if runs := f.initRuns(t, beHash); runs != 1 {
		t.Fatalf("init runs after re-offer + cold start = %d, want still 1", runs)
	}
}

// TestWireSpecDBWins: on a node that builds (control / --role=all), the DB row
// stays authoritative over any offered spec.
func TestWireSpecDBWins(t *testing.T) {
	f := newFixture(t)
	const beHash = "be-db-wins"
	f.provision(t, beHash, serverArgv(t))
	k := BackendKey(f.repoID, beHash)
	// A bogus wire spec that could never start a process.
	f.m.OfferWireSpec(k, wireCfg(t, manifest.Backend{
		Run:          []string{"/nonexistent-binary"},
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(2 * time.Second),
	}))

	port, err := f.m.EnsureRunning(context.Background(), k, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, port, "/api/health"); code != 200 {
		t.Fatalf("health = %d", code)
	}
}

func TestResolveSecretEnv(t *testing.T) {
	t.Setenv("PREVIEW_SECRET_PG_PASSWORD", "hunter2")
	env, err := resolveSecretEnv(map[string]string{
		"POSTGRES_PASSWORD": "{secret:PG_PASSWORD}",
		"COMPOSITE":         "prefix-{secret:PG_PASSWORD}-suffix",
		"PLAIN":             "left alone",
	})
	if err != nil {
		t.Fatal(err)
	}
	if env["POSTGRES_PASSWORD"] != "hunter2" || env["COMPOSITE"] != "prefix-hunter2-suffix" || env["PLAIN"] != "left alone" {
		t.Fatalf("env = %+v", env)
	}
}

func TestResolveSecretEnvMissingFailsLoudly(t *testing.T) {
	_, err := resolveSecretEnv(map[string]string{"X": "{secret:DEFINITELY_UNSET_1}"})
	if err == nil || !strings.Contains(err.Error(), "PREVIEW_SECRET_DEFINITELY_UNSET_1") {
		t.Fatalf("err = %v, want the namespaced variable named", err)
	}
}

// TestSecretEnvMissingFailsStart: a start referencing an unset secret must
// fail the attempt (surfacing as "crashed"), never export an empty credential.
func TestSecretEnvMissingFailsStart(t *testing.T) {
	f := newFixture(t)
	f.provisionCfg(t, "be-secret", manifest.Backend{
		Run:          serverArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
		Env:          map[string]string{"TOKEN": "{secret:UNSET_FOR_TEST}"},
	})
	_, err := f.m.EnsureRunning(context.Background(), BackendKey(f.repoID, "be-secret"), "demo")
	if err == nil || !strings.Contains(err.Error(), "PREVIEW_SECRET_UNSET_FOR_TEST") {
		t.Fatalf("err = %v, want undefined-secret failure", err)
	}
}

func TestInitFailureRetriesNextStart(t *testing.T) {
	f := newFixture(t)
	f.provisionCfg(t, "be-init-retry", manifest.Backend{
		Init:         [][]string{initArgv(t, "fail-once")},
		Run:          serverArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	ctx := context.Background()
	k := BackendKey(f.repoID, "be-init-retry")

	_, err := f.m.EnsureRunning(ctx, k, "demo")
	if err == nil || !strings.Contains(err.Error(), "init") {
		t.Fatalf("err = %v, want init failure", err)
	}
	if art, err := f.db.GetBackendArtifact(f.repoID, "be-init-retry"); err != nil || art.InitDoneAt != "" {
		t.Fatalf("failed init must not be recorded done: %+v, %v", art, err)
	}
	if got := f.m.Status(k); got != "crashed" {
		t.Fatalf("Status after init failure = %q, want crashed", got)
	}

	// The next start attempt re-runs init, which now succeeds.
	if _, err := f.m.EnsureRunning(ctx, k, "demo"); err != nil {
		t.Fatal(err)
	}
	if runs := f.initRuns(t, "be-init-retry"); runs != 2 {
		t.Fatalf("init runs = %d, want 2", runs)
	}
	if art, _ := f.db.GetBackendArtifact(f.repoID, "be-init-retry"); art.InitDoneAt == "" {
		t.Fatal("init success was not recorded")
	}
}

func TestInitTimeout(t *testing.T) {
	f := newFixture(t)
	f.provisionCfg(t, "be-init-slow", manifest.Backend{
		Init:         [][]string{initArgv(t, "sleep")},
		InitTimeout:  manifest.Duration(200 * time.Millisecond),
		Run:          serverArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})

	_, err := f.m.EnsureRunning(context.Background(), BackendKey(f.repoID, "be-init-slow"), "demo")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want init timeout", err)
	}
}

// --- lineage fork tests ---

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	runTestGit(t, dir, "add", "-A")
	runTestGit(t, dir, "commit", "-qm", msg)
	return runTestGit(t, dir, "rev-parse", "HEAD")
}

// waitStatus polls Status until want or the deadline.
func (f *fixture) waitStatus(t *testing.T, k Key, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for f.m.Status(k) != want {
		if time.Now().After(deadline) {
			t.Fatalf("status(%v) = %q, want %q", k, f.m.Status(k), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestIdleReap(t *testing.T) {
	f := newFixture(t)
	f.m.reapInterval = 40 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	f.m.StartReaper(ctx)

	f.provisionIdle(t, "be-idle", serverArgv(t), 150*time.Millisecond)
	k := BackendKey(f.repoID, "be-idle")
	if _, err := f.m.EnsureRunning(context.Background(), k, "demo"); err != nil {
		t.Fatal(err)
	}
	// Untouched past its idle_timeout → the reaper stops it.
	f.waitStatus(t, k, "idle")
}

func TestLRUWarmCap(t *testing.T) {
	f := newFixture(t)
	f.m.reapInterval = 40 * time.Millisecond
	f.m.SetMaxWarm(1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	f.m.StartReaper(ctx)

	f.provision(t, "be-old", serverArgv(t))
	f.provision(t, "be-new", serverArgv(t))
	kOld := BackendKey(f.repoID, "be-old")
	kNew := BackendKey(f.repoID, "be-new")
	if _, err := f.m.EnsureRunning(context.Background(), kOld, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.EnsureRunning(context.Background(), kNew, "demo"); err != nil {
		t.Fatal(err)
	}
	// The target is soft: both were just used, so both stay despite target 1.
	// Once the older one goes quiet (past the active window), the reaper
	// prunes it back to the target; the newer one survives.
	f.setTouch(t, kOld, time.Now().Add(-2*warmActiveWindow))
	f.waitStatus(t, kOld, "idle")
	if got := f.m.Status(kNew); got != "running" {
		t.Fatalf("newer backend = %q, want running", got)
	}
}

func TestForkOrInitStateDir(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// Source repo: c1 then c2.
	src := t.TempDir()
	runTestGit(t, src, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	c1 := commit(t, src, "c1")
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	c2 := commit(t, src, "c2")

	mgr := gitrepo.NewManager(filepath.Join(t.TempDir(), "repos"))
	repo, err := mgr.Add(ctx, "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// c1 was deployed with backend be1; give be1 observable state.
	f.provision(t, "be1", serverArgv(t))
	d1, err := f.db.CreateDeploy(f.repoID, c1, db.DeployMeta{Ref: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.SetDeployHashes(d1.ID, "fe1", "be1", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.db.SetDeployReady(d1.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.files.StateDirPath("demo", "be1"), "count"), []byte("42"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Start be1 so the fork has to quiesce it.
	if _, err := f.m.EnsureRunning(ctx, BackendKey(f.repoID, "be1"), "demo"); err != nil {
		t.Fatal(err)
	}

	// New backend hash at c2 forks be1's state.
	cfg, _ := json.Marshal(manifest.Backend{Run: serverArgv(t), HealthPath: "/api/health"})
	if err := f.m.ForkOrInitStateDir(ctx, repo, f.repoID, "demo", "be2", c2, string(cfg)); err != nil {
		t.Fatal(err)
	}
	if got := f.m.Status(BackendKey(f.repoID, "be1")); got != "idle" {
		t.Fatalf("ancestor status after fork = %q, want idle (quiesced)", got)
	}
	forked, err := os.ReadFile(filepath.Join(f.files.StateDirPath("demo", "be2"), "count"))
	if err != nil {
		t.Fatal(err)
	}
	if string(forked) != "42" {
		t.Fatalf("forked state = %q, want 42", forked)
	}
	art, err := f.db.GetBackendArtifact(f.repoID, "be2")
	if err != nil || art.ForkedFrom != "be1" {
		t.Fatalf("artifact row = %+v, %v", art, err)
	}

	// Idempotent.
	if err := f.m.ForkOrInitStateDir(ctx, repo, f.repoID, "demo", "be2", c2, string(cfg)); err != nil {
		t.Fatal(err)
	}

	// A commit with no deployed ancestry gets fresh state.
	if err := f.m.ForkOrInitStateDir(ctx, repo, f.repoID, "demo", "be3", c1, string(cfg)); err != nil {
		t.Fatal(err)
	}
	art3, err := f.db.GetBackendArtifact(f.repoID, "be3")
	if err != nil || art3.ForkedFrom != "" {
		t.Fatalf("fresh artifact row = %+v, %v", art3, err)
	}
	if _, err := os.Stat(filepath.Join(f.files.StateDirPath("demo", "be3"), "count")); !os.IsNotExist(err) {
		t.Fatal("fresh state dir should be empty")
	}
}

// TestIdleOverrideGovernsRunningProcesses: the dashboard's idle override
// applies at reap time, so shortening it stops previews that started under a
// longer (manifest) timeout.
func TestIdleOverrideGovernsRunningProcesses(t *testing.T) {
	f := newFixture(t)
	f.provisionIdle(t, "be-idle-override", serverArgv(t), time.Hour)
	k := BackendKey(f.repoID, "be-idle-override")
	if _, err := f.m.EnsureRunning(context.Background(), k, "demo"); err != nil {
		t.Fatal(err)
	}

	// An hour-long manifest timeout would never reap this; the override does.
	f.m.SetIdleOverride(time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	f.m.reap()
	if st := f.m.Status(k); st != StatusIdle {
		t.Fatalf("status after override reap = %q, want idle", st)
	}

	// Clearing the override restores the manifest value.
	f.m.SetIdleOverride(0)
	if _, err := f.m.EnsureRunning(context.Background(), k, "demo"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	f.m.reap()
	if st := f.m.Status(k); st != StatusRunning {
		t.Fatalf("status with override cleared = %q, want running (1h manifest timeout)", st)
	}
}

// setTouch backdates a process's recency, simulating idleness.
func (f *fixture) setTouch(t *testing.T, k Key, at time.Time) {
	t.Helper()
	f.m.mu.Lock()
	defer f.m.mu.Unlock()
	p := f.m.procs[k]
	if p == nil {
		t.Fatalf("no tracked process for %v", k)
	}
	p.lastTouch = at
}

// TestWarmTargetSparesActive: the warm target is soft — a burst above it is
// served in full, and only genuinely idle processes are pruned back.
func TestWarmTargetSparesActive(t *testing.T) {
	f := newFixture(t)
	f.provisionIdle(t, "be-active", serverArgv(t), time.Hour)
	f.provisionIdle(t, "be-stale", serverArgv(t), time.Hour)
	ctx := context.Background()
	active := BackendKey(f.repoID, "be-active")
	stale := BackendKey(f.repoID, "be-stale")
	for _, k := range []Key{active, stale} {
		if _, err := f.m.EnsureRunning(ctx, k, "demo"); err != nil {
			t.Fatal(err)
		}
	}

	// Both actively used: two over a target of one, and neither is pruned.
	f.m.SetMaxWarm(1)
	f.m.reap()
	if f.m.Status(active) != StatusRunning || f.m.Status(stale) != StatusRunning {
		t.Fatalf("an actively-used process was pruned for the target: active=%s stale=%s",
			f.m.Status(active), f.m.Status(stale))
	}

	// One goes quiet: the next reap prunes exactly it.
	f.setTouch(t, stale, time.Now().Add(-2*warmActiveWindow))
	f.m.reap()
	if got := f.m.Status(stale); got != StatusIdle {
		t.Fatalf("stale process = %s, want pruned to the target", got)
	}
	if got := f.m.Status(active); got != StatusRunning {
		t.Fatalf("active process = %s, want spared", got)
	}
}

// TestMinWarmExemptFromIdle: the floor keeps the most-recent processes warm
// past their idle timeout; older ones still idle out.
func TestMinWarmExemptFromIdle(t *testing.T) {
	f := newFixture(t)
	f.provisionIdle(t, "be-recent", serverArgv(t), time.Minute)
	f.provisionIdle(t, "be-old", serverArgv(t), time.Minute)
	ctx := context.Background()
	recent := BackendKey(f.repoID, "be-recent")
	old := BackendKey(f.repoID, "be-old")
	for _, k := range []Key{old, recent} {
		if _, err := f.m.EnsureRunning(ctx, k, "demo"); err != nil {
			t.Fatal(err)
		}
	}
	// Both idle far beyond their 1m timeout; "recent" less long ago.
	f.setTouch(t, old, time.Now().Add(-time.Hour))
	f.setTouch(t, recent, time.Now().Add(-30*time.Minute))

	f.m.SetMinWarm(1)
	f.m.reap()
	if got := f.m.Status(recent); got != StatusRunning {
		t.Fatalf("floor-protected process = %s, want running past its idle timeout", got)
	}
	if got := f.m.Status(old); got != StatusIdle {
		t.Fatalf("unprotected idle process = %s, want stopped", got)
	}
}

// TestHitStatsCountJoinedStartsCold: requests that join an in-flight cold
// start wait it out like the initiator, so they count cold — a page load
// firing many asset requests during one cold start must not record a pile of
// warm hits.
func TestHitStatsCountJoinedStartsCold(t *testing.T) {
	f := newFixture(t)
	f.provisionCfg(t, "be-hits", manifest.Backend{
		Init:         [][]string{{testExe(t), "--helper-init", "{state_dir}", "sleep"}},
		InitTimeout:  manifest.Duration(2 * time.Second),
		Run:          serverArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	k := BackendKey(f.repoID, "be-hits")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	// Initiator plus two joiners, all while init sleeps: three cold, no warm.
	for range 3 {
		f.m.EnsureRunning(ctx, k, "demo") //nolint:errcheck // ctx expiry expected
	}
	if warm, cold := f.m.HitStats(); warm != 0 || cold != 3 {
		t.Fatalf("hits during in-flight start = warm %d cold %d, want 0/3", warm, cold)
	}
	f.m.Stop(k, "test")
}

// recordEvent both persists locally and buffers for the worker heartbeat;
// DrainEvents empties the buffer (at-most-once delivery to the control node)
// and preserves order and timestamps.
func TestDrainEventsBuffersAndResets(t *testing.T) {
	f := newFixture(t)
	f.m.recordEvent(f.repoID, "beH", "start_attempt", "")
	f.m.recordEvent(f.repoID, "beH", "healthy", "port 1")

	evs := f.m.DrainEvents()
	if len(evs) != 2 || evs[0].Event != "start_attempt" || evs[1].Event != "healthy" {
		t.Fatalf("drained %+v", evs)
	}
	if evs[0].OccurredAt.IsZero() || evs[1].OccurredAt.Before(evs[0].OccurredAt) {
		t.Fatalf("timestamps not sane: %+v", evs)
	}
	if again := f.m.DrainEvents(); len(again) != 0 {
		t.Fatalf("redelivered %+v", again)
	}
	// The local trail got them too (single-node deployments read it directly).
	durs, err := f.db.StartupDurations(30)
	if err != nil || len(durs) != 1 {
		t.Fatalf("local trail durations = %v, %v", durs, err)
	}
}

// drainedEvents collects the event names buffered since the last drain.
func (f *fixture) drainedEvents(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, e := range f.m.DrainEvents() {
		names = append(names, e.Event)
	}
	return names
}

// TestFailedStartRevokesSkippedInit: init's effects can live outside this
// system (the per-preview Postgres database an onyx init creates), where a
// reaper can delete them while init_done_at still says "done". A start that
// skipped init and then failed revokes the record — in memory and in the
// row — so the next start re-runs init and repairs the effect.
func TestFailedStartRevokesSkippedInit(t *testing.T) {
	f := newFixture(t)
	const beHash = "be-revoke"
	f.provisionCfg(t, beHash, manifest.Backend{
		Init:         [][]string{initArgv(t, "marker")},
		Run:          markerServerArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	ctx := context.Background()
	k := BackendKey(f.repoID, beHash)
	marker := markerPath(f.files.StateDirPath("demo", beHash))

	if _, err := f.m.EnsureRunning(ctx, k, "demo"); err != nil {
		t.Fatal(err)
	}
	if runs := f.initRuns(t, beHash); runs != 1 {
		t.Fatalf("init runs after first start = %d, want 1", runs)
	}
	if art, err := f.db.GetBackendArtifact(f.repoID, beHash); err != nil || art.InitDoneAt == "" {
		t.Fatalf("artifact after init = %+v, %v", art, err)
	}

	// The effect vanishes out from under the flag.
	f.m.Stop(k, "test")
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	f.drainedEvents(t)

	if _, err := f.m.EnsureRunning(ctx, k, "demo"); err == nil {
		t.Fatal("expected the start to fail with the init effect gone")
	}
	if runs := f.initRuns(t, beHash); runs != 1 {
		t.Fatalf("init runs on the skipping start = %d, want still 1", runs)
	}
	if art, err := f.db.GetBackendArtifact(f.repoID, beHash); err != nil || art.InitDoneAt != "" {
		t.Fatalf("init done after a failed start that skipped init = %+v, %v; want cleared", art, err)
	}
	if ev := f.drainedEvents(t); !slices.Contains(ev, "init_revoked") {
		t.Fatalf("events = %v, want an init_revoked", ev)
	}

	// The next start re-runs init, which recreates the effect.
	port, err := f.m.EnsureRunning(ctx, k, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, port, "/api/health"); code != 200 {
		t.Fatalf("health after re-init = %d", code)
	}
	if runs := f.initRuns(t, beHash); runs != 2 {
		t.Fatalf("init runs after revocation = %d, want 2", runs)
	}
	if art, _ := f.db.GetBackendArtifact(f.repoID, beHash); art.InitDoneAt == "" {
		t.Fatal("re-run init success was not recorded")
	}
}

// TestFailedStartAfterInitRanDoesNotRevoke: only a start that SKIPPED init is
// evidence the recorded init is stale. One that just ran init and still failed
// health has already paid for it — revoking there would re-run init on every
// retry of a backend that simply cannot start.
func TestFailedStartAfterInitRanDoesNotRevoke(t *testing.T) {
	f := newFixture(t)
	const beHash = "be-no-revoke"
	f.provisionCfg(t, beHash, manifest.Backend{
		// "ok" init never creates the marker the run command demands.
		Init:         [][]string{initArgv(t, "ok")},
		Run:          markerServerArgv(t),
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	k := BackendKey(f.repoID, beHash)

	if _, err := f.m.EnsureRunning(context.Background(), k, "demo"); err == nil {
		t.Fatal("expected the start to fail")
	}
	if runs := f.initRuns(t, beHash); runs != 1 {
		t.Fatalf("init runs = %d, want 1", runs)
	}
	if art, err := f.db.GetBackendArtifact(f.repoID, beHash); err != nil || art.InitDoneAt == "" {
		t.Fatalf("init that ran and succeeded must stay recorded: %+v, %v", art, err)
	}
	if ev := f.drainedEvents(t); slices.Contains(ev, "init_revoked") {
		t.Fatalf("events = %v, want no init_revoked", ev)
	}
}
