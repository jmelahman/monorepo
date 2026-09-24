package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

func TestNormalizeProjectDir(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty", in: "", want: ""},
		{name: "whitespace only", in: "  ", want: ""},
		{name: "plain", in: "a/b", want: "a/b"},
		{name: "dot prefix stripped", in: "./a", want: "a"},
		{name: "trailing slash stripped", in: "a/", want: "a"},
		{name: "interior dotdot collapsed", in: "a/b/../c", want: "a/c"},
		{name: "repo root is empty", in: ".", want: ""},
		{name: "escaping", in: "../x", wantErr: true},
		{name: "escaping after clean", in: "a/../..", wantErr: true},
		{name: "absolute", in: "/abs", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeProjectDir(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeProjectDir(%q) = %q, nil; want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeProjectDir(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("NormalizeProjectDir(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestValidateProjectDir(t *testing.T) {
	cases := []struct {
		name      string
		dir       string
		repo      string
		mount     string
		want      string
		wantErrIn string
	}{
		{name: "empty needs nothing", dir: "", repo: "", mount: "/mnt", want: ""},
		{name: "with repo", dir: "web", repo: "/r", want: "web"},
		{name: "normalizes", dir: "./web/", repo: "/r", want: "web"},
		{name: "escaping rejected", dir: "../x", repo: "/r", wantErrIn: "stay inside"},
		{name: "requires repo", dir: "web", repo: "", mount: "/mnt", wantErrIn: "requires repo_path"},
		{name: "rejects mount", dir: "web", repo: "/r", mount: "/mnt", wantErrIn: "mutually exclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateProjectDir(tc.dir, tc.repo, tc.mount)
			if tc.wantErrIn != "" {
				if err == nil {
					t.Fatalf("ValidateProjectDir(%q, %q, %q) = %q, nil; want error containing %q", tc.dir, tc.repo, tc.mount, got, tc.wantErrIn)
				}
				if !strings.Contains(err.Error(), tc.wantErrIn) {
					t.Errorf("error = %v; want it to contain %q", err, tc.wantErrIn)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateProjectDir: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q; want %q", got, tc.want)
			}
		})
	}
}

// gitRepoDir makes a directory look like a git repo to isGitRepo.
func gitRepoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolvePaths(t *testing.T) {
	repo := gitRepoDir(t)

	t.Run("repo only falls back to the worktree", func(t *testing.T) {
		p := ResolvePaths(&db.Board{RepoPath: repo}, &db.Session{WorktreePath: "/wt"})
		if !p.HasRepo {
			t.Error("HasRepo = false; want true")
		}
		if p.MountPath != "/wt" {
			t.Errorf("MountPath = %q; want %q", p.MountPath, "/wt")
		}
		if p.ProjectDir != "" {
			t.Errorf("ProjectDir = %q; want empty", p.ProjectDir)
		}
	})

	t.Run("mount only", func(t *testing.T) {
		p := ResolvePaths(&db.Board{MountPath: "/mnt"}, &db.Session{})
		if p.HasRepo {
			t.Error("HasRepo = true; want false")
		}
		if p.MountPath != "/mnt" {
			t.Errorf("MountPath = %q; want %q", p.MountPath, "/mnt")
		}
	})

	t.Run("project dir is normalized", func(t *testing.T) {
		p := ResolvePaths(&db.Board{RepoPath: repo, ProjectDir: "./services/api/"}, &db.Session{WorktreePath: "/wt"})
		if p.ProjectDir != "services/api" {
			t.Errorf("ProjectDir = %q; want %q", p.ProjectDir, "services/api")
		}
	})

	t.Run("escaping project dir is clamped away", func(t *testing.T) {
		// Clamping matters: the value reaches dockerd as a working directory
		// and as ${localWorkspaceFolder}, so it must never escape.
		p := ResolvePaths(&db.Board{RepoPath: repo, ProjectDir: "../../etc"}, &db.Session{WorktreePath: "/wt"})
		if p.ProjectDir != "" {
			t.Errorf("ProjectDir = %q; want empty", p.ProjectDir)
		}
		if got := p.ProjectRoot("/wt"); got != "/wt" {
			t.Errorf("ProjectRoot = %q; want %q", got, "/wt")
		}
	})

	t.Run("session overrides board", func(t *testing.T) {
		p := ResolvePaths(&db.Board{RepoPath: "/board-repo", MountPath: "/board-mnt"}, &db.Session{RepoPath: repo, MountPath: "/sess-mnt"})
		if p.RepoPath != repo {
			t.Errorf("RepoPath = %q; want %q", p.RepoPath, repo)
		}
		if p.MountPath != "/sess-mnt" {
			t.Errorf("MountPath = %q; want %q", p.MountPath, "/sess-mnt")
		}
	})
}

func TestResolvedPaths_ProjectRoot(t *testing.T) {
	t.Run("empty project dir returns the worktree", func(t *testing.T) {
		p := ResolvedPaths{}
		if got := p.ProjectRoot("/wt"); got != "/wt" {
			t.Errorf("ProjectRoot = %q; want %q", got, "/wt")
		}
	})
	t.Run("descends", func(t *testing.T) {
		p := ResolvedPaths{ProjectDir: "services/api"}
		want := filepath.Join("/wt", "services", "api")
		if got := p.ProjectRoot("/wt"); got != want {
			t.Errorf("ProjectRoot = %q; want %q", got, want)
		}
	})
}
