package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/api"
	"github.com/jmelahman/kanban/internal/client"
	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/secrets"
	"github.com/jmelahman/kanban/internal/session"
)

func TestRunBoardList(t *testing.T) {
	srv, _, board := newKanbanCLITestServer(t)
	var out bytes.Buffer
	if err := runBoardList(t.Context(), srv.URL, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "ID") || !strings.Contains(got, "SLUG") || !strings.Contains(got, "NAME") {
		t.Errorf("missing header: %q", got)
	}
	if !strings.Contains(got, board.Slug) || !strings.Contains(got, board.Name) {
		t.Errorf("board not in output: %q", got)
	}
}

func TestRunTicketCreate(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)

	t.Run("default_column", func(t *testing.T) {
		var out bytes.Buffer
		_, err := runTicketCreate(t.Context(), srv.URL, &out, client.CreateTicketArgs{
			Board: board.Slug,
			Title: "Via CLI",
		}, false)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Via CLI") {
			t.Errorf("summary missing title: %q", out.String())
		}
		cols, _ := store.ListColumns(t.Context(), board.ID)
		tickets, _ := store.ListTickets(t.Context(), board.ID)
		var found *db.Ticket
		for i := range tickets {
			if tickets[i].Title == "Via CLI" {
				found = &tickets[i]
			}
		}
		if found == nil {
			t.Fatalf("ticket not created; got %+v", tickets)
		}
		if found.ColumnID != cols[0].ID {
			t.Errorf("default column = %d, want leftmost %d", found.ColumnID, cols[0].ID)
		}
	})

	t.Run("named_column_json_output", func(t *testing.T) {
		var out bytes.Buffer
		_, err := runTicketCreate(t.Context(), srv.URL, &out, client.CreateTicketArgs{
			Board:  board.Slug,
			Title:  "Named Col",
			Column: "in progress",
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		var tk db.Ticket
		if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &tk); err != nil {
			t.Fatalf("output not JSON: %q (%v)", out.String(), err)
		}
		cols, _ := store.ListColumns(t.Context(), board.ID)
		if tk.ColumnID != cols[1].ID {
			t.Errorf("ColumnID = %d, want %d (In Progress)", tk.ColumnID, cols[1].ID)
		}
	})

	t.Run("server_error_propagates", func(t *testing.T) {
		var out bytes.Buffer
		_, err := runTicketCreate(t.Context(), srv.URL, &out, client.CreateTicketArgs{
			Board: "no-such-board",
			Title: "x",
		}, false)
		if err == nil {
			t.Fatal("expected error for unknown board")
		}
	})
}

func TestRunBoardCreateAndDelete(t *testing.T) {
	srv, store, _ := newKanbanCLITestServer(t)
	repoPath := filepath.Join(t.TempDir(), "repo2")
	mustGit(t, "", "init", "-q", "-b", "main", repoPath)
	mustGit(t, repoPath, "config", "user.email", "test@example.com")
	mustGit(t, repoPath, "config", "user.name", "Test")
	mustGit(t, repoPath, "commit", "--allow-empty", "-q", "-m", "init")

	var out bytes.Buffer
	err := runBoardCreate(t.Context(), srv.URL, &out, client.CreateBoardArgs{
		Name:       "Second Board",
		RepoPath:   repoPath,
		BaseBranch: "main",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var created db.Board
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &created); err != nil {
		t.Fatalf("output not JSON: %q (%v)", out.String(), err)
	}
	if created.ID == 0 || created.Name != "Second Board" {
		t.Fatalf("board not created in response: %+v", created)
	}
	got, err := store.GetBoard(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("board not in store: %v", err)
	}
	if got.Slug != created.Slug {
		t.Errorf("slug mismatch %q vs %q", got.Slug, created.Slug)
	}

	out.Reset()
	if err := runBoardDelete(t.Context(), srv.URL, &out, created.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetBoard(t.Context(), created.ID); err == nil {
		t.Error("board still exists after delete")
	}
}

func TestInferBoardCreateArgs(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "myrepo")
	mustGit(t, "", "init", "-q", "-b", "main", repo)
	mustGit(t, repo, "config", "user.email", "test@example.com")
	mustGit(t, repo, "config", "user.name", "Test")
	mustGit(t, repo, "commit", "--allow-empty", "-q", "-m", "init")

	t.Run("infers_repo_and_name_from_cwd", func(t *testing.T) {
		t.Chdir(repo)
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{})
		if err != nil {
			t.Fatal(err)
		}
		if !samePath(got.RepoPath, repo) {
			t.Errorf("RepoPath = %q, want %q", got.RepoPath, repo)
		}
		if got.Name != "myrepo" {
			t.Errorf("Name = %q, want myrepo", got.Name)
		}
	})

	t.Run("infers_from_subdirectory", func(t *testing.T) {
		// A board created from inside a monorepo subproject is about that
		// subproject, so the name follows project_dir rather than the
		// monorepo root's basename.
		sub := filepath.Join(repo, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(sub)
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{})
		if err != nil {
			t.Fatal(err)
		}
		if !samePath(got.RepoPath, repo) {
			t.Errorf("RepoPath = %q, want %q", got.RepoPath, repo)
		}
		if got.ProjectDir != "a/b" {
			t.Errorf("ProjectDir = %q, want %q", got.ProjectDir, "a/b")
		}
		if got.Name != "b" {
			t.Errorf("Name = %q, want b", got.Name)
		}
	})

	t.Run("repo_root_infers_no_project_dir", func(t *testing.T) {
		t.Chdir(repo)
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{})
		if err != nil {
			t.Fatal(err)
		}
		if got.ProjectDir != "" {
			t.Errorf("ProjectDir = %q, want empty", got.ProjectDir)
		}
	})

	t.Run("explicit_project_dir_wins", func(t *testing.T) {
		sub := filepath.Join(repo, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(sub)
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{ProjectDir: "elsewhere"})
		if err != nil {
			t.Fatal(err)
		}
		if got.ProjectDir != "elsewhere" {
			t.Errorf("ProjectDir = %q, want %q", got.ProjectDir, "elsewhere")
		}
		if got.Name != "elsewhere" {
			t.Errorf("Name = %q, want elsewhere", got.Name)
		}
	})

	t.Run("explicit_repo_path_suppresses_project_dir_inference", func(t *testing.T) {
		// The cwd says nothing about a repo the user named explicitly.
		sub := filepath.Join(repo, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(sub)
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{RepoPath: "/elsewhere/proj"})
		if err != nil {
			t.Fatal(err)
		}
		if got.ProjectDir != "" {
			t.Errorf("ProjectDir = %q, want empty", got.ProjectDir)
		}
		if got.Name != "proj" {
			t.Errorf("Name = %q, want proj", got.Name)
		}
	})

	t.Run("linked_worktree_resolves_to_main_repo", func(t *testing.T) {
		wt := filepath.Join(dir, "wt")
		mustGit(t, repo, "worktree", "add", "-q", wt)
		t.Chdir(wt)
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{})
		if err != nil {
			t.Fatal(err)
		}
		if !samePath(got.RepoPath, repo) {
			t.Errorf("RepoPath = %q, want main repo %q", got.RepoPath, repo)
		}
		if got.Name != "myrepo" {
			t.Errorf("Name = %q, want myrepo", got.Name)
		}
	})

	t.Run("explicit_flags_win", func(t *testing.T) {
		t.Chdir(repo)
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{Name: "Custom", RepoPath: "/elsewhere"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Custom" || got.RepoPath != "/elsewhere" {
			t.Errorf("explicit args changed: %+v", got)
		}
	})

	t.Run("name_inferred_from_explicit_repo_path", func(t *testing.T) {
		t.Chdir(t.TempDir())
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{RepoPath: "/somewhere/proj"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "proj" {
			t.Errorf("Name = %q, want proj", got.Name)
		}
	})

	t.Run("errors_outside_git_repo", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if _, err := inferBoardCreateArgs(client.CreateBoardArgs{}); err == nil {
			t.Fatal("expected error outside a git repo")
		}
	})

	t.Run("mount_path_only_requires_name", func(t *testing.T) {
		if _, err := inferBoardCreateArgs(client.CreateBoardArgs{MountPath: "/workspace"}); err == nil {
			t.Fatal("expected error for --mount-path without --name")
		}
		got, err := inferBoardCreateArgs(client.CreateBoardArgs{MountPath: "/workspace", Name: "WS"})
		if err != nil {
			t.Fatal(err)
		}
		if got.RepoPath != "" || got.Name != "WS" {
			t.Errorf("mount-path args changed: %+v", got)
		}
	})
}

func TestRunBoardCreateInferred(t *testing.T) {
	srv, store, _ := newKanbanCLITestServer(t)
	repo := filepath.Join(t.TempDir(), "inferred-repo")
	mustGit(t, "", "init", "-q", "-b", "trunk", repo)
	mustGit(t, repo, "config", "user.email", "test@example.com")
	mustGit(t, repo, "config", "user.name", "Test")
	mustGit(t, repo, "commit", "--allow-empty", "-q", "-m", "init")
	t.Chdir(repo)

	args, err := inferBoardCreateArgs(client.CreateBoardArgs{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runBoardCreate(t.Context(), srv.URL, &out, args, true); err != nil {
		t.Fatal(err)
	}
	var created db.Board
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &created); err != nil {
		t.Fatalf("output not JSON: %q (%v)", out.String(), err)
	}
	if created.Name != "inferred-repo" {
		t.Errorf("Name = %q, want inferred-repo", created.Name)
	}
	if created.BaseBranch != "trunk" {
		t.Errorf("BaseBranch = %q, want trunk (detected by server)", created.BaseBranch)
	}
	if _, err := store.GetBoard(t.Context(), created.ID); err != nil {
		t.Fatalf("board not in store: %v", err)
	}
}

func TestResolveBoardIdent(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)

	t.Run("explicit_arg_passthrough", func(t *testing.T) {
		got, err := resolveBoardIdent(t.Context(), srv.URL, []string{"some-slug"})
		if err != nil {
			t.Fatal(err)
		}
		if got != "some-slug" {
			t.Errorf("ident = %q, want some-slug", got)
		}
	})

	t.Run("infers_board_from_cwd", func(t *testing.T) {
		t.Chdir(board.RepoPath)
		got, err := resolveBoardIdent(t.Context(), srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := strconv.FormatInt(board.ID, 10); got != want {
			t.Errorf("ident = %q, want %q", got, want)
		}

		// The summary of an inferred lookup shows the repo and base branch.
		var out bytes.Buffer
		if err := runBoardGet(t.Context(), srv.URL, &out, got, false); err != nil {
			t.Fatal(err)
		}
		if s := out.String(); !strings.Contains(s, board.Slug) || !strings.Contains(s, "repo=") || !strings.Contains(s, "base=main") {
			t.Errorf("summary missing inferred fields: %q", s)
		}
	})

	t.Run("errors_when_no_board_matches", func(t *testing.T) {
		other := filepath.Join(t.TempDir(), "unrelated")
		mustGit(t, "", "init", "-q", "-b", "main", other)
		t.Chdir(other)
		if _, err := resolveBoardIdent(t.Context(), srv.URL, nil); err == nil {
			t.Fatal("expected error when no board matches the cwd repo")
		}
	})

	t.Run("errors_when_ambiguous", func(t *testing.T) {
		second := &db.Board{
			Name:       "CLI Board 2",
			Slug:       "cli-board-2",
			RepoPath:   board.RepoPath,
			BaseBranch: "main",
		}
		if err := store.CreateBoard(context.Background(), second); err != nil {
			t.Fatal(err)
		}
		t.Chdir(board.RepoPath)
		_, err := resolveBoardIdent(t.Context(), srv.URL, nil)
		if err == nil {
			t.Fatal("expected error when two boards share the repo")
		}
		if !strings.Contains(err.Error(), second.Slug) {
			t.Errorf("ambiguity error should list candidates: %v", err)
		}
	})
}

// TestResolveBoardIdent_ProjectDir is the end-to-end half of
// TestBoardsForPrefix: two boards on one monorepo repo_path must still
// auto-detect, chosen by where in the repo the command runs.
func TestResolveBoardIdent_ProjectDir(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)

	sub := &db.Board{
		Name:       "Subproject",
		Slug:       "subproject",
		RepoPath:   board.RepoPath,
		ProjectDir: "services/api",
		BaseBranch: "main",
	}
	if err := store.CreateBoard(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(board.RepoPath, "services", "api")
	nested := filepath.Join(projectDir, "internal", "handler")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		dir  string
		want int64
	}{
		{name: "repo root picks the whole-repo board", dir: board.RepoPath, want: board.ID},
		{name: "project dir picks the subproject board", dir: projectDir, want: sub.ID},
		{name: "nested dir picks the subproject board", dir: nested, want: sub.ID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(tc.dir)
			got, err := resolveBoardIdent(t.Context(), srv.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := strconv.FormatInt(tc.want, 10); got != want {
				t.Errorf("ident = %q, want %q", got, want)
			}
		})
	}
}

func TestRunBoardStateAndArchived(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)
	tk := &db.Ticket{BoardID: board.ID, Title: "to archive"}
	cols, _ := store.ListColumns(t.Context(), board.ID)
	tk.ColumnID = cols[0].ID
	tk.Slug = "to-archive"
	if err := store.CreateTicket(t.Context(), tk); err != nil {
		t.Fatal(err)
	}
	if err := store.ArchiveTicket(t.Context(), tk.ID); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runBoardArchived(t.Context(), srv.URL, &out, board.Slug); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "to archive") {
		t.Errorf("archived listing missing ticket: %q", out.String())
	}

	out.Reset()
	if err := runBoardState(t.Context(), srv.URL, &out, board.Slug); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"columns"`) || !strings.Contains(out.String(), `"tickets"`) {
		t.Errorf("state output missing keys: %q", out.String())
	}

	out.Reset()
	if err := runBoardArchivedClear(t.Context(), srv.URL, &out, board.Slug); err != nil {
		t.Fatal(err)
	}
	remaining, _ := store.ListArchivedTickets(t.Context(), board.ID)
	if len(remaining) != 0 {
		t.Errorf("expected 0 archived after clear, got %d", len(remaining))
	}
}

func TestRunTicketLifecycle(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)
	cols, _ := store.ListColumns(t.Context(), board.ID)
	tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "lifecycle", Slug: "lifecycle"}
	if err := store.CreateTicket(t.Context(), tk); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	newTitle := "renamed"
	if err := runTicketUpdate(t.Context(), srv.URL, &out, tk.ID,
		client.UpdateTicketArgs{Title: &newTitle}, false); err != nil {
		t.Fatal(err)
	}
	got, _ := store.GetTicket(t.Context(), tk.ID)
	if got.Title != newTitle {
		t.Errorf("title not updated: %q", got.Title)
	}

	out.Reset()
	if err := runTicketMove(t.Context(), srv.URL, &out, tk.ID,
		client.MoveTicketArgs{ColumnID: cols[1].ID, Position: 0}); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetTicket(t.Context(), tk.ID)
	if got.ColumnID != cols[1].ID {
		t.Errorf("column not changed: got %d want %d", got.ColumnID, cols[1].ID)
	}

	out.Reset()
	c := client.New(srv.URL, nil)
	if err := c.ArchiveTicket(t.Context(), tk.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetTicket(t.Context(), tk.ID)
	if got.ArchivedAt == nil {
		t.Errorf("ticket not archived")
	}

	if err := c.UnarchiveTicket(t.Context(), tk.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetTicket(t.Context(), tk.ID)
	if got.ArchivedAt != nil {
		t.Errorf("ticket still archived")
	}
}

func TestRunColumnArchiveAll(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)
	cols, _ := store.ListColumns(t.Context(), board.ID)
	for i := 0; i < 3; i++ {
		tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "t", Slug: "t"}
		if err := store.CreateTicket(t.Context(), tk); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := runColumnArchiveAll(t.Context(), srv.URL, &out, cols[0].ID); err != nil {
		t.Fatal(err)
	}
	tickets, _ := store.ListTicketsInColumn(t.Context(), cols[0].ID)
	if len(tickets) != 0 {
		t.Errorf("expected column empty after archive-all, got %d tickets", len(tickets))
	}
	archived, _ := store.ListArchivedTickets(t.Context(), board.ID)
	if len(archived) != 3 {
		t.Errorf("expected 3 archived tickets, got %d", len(archived))
	}
}

func TestRunEnvLifecycle(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)

	var out bytes.Buffer
	if err := runEnvSet(t.Context(), srv.URL, &out, board.Slug, []string{"MY_API_KEY=s3cret", "OTHER=x"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); strings.Contains(got, "s3cret") {
		t.Errorf("env set printed a value: %q", got)
	}

	out.Reset()
	if err := runEnvList(t.Context(), srv.URL, &out, board.Slug); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "KEY") || !strings.Contains(got, "MY_API_KEY") || !strings.Contains(got, "OTHER") {
		t.Errorf("env list missing keys: %q", got)
	}
	if strings.Contains(got, "s3cret") {
		t.Errorf("env list printed a value: %q", got)
	}
	vars, err := store.GetBoardEnvVars(t.Context(), board.ID)
	if err != nil || vars["MY_API_KEY"] != "s3cret" {
		t.Errorf("value not persisted: vars=%v err=%v", vars, err)
	}

	out.Reset()
	if err := runEnvUnset(t.Context(), srv.URL, &out, board.Slug, []string{"MY_API_KEY"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); strings.Contains(got, "MY_API_KEY") {
		t.Errorf("env unset still lists removed key: %q", got)
	}

	// Malformed pair errors before hitting the server.
	if err := runEnvSet(t.Context(), srv.URL, &out, board.Slug, []string{"NO_EQUALS"}); err == nil {
		t.Error("expected error for KEY without =")
	}
}

func newKanbanCLITestServer(t *testing.T) (*httptest.Server, *db.Store, *db.Board) {
	t.Helper()
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")
	mustGit(t, "", "init", "-q", "-b", "main", repoPath)
	mustGit(t, repoPath, "config", "user.email", "test@example.com")
	mustGit(t, repoPath, "config", "user.name", "Test")
	mustGit(t, repoPath, "commit", "--allow-empty", "-q", "-m", "init")

	store, err := db.Open(filepath.Join(dir, "kanban.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	envKey, err := secrets.NewRandomKey()
	if err != nil {
		t.Fatal(err)
	}
	box, err := secrets.NewBox(envKey)
	if err != nil {
		t.Fatal(err)
	}
	store.SetEnvCipher(box)

	dockerCli, err := docker.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dockerCli.Close() })

	cfg := &config.Config{DataDir: dir, PortRangeStart: 13000, PortRangeEnd: 13099}
	hookRunner := hooks.NewRunner(store)
	sessionMgr := session.NewManager(store, dockerCli, hookRunner)
	handler := api.NewMux(api.Deps{
		Store: store, Docker: dockerCli, Sessions: sessionMgr, Hooks: hookRunner, Config: cfg,
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	board := &db.Board{
		Name:         "CLI Board",
		Slug:         "cli-board",
		RepoPath:     repoPath,
		WorktreeRoot: filepath.Join(dir, "worktrees", "cli"),
		BaseBranch:   "main",
	}
	if err := store.CreateBoard(context.Background(), board); err != nil {
		t.Fatal(err)
	}
	return srv, store, board
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// TestTicketCreateCommand exercises the cobra wiring of `ticket create`
// and `ticket attach` without a terminal: board inference, the flag
// combinations that must fail before creating anything, and --board
// overriding inference.
func TestTicketCreateCommand(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)
	restore := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = restore })

	run := func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		cmd := ticketCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append(args, "--server", srv.URL))
		err := cmd.ExecuteContext(t.Context())
		return out.String(), err
	}
	countTickets := func(t *testing.T) int {
		t.Helper()
		tickets, err := store.ListTickets(t.Context(), board.ID)
		if err != nil {
			t.Fatal(err)
		}
		return len(tickets)
	}

	t.Run("infers_board_from_cwd", func(t *testing.T) {
		t.Chdir(board.RepoPath)
		out, err := run(t, "create", "--title", "From cwd", "--body", "details")
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if !strings.Contains(out, "From cwd") {
			t.Errorf("summary missing title: %q", out)
		}
		tickets, _ := store.ListTickets(t.Context(), board.ID)
		var found bool
		for _, tk := range tickets {
			if tk.Title == "From cwd" && tk.Body == "details" {
				found = true
			}
		}
		if !found {
			t.Errorf("ticket not created on inferred board; got %+v", tickets)
		}
	})

	t.Run("no_title_without_tty_fails_before_creating", func(t *testing.T) {
		t.Chdir(board.RepoPath)
		before := countTickets(t)
		_, err := run(t, "create")
		if err == nil || !strings.Contains(err.Error(), "--title is required") {
			t.Errorf("err = %v", err)
		}
		if got := countTickets(t); got != before {
			t.Errorf("tickets = %d, want %d (nothing created)", got, before)
		}
	})

	t.Run("attach_without_tty_fails_before_creating", func(t *testing.T) {
		t.Chdir(board.RepoPath)
		before := countTickets(t)
		_, err := run(t, "create", "--title", "Attach me", "--attach")
		if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
			t.Errorf("err = %v", err)
		}
		if got := countTickets(t); got != before {
			t.Errorf("tickets = %d, want %d (nothing created)", got, before)
		}
	})

	t.Run("explicit_board_outside_repo", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if _, err := run(t, "create", "--title", "No repo"); err == nil {
			t.Error("expected an error when no board can be inferred")
		}
		out, err := run(t, "create", "--board", board.Slug, "--title", "Explicit board", "--json")
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		var tk db.Ticket
		if err := json.Unmarshal(bytes.TrimSpace([]byte(out)), &tk); err != nil {
			t.Fatalf("output not JSON: %q (%v)", out, err)
		}
		if tk.BoardID != board.ID || tk.Title != "Explicit board" {
			t.Errorf("ticket = %+v", tk)
		}
	})

	t.Run("attach_without_tty", func(t *testing.T) {
		_, err := run(t, "attach", "1")
		if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
			t.Errorf("err = %v", err)
		}
		_, err = run(t, "attach", "x")
		if err == nil {
			t.Error("expected an error for a non-numeric id")
		}
	})

	t.Run("harness_flag_checks", func(t *testing.T) {
		t.Chdir(board.RepoPath)
		before := countTickets(t)
		_, err := run(t, "create", "--title", "No attach", "--harness", "pi")
		if err == nil || !strings.Contains(err.Error(), "--harness needs --attach") {
			t.Errorf("create --harness without --attach: err = %v", err)
		}
		if got := countTickets(t); got != before {
			t.Errorf("tickets = %d, want %d (nothing created)", got, before)
		}
		_, err = run(t, "attach", "1", "--shell", "--harness", "pi")
		if err == nil || !strings.Contains(err.Error(), "--shell") {
			t.Errorf("attach --shell --harness: err = %v", err)
		}
		_, err = run(t, "attach", "1", "--harness", "nope")
		if err == nil || !strings.Contains(err.Error(), `unknown harness "nope"`) {
			t.Errorf("attach --harness nope: err = %v", err)
		}
	})

	t.Run("attach_without_id_needs_tty", func(t *testing.T) {
		t.Chdir(board.RepoPath)
		_, err := run(t, "attach")
		if err == nil || !strings.Contains(err.Error(), "ticket id is required") {
			t.Errorf("err = %v", err)
		}
		_, err = run(t, "attach", "--board", board.Slug)
		if err == nil || !strings.Contains(err.Error(), "ticket id is required") {
			t.Errorf("--board: err = %v", err)
		}
		// A bad detach sequence is reported before anything else happens.
		_, err = run(t, "attach", "--detach-keys", "ctrl-")
		if err == nil || !strings.Contains(err.Error(), "detach keys") {
			t.Errorf("bad --detach-keys: err = %v", err)
		}
		_, err = run(t, "attach", "1", "2")
		if err == nil {
			t.Error("expected an error for two positional args")
		}
	})
}

func TestLoadBoardTickets(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)
	cols, _ := store.ListColumns(t.Context(), board.ID)
	if len(cols) < 2 {
		t.Fatalf("need at least two columns, got %d", len(cols))
	}
	mk := func(col int, title string) *db.Ticket {
		t.Helper()
		tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[col].ID, Title: title, Slug: strings.ToLower(title)}
		if err := store.CreateTicket(t.Context(), tk); err != nil {
			t.Fatal(err)
		}
		return tk
	}
	// Created out of board order on purpose: the second column's ticket
	// first, then two in the first column.
	inProgress := mk(1, "Second")
	first := mk(0, "First")
	third := mk(0, "Third")
	archived := mk(0, "Archived")
	if err := store.ArchiveTicket(t.Context(), archived.ID); err != nil {
		t.Fatal(err)
	}
	sess := &db.Session{TicketID: inProgress.ID, Status: db.SessionStatusWorking}
	if err := store.UpsertSession(t.Context(), sess); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSessionHarness(t.Context(), sess.ID, "pi"); err != nil {
		t.Fatal(err)
	}

	label, items, err := loadBoardTickets(t.Context(), srv.URL, board.Slug, false)
	if err != nil {
		t.Fatal(err)
	}
	if label != "CLI Board (cli-board)" {
		t.Errorf("label = %q", label)
	}
	want := []pickerItem{
		{ID: first.ID, Title: "First", Column: cols[0].Name},
		{ID: third.ID, Title: "Third", Column: cols[0].Name},
		{ID: inProgress.ID, Title: "Second", Column: cols[1].Name, Status: db.SessionStatusWorking, Harness: "pi"},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %+v, want %+v", items, want)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Errorf("items[%d] = %+v, want %+v", i, items[i], want[i])
		}
	}

	// The archived listing is a different endpoint but the same grouping:
	// only the archived ticket, under the column it was archived from. It is
	// what `ticket unarchive` and `ticket delete` pick from.
	label, items, err = loadBoardTickets(t.Context(), srv.URL, board.Slug, true)
	if err != nil {
		t.Fatal(err)
	}
	if label != "CLI Board (cli-board)" {
		t.Errorf("archived label = %q", label)
	}
	wantArchived := []pickerItem{{ID: archived.ID, Title: "Archived", Column: cols[0].Name}}
	if len(items) != 1 || items[0] != wantArchived[0] {
		t.Errorf("archived items = %+v, want %+v", items, wantArchived)
	}

	if _, _, err := loadBoardTickets(t.Context(), srv.URL, "no-such-board", false); err == nil {
		t.Error("expected an error for an unknown board")
	}
}

// TestTicketArg covers the shared "id or picker" resolution every ticket
// subcommand runs: an explicit id never opens a screen, a bad one is
// rejected as a parse error, and an absent one falls through to the picker
// (which without a terminal is an error rather than a hang).
func TestTicketArg(t *testing.T) {
	srv, _, board := newKanbanCLITestServer(t)
	restore := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = restore })

	// An unreachable server proves nothing was fetched for an explicit id.
	id, err := ticketArg(t.Context(), "http://127.0.0.1:1", []string{"42"}, board.Slug, attachAction, false)
	if err != nil || id != 42 {
		t.Errorf("ticketArg(42) = %d, %v", id, err)
	}
	if _, err := ticketArg(t.Context(), srv.URL, []string{"abc"}, board.Slug, attachAction, false); err == nil ||
		!strings.Contains(err.Error(), "ticket id") {
		t.Errorf("bad id: err = %v", err)
	}
	if _, err := ticketArg(t.Context(), srv.URL, nil, board.Slug, attachAction, false); err == nil ||
		!strings.Contains(err.Error(), "interactive terminal") {
		t.Errorf("no id without a tty: err = %v", err)
	}
}

// TestLoadTicketInfo checks the assembly `ticket info` does: a bare ticket
// id resolves to its board, the column it sits in, the session working on
// it, and that session's ports.
func TestLoadTicketInfo(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)
	cols, _ := store.ListColumns(t.Context(), board.ID)
	tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[1].ID, Title: "Info me", Slug: "info-me", Body: "why"}
	if err := store.CreateTicket(t.Context(), tk); err != nil {
		t.Fatal(err)
	}

	info, err := loadTicketInfo(t.Context(), srv.URL, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Ticket.Title != "Info me" || info.Ticket.Body != "why" {
		t.Errorf("ticket = %+v", info.Ticket)
	}
	if info.Board.ID != board.ID || info.Board.Slug != "cli-board" {
		t.Errorf("board = %+v", info.Board)
	}
	if info.Column != cols[1].Name {
		t.Errorf("column = %q, want %q", info.Column, cols[1].Name)
	}
	if info.Session != nil {
		t.Errorf("session = %+v, want none before one is started", info.Session)
	}

	sess := &db.Session{TicketID: tk.ID, Status: db.SessionStatusWorking, BranchName: "kanban/cli-board/info-me"}
	if err := store.UpsertSession(t.Context(), sess); err != nil {
		t.Fatal(err)
	}
	if err := store.CreatePort(t.Context(), &db.PortAllocation{
		SessionID: sess.ID, Label: "web", ContainerPort: 5173, HostPort: 13001, ProxyActive: true,
	}); err != nil {
		t.Fatal(err)
	}

	info, err = loadTicketInfo(t.Context(), srv.URL, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Session == nil || info.Session.BranchName != "kanban/cli-board/info-me" {
		t.Fatalf("session = %+v", info.Session)
	}
	if len(info.Ports) != 1 || info.Ports[0].Label != "web" || info.Ports[0].HostPort != 13001 {
		t.Errorf("ports = %+v", info.Ports)
	}

	if _, err := loadTicketInfo(t.Context(), srv.URL, tk.ID+9999); err == nil {
		t.Error("expected an error for an unknown ticket id")
	}
}

// TestPickTicketEmptyBoard covers the branch of the interactive path that
// runs before any screen is opened: a board with nothing to attach to is
// an error, not an empty list.
func TestPickTicketEmptyBoard(t *testing.T) {
	srv, _, board := newKanbanCLITestServer(t)
	restore := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = restore })

	_, err := pickTicket(t.Context(), srv.URL, board.Slug, attachAction, false)
	if err == nil || !strings.Contains(err.Error(), "no open tickets") {
		t.Errorf("err = %v", err)
	}
	// The archived list is empty on a fresh board too, and says so in its
	// own words rather than the open list's.
	_, err = pickTicket(t.Context(), srv.URL, board.Slug, pickerAction{"Delete ticket", "delete"}, true)
	if err == nil || !strings.Contains(err.Error(), "no archived tickets") {
		t.Errorf("archived err = %v", err)
	}
	t.Chdir(t.TempDir())
	if _, err := pickTicket(t.Context(), srv.URL, "", attachAction, false); err == nil {
		t.Error("expected an error when no board can be inferred")
	}
}

// TestBoardsForPrefix covers the narrowing that makes `[id]` auto-detection
// work in a monorepo. Without it, the second board on a shared repo_path
// breaks auto-detection for every subcommand, everywhere in the repo.
func TestBoardsForPrefix(t *testing.T) {
	root := client.Board{ID: 1, Slug: "root"}
	api := client.Board{ID: 2, Slug: "api", ProjectDir: "services/api"}
	apiWeb := client.Board{ID: 3, Slug: "api-web", ProjectDir: "services/api/web"}
	other := client.Board{ID: 4, Slug: "worker", ProjectDir: "services/worker"}
	apiDup := client.Board{ID: 5, Slug: "api-2", ProjectDir: "services/api"}

	cases := []struct {
		name   string
		boards []client.Board
		prefix string
		want   []int64
	}{
		{
			name:   "single whole-repo board is the catch-all",
			boards: []client.Board{root},
			prefix: "services/api",
			want:   []int64{1},
		},
		{
			name:   "most specific project dir wins",
			boards: []client.Board{root, api, other},
			prefix: "services/api",
			want:   []int64{2},
		},
		{
			name:   "longest prefix wins over a shorter ancestor",
			boards: []client.Board{root, api, apiWeb},
			prefix: "services/api/web/src",
			want:   []int64{3},
		},
		{
			name:   "falls back to the ancestor outside the deeper board",
			boards: []client.Board{root, api, apiWeb},
			prefix: "services/api/cmd",
			want:   []int64{2},
		},
		{
			name:   "repo root matches only the whole-repo board",
			boards: []client.Board{root, api},
			prefix: "",
			want:   []int64{1},
		},
		{
			name:   "sibling prefix does not match",
			boards: []client.Board{api},
			prefix: "services/api-staging",
			want:   nil,
		},
		{
			name:   "unrelated subdirectory falls through to nothing",
			boards: []client.Board{api},
			prefix: "docs",
			want:   nil,
		},
		{
			name:   "duplicate project dirs stay ambiguous",
			boards: []client.Board{root, api, apiDup},
			prefix: "services/api",
			want:   []int64{2, 5},
		},
		{
			name:   "tolerates stored slashes",
			boards: []client.Board{{ID: 9, Slug: "messy", ProjectDir: "/services/api/"}},
			prefix: "services/api",
			want:   []int64{9},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := boardsForPrefix(tc.boards, tc.prefix)
			ids := make([]int64, len(got))
			for i, b := range got {
				ids[i] = b.ID
			}
			if len(ids) != len(tc.want) {
				t.Fatalf("ids = %v; want %v", ids, tc.want)
			}
			for i := range ids {
				if ids[i] != tc.want[i] {
					t.Fatalf("ids = %v; want %v", ids, tc.want)
				}
			}
		})
	}
}
