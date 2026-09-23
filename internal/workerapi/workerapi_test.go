package workerapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/execstream"
	"github.com/jmelahman/local-preview/internal/manifest"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// TestMain doubles as the supervised child (same trick as the supervise
// package's tests): re-executed with --helper-server it serves a health
// endpoint, so the worker-over-empty-DB test starts a real process.
func TestMain(m *testing.M) {
	if i := slices.Index(os.Args, "--helper-init"); i >= 0 && i+1 < len(os.Args) {
		// Stands in for an init step whose effect lives in an external
		// service (the per-preview database): it creates the file the run
		// command below refuses to start without.
		os.WriteFile(os.Args[i+1], []byte("x"), 0o644) //nolint:errcheck
		return
	}
	if i := slices.Index(os.Args, "--helper-server"); i >= 0 && i+1 < len(os.Args) {
		if j := slices.Index(os.Args, "--needs-marker"); j >= 0 && j+1 < len(os.Args) {
			if _, err := os.Stat(os.Args[j+1]); err != nil {
				os.Exit(4)
			}
		}
		http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		http.ListenAndServe("127.0.0.1:"+os.Args[i+1], nil) //nolint:errcheck
		return
	}
	os.Exit(m.Run())
}

// fakeSup records calls and returns programmed results.
type fakeSup struct {
	mu           sync.Mutex
	ensureKey    supervise.Key
	ensureRepo   string
	ensurePort   int
	ensureErr    error
	offered      map[supervise.Key]supervise.WireSpec
	stopped      []supervise.Key
	status       string
	failDetail   string
	report       []supervise.ProcReport
	runLog       supervise.RunLog
	running      int
	maxWarm      int
	minWarm      int
	idleOverride time.Duration
	warmHits     int64
	coldStarts   int64
	events       []supervise.ProcEventRecord
	execFn       func(supervise.Key, supervise.ExecOptions, io.ReadWriter) error
}

func (f *fakeSup) OfferWireSpec(k supervise.Key, s supervise.WireSpec) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.offered == nil {
		f.offered = map[supervise.Key]supervise.WireSpec{}
	}
	f.offered[k] = s
}

func (f *fakeSup) EnsureRunning(_ context.Context, k supervise.Key, repo string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureKey = k
	f.ensureRepo = repo
	return f.ensurePort, f.ensureErr
}
func (f *fakeSup) Stop(k supervise.Key, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, k)
}
func (f *fakeSup) Status(k supervise.Key) string { return f.status }
func (f *fakeSup) LastFailure(k supervise.Key) (supervise.Failure, bool) {
	return supervise.Failure{Detail: f.failDetail}, f.failDetail != ""
}
func (f *fakeSup) Report(context.Context) []supervise.ProcReport { return f.report }
func (f *fakeSup) RunLog(repo, side, hash string, attempt int, offset int64) (supervise.RunLog, error) {
	return f.runLog, nil
}
func (f *fakeSup) Exec(_ context.Context, k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error {
	if f.execFn != nil {
		return f.execFn(k, opts, stream)
	}
	return nil
}
func (f *fakeSup) SetMaxWarm(n int)                { f.maxWarm = n }
func (f *fakeSup) MinWarm() int                    { return f.minWarm }
func (f *fakeSup) SetMinWarm(n int)                { f.minWarm = n }
func (f *fakeSup) IdleOverride() time.Duration     { return f.idleOverride }
func (f *fakeSup) SetIdleOverride(d time.Duration) { f.idleOverride = d }
func (f *fakeSup) Running() int                    { return f.running }
func (f *fakeSup) MaxWarm() int                    { return f.maxWarm }
func (f *fakeSup) HitStats() (warm, cold int64)    { return f.warmHits, f.coldStarts }
func (f *fakeSup) DrainEvents() []supervise.ProcEventRecord {
	out := f.events
	f.events = nil
	return out
}

const secret = "s3cr3t"

func testServer(t *testing.T, sup Supervisor) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(NewServer(sup, secret).Handler())
	c := NewClient(srv.URL, "10.0.0.7", secret, srv.Client())
	return c, srv.Close
}

func TestEnsureRoundTrip(t *testing.T) {
	sup := &fakeSup{ensurePort: 42123}
	c, done := testServer(t, sup)
	defer done()

	k := supervise.FrontendKey(7, "feHASH", "beHASH")
	addr, err := c.EnsureRunning(context.Background(), k, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "10.0.0.7:42123" {
		t.Fatalf("addr = %q, want 10.0.0.7:42123", addr)
	}
	if sup.ensureKey != k || sup.ensureRepo != "demo" {
		t.Fatalf("worker saw key=%+v repo=%q", sup.ensureKey, sup.ensureRepo)
	}
}

func TestEnsureErrorPropagates(t *testing.T) {
	sup := &fakeSup{ensureErr: errors.New("artifact files missing (evicted?)")}
	c, done := testServer(t, sup)
	defer done()
	_, err := c.EnsureRunning(context.Background(), supervise.BackendKey(1, "h"), "demo")
	if err == nil || err.Error() != "artifact files missing (evicted?)" {
		t.Fatalf("err = %v, want the worker's detail", err)
	}
}

func TestStopAndStatus(t *testing.T) {
	sup := &fakeSup{status: supervise.StatusRunning}
	c, done := testServer(t, sup)
	defer done()

	k := supervise.BackendKey(3, "h3")
	if err := c.Stop(context.Background(), k, "manual"); err != nil {
		t.Fatal(err)
	}
	if len(sup.stopped) != 1 || sup.stopped[0] != k {
		t.Fatalf("stopped = %v, want [%v]", sup.stopped, k)
	}
	st, _, err := c.Status(context.Background(), k)
	if err != nil || st != supervise.StatusRunning {
		t.Fatalf("status = %q, %v", st, err)
	}
}

// TestStatusCarriesFailureDetail: a crashed status travels with its cause.
func TestStatusCarriesFailureDetail(t *testing.T) {
	sup := &fakeSup{status: supervise.StatusCrashed, failDetail: "exit status 3"}
	c, done := testServer(t, sup)
	defer done()
	st, detail, err := c.Status(context.Background(), supervise.BackendKey(1, "h"))
	if err != nil || st != supervise.StatusCrashed || detail != "exit status 3" {
		t.Fatalf("status = %q detail = %q, %v", st, detail, err)
	}
}

// TestReportAndRunLogRoundTrip: the bulk report and run-log slices cross the
// wire intact — keys, stats, and chunk fields alike.
func TestReportAndRunLogRoundTrip(t *testing.T) {
	cpu := 12.5
	touch := time.Now().Add(-3 * time.Minute).Truncate(time.Second)
	k := supervise.FrontendKey(7, "feHASH", "beHASH")
	sup := &fakeSup{
		report: []supervise.ProcReport{{
			Key: k, Repo: "demo", Status: supervise.StatusRunning, LastTouch: touch,
			Stats: &supervise.ProcessStats{Runtime: "container", CPUPercent: &cpu, MemoryBytes: 42, MemoryLimitBytes: 100},
		}, {
			Key: supervise.BackendKey(7, "beHASH"), Status: supervise.StatusCrashed, Error: "boom",
		}},
		runLog: supervise.RunLog{Attempt: 3, Offset: 17, Content: "hello", Truncated: true},
	}
	c, done := testServer(t, sup)
	defer done()

	procs, err := c.Report(context.Background())
	if err != nil || len(procs) != 2 {
		t.Fatalf("report = %+v, %v", procs, err)
	}
	if procs[0].Key != k || procs[0].Repo != "demo" || procs[0].Stats == nil || *procs[0].Stats.CPUPercent != cpu {
		t.Fatalf("proc[0] = %+v", procs[0])
	}
	// Recency survives the wire — the control node's fleet-wide min-warm
	// ranking depends on it.
	if !procs[0].LastTouch.Equal(touch) {
		t.Fatalf("proc[0].LastTouch = %v, want %v", procs[0].LastTouch, touch)
	}
	if procs[1].Status != supervise.StatusCrashed || procs[1].Error != "boom" {
		t.Fatalf("proc[1] = %+v", procs[1])
	}
	if !procs[1].LastTouch.IsZero() {
		t.Fatalf("crashed record LastTouch = %v, want zero (omitted on the wire)", procs[1].LastTouch)
	}

	chunk, err := c.RunLog(context.Background(), "demo", "fe", "feHASH", 0, 0)
	if err != nil || chunk != sup.runLog {
		t.Fatalf("runlog = %+v, %v", chunk, err)
	}
}

func TestHeartbeat(t *testing.T) {
	sup := &fakeSup{running: 5, maxWarm: 12, events: []supervise.ProcEventRecord{
		{RepoID: 7, BeHash: "h", Event: "healthy", Detail: "port 1"},
	}}
	c, done := testServer(t, sup)
	defer done()
	hb, err := c.Heartbeat(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hb.Running != 5 || hb.MaxWarm != 12 || hb.Draining {
		t.Fatalf("heartbeat = %+v", hb)
	}
	// Buffered process events ride along, drained at read: a second
	// heartbeat must not redeliver them.
	if len(hb.Events) != 1 || hb.Events[0].Event != "healthy" || hb.Events[0].RepoID != 7 {
		t.Fatalf("heartbeat events = %+v", hb.Events)
	}
	if hb2, _ := c.Heartbeat(context.Background()); len(hb2.Events) != 0 {
		t.Fatalf("events redelivered: %+v", hb2.Events)
	}
}

// TestEnsureWireSpecOnEmptyDB is the regression test for the worker-tier gap
// the fakes cannot see: a real supervise.Manager over a fresh, empty DB — a
// --role=worker node — must start a process from the wire-supplied spec
// alone. Before specs traveled on the wire, this failed "backend artifact not
// provisioned" and every remote ensure was a 502.
func TestEnsureWireSpecOnEmptyDB(t *testing.T) {
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
	m := supervise.New(database, files, filepath.Join(root, "logs"))
	t.Cleanup(m.StopAll)

	// The artifact *files* are resident (in production: hydrated from the S3
	// tier); only the DB rows are absent.
	const beHash = "behash4worker001"
	scratch, _, err := files.NewScratchDir("be")
	if err != nil {
		t.Fatal(err)
	}
	if err := files.PublishBackend("demo", beHash, scratch, false); err != nil {
		t.Fatal(err)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest.Backend{
		Run:          []string{exe, "--helper-server", "{port}"},
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewServer(m, secret).Handler())
	defer srv.Close()
	c := NewClient(srv.URL, "127.0.0.1", secret, srv.Client())
	c.SpecResolver = func(k supervise.Key) (supervise.WireSpec, error) {
		return supervise.WireSpec{RunConfig: string(raw)}, nil
	}

	addr, err := c.EnsureRunning(context.Background(), supervise.BackendKey(1, beHash), "demo")
	if err != nil {
		t.Fatalf("ensure over empty DB with wire spec: %v", err)
	}
	res, err := http.Get(fmt.Sprintf("http://%s/api/health", addr))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health = %d, want 200", res.StatusCode)
	}
	// The worker recomputed the state dir against its own store root.
	if _, err := os.Stat(files.StateDirPath("demo", beHash)); err != nil {
		t.Fatalf("worker-local state dir: %v", err)
	}
}

// TestEnsureCarriesSpecs asserts the wire shape: the resolver's spec (and the
// peer backend's, for a process-mode frontend) reach the worker's supervisor.
func TestEnsureCarriesSpecs(t *testing.T) {
	sup := &fakeSup{ensurePort: 1}
	c, done := testServer(t, sup)
	defer done()
	c.SpecResolver = func(k supervise.Key) (supervise.WireSpec, error) {
		return supervise.WireSpec{RunConfig: `{"side":"` + string(k.Side) + `"}`, InitDone: k.Side == supervise.SideBackend}, nil
	}

	k := supervise.FrontendKey(7, "feHASH", "beHASH")
	if _, err := c.EnsureRunning(context.Background(), k, "demo"); err != nil {
		t.Fatal(err)
	}
	sup.mu.Lock()
	defer sup.mu.Unlock()
	fe, ok := sup.offered[k]
	if !ok || fe.RunConfig != `{"side":"fe"}` || fe.InitDone {
		t.Fatalf("frontend spec = %+v, %v", fe, ok)
	}
	be, ok := sup.offered[supervise.BackendKey(7, "beHASH")]
	if !ok || be.RunConfig != `{"side":"be"}` || !be.InitDone {
		t.Fatalf("peer backend spec = %+v, %v", be, ok)
	}
}

// TestEnsureAdoptsRemoteInitDone asserts the control side records a worker's
// init result: a successful backend ensure whose shipped spec had
// InitDone=false invokes InitMarker exactly once. An already-done spec, a
// frontend ensure (which proves nothing about the peer backend — it only
// starts one when {backend_url} is referenced), and a failed ensure must not.
// Without adoption, init_done_at never reached a routing-only control node's
// DB and every cold placement on a fresh worker re-ran init.
func TestEnsureAdoptsRemoteInitDone(t *testing.T) {
	sup := &fakeSup{ensurePort: 1}
	c, done := testServer(t, sup)
	defer done()

	var marked []supervise.Key
	c.InitMarker = func(k supervise.Key) error {
		marked = append(marked, k)
		return nil
	}
	initDone := false
	c.SpecResolver = func(k supervise.Key) (supervise.WireSpec, error) {
		return supervise.WireSpec{RunConfig: "{}", InitDone: initDone}, nil
	}

	be := supervise.BackendKey(3, "beHASH")
	if _, err := c.EnsureRunning(context.Background(), be, "demo"); err != nil {
		t.Fatal(err)
	}
	if len(marked) != 1 || marked[0] != be {
		t.Fatalf("marked = %v, want exactly [%v]", marked, be)
	}

	// Shipped spec already done: no redundant write on every warm hit.
	marked, initDone = nil, true
	if _, err := c.EnsureRunning(context.Background(), be, "demo"); err != nil {
		t.Fatal(err)
	}
	if len(marked) != 0 {
		t.Fatalf("marked on an already-done spec: %v", marked)
	}

	marked, initDone = nil, false
	fe := supervise.FrontendKey(3, "feHASH", "beHASH")
	if _, err := c.EnsureRunning(context.Background(), fe, "demo"); err != nil {
		t.Fatal(err)
	}
	if len(marked) != 0 {
		t.Fatalf("marked off a frontend ensure: %v", marked)
	}

	marked = nil
	sup.ensureErr = errors.New("init failed on the worker")
	if _, err := c.EnsureRunning(context.Background(), be, "demo"); err == nil {
		t.Fatal("expected the ensure to fail")
	}
	if len(marked) != 0 {
		t.Fatalf("marked off a failed ensure: %v", marked)
	}
}

// TestAdoptRemoteInitDoneReachesTheWire closes the loop against a real
// control-side Manager and DB: adopting a worker's init flips what the next
// ensure ships, so a fresh worker skips init instead of re-running it.
func TestAdoptRemoteInitDoneReachesTheWire(t *testing.T) {
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
	m := supervise.New(database, files, filepath.Join(root, "logs"))
	t.Cleanup(m.StopAll)

	k := supervise.BackendKey(1, "behashadopt00001")
	if err := database.CreateBackendArtifact(db.BackendArtifact{
		RepoID: k.RepoID, BeHash: k.Hash, StateDir: filepath.Join(root, "state", "x"), RunConfig: "{}",
	}); err != nil {
		t.Fatal(err)
	}

	if ws, err := m.ResolveWireSpec(k); err != nil || ws.InitDone {
		t.Fatalf("fresh artifact: spec = %+v, %v (want InitDone=false)", ws, err)
	}
	if err := m.AdoptRemoteInitDone(k); err != nil {
		t.Fatal(err)
	}
	if ws, err := m.ResolveWireSpec(k); err != nil || !ws.InitDone {
		t.Fatalf("after adoption: spec = %+v, %v (want InitDone=true)", ws, err)
	}
}

func TestAuthRequired(t *testing.T) {
	sup := &fakeSup{}
	srv := httptest.NewServer(NewServer(sup, secret).Handler())
	defer srv.Close()

	// No secret → 401.
	bad := NewClient(srv.URL, "h", "wrong-secret", srv.Client())
	if _, err := bad.Heartbeat(context.Background()); err == nil {
		t.Fatal("expected auth failure with the wrong secret")
	}

	// Raw request with no header at all → 401.
	res, err := srv.Client().Get(srv.URL + pathHeartbeat)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-auth status = %d, want 401", res.StatusCode)
	}
}

// TestConfigureRoundTrip: the control node pushes a runtime warm cap.
func TestConfigureRoundTrip(t *testing.T) {
	sup := &fakeSup{maxWarm: 12}
	c, done := testServer(t, sup)
	defer done()
	five, two, ninety := 5, 2, 90
	if err := c.Configure(context.Background(), WorkerConfig{MaxWarm: &five, MinWarm: &two, IdleTimeoutSeconds: &ninety}); err != nil {
		t.Fatal(err)
	}
	if sup.maxWarm != 5 || sup.minWarm != 2 || sup.idleOverride != 90*time.Second {
		t.Fatalf("maxWarm = %d minWarm = %d idle = %s, want 5 / 2 / 90s", sup.maxWarm, sup.minWarm, sup.idleOverride)
	}

	// A partial push touches only what it names.
	if err := c.Configure(context.Background(), WorkerConfig{MaxWarm: &five}); err != nil {
		t.Fatal(err)
	}
	if sup.idleOverride != 90*time.Second {
		t.Fatalf("idle changed by a push that didn't name it: %s", sup.idleOverride)
	}
}

// TestExecRoundTrip: a full exec session over the wire — the WebSocket
// upgrade, the key and options crossing as query parameters, stdin frames
// reaching the supervisor, and output/exit frames coming back.
func TestExecRoundTrip(t *testing.T) {
	var mu sync.Mutex
	var gotK supervise.Key
	var gotOpts supervise.ExecOptions
	sup := &fakeSup{execFn: func(k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error {
		mu.Lock()
		gotK, gotOpts = k, opts
		mu.Unlock()
		f, err := execstream.ReadFrame(stream)
		if err != nil || f.Type != execstream.FrameStdin {
			t.Errorf("supervisor read frame %+v, %v; want stdin", f, err)
			return err
		}
		fw := execstream.NewWriter(stream)
		if err := fw.WriteFrame(execstream.FrameStdout, f.Payload); err != nil {
			return err
		}
		return fw.WriteFrame(execstream.FrameExit, []byte{7})
	}}
	c, done := testServer(t, sup)
	defer done()

	k := supervise.FrontendKey(7, "feHASH", "beHASH")
	opts := supervise.ExecOptions{Cmd: []string{"sh", "-c", "echo hi"}, TTY: true, Stdin: true, Term: "xterm"}
	caller, cli := net.Pipe()
	sessionErr := make(chan error, 1)
	go func() { sessionErr <- c.Exec(context.Background(), k, opts, cli) }()

	fw := execstream.NewWriter(caller)
	if err := fw.WriteFrame(execstream.FrameStdin, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	f, err := execstream.ReadFrame(caller)
	if err != nil || f.Type != execstream.FrameStdout || string(f.Payload) != "hello" {
		t.Fatalf("stdout frame = %+v, %v", f, err)
	}
	f, err = execstream.ReadFrame(caller)
	if err != nil || f.Type != execstream.FrameExit || len(f.Payload) != 1 || f.Payload[0] != 7 {
		t.Fatalf("exit frame = %+v, %v", f, err)
	}
	if err := <-sessionErr; err != nil {
		t.Fatalf("Exec returned %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotK != k {
		t.Fatalf("supervisor saw key %+v, want %+v", gotK, k)
	}
	if !slices.Equal(gotOpts.Cmd, opts.Cmd) || !gotOpts.TTY || !gotOpts.Stdin || gotOpts.Term != "xterm" {
		t.Fatalf("supervisor saw opts %+v, want %+v", gotOpts, opts)
	}
}

// TestExecErrorArrivesAsFrame: an orchestration failure after the upgrade
// (nothing running for the key) reaches the caller as FrameError.
func TestExecErrorArrivesAsFrame(t *testing.T) {
	sup := &fakeSup{execFn: func(supervise.Key, supervise.ExecOptions, io.ReadWriter) error {
		return errors.New("preview process is not running")
	}}
	c, done := testServer(t, sup)
	defer done()

	caller, cli := net.Pipe()
	go c.Exec(context.Background(), supervise.BackendKey(1, "h"), supervise.ExecOptions{Cmd: []string{"sh"}}, cli) //nolint:errcheck
	f, err := execstream.ReadFrame(caller)
	if err != nil || f.Type != execstream.FrameError || string(f.Payload) != "preview process is not running" {
		t.Fatalf("frame = %+v, %v; want FrameError with the detail", f, err)
	}
}

// TestFailedStartRevokesInitOverTheWire drives a REAL supervise.Manager over
// an empty DB (a worker resolves its run spec from the wire, so a fake
// supervisor would prove nothing about the skip/revoke decision). The control
// side keeps offering InitDone=true — its DB row may still say done — yet a
// start that skipped init and failed must make the very next ensure re-run
// init, and must tell the control node to withdraw the record. A transport
// error, where the worker may never have run anything, must not.
func TestFailedStartRevokesInitOverTheWire(t *testing.T) {
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
	m := supervise.New(database, files, filepath.Join(root, "logs"))
	t.Cleanup(m.StopAll)

	const beHash = "behashrevoke0001"
	scratch, _, err := files.NewScratchDir("be")
	if err != nil {
		t.Fatal(err)
	}
	if err := files.PublishBackend("demo", beHash, scratch, false); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join("{state_dir}", "init-marker")
	raw, err := json.Marshal(manifest.Backend{
		Init:         [][]string{{exe, "--helper-init", marker}},
		Run:          []string{exe, "--helper-server", "{port}", "--needs-marker", marker},
		HealthPath:   "/api/health",
		StartTimeout: manifest.Duration(10 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewServer(m, secret).Handler())
	defer srv.Close()
	c := NewClient(srv.URL, "127.0.0.1", secret, srv.Client())
	// The control node believes init is done — and keeps saying so.
	c.SpecResolver = func(k supervise.Key) (supervise.WireSpec, error) {
		return supervise.WireSpec{RunConfig: string(raw), InitDone: true}, nil
	}
	var revoked []supervise.Key
	c.InitRevoker = func(k supervise.Key) error {
		revoked = append(revoked, k)
		return nil
	}

	k := supervise.BackendKey(1, beHash)
	ctx := context.Background()
	if _, err := c.EnsureRunning(ctx, k, "demo"); err == nil {
		t.Fatal("expected the start to fail: init was skipped and its effect is missing")
	}
	if len(revoked) != 1 || revoked[0] != k {
		t.Fatalf("revoked = %v, want exactly [%v] for a worker-reported start failure", revoked, k)
	}

	// Same offer (InitDone=true), but the worker's revocation outranks it.
	addr, err := c.EnsureRunning(ctx, k, "demo")
	if err != nil {
		t.Fatalf("ensure after revocation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(files.StateDirPath("demo", beHash), "init-marker")); err != nil {
		t.Fatalf("init did not re-run on the worker: %v", err)
	}
	res, err := http.Get(fmt.Sprintf("http://%s/api/health", addr))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health = %d, want 200", res.StatusCode)
	}

	// A worker we cannot reach proves nothing: no revocation.
	dead := NewClient("http://127.0.0.1:1", "127.0.0.1", secret, srv.Client())
	dead.SpecResolver = c.SpecResolver
	revoked = nil
	dead.InitRevoker = func(k supervise.Key) error {
		revoked = append(revoked, k)
		return nil
	}
	if _, err := dead.EnsureRunning(ctx, k, "demo"); err == nil {
		t.Fatal("expected a transport error")
	}
	if len(revoked) != 0 {
		t.Fatalf("revoked off a transport error: %v", revoked)
	}
}
