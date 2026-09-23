package supervise

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/devcontainer"
	"github.com/jmelahman/local-preview/internal/dockerapi"
	"github.com/jmelahman/local-preview/internal/manifest"
)

// dockerOrSkip follows containerrunner_test's two-tier skip: no daemon →
// skip; daemon that doesn't share this filesystem (sibling daemon) → skip,
// since artifact bind mounts would silently resolve on the wrong host.
func dockerOrSkip(t *testing.T, probeDir string) *dockerapi.Client {
	t.Helper()
	ctx := context.Background()
	cli, err := dockerapi.Connect(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	marker := filepath.Join(probeDir, "marker")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var sink strings.Builder
	if err := cli.RunStep(ctx, dockerapi.Step{
		Image: "busybox:1",
		Cmd:   []string{"sh", "-c", "test -f /probe/marker"},
		Binds: []string{probeDir + ":/probe"},
	}, &sink); err != nil {
		t.Skipf("daemon does not share this filesystem (sibling daemon?): %v", err)
	}
	return cli
}

// provisionContainer publishes a backend artifact whose run command starts
// busybox httpd inside a container, dumping its env to a served file first
// so the test can observe env expansion from outside.
func (f *fixture) provisionContainer(t *testing.T, beHash string, env map[string]string) {
	t.Helper()
	f.provisionContainerCfg(t, beHash, backendRunConfig{Backend: manifest.Backend{
		Run:          []string{"sh", "-c", "env > env.txt && exec busybox httpd -f -p 0.0.0.0:{port} -h ."},
		RunImage:     "busybox:1",
		Env:          env,
		HealthPath:   "/health.txt",
		StartTimeout: manifest.Duration(60 * time.Second),
	}})
}

// provisionContainerCfg publishes a backend artifact (with a served
// health.txt) under an arbitrary stored run config.
func (f *fixture) provisionContainerCfg(t *testing.T, beHash string, cfg backendRunConfig) {
	t.Helper()
	scratch, _, err := f.files.NewScratchDir("be")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, "health.txt"), []byte("ok"), 0o644); err != nil {
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

// readRunLog returns the first run-log file of a backend hash.
func (f *fixture) readRunLog(t *testing.T, beHash string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.m.logsDir, "demo", "run", "be-"+beHash, "1.log"))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}
	return string(b)
}

func TestContainerBackendLifecycle(t *testing.T) {
	f := newFixture(t)
	dockerOrSkip(t, t.TempDir())

	const beHash = "be-container-0000"
	f.provisionContainer(t, beHash, map[string]string{
		"PREVIEW_MARKER": "repo={repo} hash={hash}",
	})
	k := BackendKey(f.repoID, beHash)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	port, err := f.m.EnsureRunning(ctx, k, "demo")
	if err != nil {
		t.Fatalf("EnsureRunning: %v", err)
	}
	if code, body := get(t, port, "/health.txt"); code != 200 || !strings.Contains(body, "ok") {
		t.Fatalf("health: %d %q", code, body)
	}
	// Env expansion observable through the served env dump.
	if _, body := get(t, port, "/env.txt"); !strings.Contains(body, "PREVIEW_MARKER=repo=demo hash="+beHash[:12]) {
		t.Fatalf("env not expanded: %q", body)
	}
	if got := f.m.Status(k); got != "running" {
		t.Fatalf("status = %q", got)
	}

	// Container stats come from the daemon; the second sample carries a CPU
	// percentage.
	first := f.m.Stats(ctx, k)
	if first == nil || first.Runtime != "container" || first.MemoryBytes == 0 {
		t.Fatalf("first stats sample = %+v", first)
	}
	time.Sleep(50 * time.Millisecond)
	second := f.m.Stats(ctx, k)
	if second == nil || second.CPUPercent == nil || *second.CPUPercent < 0 {
		t.Fatalf("second stats sample = %+v, want cpu_percent set", second)
	}

	f.m.Stop(k, "test")
	if got := f.m.Status(k); got != "idle" {
		t.Fatalf("status after stop = %q", got)
	}
	if s := f.m.Stats(ctx, k); s != nil {
		t.Fatalf("stats after stop = %+v, want nil", s)
	}
	// The container is removed, not just stopped.
	cli, _ := dockerapi.Connect(context.Background())
	cs, err := cli.ListContainersByLabel(context.Background(), "local-preview.hash", beHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 0 {
		t.Fatalf("container still present after stop: %v", cs)
	}
	// And so is its deploy network — a leaked bridge per backend hash ever
	// served is what exhausted the daemon's address pools on a busy worker.
	if _, ok, err := cli.FindNetworkByName(context.Background(), networkName("demo", beHash)); err != nil || ok {
		t.Fatalf("deploy network still present after stop (ok=%v err=%v)", ok, err)
	}
}

// TestContainerFrontendBackendPair proves the deploy network: starting a
// process-mode frontend brings its peer backend up first, and the frontend
// container reaches it through the `backend` DNS alias carried in
// {backend_url}.
func TestContainerFrontendBackendPair(t *testing.T) {
	f := newFixture(t)
	cli := dockerOrSkip(t, t.TempDir())

	const beHash = "be-pair-000000"
	const feHash = "fe-pair-000000"
	f.provisionContainer(t, beHash, nil)

	scratch, _, err := f.files.NewScratchDir("fe")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, "index.html"), []byte("fe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.files.PublishFrontend("demo", feHash, scratch, false); err != nil {
		t.Fatal(err)
	}
	cfg := frontendRunConfig{Frontend: manifest.Frontend{
		Run: []string{"sh", "-c",
			"wget -qO backend.txt \"$INTERNAL_URL/health.txt\" && env > env.txt && exec busybox httpd -f -p 0.0.0.0:{port} -h ."},
		RunImage:     "busybox:1",
		Env:          map[string]string{"INTERNAL_URL": "{backend_url}"},
		HealthPath:   "/index.html",
		StartTimeout: manifest.Duration(60 * time.Second),
	}}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.CreateFrontendArtifact(db.FrontendArtifact{
		RepoID: f.repoID, FeHash: feHash, RunConfig: string(raw),
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	fePort, err := f.m.EnsureRunning(ctx, FrontendKey(f.repoID, feHash, beHash), "demo")
	if err != nil {
		t.Fatalf("EnsureRunning frontend: %v", err)
	}
	// The frontend fetched the backend's health file over the deploy
	// network before serving — cross-container DNS via the alias works.
	if code, body := get(t, fePort, "/backend.txt"); code != 200 || !strings.Contains(body, "ok") {
		t.Fatalf("backend.txt via alias: %d %q", code, body)
	}
	if _, body := get(t, fePort, "/env.txt"); !strings.Contains(body, "INTERNAL_URL=http://backend:") {
		t.Fatalf("backend_url not expanded to alias: %q", body)
	}
	// The peer backend was started implicitly.
	if got := f.m.Status(BackendKey(f.repoID, beHash)); got != "running" {
		t.Fatalf("peer backend status = %q", got)
	}

	// The deploy network is last-one-out: the backend's exit leaves it to
	// the frontend still attached; the frontend's exit reclaims it.
	netName := networkName("demo", beHash)
	if _, ok, err := cli.FindNetworkByName(ctx, netName); err != nil || !ok {
		t.Fatalf("deploy network %s missing while the pair runs (ok=%v err=%v)", netName, ok, err)
	}
	f.m.Stop(BackendKey(f.repoID, beHash), "test")
	if _, ok, err := cli.FindNetworkByName(ctx, netName); err != nil || !ok {
		t.Fatalf("deploy network removed while the frontend still holds it (ok=%v err=%v)", ok, err)
	}
	f.m.Stop(FrontendKey(f.repoID, feHash, beHash), "test")
	if _, ok, err := cli.FindNetworkByName(ctx, netName); err != nil || ok {
		t.Fatalf("deploy network leaked after both sides stopped (ok=%v err=%v)", ok, err)
	}
}

func TestContainerReclaimAndPurge(t *testing.T) {
	f := newFixture(t)
	cli := dockerOrSkip(t, t.TempDir())
	ctx := context.Background()

	// A leftover labeled container from an "unclean exit".
	id, err := cli.CreateContainer(ctx, dockerapi.ContainerSpec{
		Image: "busybox:1",
		Cmd:   []string{"sleep", "600"},
		Labels: map[string]string{
			dockerapi.ManagedLabel: "1",
			"local-preview.repo":   "demo",
		},
		Port: 18080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.StartContainer(ctx, id); err != nil {
		t.Fatal(err)
	}

	f.m.ReclaimOrphans()
	cs, err := cli.ListContainersByLabel(ctx, dockerapi.ManagedLabel, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.ID == id {
			t.Fatalf("orphan survived reclaim: %v", cs)
		}
	}
}

// TestDevcontainerRuntimeBackend: with no run_image, the devcontainer
// stored in the run config is the default runtime — the process runs
// containered with the devcontainer's cache volumes mounted and HOME
// pointed at its remote-user home.
func TestDevcontainerRuntimeBackend(t *testing.T) {
	f := newFixture(t)
	dockerOrSkip(t, t.TempDir())

	const beHash = "be-devc-0000000"
	f.provisionContainerCfg(t, beHash, backendRunConfig{
		Backend: manifest.Backend{
			// Writing into the volume target proves the mount exists (the
			// redirect fails and the start never turns healthy otherwise).
			Run: []string{"sh", "-c",
				"env > env.txt && echo warm > /devc-cache/marker && exec busybox httpd -f -p 0.0.0.0:{port} -h ."},
			HealthPath:   "/health.txt",
			StartTimeout: manifest.Duration(60 * time.Second),
		},
		Devcontainer: &devcontainer.Config{
			Image:  "busybox:1",
			Mounts: []devcontainer.Mount{{Source: "local-preview-test-devc", Target: "/devc-cache"}},
			Home:   "/home/devuser",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	port, err := f.m.EnsureRunning(ctx, BackendKey(f.repoID, beHash), "demo")
	if err != nil {
		t.Fatalf("EnsureRunning: %v", err)
	}
	if code, body := get(t, port, "/health.txt"); code != 200 || !strings.Contains(body, "ok") {
		t.Fatalf("health: %d %q", code, body)
	}
	if _, body := get(t, port, "/env.txt"); !strings.Contains(body, "HOME=/home/devuser") {
		t.Fatalf("HOME not pointed at the devcontainer home: %q", body)
	}
	if log := f.readRunLog(t, beHash); !strings.Contains(log, "[devcontainer runtime: busybox:1]") {
		t.Fatalf("run log missing devcontainer line: %q", log)
	}
	f.m.Stop(BackendKey(f.repoID, beHash), "test")
}

// TestDevcontainerRuntimeFallsBackToHost: the devcontainer default is soft —
// with no reachable daemon the process starts on the host and the run log
// says why (an explicit run_image fails the start instead).
func TestDevcontainerRuntimeFallsBackToHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	f := newFixture(t)

	const beHash = "be-devc-host-00"
	f.provisionRunCfg(t, beHash, backendRunConfig{
		Backend: manifest.Backend{
			Run:          serverArgv(t),
			HealthPath:   "/api/health",
			StartTimeout: manifest.Duration(10 * time.Second),
		},
		Devcontainer: &devcontainer.Config{Image: "busybox:1"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	port, err := f.m.EnsureRunning(ctx, BackendKey(f.repoID, beHash), "demo")
	if err != nil {
		t.Fatalf("EnsureRunning: %v", err)
	}
	if code, _ := get(t, port, "/api/health"); code != 200 {
		t.Fatalf("health on host fallback: %d", code)
	}
	log := f.readRunLog(t, beHash)
	if !strings.Contains(log, "devcontainer busybox:1 unavailable") || !strings.Contains(log, "running on the host") {
		t.Fatalf("run log missing fallback note: %q", log)
	}
}
