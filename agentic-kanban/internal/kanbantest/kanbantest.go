// Package kanbantest is the shared harness for tests that need the real HTTP
// server: a temp data dir, a temp git repo, an on-disk SQLite DB and the full
// mux behind an httptest.Server.
package kanbantest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/api"
	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/gittest"
	"github.com/jmelahman/kanban/internal/hooks"
	previewsvc "github.com/jmelahman/kanban/internal/previews"
	"github.com/jmelahman/kanban/internal/push"
	"github.com/jmelahman/kanban/internal/secrets"
	"github.com/jmelahman/kanban/internal/session"
)

// IsolateEnv is what every TestMain that boots the server calls: git is cut
// off from the caller's environment and gitconfig (gittest.IsolateEnv), the
// developer's kanban user config is hidden, and docker is pointed at a dead
// socket.
//
// The user config layers over every test board's .kanban.toml: a [merge]
// section there would otherwise decide which strategies these tests see. See
// REGRESSIONS.md: "Tests inherit the developer's kanban user config". A test
// that needs a user file points $KANBAN_CONFIG at its own with t.Setenv.
//
// Docker is dead because New builds a real local-preview orchestrator, whose
// New runs ReclaimOrphans: that force-removes every managed container and
// network on the daemon, killing local-preview's supervise container tests
// when prek runs both projects' go-test hooks concurrently. Sessions must not
// reach a daemon either, or they really spawn (and leak) containers. None of
// these tests need docker.
func IsolateEnv() {
	gittest.IsolateEnv()
	os.Setenv("KANBAN_CONFIG", os.DevNull)
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
}

// MustGit runs git in dir (the process cwd when dir is "") and fails the
// test on error.
func MustGit(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// Env is a running server over a temp data dir and a temp git repo. It wires
// up the full HTTP stack against ephemeral, real dependencies, including the
// actual Docker SDK client (which constructs without contacting the daemon —
// calls that try to reach Docker fail fast and are asserted as such).
type Env struct {
	Dir      string // temp data dir
	RepoPath string // git repo with one empty commit on main
	// ManifestDir holds out-of-repo manifests (<repo-name>.toml) for repos
	// that can't carry one.
	ManifestDir string
	Store       *db.Store
	Config      *config.Config
	Server      *httptest.Server
	Sessions    *session.Manager
	Previews    *orchestrator.Orchestrator
}

// New boots the server. Everything is torn down by t.Cleanup.
func New(t testing.TB) *Env {
	t.Helper()
	dir := t.TempDir()

	repoPath := filepath.Join(dir, "repo")
	MustGit(t, "", "init", "-q", "-b", "main", repoPath)
	MustGit(t, repoPath, "config", "user.email", "test@example.com")
	MustGit(t, repoPath, "config", "user.name", "Test")
	MustGit(t, repoPath, "commit", "--allow-empty", "-q", "-m", "init")

	store, err := db.Open(filepath.Join(dir, "kanban.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	// Board env var values are encrypted inside the Store; production wires
	// a key in run(), tests get an ephemeral one.
	envKey, err := secrets.NewRandomKey()
	if err != nil {
		t.Fatal(err)
	}
	box, err := secrets.NewBox(envKey)
	if err != nil {
		t.Fatal(err)
	}
	store.SetEnvCipher(box)

	cfg := &config.Config{DataDir: dir, PortRangeStart: 13000, PortRangeEnd: 13099}
	dockerCli, err := docker.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dockerCli.Close() })
	hookRunner := hooks.NewRunner(store)
	sessionMgr := session.NewManager(store, dockerCli, hookRunner)

	// Out-of-repo manifests resolve from the developer's real config dir in
	// production; point them at a per-test dir so the suite can't see (or
	// depend on) whatever manifests the host happens to have.
	manifestDir := filepath.Join(dir, "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(previewsvc.ManifestDirEnv, manifestDir)

	previewOrch, err := orchestrator.New(orchestrator.Options{
		DataDir: filepath.Join(dir, "previews"),
		Addr:    ":7474",
		// Mirror production wiring (cmd/server): preview.toml or a
		// [previews] table in .kanban.toml, then a server-side
		// <board-slug>.toml for repos that can't carry one.
		ManifestSources: []orchestrator.ManifestSource{
			{Path: "preview.toml"},
			{Path: ".kanban.toml", Table: "previews"},
		},
		LocalManifestDir: manifestDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { previewOrch.Close() })

	// The default push client refuses non-public addresses, and the fake push
	// service in these tests lives on loopback.
	pushSvc := push.New(store)
	pushSvc.SetHTTPClient(http.DefaultClient)

	handler := api.NewMux(api.Deps{
		Store: store, Docker: dockerCli, Sessions: sessionMgr, Hooks: hookRunner, Config: cfg,
		Previews: previewOrch, Push: pushSvc,
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &Env{Dir: dir, RepoPath: repoPath, Store: store, Config: cfg, Server: srv, Sessions: sessionMgr,
		Previews: previewOrch, ManifestDir: manifestDir}
}
