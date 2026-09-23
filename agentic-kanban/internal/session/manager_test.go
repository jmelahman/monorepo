package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/kanbantoml"
)

func TestResolveBranchPrefix(t *testing.T) {
	// Neutralise the user-level config so tests don't pick up the dev
	// machine's ~/.config/kanban/config.toml.
	t.Setenv("KANBAN_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	t.Run("board override wins", func(t *testing.T) {
		repoDir := t.TempDir()
		writeProjectToml(t, repoDir, `[branches]
prefix = "proj/"`)
		got := resolveBranchPrefix(&db.Board{Slug: "demo", BranchPrefix: "feat"}, repoDir)
		if got != "feat" {
			t.Errorf("got %q; want feat", got)
		}
	})

	t.Run("toml fallback when board empty", func(t *testing.T) {
		repoDir := t.TempDir()
		writeProjectToml(t, repoDir, `[branches]
prefix = "proj"`)
		got := resolveBranchPrefix(&db.Board{Slug: "demo"}, repoDir)
		if got != "proj" {
			t.Errorf("got %q; want proj", got)
		}
	})

	t.Run("hardcoded default when neither set", func(t *testing.T) {
		got := resolveBranchPrefix(&db.Board{Slug: "demo"}, t.TempDir())
		if got != "kanban/demo" {
			t.Errorf("got %q; want kanban/demo", got)
		}
	})

	t.Run("whitespace-only board prefix falls through", func(t *testing.T) {
		got := resolveBranchPrefix(&db.Board{Slug: "demo", BranchPrefix: "   "}, t.TempDir())
		if got != "kanban/demo" {
			t.Errorf("got %q; want kanban/demo", got)
		}
	})
}

func writeProjectToml(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, ".kanban.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestApplyKanbanDevcontainerOverrides_AppendsAndMergesEnv(t *testing.T) {
	cfg := &docker.DevcontainerConfig{
		RunArgs: []string{"--cap-add=NET_ADMIN"},
		Mounts:  []string{"type=volume,source=cache,target=/cache"},
		ContainerEnv: map[string]string{
			"LANG": "C.UTF-8",
			"TZ":   "UTC",
		},
	}
	dev := &kanbantoml.DevcontainerSection{
		RunArgs: []string{"--cap-add=SYS_PTRACE"},
		Mounts:  []string{"type=bind,source=/tmp/ssh-agent.sock,target=/tmp/ssh-agent.sock"},
		ContainerEnv: map[string]string{
			"LANG":          "en_US.UTF-8",
			"SSH_AUTH_SOCK": "/tmp/ssh-agent.sock",
		},
	}

	applyKanbanDevcontainerOverrides(cfg, dev, nil)

	if got := len(cfg.RunArgs); got != 2 {
		t.Errorf("len(RunArgs) = %d; want 2", got)
	}
	if cfg.RunArgs[1] != "--cap-add=SYS_PTRACE" {
		t.Errorf("RunArgs[1] = %q; want --cap-add=SYS_PTRACE", cfg.RunArgs[1])
	}
	if got := len(cfg.Mounts); got != 2 {
		t.Errorf("len(Mounts) = %d; want 2", got)
	}
	if cfg.ContainerEnv["LANG"] != "en_US.UTF-8" {
		t.Errorf("ContainerEnv[LANG] = %q; want en_US.UTF-8 (kanban override)", cfg.ContainerEnv["LANG"])
	}
	if cfg.ContainerEnv["TZ"] != "UTC" {
		t.Errorf("ContainerEnv[TZ] = %q; want UTC (preserved)", cfg.ContainerEnv["TZ"])
	}
	if cfg.ContainerEnv["SSH_AUTH_SOCK"] != "/tmp/ssh-agent.sock" {
		t.Errorf("ContainerEnv[SSH_AUTH_SOCK] = %q; want /tmp/ssh-agent.sock", cfg.ContainerEnv["SSH_AUTH_SOCK"])
	}
}

func TestMergeEnv_ProtectedKeysWin(t *testing.T) {
	base := map[string]string{
		"MY_API_KEY":        "s3cret",
		"KANBAN_SESSION_ID": "spoofed", // colliding board var must lose
	}
	protect := map[string]string{
		"KANBAN_SESSION_ID": "42",
		"KANBAN_API_URL":    "http://kanban:7474",
	}

	got := mergeEnv(base, protect)

	want := map[string]string{
		"MY_API_KEY":        "s3cret",
		"KANBAN_SESSION_ID": "42",
		"KANBAN_API_URL":    "http://kanban:7474",
	}
	if len(got) != len(want) {
		t.Fatalf("mergeEnv = %v; want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("mergeEnv[%q] = %q; want %q", k, got[k], v)
		}
	}
	// Inputs must not be mutated.
	if base["KANBAN_SESSION_ID"] != "spoofed" {
		t.Error("mergeEnv mutated its base map")
	}
}

func TestApplyKanbanDevcontainerOverrides_NilDev(t *testing.T) {
	cfg := &docker.DevcontainerConfig{RunArgs: []string{"--privileged"}}
	applyKanbanDevcontainerOverrides(cfg, nil, nil)
	if len(cfg.RunArgs) != 1 || cfg.RunArgs[0] != "--privileged" {
		t.Errorf("RunArgs mutated by nil dev: %v", cfg.RunArgs)
	}
}

func TestApplyKanbanDevcontainerOverrides_InitializesEnvMap(t *testing.T) {
	cfg := &docker.DevcontainerConfig{}
	dev := &kanbantoml.DevcontainerSection{
		ContainerEnv: map[string]string{"FOO": "bar"},
	}
	applyKanbanDevcontainerOverrides(cfg, dev, nil)
	if cfg.ContainerEnv["FOO"] != "bar" {
		t.Errorf("ContainerEnv[FOO] = %q; want bar", cfg.ContainerEnv["FOO"])
	}
}

func TestApplyKanbanDevcontainerOverrides_BuiltInDockerSocket(t *testing.T) {
	// Seed an XDG_RUNTIME_DIR socket so DockerSocketMount resolves a
	// non-empty mount on hosts without /var/run/docker.sock.
	runDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runDir, "docker.sock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", runDir)

	expectedMount := docker.DockerSocketMount()
	if expectedMount == "" {
		t.Fatal("DockerSocketMount returned empty despite seeded XDG socket")
	}
	hasSocket := func(cfg *docker.DevcontainerConfig) bool {
		for _, m := range cfg.Mounts {
			if m == expectedMount {
				return true
			}
		}
		return false
	}
	boolPtr := func(b bool) *bool { return &b }

	t.Run("default omits socket on built-in", func(t *testing.T) {
		cfg := &docker.DevcontainerConfig{BuiltIn: true}
		applyKanbanDevcontainerOverrides(cfg, nil, nil)
		if hasSocket(cfg) {
			t.Errorf("docker socket present by default; mounts = %v", cfg.Mounts)
		}
	})

	t.Run("docker_socket=true adds the mount", func(t *testing.T) {
		cfg := &docker.DevcontainerConfig{BuiltIn: true}
		applyKanbanDevcontainerOverrides(cfg, &kanbantoml.DevcontainerSection{
			DockerSocket: boolPtr(true),
		}, nil)
		if !hasSocket(cfg) {
			t.Errorf("docker socket missing despite docker_socket=true; mounts = %v", cfg.Mounts)
		}
	})

	t.Run("non-built-in configs are not auto-mounted", func(t *testing.T) {
		cfg := &docker.DevcontainerConfig{}
		applyKanbanDevcontainerOverrides(cfg, nil, nil)
		if hasSocket(cfg) {
			t.Errorf("hand-written config got auto-socket; mounts = %v", cfg.Mounts)
		}
	})
}

func TestApplyKanbanDevcontainerOverrides_BuiltInClaudeConfig(t *testing.T) {
	// Seed a HOME with both files present so ClaudeConfigMounts returns the
	// expected pair on hosts that don't have ~/.claude. Pin the remote
	// user/home so the auto-detect path doesn't pick a different target
	// based on whatever UID owns the temp dir on the runner.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DEVCONTAINER_REMOTE_USER", "dev")
	t.Setenv("DEVCONTAINER_REMOTE_HOME", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Seeded XDG socket prevents the docker_socket default from contributing
	// other mounts that could confuse the assertions.
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	hasClaude := func(cfg *docker.DevcontainerConfig) bool {
		for _, m := range cfg.Mounts {
			if m == "type=bind,source="+filepath.Join(home, ".claude")+",target=/home/dev/.claude" {
				return true
			}
		}
		return false
	}
	boolPtr := func(b bool) *bool { return &b }

	t.Run("default mounts claude config on built-in", func(t *testing.T) {
		cfg := &docker.DevcontainerConfig{BuiltIn: true}
		applyKanbanDevcontainerOverrides(cfg, nil, nil)
		if !hasClaude(cfg) {
			t.Errorf("claude config missing; mounts = %v", cfg.Mounts)
		}
	})

	t.Run("claude_config=false drops the mount", func(t *testing.T) {
		cfg := &docker.DevcontainerConfig{BuiltIn: true}
		applyKanbanDevcontainerOverrides(cfg, &kanbantoml.DevcontainerSection{
			ClaudeConfig: boolPtr(false),
		}, nil)
		if hasClaude(cfg) {
			t.Errorf("claude config present despite claude_config=false; mounts = %v", cfg.Mounts)
		}
	})

	t.Run("non-built-in configs are not auto-mounted", func(t *testing.T) {
		cfg := &docker.DevcontainerConfig{}
		applyKanbanDevcontainerOverrides(cfg, nil, nil)
		if hasClaude(cfg) {
			t.Errorf("hand-written config got auto-claude; mounts = %v", cfg.Mounts)
		}
	})

	t.Run("override=false beats claude_config=true in toml", func(t *testing.T) {
		cfg := &docker.DevcontainerConfig{BuiltIn: true}
		applyKanbanDevcontainerOverrides(cfg, &kanbantoml.DevcontainerSection{
			ClaudeConfig: boolPtr(true),
		}, boolPtr(false))
		if hasClaude(cfg) {
			t.Errorf("claude config present despite override=false; mounts = %v", cfg.Mounts)
		}
	})

	t.Run("override=true beats claude_config=false in toml", func(t *testing.T) {
		cfg := &docker.DevcontainerConfig{BuiltIn: true}
		applyKanbanDevcontainerOverrides(cfg, &kanbantoml.DevcontainerSection{
			ClaudeConfig: boolPtr(false),
		}, boolPtr(true))
		if !hasClaude(cfg) {
			t.Errorf("claude config missing despite override=true; mounts = %v", cfg.Mounts)
		}
	})
}

// newReconcileEnv seeds a board, a ticket, and a session row claiming a live
// container, wired to a manager whose docker client can't reach a daemon so
// Stop's container teardown is a fast no-op.
func newReconcileEnv(t *testing.T, status string) (*Manager, *db.Store, *db.Board, *db.Ticket, *db.Session) {
	t.Helper()
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	store, err := db.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	ctx := context.Background()

	board := &db.Board{Name: "Reconcile", Slug: "reconcile"}
	if err := store.CreateBoard(ctx, board); err != nil {
		t.Fatal(err)
	}
	cols, err := store.ListColumns(ctx, board.ID)
	if err != nil || len(cols) == 0 {
		t.Fatalf("columns: %v", err)
	}
	ticket := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "Dead container", Slug: "dead-container"}
	if err := store.CreateTicket(ctx, ticket); err != nil {
		t.Fatal(err)
	}
	container := "c0ffee"
	sess := &db.Session{
		TicketID:     ticket.ID,
		WorktreePath: t.TempDir(),
		Status:       status,
		ContainerID:  &container,
	}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	dc, err := docker.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dc.Close() })
	return NewManager(store, dc, hooks.NewRunner(store)), store, board, ticket, sess
}

func containerOf(s *db.Session) string {
	if s.ContainerID == nil {
		return ""
	}
	return *s.ContainerID
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()

	t.Run("dead_container_stops_session", func(t *testing.T) {
		m, store, _, _, sess := newReconcileEnv(t, db.SessionStatusIdle)
		var probed []string
		m.SetContainerProbe(func(_ context.Context, id string) (bool, error) {
			probed = append(probed, id)
			return false, nil
		})
		got, err := m.Reconcile(ctx, sess)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != db.SessionStatusStopped || containerOf(got) != "" || got.StoppedAt == nil {
			t.Errorf("reconciled session = %+v, want stopped with no container", got)
		}
		if len(probed) != 1 || probed[0] != "c0ffee" {
			t.Errorf("probed %v, want [c0ffee]", probed)
		}
		fresh, err := store.GetSession(ctx, sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		if fresh.Status != db.SessionStatusStopped || containerOf(fresh) != "" {
			t.Errorf("persisted session = %+v, want stopped with no container", fresh)
		}
	})

	t.Run("running_container_is_left_alone", func(t *testing.T) {
		m, _, _, _, sess := newReconcileEnv(t, db.SessionStatusWorking)
		m.SetContainerProbe(func(context.Context, string) (bool, error) { return true, nil })
		got, err := m.Reconcile(ctx, sess)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != db.SessionStatusWorking || containerOf(got) != "c0ffee" {
			t.Errorf("session = %+v, want untouched", got)
		}
	})

	t.Run("inspect_failure_is_not_evidence", func(t *testing.T) {
		m, _, _, _, sess := newReconcileEnv(t, db.SessionStatusIdle)
		m.SetContainerProbe(func(context.Context, string) (bool, error) { return false, errors.New("daemon unreachable") })
		got, err := m.Reconcile(ctx, sess)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != db.SessionStatusIdle || containerOf(got) != "c0ffee" {
			t.Errorf("session = %+v, want untouched", got)
		}
	})

	t.Run("rows_without_a_live_claim_are_not_probed", func(t *testing.T) {
		for _, status := range []string{db.SessionStatusStopped, db.SessionStatusError, db.SessionStatusStarting} {
			m, _, _, _, sess := newReconcileEnv(t, status)
			m.SetContainerProbe(func(context.Context, string) (bool, error) {
				t.Errorf("status %q: probe called", status)
				return false, nil
			})
			got, err := m.Reconcile(ctx, sess)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != status {
				t.Errorf("status %q became %q", status, got.Status)
			}
		}
		m, _, _, _, sess := newReconcileEnv(t, db.SessionStatusIdle)
		empty := ""
		sess.ContainerID = &empty
		m.SetContainerProbe(func(context.Context, string) (bool, error) {
			t.Error("probe called for a session with no container id")
			return false, nil
		})
		if _, err := m.Reconcile(ctx, sess); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("no_probe_disables_the_check", func(t *testing.T) {
		m, _, _, _, sess := newReconcileEnv(t, db.SessionStatusIdle)
		m.SetContainerProbe(nil)
		got, err := m.Reconcile(ctx, sess)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != db.SessionStatusIdle {
			t.Errorf("session = %+v, want untouched", got)
		}
	})

	t.Run("ensure_returns_the_reconciled_row", func(t *testing.T) {
		m, _, board, ticket, _ := newReconcileEnv(t, db.SessionStatusIdle)
		m.SetContainerProbe(func(context.Context, string) (bool, error) { return false, nil })
		got, err := m.Ensure(ctx, board, ticket)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != db.SessionStatusStopped || containerOf(got) != "" {
			t.Errorf("ensured session = %+v, want stopped with no container", got)
		}
	})

	t.Run("start_restarts_instead_of_returning_the_stale_row", func(t *testing.T) {
		m, store, _, _, sess := newReconcileEnv(t, db.SessionStatusIdle)
		probes := 0
		m.SetContainerProbe(func(context.Context, string) (bool, error) {
			probes++
			return false, nil
		})
		// No daemon, so the restart fails at spawn: the point is that Start
		// tried at all rather than handing back the idle row as a no-op.
		if _, err := m.Start(ctx, sess.ID, nil); err == nil {
			t.Fatal("Start succeeded without a docker daemon")
		}
		if probes != 1 {
			t.Errorf("probes = %d, want 1", probes)
		}
		fresh, err := store.GetSession(ctx, sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		if fresh.Status != db.SessionStatusError || containerOf(fresh) != "" {
			t.Errorf("session after failed restart = %+v, want error with no container", fresh)
		}
	})
}

func TestStopAgentUnless(t *testing.T) {
	ctx := context.Background()
	type call struct{ container, marker string }
	stub := func(m *Manager, err error) *[]call {
		var calls []call
		m.endPTY = func(_ context.Context, container, marker string) error {
			calls = append(calls, call{container, marker})
			return err
		}
		return &calls
	}
	statusOf := func(t *testing.T, store *db.Store, id int64) string {
		t.Helper()
		sess, err := store.GetSession(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return sess.Status
	}

	t.Run("different_harness_is_stopped", func(t *testing.T) {
		m, store, _, _, sess := newReconcileEnv(t, db.SessionStatusWorking)
		calls := stub(m, nil)
		addFakeBroker(t, m.brokers, sess.ID, "agent", "claude")
		shell, _ := addFakeBroker(t, m.brokers, sess.ID, "shell", "")

		if !m.StopAgentUnless(ctx, sess.ID, "pi") {
			t.Fatal("StopAgentUnless = false; want the claude agent stopped")
		}
		if want := []call{{"c0ffee", "agent-TEST"}}; len(*calls) != 1 || (*calls)[0] != want[0] {
			t.Errorf("endPTY calls = %v; want %v", *calls, want)
		}
		if got := statusOf(t, store, sess.ID); got != db.SessionStatusIdle {
			t.Errorf("status = %q; want idle once the agent that reported working is gone", got)
		}
		if shell.closed {
			t.Error("shell broker was closed")
		}
	})

	t.Run("failed_kill_still_resets_status", func(t *testing.T) {
		m, store, _, _, sess := newReconcileEnv(t, db.SessionStatusAwaitingPerm)
		stub(m, errors.New("exec failed"))
		addFakeBroker(t, m.brokers, sess.ID, "agent", "claude")
		if !m.StopAgentUnless(ctx, sess.ID, "pi") {
			t.Fatal("StopAgentUnless = false; want true")
		}
		if got := statusOf(t, store, sess.ID); got != db.SessionStatusIdle {
			t.Errorf("status = %q; want idle", got)
		}
	})

	t.Run("same_harness_is_left_running", func(t *testing.T) {
		m, store, _, _, sess := newReconcileEnv(t, db.SessionStatusWorking)
		calls := stub(m, nil)
		agent, _ := addFakeBroker(t, m.brokers, sess.ID, "agent", "pi")
		if m.StopAgentUnless(ctx, sess.ID, "pi") {
			t.Fatal("StopAgentUnless = true; want the pi agent kept")
		}
		if len(*calls) != 0 || agent.closed {
			t.Errorf("agent disturbed: endPTY calls %v, closed %v", *calls, agent.closed)
		}
		if got := statusOf(t, store, sess.ID); got != db.SessionStatusWorking {
			t.Errorf("status = %q; want working (untouched)", got)
		}
	})

	t.Run("no_agent_running", func(t *testing.T) {
		m, store, _, _, sess := newReconcileEnv(t, db.SessionStatusWorking)
		calls := stub(m, nil)
		if m.StopAgentUnless(ctx, sess.ID, "pi") {
			t.Fatal("StopAgentUnless = true with no agent running")
		}
		if len(*calls) != 0 {
			t.Errorf("endPTY calls = %v; want none", *calls)
		}
		if got := statusOf(t, store, sess.ID); got != db.SessionStatusWorking {
			t.Errorf("status = %q; want working (untouched)", got)
		}
	})
}

// TestEndPTYScript runs the real script against local processes. Outside a
// container a tagged process's parent is this test rather than PPid 0, so
// the leader check is pointed at our pid.
func TestEndPTYScript(t *testing.T) {
	if _, err := os.Stat("/proc/self/environ"); err != nil {
		t.Skip("no /proc")
	}
	const leaderCheck = `'^PPid:[[:space:]]*0$'`
	if !strings.Contains(endPTYScript, leaderCheck) {
		t.Fatalf("endPTYScript no longer contains %s", leaderCheck)
	}
	script := strings.Replace(endPTYScript, leaderCheck, fmt.Sprintf(`'^PPid:[[:space:]]*%d$'`, os.Getpid()), 1)

	start := func(marker string) *exec.Cmd {
		cmd := exec.Command("sleep", "60")
		cmd.Env = append(os.Environ(), ptyMarkerEnv+"="+marker)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		return cmd
	}
	target := start("agent-TARGET")
	bystander := start("agent-OTHER")

	out, err := exec.Command("sh", "-c", script, "sh", "agent-TARGET").CombinedOutput()
	if err != nil {
		t.Fatalf("script: %v\n%s", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), strconv.Itoa(target.Process.Pid); got != want {
		t.Errorf("script output = %q; want just the signalled pid %s", got, want)
	}
	var exitErr *exec.ExitError
	if err := target.Wait(); !errors.As(err, &exitErr) || exitErr.Sys().(syscall.WaitStatus).Signal() != syscall.SIGHUP {
		t.Errorf("target exit = %v; want killed by SIGHUP", err)
	}
	if err := bystander.Process.Signal(syscall.Signal(0)); err != nil {
		t.Errorf("process with another marker was signalled: %v", err)
	}

	out, err = exec.Command("sh", "-c", script, "sh", "agent-MISSING").CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Errorf("no match: err %v, output %q; want silent success", err, out)
	}
}
