package kanbantoml

import (
	"os"
	"path/filepath"
	"testing"
)

// withUserConfig redirects the user-config lookup at $XDG_CONFIG_HOME and
// writes contents to that location, returning the temp dir for cleanup hooks.
// An empty contents string skips writing the file (so absence is testable).
func withUserConfig(t *testing.T, contents string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if contents == "" {
		return
	}
	cfgDir := filepath.Join(dir, "kanban")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}
}

func writeProject(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	if contents != "" {
		if err := os.WriteFile(filepath.Join(dir, ".kanban.toml"), []byte(contents), 0o644); err != nil {
			t.Fatalf("write project config: %v", err)
		}
	}
	return dir
}

func TestLoad_ErrorsSectionAbsentDefaults(t *testing.T) {
	withUserConfig(t, "")
	repo := writeProject(t, "")
	f := Load(repo)
	if f.Errors != nil {
		t.Fatalf("Errors should be nil when no config sets it; got %+v", f.Errors)
	}
}

func TestLoad_ErrorsSectionEnabled(t *testing.T) {
	withUserConfig(t, "")
	repo := writeProject(t, `[errors]
enabled = true
board_name = "Bug Tracker"
`)
	f := Load(repo)
	if f.Errors == nil {
		t.Fatal("Errors section missing")
	}
	if f.Errors.Enabled == nil || !*f.Errors.Enabled {
		t.Errorf("enabled = %v; want true", f.Errors.Enabled)
	}
	if f.Errors.BoardName == nil || *f.Errors.BoardName != "Bug Tracker" {
		t.Errorf("board_name = %v; want \"Bug Tracker\"", f.Errors.BoardName)
	}
}

func TestLoad_ErrorsSectionUserOverridesProject(t *testing.T) {
	withUserConfig(t, `[errors]
enabled = false
`)
	repo := writeProject(t, `[errors]
enabled = true
board_name = "ProjectErrors"
`)
	f := Load(repo)
	if f.Errors == nil {
		t.Fatal("Errors section missing")
	}
	if f.Errors.Enabled == nil || *f.Errors.Enabled {
		t.Errorf("enabled = %v; want false (user override)", f.Errors.Enabled)
	}
	if f.Errors.BoardName == nil || *f.Errors.BoardName != "ProjectErrors" {
		t.Errorf("board_name = %v; want project value preserved", f.Errors.BoardName)
	}
}

func TestLoad_DevToolbarAbsentDefaults(t *testing.T) {
	withUserConfig(t, "")
	repo := writeProject(t, "")
	f := Load(repo)
	if f.DevToolbar != nil {
		t.Fatalf("DevToolbar should be nil when unset; got %+v", f.DevToolbar)
	}
}

func TestLoad_DevToolbarUserOverridesProject(t *testing.T) {
	withUserConfig(t, `[dev_toolbar]
enabled = true
`)
	repo := writeProject(t, `[dev_toolbar]
enabled = false
`)
	f := Load(repo)
	if f.DevToolbar == nil || f.DevToolbar.Enabled == nil || !*f.DevToolbar.Enabled {
		t.Fatalf("dev_toolbar.enabled = %+v; want true (user override)", f.DevToolbar)
	}
}

func TestLoad_BuildCopIntervalUserOverridesProject(t *testing.T) {
	withUserConfig(t, `[buildcop]
interval = "10m"
`)
	repo := writeProject(t, `[buildcop]
enabled = true
interval = "2m"
`)
	f := Load(repo)
	if f.BuildCop == nil {
		t.Fatal("buildcop section missing")
	}
	if f.BuildCop.Enabled == nil || !*f.BuildCop.Enabled {
		t.Errorf("enabled = %v; want true (project preserved)", f.BuildCop.Enabled)
	}
	if f.BuildCop.Interval == nil || *f.BuildCop.Interval != "10m" {
		t.Errorf("interval = %v; want \"10m\" (user override)", f.BuildCop.Interval)
	}
}

func TestLoad_UserOverridesProject_Sync(t *testing.T) {
	withUserConfig(t, `[sync]
allow_rebase = false
`)
	repo := writeProject(t, `[sync]
allow_rebase = true
allow_merge = false
`)

	f := Load(repo)
	if f.Sync == nil {
		t.Fatal("sync section missing")
	}
	if f.Sync.AllowRebase == nil || *f.Sync.AllowRebase {
		t.Errorf("allow_rebase = %v; want false (user override)", f.Sync.AllowRebase)
	}
	if f.Sync.AllowMerge == nil || *f.Sync.AllowMerge {
		t.Errorf("allow_merge = %v; want false (project preserved)", f.Sync.AllowMerge)
	}
}

func TestLoad_UserOverridesProject_Merge(t *testing.T) {
	withUserConfig(t, `[merge]
allow_merge_commit = true
`)
	repo := writeProject(t, `[merge]
allow_merge_commit = false
allow_squash = true
allow_rebase = false
`)

	f := Load(repo)
	if f.Merge == nil {
		t.Fatal("merge section missing")
	}
	if f.Merge.AllowMergeCommit == nil || !*f.Merge.AllowMergeCommit {
		t.Errorf("allow_merge_commit = %v; want true (user override)", f.Merge.AllowMergeCommit)
	}
	if f.Merge.AllowSquash == nil || !*f.Merge.AllowSquash {
		t.Errorf("allow_squash = %v; want true (project preserved)", f.Merge.AllowSquash)
	}
	if f.Merge.AllowRebase == nil || *f.Merge.AllowRebase {
		t.Errorf("allow_rebase = %v; want false (project preserved)", f.Merge.AllowRebase)
	}
}

func TestLoad_MergeAICommitMessage(t *testing.T) {
	withUserConfig(t, "")
	repo := writeProject(t, `[merge]
ai_commit_message = true
`)

	f := Load(repo)
	if f.Merge == nil || f.Merge.AICommitMessage == nil {
		t.Fatal("ai_commit_message missing")
	}
	if !*f.Merge.AICommitMessage {
		t.Errorf("ai_commit_message = %v; want true", *f.Merge.AICommitMessage)
	}
}

func TestLoad_UserOverridesProject_Harness(t *testing.T) {
	withUserConfig(t, `[harness]
id = "pi"
`)
	repo := writeProject(t, `[harness]
id = "claude"
`)

	f := Load(repo)
	if f.Harness == nil || f.Harness.ID == nil {
		t.Fatal("harness id missing")
	}
	if *f.Harness.ID != "pi" {
		t.Errorf("harness id = %q; want \"pi\"", *f.Harness.ID)
	}
}

func TestLoad_UserOnly(t *testing.T) {
	withUserConfig(t, `[github]
auto_move = true
draft_column = "WIP"
`)
	repo := writeProject(t, ``)

	f := Load(repo)
	if f.GitHub == nil {
		t.Fatal("github section missing")
	}
	if f.GitHub.AutoMove == nil || !*f.GitHub.AutoMove {
		t.Errorf("auto_move = %v; want true", f.GitHub.AutoMove)
	}
	if f.GitHub.DraftColumn == nil || *f.GitHub.DraftColumn != "WIP" {
		t.Errorf("draft_column = %v; want WIP", f.GitHub.DraftColumn)
	}
}

func TestLoad_ProjectOnly(t *testing.T) {
	withUserConfig(t, "")
	repo := writeProject(t, `[merge]
allow_squash = false
`)

	f := Load(repo)
	if f.Merge == nil || f.Merge.AllowSquash == nil || *f.Merge.AllowSquash {
		t.Errorf("allow_squash = %v; want false", f.Merge.AllowSquash)
	}
}

func TestLoad_TasksMergeByLabel(t *testing.T) {
	withUserConfig(t, `[[task]]
label = "Frontend"
container_port = 8080

[[task]]
label = "Tests"
container_port = 9000
`)
	repo := writeProject(t, `[[task]]
label = "Backend"
container_port = 7474

[[task]]
label = "Frontend"
container_port = 5173
`)

	f := Load(repo)
	if got := len(f.Tasks); got != 3 {
		t.Fatalf("len(tasks) = %d; want 3 (Backend, Frontend, Tests)", got)
	}

	port, ok := f.PortFor("Backend")
	if !ok || port != 7474 {
		t.Errorf("Backend port = %d, ok = %v; want 7474, true", port, ok)
	}
	port, ok = f.PortFor("Frontend")
	if !ok || port != 8080 {
		t.Errorf("Frontend port = %d, ok = %v; want 8080 (user override), true", port, ok)
	}
	port, ok = f.PortFor("Tests")
	if !ok || port != 9000 {
		t.Errorf("Tests port = %d, ok = %v; want 9000 (user-only), true", port, ok)
	}
}

func TestLoad_DevcontainerAppendsAndMergesEnv(t *testing.T) {
	withUserConfig(t, `[devcontainer]
mounts = ["type=bind,source=/tmp/ssh-agent.sock,target=/tmp/ssh-agent.sock"]
run_args = ["--cap-add=SYS_PTRACE"]

[devcontainer.container_env]
SSH_AUTH_SOCK = "/tmp/ssh-agent.sock"
LANG = "en_US.UTF-8"
`)
	repo := writeProject(t, `[devcontainer]
mounts = ["type=volume,source=node_modules,target=/workspace/node_modules"]

[devcontainer.container_env]
LANG = "C.UTF-8"
TZ = "UTC"
`)

	d := Load(repo).Devcontainer
	if d == nil {
		t.Fatal("devcontainer section missing")
	}

	if got := len(d.Mounts); got != 2 {
		t.Errorf("len(mounts) = %d; want 2 (project + user appended)", got)
	}
	if d.Mounts[0] != "type=volume,source=node_modules,target=/workspace/node_modules" {
		t.Errorf("mounts[0] = %q; want project entry first", d.Mounts[0])
	}
	if d.Mounts[1] != "type=bind,source=/tmp/ssh-agent.sock,target=/tmp/ssh-agent.sock" {
		t.Errorf("mounts[1] = %q; want user entry second", d.Mounts[1])
	}

	if got := len(d.RunArgs); got != 1 || d.RunArgs[0] != "--cap-add=SYS_PTRACE" {
		t.Errorf("run_args = %v; want [--cap-add=SYS_PTRACE]", d.RunArgs)
	}

	if got := d.ContainerEnv["LANG"]; got != "en_US.UTF-8" {
		t.Errorf("container_env[LANG] = %q; want en_US.UTF-8 (user override)", got)
	}
	if got := d.ContainerEnv["TZ"]; got != "UTC" {
		t.Errorf("container_env[TZ] = %q; want UTC (project preserved)", got)
	}
	if got := d.ContainerEnv["SSH_AUTH_SOCK"]; got != "/tmp/ssh-agent.sock" {
		t.Errorf("container_env[SSH_AUTH_SOCK] = %q; want /tmp/ssh-agent.sock (user-only)", got)
	}
}

func TestUserPath_KanbanConfigEnvOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/should/not/be/used")
	t.Setenv("KANBAN_CONFIG", "/tmp/custom/kanban.toml")

	got, err := UserPath()
	if err != nil {
		t.Fatalf("UserPath: %v", err)
	}
	if got != "/tmp/custom/kanban.toml" {
		t.Errorf("UserPath = %q; want /tmp/custom/kanban.toml", got)
	}
}

func TestLoad_NoFiles(t *testing.T) {
	withUserConfig(t, "")
	repo := writeProject(t, "")

	f := Load(repo)
	if f.Harness != nil || f.Sync != nil || f.Merge != nil || f.GitHub != nil || f.Devcontainer != nil || f.Tasks != nil {
		t.Errorf("expected fully empty File, got %+v", f)
	}
}

func TestWriteUserHarness_RoundTrip(t *testing.T) {
	withUserConfig(t, "")

	if err := WriteUserHarness("pi"); err != nil {
		t.Fatalf("write: %v", err)
	}
	f := Load("")
	if f.Harness == nil || f.Harness.ID == nil || *f.Harness.ID != "pi" {
		t.Fatalf("after write, harness id = %v; want pi", f.Harness)
	}

	// Clearing removes the file when nothing else is in it.
	if err := WriteUserHarness(""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	path, _ := UserPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("user config still exists after clear: err=%v", err)
	}
}

func TestWriteUserSignCommits_RoundTrip(t *testing.T) {
	withUserConfig(t, "")

	if err := WriteUserSignCommits(true); err != nil {
		t.Fatalf("write: %v", err)
	}
	f := Load("")
	if f.Git == nil || f.Git.SignCommits == nil || !*f.Git.SignCommits {
		t.Fatalf("after write, git.sign_commits = %v; want true", f.Git)
	}

	// Disabling (the default) clears the [git] section; with nothing else in
	// the file, the file is removed.
	if err := WriteUserSignCommits(false); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if f := Load(""); f.Git != nil {
		t.Errorf("git section still present after disable: %v", f.Git)
	}
}

func TestWriteUserHarness_PreservesOtherKeys(t *testing.T) {
	withUserConfig(t, `[sync]
allow_rebase = false
`)

	if err := WriteUserHarness("claude"); err != nil {
		t.Fatalf("write: %v", err)
	}
	f := Load("")
	if f.Harness == nil || f.Harness.ID == nil || *f.Harness.ID != "claude" {
		t.Errorf("harness id = %v; want claude", f.Harness)
	}
	if f.Sync == nil || f.Sync.AllowRebase == nil || *f.Sync.AllowRebase {
		t.Errorf("sync.allow_rebase = %v; want false (preserved)", f.Sync)
	}
}

func TestLoad_DevcontainerImageUserOverridesProject(t *testing.T) {
	withUserConfig(t, `[devcontainer]
image = "ghcr.io/me/custom:latest"
`)
	repo := writeProject(t, `[devcontainer]
image = "ghcr.io/team/base:1"
docker_socket = true
`)

	d := Load(repo).Devcontainer
	if d == nil || d.Image == nil {
		t.Fatal("devcontainer.image missing")
	}
	if *d.Image != "ghcr.io/me/custom:latest" {
		t.Errorf("image = %q; want user override", *d.Image)
	}
	if d.DockerSocket == nil || !*d.DockerSocket {
		t.Errorf("docker_socket = %v; want project value preserved", d.DockerSocket)
	}

	withUserConfig(t, "")
	d = Load(repo).Devcontainer
	if d == nil || d.Image == nil || *d.Image != "ghcr.io/team/base:1" {
		t.Errorf("image = %v; want project value when user file is absent", d.Image)
	}
}

func TestProjectPath_Lookup(t *testing.T) {
	dotConfig := filepath.Join(".config", "kanban", "config.toml")
	cases := []struct {
		name  string
		files []string // created under the repo; a trailing "/" makes a dir
		want  string
	}{
		{"none", nil, ".kanban.toml"},
		{"none with .config dir", []string{".config/"}, dotConfig},
		{"root file", []string{".kanban.toml"}, ".kanban.toml"},
		{"config file", []string{dotConfig}, dotConfig},
		{"both prefers root", []string{".kanban.toml", dotConfig}, ".kanban.toml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			for _, f := range tc.files {
				path := filepath.Join(repo, f)
				if f[len(f)-1] == '/' {
					if err := os.MkdirAll(path, 0o755); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got, want := ProjectPath(repo), filepath.Join(repo, tc.want); got != want {
				t.Errorf("ProjectPath = %q; want %q", got, want)
			}
		})
	}
}

func TestLoadFrom_DotConfigProjectFiles(t *testing.T) {
	withUserConfig(t, "")
	repo := t.TempDir()
	sub := filepath.Join(repo, "services", "api")
	for dir, body := range map[string]string{
		repo: "[branches]\nprefix = \"root\"\n[plans]\ndir = \"root-plans\"\n",
		sub:  "[branches]\nprefix = \"sub\"\n",
	} {
		cfg := filepath.Join(dir, ".config", "kanban")
		if err := os.MkdirAll(cfg, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfg, "config.toml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f := LoadFrom(repo, sub)
	if f.Branches == nil || f.Branches.Prefix == nil || *f.Branches.Prefix != "sub" {
		t.Errorf("branches.prefix = %+v; want subproject's \"sub\"", f.Branches)
	}
	if f.Plans == nil || f.Plans.Dir == nil || *f.Plans.Dir != "root-plans" {
		t.Errorf("plans.dir = %+v; want root's \"root-plans\"", f.Plans)
	}
}

// An undot-managed repo keeps .kanban.toml as a root symlink into .config/;
// editing it must update the target, not replace the link with a file.
func TestEditFile_WritesThroughSymlink(t *testing.T) {
	repo := t.TempDir()
	target := filepath.Join(repo, ".config", ".kanban.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("[branches]\nprefix = \"old\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(repo, ".kanban.toml")
	if err := os.Symlink(filepath.Join(".config", ".kanban.toml"), link); err != nil {
		t.Fatal(err)
	}

	path := ProjectPath(repo)
	if err := EditFile(path, map[string]any{"branches.prefix": "new"}, nil); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("root .kanban.toml is no longer a symlink (err=%v)", err)
	}
	f := readFileAt(target)
	if f.Branches == nil || f.Branches.Prefix == nil || *f.Branches.Prefix != "new" {
		t.Errorf("target branches.prefix = %+v; want \"new\"", f.Branches)
	}

	// Clearing the last key removes the target file, leaving a dangling link
	// for undot to manage rather than deleting the link itself.
	if err := EditFile(path, nil, []string{"branches.prefix"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("target should be removed when emptied; stat err = %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symlink should survive: %v", err)
	}

	// The link now dangles. With .config/ present, a later edit must still go
	// through it — recreating the target — not land in a second file at
	// .config/kanban/config.toml or replace the link with a plain file.
	path = ProjectPath(repo)
	if path != link {
		t.Fatalf("ProjectPath with dangling link = %q; want %q", path, link)
	}
	if err := EditFile(path, map[string]any{"branches.prefix": "again"}, nil); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("root .kanban.toml is no longer a symlink (err=%v)", err)
	}
	f = readFileAt(target)
	if f.Branches == nil || f.Branches.Prefix == nil || *f.Branches.Prefix != "again" {
		t.Errorf("recreated target branches.prefix = %+v; want \"again\"", f.Branches)
	}
	if _, err := os.Stat(filepath.Join(repo, ".config", "kanban", "config.toml")); !os.IsNotExist(err) {
		t.Errorf("no second project file expected; stat err = %v", err)
	}
}

func TestProjectFile_Containment(t *testing.T) {
	outside := t.TempDir()
	cases := []struct {
		name    string
		setup   func(t *testing.T, repo string)
		wantErr bool
	}{
		{"plain file", func(t *testing.T, repo string) {
			mustWrite(t, filepath.Join(repo, ".kanban.toml"))
		}, false},
		{"undot link inside", func(t *testing.T, repo string) {
			mustWrite(t, filepath.Join(repo, ".config", ".kanban.toml"))
			mustLink(t, filepath.Join(".config", ".kanban.toml"), filepath.Join(repo, ".kanban.toml"))
		}, false},
		{"dangling link inside", func(t *testing.T, repo string) {
			mustLink(t, filepath.Join(".config", ".kanban.toml"), filepath.Join(repo, ".kanban.toml"))
		}, false},
		{"link outside", func(t *testing.T, repo string) {
			victim := filepath.Join(outside, "victim.toml")
			mustWrite(t, victim)
			mustLink(t, victim, filepath.Join(repo, ".kanban.toml"))
		}, true},
		{"dangling link outside", func(t *testing.T, repo string) {
			mustLink(t, filepath.Join(outside, "nope", "x.toml"), filepath.Join(repo, ".kanban.toml"))
		}, true},
		{"relative link climbing out", func(t *testing.T, repo string) {
			mustLink(t, filepath.Join("..", "x.toml"), filepath.Join(repo, ".kanban.toml"))
		}, true},
		{".config dir linked outside", func(t *testing.T, repo string) {
			mustLink(t, outside, filepath.Join(repo, ".config"))
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			tc.setup(t, repo)
			path, err := ProjectFile(repo)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ProjectFile = %q, %v; wantErr %v", path, err, tc.wantErr)
			}
		})
	}
}

// A repo reached through a symlinked directory still contains its own files.
func TestProjectFile_RepoBehindSymlink(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, ".config", ".kanban.toml"))
	mustLink(t, filepath.Join(".config", ".kanban.toml"), filepath.Join(repo, ".kanban.toml"))
	alias := filepath.Join(t.TempDir(), "alias")
	mustLink(t, repo, alias)
	if _, err := ProjectFile(alias); err != nil {
		t.Fatalf("ProjectFile via aliased repo: %v", err)
	}
}

func TestLoad_IgnoresProjectLinkOutsideRepo(t *testing.T) {
	withUserConfig(t, "")
	victim := filepath.Join(t.TempDir(), "victim.toml")
	if err := os.WriteFile(victim, []byte("[branches]\nprefix = \"leaked\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	mustLink(t, victim, filepath.Join(repo, ".kanban.toml"))
	if f := Load(repo); f.Branches != nil {
		t.Fatalf("Load read through an escaping link: %+v", f.Branches)
	}
	if project, _ := LoadLayers(repo); project.Branches != nil {
		t.Fatalf("LoadLayers read through an escaping link: %+v", project.Branches)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
