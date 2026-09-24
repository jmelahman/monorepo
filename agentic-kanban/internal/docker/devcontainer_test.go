package docker

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"
)

func TestStreamPullProgress(t *testing.T) {
	stream := strings.Join([]string{
		`{"status":"Pulling from library/alpine","id":"latest"}`,
		`{"status":"Pulling fs layer","progressDetail":{},"id":"abc"}`,
		`{"status":"Pulling fs layer","progressDetail":{},"id":"def"}`,
		`{"status":"Downloading","progressDetail":{"current":50,"total":100},"id":"abc"}`,
		`{"status":"Downloading","progressDetail":{"current":200,"total":400},"id":"def"}`,
		`{"status":"Pull complete","progressDetail":{},"id":"abc"}`,
		`{"status":"Pull complete","progressDetail":{},"id":"def"}`,
		`{"status":"Status: Downloaded newer image"}`,
	}, "\n")
	var snaps []PullProgress
	if err := streamPullProgress(strings.NewReader(stream), "alpine:latest", func(p PullProgress) {
		snaps = append(snaps, p)
	}); err != nil {
		t.Fatalf("streamPullProgress: %v", err)
	}
	if len(snaps) == 0 {
		t.Fatal("no snapshots emitted")
	}
	last := snaps[len(snaps)-1]
	if !last.Done {
		t.Errorf("final snapshot Done = false, want true")
	}
	// "Pull complete" snaps each layer's current to its total, so the final
	// emission must report a fully-fetched 500/500 across both layers.
	if last.Current != 500 || last.Total != 500 {
		t.Errorf("final aggregate = %d/%d; want 500/500", last.Current, last.Total)
	}
	if last.Layers != 2 {
		t.Errorf("final layers = %d; want 2", last.Layers)
	}
}

func TestStreamPullProgress_Error(t *testing.T) {
	stream := `{"errorDetail":{"message":"boom"},"error":"boom"}`
	err := streamPullProgress(strings.NewReader(stream), "x", func(PullProgress) {})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v; want error containing \"boom\"", err)
	}
}

func TestSubstitute(t *testing.T) {
	t.Setenv("KANBAN_TEST_SET", "world")
	t.Setenv("KANBAN_TEST_EMPTY", "")

	ctx := NewSubstitutionContext("/host/onyx", "/workspace", "/host/onyx")

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", "hello"},
		{"localEnv set", "${localEnv:KANBAN_TEST_SET}", "world"},
		{"localEnv unset no default", "${localEnv:KANBAN_TEST_UNSET}", ""},
		{"localEnv unset with default", "${localEnv:KANBAN_TEST_UNSET:dev}", "dev"},
		{"localEnv set ignores default", "${localEnv:KANBAN_TEST_SET:dev}", "world"},
		{"localEnv empty-but-set keeps empty", "${localEnv:KANBAN_TEST_EMPTY:dev}", ""},
		{"localWorkspaceFolder", "${localWorkspaceFolder}/sub", "/host/onyx/sub"},
		{"localWorkspaceFolderBasename", "${localWorkspaceFolderBasename}", "onyx"},
		{"containerWorkspaceFolder", "${containerWorkspaceFolder}", "/workspace"},
		{"containerWorkspaceFolderBasename", "${containerWorkspaceFolderBasename}", "workspace"},
		{"containerEnv left literal", "${containerEnv:PATH}", "${containerEnv:PATH}"},
		{"unknown var left literal", "${notARealVar}", "${notARealVar}"},
		{"multiple vars in one string",
			"user=${localEnv:KANBAN_TEST_SET},dir=${containerWorkspaceFolder}",
			"user=world,dir=/workspace"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Substitute(tc.in, ctx); got != tc.want {
				t.Errorf("Substitute(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSubstitute_DevcontainerIDIsStable(t *testing.T) {
	a := NewSubstitutionContext("/foo", "/workspace", "/foo").DevcontainerID
	b := NewSubstitutionContext("/foo", "/workspace", "/foo").DevcontainerID
	c := NewSubstitutionContext("/bar", "/workspace", "/bar").DevcontainerID
	if a != b {
		t.Errorf("devcontainerId not stable for same worktree: %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("devcontainerId collided across worktrees: %q", a)
	}
}

func TestSubstitute_DevcontainerIDIgnoresProjectDir(t *testing.T) {
	// The id identifies the container, which is the same container whichever
	// subdirectory the agent sits in. Seeding it from the project-joined path
	// would rename every ${devcontainerId} named volume on upgrade and orphan
	// warm caches.
	opts := SpawnOptions{WorktreePath: "/host/wt"}
	plain := resolveWorkspaceLayout(opts, "/workspace")

	opts.ProjectDir = "services/api"
	sub := resolveWorkspaceLayout(opts, "/workspace")

	if sub.ContainerFolder == plain.ContainerFolder {
		t.Fatalf("project_dir did not change the working directory: %q", sub.ContainerFolder)
	}
	a := NewSubstitutionContext(plain.LocalFolder, plain.ContainerFolder, plain.MountSource).DevcontainerID
	b := NewSubstitutionContext(sub.LocalFolder, sub.ContainerFolder, sub.MountSource).DevcontainerID
	if a != b {
		t.Errorf("devcontainerId changed with project_dir: %q vs %q", a, b)
	}
}

func TestSubstitute_WorkspaceFolderPairing(t *testing.T) {
	opts := SpawnOptions{WorktreePath: "/host/monorepo", ProjectDir: "services/api"}
	layout := resolveWorkspaceLayout(opts, "/workspace")

	if layout.MountSource != "/host/monorepo" {
		t.Errorf("MountSource = %q; want %q", layout.MountSource, "/host/monorepo")
	}
	if layout.MountTarget != "/workspace" {
		t.Errorf("MountTarget = %q; want %q", layout.MountTarget, "/workspace")
	}
	if want := filepath.Join("/host/monorepo", "services", "api"); layout.LocalFolder != want {
		t.Errorf("LocalFolder = %q; want %q", layout.LocalFolder, want)
	}
	if layout.ContainerFolder != "/workspace/services/api" {
		t.Errorf("ContainerFolder = %q; want %q", layout.ContainerFolder, "/workspace/services/api")
	}

	ctx := NewSubstitutionContext(layout.LocalFolder, layout.ContainerFolder, layout.MountSource)
	cases := map[string]string{
		"${localWorkspaceFolder}":             filepath.Join("/host/monorepo", "services", "api"),
		"${localWorkspaceFolderBasename}":     "api",
		"${containerWorkspaceFolder}":         "/workspace/services/api",
		"${containerWorkspaceFolderBasename}": "api",
	}
	for in, want := range cases {
		if got := Substitute(in, ctx); got != want {
			t.Errorf("Substitute(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestBuildContainerConfig_ProjectDir(t *testing.T) {
	// The regression guard for the whole monorepo feature: the *whole*
	// worktree still binds at the workspace folder (git needs its index and
	// gitdir pointer), and only the working directory moves down.
	cases := []struct {
		name            string
		workspaceFolder string
		wantTarget      string
		wantWorkingDir  string
	}{
		{
			name:            "default workspace folder",
			workspaceFolder: "/workspace",
			wantTarget:      "/workspace",
			wantWorkingDir:  "/workspace/services/api",
		},
		{
			name:            "custom workspace folder",
			workspaceFolder: "/src",
			wantTarget:      "/src",
			wantWorkingDir:  "/src/services/api",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &DevcontainerConfig{WorkspaceFolder: tc.workspaceFolder}
			opts := SpawnOptions{
				WorktreePath:  "/host/wt",
				ProjectDir:    "services/api",
				ContainerName: "test",
			}
			layout := resolveWorkspaceLayout(opts, cfg.WorkspaceFolder)

			hostCfg, _, containerCfg, err := buildContainerConfig(cfg, opts, layout, "img", "")
			if err != nil {
				t.Fatalf("buildContainerConfig: %v", err)
			}
			var ws *mount.Mount
			for i, m := range hostCfg.Mounts {
				if m.Target == tc.wantTarget {
					ws = &hostCfg.Mounts[i]
				}
			}
			if ws == nil {
				t.Fatalf("no mount targeting %q; got %#v", tc.wantTarget, hostCfg.Mounts)
			}
			if ws.Source != "/host/wt" {
				t.Errorf("workspace mount source = %q; want the whole worktree %q", ws.Source, "/host/wt")
			}
			if containerCfg.WorkingDir != tc.wantWorkingDir {
				t.Errorf("WorkingDir = %q; want %q", containerCfg.WorkingDir, tc.wantWorkingDir)
			}
		})
	}
}

func TestDevcontainerConfig_Substitute(t *testing.T) {
	t.Setenv("KANBAN_TEST_USER", "root")
	t.Setenv("KANBAN_TEST_HOME", "/h")

	cfg := &DevcontainerConfig{
		Name:  "${localEnv:KANBAN_TEST_USER}-box",
		Image: "img",
		Build: BuildConfig{
			Args: map[string]string{"USER": "${localEnv:KANBAN_TEST_USER}"},
		},
		RunArgs: []string{"--name", "${localEnv:KANBAN_TEST_USER}"},
		Mounts: []string{
			"source=${localEnv:KANBAN_TEST_HOME}/.claude,target=/root/.claude,type=bind",
		},
		WorkspaceMount:   "source=${localWorkspaceFolder},target=${containerWorkspaceFolder}",
		WorkspaceFolder:  "/workspace",
		RemoteUser:       "${localEnv:KANBAN_TEST_USER:dev}",
		ContainerEnv:     map[string]string{"GH_TOKEN": "${localEnv:KANBAN_TEST_HOME}"},
		PostStartCommand: "echo ${localWorkspaceFolderBasename}",
	}

	cfg.Substitute(NewSubstitutionContext("/host/proj", cfg.WorkspaceFolder, "/host/proj"))

	want := &DevcontainerConfig{
		Name:  "root-box",
		Image: "img",
		Build: BuildConfig{
			Args: map[string]string{"USER": "root"},
		},
		RunArgs: []string{"--name", "root"},
		Mounts: []string{
			"source=/h/.claude,target=/root/.claude,type=bind",
		},
		WorkspaceMount:   "source=/host/proj,target=/workspace",
		WorkspaceFolder:  "/workspace",
		RemoteUser:       "root",
		ContainerEnv:     map[string]string{"GH_TOKEN": "/h"},
		PostStartCommand: "echo proj",
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Substitute mismatch.\n got: %#v\nwant: %#v", cfg, want)
	}
}

func TestDevcontainerConfig_Substitute_RemoteUserDefault(t *testing.T) {
	// Reproduces the original bug: ${localEnv:VAR:default} in remoteUser was
	// passed verbatim to Docker, causing "unable to find user ${localEnv:".
	// Kanban session containers export DEVCONTAINER_REMOTE_USER themselves;
	// unset it (t.Setenv registers the restore) so the default-applied half
	// holds when the tests run inside one.
	t.Setenv("DEVCONTAINER_REMOTE_USER", "")
	os.Unsetenv("DEVCONTAINER_REMOTE_USER")
	cfg := &DevcontainerConfig{RemoteUser: "${localEnv:DEVCONTAINER_REMOTE_USER:dev}"}
	cfg.Substitute(NewSubstitutionContext("/x", "/workspace", "/x"))
	if cfg.RemoteUser != "dev" {
		t.Errorf("RemoteUser = %q; want %q (default applied)", cfg.RemoteUser, "dev")
	}

	t.Setenv("DEVCONTAINER_REMOTE_USER", "root")
	cfg = &DevcontainerConfig{RemoteUser: "${localEnv:DEVCONTAINER_REMOTE_USER:dev}"}
	cfg.Substitute(NewSubstitutionContext("/x", "/workspace", "/x"))
	if cfg.RemoteUser != "root" {
		t.Errorf("RemoteUser = %q; want %q (env var applied)", cfg.RemoteUser, "root")
	}
}

func TestBuildContainerConfig_SourceRepoGitMount(t *testing.T) {
	// Simulate a parent repo on disk with a real .git directory; the worktree's
	// gitdir pointer references this absolute path, so we bind-mount it as-is
	// into the container.
	repo := t.TempDir()
	gitDir := filepath.Join(repo, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &DevcontainerConfig{WorkspaceFolder: "/workspace"}
	opts := SpawnOptions{
		WorktreePath:   "/host/worktree",
		SourceRepoPath: repo,
		ContainerName:  "test",
	}

	hostCfg, _, _, err := buildContainerConfig(cfg, opts, resolveWorkspaceLayout(opts, cfg.WorkspaceFolder), "img", "")
	if err != nil {
		t.Fatalf("buildContainerConfig: %v", err)
	}

	var found bool
	for _, m := range hostCfg.Mounts {
		if m.Type == mount.TypeBind && m.Source == gitDir && m.Target == gitDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf(".git bind mount missing.\n got mounts: %#v\n want bind src=target=%q", hostCfg.Mounts, gitDir)
	}
}

func TestBuildContainerConfig_TranslatesHostPaths(t *testing.T) {
	// When kanban runs inside a devcontainer, paths it sees as /workspace/...
	// must be rewritten to the matching host path before being handed to
	// dockerd (which only knows host paths).
	t.Setenv("KANBAN_HOST_WORKSPACE", "/host/proj")

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &DevcontainerConfig{
		WorkspaceFolder: "/workspace",
		Mounts:          []string{"type=bind,source=/workspace/extra,target=/extra"},
	}
	opts := SpawnOptions{
		MountPath:        "/workspace/wt",
		RepoWorktreePath: "/workspace/wt-repo",
		SourceRepoPath:   repo, // outside the translation prefix; passes through
		ContainerName:    "test",
	}

	hostCfg, _, _, err := buildContainerConfig(cfg, opts, resolveWorkspaceLayout(opts, cfg.WorkspaceFolder), "img", "")
	if err != nil {
		t.Fatalf("buildContainerConfig: %v", err)
	}

	wantSources := map[string]string{
		"/workspace":                "", // workspace mount target
		"/repository":               "", // repo worktree target
		"/extra":                    "", // user-supplied extra mount
		filepath.Join(repo, ".git"): "", // .git target stays in-container
	}
	wantSources["/workspace"] = "/host/proj/wt"
	wantSources["/repository"] = "/host/proj/wt-repo"
	wantSources["/extra"] = "/host/proj/extra"
	wantSources[filepath.Join(repo, ".git")] = filepath.Join(repo, ".git")

	for _, m := range hostCfg.Mounts {
		want, ok := wantSources[m.Target]
		if !ok {
			continue
		}
		if m.Source != want {
			t.Errorf("mount target=%q: source = %q; want %q", m.Target, m.Source, want)
		}
		delete(wantSources, m.Target)
	}
	if len(wantSources) > 0 {
		t.Errorf("missing mounts for targets: %v\n got: %#v", wantSources, hostCfg.Mounts)
	}
}

func TestBuildContainerConfig_NoGitMountWhenSourceMissing(t *testing.T) {
	cfg := &DevcontainerConfig{WorkspaceFolder: "/workspace"}
	opts := SpawnOptions{WorktreePath: "/host/worktree", ContainerName: "test"}

	hostCfg, _, _, err := buildContainerConfig(cfg, opts, resolveWorkspaceLayout(opts, cfg.WorkspaceFolder), "img", "")
	if err != nil {
		t.Fatalf("buildContainerConfig: %v", err)
	}

	if len(hostCfg.Mounts) != 1 {
		t.Errorf("expected only the workspace mount; got %#v", hostCfg.Mounts)
	}
}

func TestLoadDevcontainer_FallbackOrder(t *testing.T) {
	write := func(t *testing.T, path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("prefers .devcontainer/devcontainer.json", func(t *testing.T) {
		repo := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		write(t, filepath.Join(repo, ".devcontainer", "devcontainer.json"), `{"name":"primary"}`)
		write(t, filepath.Join(repo, ".devcontainer.json"), `{"name":"alt"}`)
		write(t, filepath.Join(xdg, "kanban", "devcontainer.json"), `{"name":"user"}`)

		cfg, err := LoadDevcontainer(repo)
		if err != nil {
			t.Fatalf("LoadDevcontainer: %v", err)
		}
		if cfg.Name != "primary" {
			t.Errorf("Name = %q; want %q", cfg.Name, "primary")
		}
	})

	t.Run("falls back to .devcontainer.json", func(t *testing.T) {
		repo := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		write(t, filepath.Join(repo, ".devcontainer.json"), `{"name":"alt"}`)
		write(t, filepath.Join(xdg, "kanban", "devcontainer.json"), `{"name":"user"}`)

		cfg, err := LoadDevcontainer(repo)
		if err != nil {
			t.Fatalf("LoadDevcontainer: %v", err)
		}
		if cfg.Name != "alt" {
			t.Errorf("Name = %q; want %q", cfg.Name, "alt")
		}
	})

	t.Run("falls back to user single-file config when repo has none", func(t *testing.T) {
		repo := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		write(t, filepath.Join(xdg, "kanban", "devcontainer.json"), `{"name":"user"}`)

		cfg, err := LoadDevcontainer(repo)
		if err != nil {
			t.Fatalf("LoadDevcontainer: %v", err)
		}
		if cfg.Name != "user" {
			t.Errorf("Name = %q; want %q", cfg.Name, "user")
		}
		wantDir := filepath.Join(xdg, "kanban")
		if cfg.ConfigDir != wantDir {
			t.Errorf("ConfigDir = %q; want %q", cfg.ConfigDir, wantDir)
		}
	})

	t.Run("prefers user .devcontainer/ dir over single-file", func(t *testing.T) {
		repo := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		write(t, filepath.Join(xdg, "kanban", ".devcontainer", "devcontainer.json"), `{"name":"user-dir"}`)
		write(t, filepath.Join(xdg, "kanban", "devcontainer.json"), `{"name":"user-file"}`)

		cfg, err := LoadDevcontainer(repo)
		if err != nil {
			t.Fatalf("LoadDevcontainer: %v", err)
		}
		if cfg.Name != "user-dir" {
			t.Errorf("Name = %q; want %q", cfg.Name, "user-dir")
		}
		wantDir := filepath.Join(xdg, "kanban", ".devcontainer")
		if cfg.ConfigDir != wantDir {
			t.Errorf("ConfigDir = %q; want %q", cfg.ConfigDir, wantDir)
		}
	})

	t.Run("prefers the subproject over the worktree root", func(t *testing.T) {
		repo := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		project := filepath.Join(repo, "services", "api")
		write(t, filepath.Join(project, ".devcontainer", "devcontainer.json"), `{"name":"project"}`)
		write(t, filepath.Join(repo, ".devcontainer", "devcontainer.json"), `{"name":"root"}`)
		write(t, filepath.Join(xdg, "kanban", "devcontainer.json"), `{"name":"user"}`)

		cfg, err := LoadDevcontainerFrom(project, repo)
		if err != nil {
			t.Fatalf("LoadDevcontainerFrom: %v", err)
		}
		if cfg.Name != "project" {
			t.Errorf("Name = %q; want %q", cfg.Name, "project")
		}
	})

	t.Run("falls back to the worktree root when the subproject has none", func(t *testing.T) {
		repo := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		project := filepath.Join(repo, "services", "api")
		if err := os.MkdirAll(project, 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(repo, ".devcontainer", "devcontainer.json"), `{"name":"root"}`)
		write(t, filepath.Join(xdg, "kanban", "devcontainer.json"), `{"name":"user"}`)

		cfg, err := LoadDevcontainerFrom(project, repo)
		if err != nil {
			t.Fatalf("LoadDevcontainerFrom: %v", err)
		}
		if cfg.Name != "root" {
			t.Errorf("Name = %q; want %q", cfg.Name, "root")
		}
	})

	t.Run("duplicate roots probe once and still reach the user fallback", func(t *testing.T) {
		// A whole-repo board passes the same path twice (project root ==
		// worktree). Dedupe must not change the outcome.
		repo := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		write(t, filepath.Join(xdg, "kanban", "devcontainer.json"), `{"name":"user"}`)

		cfg, err := LoadDevcontainerFrom(repo, repo)
		if err != nil {
			t.Fatalf("LoadDevcontainerFrom: %v", err)
		}
		if cfg.Name != "user" {
			t.Errorf("Name = %q; want %q", cfg.Name, "user")
		}
	})

	t.Run("falls back to built-in when neither repo nor user config has one", func(t *testing.T) {
		repo := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		cfg, err := LoadDevcontainer(repo)
		if err != nil {
			t.Fatalf("LoadDevcontainer: %v", err)
		}
		if !cfg.BuiltIn {
			t.Errorf("BuiltIn = false; want true")
		}
		if cfg.Image != BuiltinImage {
			t.Errorf("Image = %q; want %q", cfg.Image, BuiltinImage)
		}
	})
}

func TestLoadDevcontainer_JSONCAndObjectMounts(t *testing.T) {
	repo := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	body := `{
	  // Comments, trailing commas, and object-form mounts are all legal in
	  // devcontainer.json.
	  "image": "example/img:1",
	  "containerUser": "node",
	  "mounts": [
	    "source=vol1,target=/a,type=volume",
	    {"source": "vol2", "target": "/b", "type": "volume"},
	  ],
	}`
	path := filepath.Join(repo, ".devcontainer", "devcontainer.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadDevcontainer(repo)
	if err != nil {
		t.Fatalf("LoadDevcontainer: %v", err)
	}
	if cfg.Image != "example/img:1" {
		t.Errorf("Image = %q; want %q", cfg.Image, "example/img:1")
	}
	if cfg.ContainerUser != "node" {
		t.Errorf("ContainerUser = %q; want %q", cfg.ContainerUser, "node")
	}
	want := MountList{
		"source=vol1,target=/a,type=volume",
		"type=volume,source=vol2,target=/b",
	}
	if !reflect.DeepEqual(cfg.Mounts, want) {
		t.Errorf("Mounts = %#v; want %#v", cfg.Mounts, want)
	}
}

func TestBuildContainerConfig_ContainerUserFallback(t *testing.T) {
	opts := SpawnOptions{WorktreePath: "/tmp/wt"}

	cfg := &DevcontainerConfig{WorkspaceFolder: "/workspace", ContainerUser: "node"}
	_, _, containerCfg, err := buildContainerConfig(cfg, opts, resolveWorkspaceLayout(opts, cfg.WorkspaceFolder), "img", "")
	if err != nil {
		t.Fatalf("buildContainerConfig: %v", err)
	}
	if containerCfg.User != "node" {
		t.Errorf("User = %q; want containerUser fallback %q", containerCfg.User, "node")
	}

	cfg = &DevcontainerConfig{WorkspaceFolder: "/workspace", RemoteUser: "dev", ContainerUser: "node"}
	_, _, containerCfg, err = buildContainerConfig(cfg, opts, resolveWorkspaceLayout(opts, cfg.WorkspaceFolder), "img", "")
	if err != nil {
		t.Fatalf("buildContainerConfig: %v", err)
	}
	if containerCfg.User != "dev" {
		t.Errorf("User = %q; want remoteUser %q to win", containerCfg.User, "dev")
	}
}

func TestResolveBuildPaths(t *testing.T) {
	write := func(t *testing.T, path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("FROM scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("repo .devcontainer/ Dockerfile", func(t *testing.T) {
		repo := t.TempDir()
		write(t, filepath.Join(repo, ".devcontainer", "Dockerfile"))
		cfg := &DevcontainerConfig{ConfigDir: filepath.Join(repo, ".devcontainer")}

		ctxDir, dfPath := resolveBuildPaths(cfg, repo)
		if ctxDir != filepath.Join(repo, ".devcontainer") {
			t.Errorf("contextDir = %q; want %q", ctxDir, filepath.Join(repo, ".devcontainer"))
		}
		if dfPath != filepath.Join(repo, ".devcontainer", "Dockerfile") {
			t.Errorf("dockerfilePath = %q; want %q", dfPath, filepath.Join(repo, ".devcontainer", "Dockerfile"))
		}
	})

	t.Run("repo .devcontainer/ json with repo-root Dockerfile", func(t *testing.T) {
		repo := t.TempDir()
		write(t, filepath.Join(repo, "Dockerfile"))
		cfg := &DevcontainerConfig{ConfigDir: filepath.Join(repo, ".devcontainer")}

		_, dfPath := resolveBuildPaths(cfg, repo)
		if dfPath != filepath.Join(repo, "Dockerfile") {
			t.Errorf("dockerfilePath = %q; want fallback to %q", dfPath, filepath.Join(repo, "Dockerfile"))
		}
	})

	t.Run("subproject Dockerfile beats the monorepo root's", func(t *testing.T) {
		// Spawn hands resolveBuildPaths the project root first, so a subproject
		// devcontainer.json naming a sibling Dockerfile must not silently build
		// the monorepo root's instead.
		repo := t.TempDir()
		project := filepath.Join(repo, "services", "api")
		write(t, filepath.Join(repo, "Dockerfile"))
		write(t, filepath.Join(project, "Dockerfile"))
		cfg := &DevcontainerConfig{ConfigDir: filepath.Join(project, ".devcontainer")}

		ctxDir, dfPath := resolveBuildPaths(cfg, project, repo)
		if dfPath != filepath.Join(project, "Dockerfile") {
			t.Errorf("dockerfilePath = %q; want %q", dfPath, filepath.Join(project, "Dockerfile"))
		}
		if ctxDir != project {
			t.Errorf("contextDir = %q; want %q", ctxDir, project)
		}
	})

	t.Run("an inherited repo-root config keeps the repo-root Dockerfile", func(t *testing.T) {
		// The mirror image: the subproject ships no devcontainer.json, so the
		// repo root's is loaded. Its `Dockerfile` means the one beside it —
		// not an unrelated file that happens to sit in the subproject.
		repo := t.TempDir()
		project := filepath.Join(repo, "services", "api")
		write(t, filepath.Join(repo, "Dockerfile"))
		write(t, filepath.Join(project, "Dockerfile"))
		cfg := &DevcontainerConfig{ConfigDir: filepath.Join(repo, ".devcontainer")}

		ctxDir, dfPath := resolveBuildPaths(cfg, project, repo)
		if dfPath != filepath.Join(repo, "Dockerfile") {
			t.Errorf("dockerfilePath = %q; want %q", dfPath, filepath.Join(repo, "Dockerfile"))
		}
		if ctxDir != repo {
			t.Errorf("contextDir = %q; want %q", ctxDir, repo)
		}
	})

	t.Run("nothing on disk reports the path the config named", func(t *testing.T) {
		repo := t.TempDir()
		project := filepath.Join(repo, "services", "api")
		cfg := &DevcontainerConfig{ConfigDir: filepath.Join(project, ".devcontainer")}

		_, dfPath := resolveBuildPaths(cfg, project, repo)
		want := filepath.Join(project, ".devcontainer", "Dockerfile")
		if dfPath != want {
			t.Errorf("dockerfilePath = %q; want %q", dfPath, want)
		}
	})

	t.Run("user .devcontainer/ Dockerfile resolves under user dir", func(t *testing.T) {
		repo := t.TempDir()
		userDir := t.TempDir()
		userDevcontainer := filepath.Join(userDir, ".devcontainer")
		write(t, filepath.Join(userDevcontainer, "Dockerfile"))
		cfg := &DevcontainerConfig{ConfigDir: userDevcontainer}

		ctxDir, dfPath := resolveBuildPaths(cfg, repo)
		if ctxDir != userDevcontainer {
			t.Errorf("contextDir = %q; want %q", ctxDir, userDevcontainer)
		}
		if dfPath != filepath.Join(userDevcontainer, "Dockerfile") {
			t.Errorf("dockerfilePath = %q; want %q", dfPath, filepath.Join(userDevcontainer, "Dockerfile"))
		}
	})

	t.Run("user single-file json with sibling Dockerfile", func(t *testing.T) {
		repo := t.TempDir()
		userDir := t.TempDir()
		write(t, filepath.Join(userDir, "Dockerfile"))
		cfg := &DevcontainerConfig{ConfigDir: userDir}

		ctxDir, dfPath := resolveBuildPaths(cfg, repo)
		if ctxDir != userDir {
			t.Errorf("contextDir = %q; want %q", ctxDir, userDir)
		}
		if dfPath != filepath.Join(userDir, "Dockerfile") {
			t.Errorf("dockerfilePath = %q; want %q", dfPath, filepath.Join(userDir, "Dockerfile"))
		}
	})

	t.Run("custom dockerfile name and context", func(t *testing.T) {
		userDir := t.TempDir()
		write(t, filepath.Join(userDir, "build", "Dockerfile.dev"))
		cfg := &DevcontainerConfig{
			ConfigDir: userDir,
			Build:     BuildConfig{Dockerfile: "build/Dockerfile.dev", Context: "build"},
		}

		ctxDir, dfPath := resolveBuildPaths(cfg, "/unused/repo")
		if ctxDir != filepath.Join(userDir, "build") {
			t.Errorf("contextDir = %q; want %q", ctxDir, filepath.Join(userDir, "build"))
		}
		if dfPath != filepath.Join(userDir, "build", "Dockerfile.dev") {
			t.Errorf("dockerfilePath = %q; want %q", dfPath, filepath.Join(userDir, "build", "Dockerfile.dev"))
		}
	})
}

func TestClaudeConfigMounts(t *testing.T) {
	t.Run("returns mounts for both files when present", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		// Pin the user explicitly so the auto-detect path doesn't pick a
		// different target based on whatever UID owns the temp dir on the
		// runner — the assertion below is about mount construction, not
		// identity resolution.
		t.Setenv("DEVCONTAINER_REMOTE_USER", "dev")
		t.Setenv("DEVCONTAINER_REMOTE_HOME", "")
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}

		want := []string{
			"type=bind,source=" + filepath.Join(home, ".claude") + ",target=/home/dev/.claude",
			"type=bind,source=" + filepath.Join(home, ".claude.json") + ",target=/home/dev/.claude.json",
		}
		got := ClaudeConfigMounts()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ClaudeConfigMounts() = %v; want %v", got, want)
		}
	})

	t.Run("skips files that don't exist", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}

		got := ClaudeConfigMounts()
		if len(got) != 1 || !strings.Contains(got[0], ".claude.json") {
			t.Errorf("ClaudeConfigMounts() = %v; want only .claude.json mount", got)
		}
	})

	t.Run("returns nil when nothing exists", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if got := ClaudeConfigMounts(); got != nil {
			t.Errorf("ClaudeConfigMounts() = %v; want nil", got)
		}
	})

	t.Run("DEVCONTAINER_REMOTE_HOME redirects targets", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("DEVCONTAINER_REMOTE_HOME", "/root")
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}

		want := []string{
			"type=bind,source=" + filepath.Join(home, ".claude") + ",target=/root/.claude",
			"type=bind,source=" + filepath.Join(home, ".claude.json") + ",target=/root/.claude.json",
		}
		got := ClaudeConfigMounts()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ClaudeConfigMounts() = %v; want %v", got, want)
		}
	})
}

func TestBuiltinDevcontainer_RemoteUser(t *testing.T) {
	t.Run("falls back to dev when env unset and no ~/.claude", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("DEVCONTAINER_REMOTE_USER", "")
		cfg := BuiltinDevcontainer()
		if cfg.RemoteUser != "dev" {
			t.Errorf("RemoteUser = %q; want %q", cfg.RemoteUser, "dev")
		}
	})

	t.Run("honors DEVCONTAINER_REMOTE_USER", func(t *testing.T) {
		t.Setenv("DEVCONTAINER_REMOTE_USER", "root")
		cfg := BuiltinDevcontainer()
		if cfg.RemoteUser != "root" {
			t.Errorf("RemoteUser = %q; want %q", cfg.RemoteUser, "root")
		}
	})
}

func TestBuiltinIdentity(t *testing.T) {
	// The auto-detect path stats the host's ~/.claude and picks the
	// built-in account whose UID owns it. The test process owns whatever
	// files it creates, so the expected detection result depends on
	// $UID — UID 0 maps to root, UID 1000 maps to dev, everything else
	// falls back to dev.
	expectedForCurrentUID := func() string {
		switch os.Getuid() {
		case 0:
			return "root"
		case 1000:
			return "dev"
		default:
			return "dev"
		}
	}
	expectedHomeFor := func(user string) string {
		if user == "root" {
			return "/root"
		}
		return "/home/dev"
	}

	t.Run("auto-detects from .claude owner", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("DEVCONTAINER_REMOTE_USER", "")
		t.Setenv("DEVCONTAINER_REMOTE_HOME", "")
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
			t.Fatal(err)
		}
		wantUser := expectedForCurrentUID()
		gotUser, gotHome := builtinIdentity()
		if gotUser != wantUser {
			t.Errorf("user = %q; want %q (UID %d)", gotUser, wantUser, os.Getuid())
		}
		if gotHome != expectedHomeFor(wantUser) {
			t.Errorf("home = %q; want %q", gotHome, expectedHomeFor(wantUser))
		}
	})

	t.Run("auto-detects from .claude.json when directory missing", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("DEVCONTAINER_REMOTE_USER", "")
		t.Setenv("DEVCONTAINER_REMOTE_HOME", "")
		if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		wantUser := expectedForCurrentUID()
		gotUser, _ := builtinIdentity()
		if gotUser != wantUser {
			t.Errorf("user = %q; want %q (UID %d)", gotUser, wantUser, os.Getuid())
		}
	})

	t.Run("falls back to dev when nothing to stat", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("DEVCONTAINER_REMOTE_USER", "")
		t.Setenv("DEVCONTAINER_REMOTE_HOME", "")
		gotUser, gotHome := builtinIdentity()
		if gotUser != "dev" {
			t.Errorf("user = %q; want %q", gotUser, "dev")
		}
		if gotHome != "/home/dev" {
			t.Errorf("home = %q; want %q", gotHome, "/home/dev")
		}
	})

	t.Run("explicit DEVCONTAINER_REMOTE_USER wins over auto-detect", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("DEVCONTAINER_REMOTE_USER", "root")
		t.Setenv("DEVCONTAINER_REMOTE_HOME", "")
		// Seed a file owned by whatever UID the test runs as; the explicit
		// env var should pin "root" regardless.
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
			t.Fatal(err)
		}
		gotUser, gotHome := builtinIdentity()
		if gotUser != "root" {
			t.Errorf("user = %q; want %q", gotUser, "root")
		}
		if gotHome != "/root" {
			t.Errorf("home = %q; want %q (derived from user)", gotHome, "/root")
		}
	})

	t.Run("user-only env still derives a matching home", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("DEVCONTAINER_REMOTE_USER", "root")
		t.Setenv("DEVCONTAINER_REMOTE_HOME", "")
		_, gotHome := builtinIdentity()
		if gotHome != "/root" {
			t.Errorf("home = %q; want %q", gotHome, "/root")
		}
	})

	t.Run("home-only env still resolves a user", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("DEVCONTAINER_REMOTE_USER", "")
		t.Setenv("DEVCONTAINER_REMOTE_HOME", "/custom")
		// No claude on disk → falls back to dev.
		gotUser, gotHome := builtinIdentity()
		if gotUser != "dev" {
			t.Errorf("user = %q; want %q", gotUser, "dev")
		}
		if gotHome != "/custom" {
			t.Errorf("home = %q; want %q", gotHome, "/custom")
		}
	})
}

func TestBuildContainerConfig_HostDockerInternalAlias(t *testing.T) {
	cfg := &DevcontainerConfig{WorkspaceFolder: "/workspace"}
	opts := SpawnOptions{WorktreePath: "/host/worktree", ContainerName: "test"}

	cases := []struct {
		name      string
		gatewayIP string
		want      string
	}{
		{"falls back to host-gateway", "", "host.docker.internal:host-gateway"},
		{"uses explicit gateway IP", "172.19.0.1", "host.docker.internal:172.19.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hostCfg, _, _, err := buildContainerConfig(cfg, opts, resolveWorkspaceLayout(opts, cfg.WorkspaceFolder), "img", tc.gatewayIP)
			if err != nil {
				t.Fatalf("buildContainerConfig: %v", err)
			}
			if !reflect.DeepEqual(hostCfg.ExtraHosts, []string{tc.want}) {
				t.Errorf("ExtraHosts = %v; want [%q]", hostCfg.ExtraHosts, tc.want)
			}
		})
	}
}
