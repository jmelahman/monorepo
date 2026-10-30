package server

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/orchard"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/kanbantest"
)

const releaseTestPrefix = "tools/foo"

// newReleaseWorld is a server with one board on a monorepo whose subtree is in
// sync with its upstream, with the current directory in that monorepo. Neither
// remote is on GitHub, so a release skips watching CI.
func newReleaseWorld(t *testing.T) (env *kanbantest.Env, origin string) {
	t.Helper()
	env = kanbantest.New(t)
	dir := t.TempDir()
	origin = filepath.Join(dir, "origin.git")
	upstream := filepath.Join(dir, "foo.git")
	mustGit(t, "", "init", "-q", "--bare", "-b", "main", origin)
	mustGit(t, "", "init", "-q", "--bare", "-b", "master", upstream)
	work := filepath.Join(dir, "foo")
	mustGit(t, "", "init", "-q", "-b", "master", work)
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "add", "README")
	mustGit(t, work, "-c", "user.name=T", "-c", "user.email=t@example.com", "commit", "-q", "-m", "Start foo")
	mustGit(t, work, "push", "-q", upstream, "master")

	o, err := orchard.Open(env.RepoPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Add(config.Subtree{Prefix: releaseTestPrefix, Remote: upstream, Branch: "master"}, ""); err != nil {
		t.Fatal(err)
	}
	mustGit(t, env.RepoPath, "add", o.Config.Manifest)
	mustGit(t, env.RepoPath, "commit", "-q", "-m", "List tools/foo")
	mustGit(t, env.RepoPath, "remote", "add", "origin", origin)
	mustGit(t, env.RepoPath, "push", "-q", "origin", "main")

	board := &db.Board{Name: "Mono", Slug: "mono", RepoPath: env.RepoPath, BaseBranch: "main", GitAuthorName: "Release Bot", GitAuthorEmail: "bot@example.com"}
	if err := env.Store.CreateBoard(context.Background(), board); err != nil {
		t.Fatal(err)
	}
	t.Chdir(env.RepoPath)

	// No terminal, whatever ran the tests.
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	restore := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() {
		os.Stdin = restore
		_ = stdin.Close()
	})
	return env, origin
}

func originTags(t *testing.T, origin string) string {
	t.Helper()
	out, err := exec.Command("git", "ls-remote", "--tags", origin).CombinedOutput()
	if err != nil {
		t.Fatalf("ls-remote: %v\n%s", err, out)
	}
	return string(out)
}

// releaseCLI runs `kanban release args...` and returns its output.
func releaseCLI(t *testing.T, env *kanbantest.Env, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := releaseCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetArgs(append(args, "--server", env.Server.URL))
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func TestReleaseCommands(t *testing.T) {
	env, origin := newReleaseWorld(t)

	if _, err := releaseCLI(t, env, "", "remind", releaseTestPrefix+"/", "--minor", "the", "new", "flag"); err != nil {
		t.Fatal(err)
	}
	if _, err := releaseCLI(t, env, "", "remind", releaseTestPrefix, "again"); err == nil {
		t.Error("second open reminder accepted")
	}
	out, err := releaseCLI(t, env, "", "list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{releaseTestPrefix, "minor", "the new flag"} {
		if !strings.Contains(out, want) {
			t.Errorf("list lacks %q:\n%s", want, out)
		}
	}

	// Without a terminal, only --yes can confirm.
	if _, err := releaseCLI(t, env, "y\n", "run", releaseTestPrefix); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("run without a terminal = %v, want it to ask for --yes", err)
	}
	if tags := originTags(t, origin); tags != "" {
		t.Fatalf("tagged without confirmation: %s", tags)
	}

	// The bump comes from the reminder, which the release closes.
	out, err = releaseCLI(t, env, "", "run", releaseTestPrefix, "--yes")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "released "+releaseTestPrefix+" v0.1.0") {
		t.Errorf("run output:\n%s", out)
	}
	if tags := originTags(t, origin); !strings.Contains(tags, releaseTestPrefix+"/v0.1.0") {
		t.Errorf("origin tags: %s", tags)
	}
	if _, err := releaseCLI(t, env, "", "dismiss", releaseTestPrefix); err == nil {
		t.Error("the release left the reminder open")
	}

	// A finished run's log replays.
	out, err = releaseCLI(t, env, "", "log", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "v0.1.0") {
		t.Errorf("log:\n%s", out)
	}

	if _, err := releaseCLI(t, env, "", "remind", releaseTestPrefix, "next"); err != nil {
		t.Fatal(err)
	}
	if _, err := releaseCLI(t, env, "", "dismiss", releaseTestPrefix); err != nil {
		t.Errorf("dismiss: %v", err)
	}
}

func TestReleaseRunBlockedByMirror(t *testing.T) {
	env, origin := newReleaseWorld(t)
	path := filepath.Join(env.RepoPath, releaseTestPrefix, "new.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, env.RepoPath, "add", "--", path)
	mustGit(t, env.RepoPath, "commit", "-q", "-m", "Change it")
	mustGit(t, env.RepoPath, "push", "-q", "origin", "main")

	if _, err := releaseCLI(t, env, "", "run", releaseTestPrefix, "--yes"); err == nil || !strings.Contains(err.Error(), "mirror") {
		t.Errorf("run = %v, want the mirror to block it", err)
	}
	if tags := originTags(t, origin); tags != "" {
		t.Errorf("tagged anyway: %s", tags)
	}
}

func waitForShown(t *testing.T, s *shownScreen, want string, present bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(s.text(), want) == present {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("waiting for %q to be shown=%v:\n%s", want, present, s.text())
}

func TestReleaseView(t *testing.T) {
	env, origin := newReleaseWorld(t)
	if _, err := releaseCLI(t, env, "", "remind", releaseTestPrefix, "--minor", "the new flag"); err != nil {
		t.Fatal(err)
	}

	screen := &shownScreen{SimulationScreen: newFormScreen(t, 120, 30)}
	done := make(chan error, 1)
	go func() { done <- runReleaseView(t.Context(), screen, env.Server.URL) }()

	waitForShown(t, screen, "the new flag", true)
	screen.InjectKey(tcell.KeyRune, 'r', tcell.ModNone)
	waitForShown(t, screen, "y/n", true)

	// Declining changes nothing.
	screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone)
	waitForShown(t, screen, "y/n", false)
	if tags := originTags(t, origin); tags != "" {
		t.Fatalf("tagged after declining: %s", tags)
	}

	screen.InjectKey(tcell.KeyRune, 'r', tcell.ModNone)
	waitForShown(t, screen, "y/n", true)
	screen.InjectKey(tcell.KeyRune, 'y', tcell.ModNone)
	waitForShown(t, screen, "· released", true)
	if tags := originTags(t, origin); !strings.Contains(tags, releaseTestPrefix+"/v0.1.0") {
		t.Errorf("origin tags: %s", tags)
	}

	screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the view didn't close")
	}
}
