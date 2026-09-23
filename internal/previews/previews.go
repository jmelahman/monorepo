// Package previews bridges kanban and the embedded local-preview
// orchestrator: naming boards for the preview subdomain and executing
// preview build steps inside the target repo's devcontainer.
package previews

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
)

// BuildsEnv selects preview build execution: "devcontainer" (default when
// Docker is available) or "host".
const BuildsEnv = "KANBAN_PREVIEW_BUILDS"

// AutoDeployEnv disables deploy-on-idle when set to "0" or "false".
const AutoDeployEnv = "KANBAN_PREVIEW_AUTO_DEPLOY"

// ManifestDirEnv overrides the directory searched for out-of-repo preview
// manifests.
const ManifestDirEnv = "KANBAN_PREVIEW_MANIFESTS"

// RepoName derives the orchestrator repo name — a DNS label, since it
// becomes the subdomain segment — from the board slug.
func RepoName(b *db.Board) string {
	var sb strings.Builder
	for _, c := range strings.ToLower(b.Slug) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			sb.WriteRune(c)
		}
	}
	name := strings.Trim(sb.String(), "-")
	if name == "" {
		return fmt.Sprintf("board-%d", b.ID)
	}
	if len(name) > 63 {
		name = strings.Trim(name[:63], "-")
	}
	return name
}

// AutoDeployEnabled reports whether deploy-on-idle is on (the default).
func AutoDeployEnabled() bool {
	v := os.Getenv(AutoDeployEnv)
	return v != "0" && !strings.EqualFold(v, "false")
}

// ManifestDir returns the directory searched for out-of-repo preview
// manifests: `<dir>/<repo-name>.toml`, in the plain preview.toml schema,
// for repos that can't carry a manifest upstream (a vendored fork, or an
// upstream that won't take the file). The repo name is RepoName(board), so
// the file is named after the board slug.
//
// It deliberately mirrors local-preview's own convention —
// $PREVIEW_CONFIG_DIR, else the platform config dir plus "preview", then
// "manifests" — so one manifest serves both the `preview` CLI and kanban.
// $KANBAN_PREVIEW_MANIFESTS overrides it (a containerized kanban wants a
// mounted path, not the server user's home). Returns "" when no config dir
// is resolvable, which disables the lookup.
func ManifestDir() string {
	if dir := os.Getenv(ManifestDirEnv); dir != "" {
		return dir
	}
	root := os.Getenv("PREVIEW_CONFIG_DIR")
	if root == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		root = filepath.Join(base, "preview")
	}
	return filepath.Join(root, "manifests")
}

// Onboarded reports whether a board can be previewed: its checkout carries
// a manifest — a preview.toml, or a [previews] table in .kanban.toml — or
// the server holds an out-of-repo manifest for repoName. It's the cheap
// gate for auto-deploys (the substring probe is a heuristic; the real parse
// happens at build time, where a malformed manifest fails visibly).
func Onboarded(worktreeDir, repoName string) bool {
	if _, err := os.Stat(filepath.Join(worktreeDir, "preview.toml")); err == nil {
		return true
	}
	if b, err := os.ReadFile(filepath.Join(worktreeDir, ".kanban.toml")); err == nil &&
		strings.Contains(string(b), "[previews") {
		return true
	}
	if dir := ManifestDir(); dir != "" && repoName != "" {
		if _, err := os.Stat(filepath.Join(dir, repoName+".toml")); err == nil {
			return true
		}
	}
	return false
}

// DockerRunner executes preview build steps inside the target repo's
// devcontainer, resolved at the deployed commit — old commits build with the
// environment they shipped with.
//
// The orchestrator resolves the devcontainer itself and hands it over on the
// RunSpec (image, named cache volumes, remote home); this runner prefers that
// and only falls back to reading the extracted tree when the orchestrator
// declined — most importantly for Dockerfile-built devcontainers, which it
// doesn't support and kanban's content-addressed image cache does. Repos
// without a devcontainer at all build in the builtin session image.
type DockerRunner struct {
	docker *docker.Client
}

// NewDockerRunner returns a Runner for preview build steps.
func NewDockerRunner(d *docker.Client) *DockerRunner {
	return &DockerRunner{docker: d}
}

// buildUser picks the container user so files written into the bind-mounted
// scratch dir stay owned by the kanban host user: under a rootless daemon
// that's container root (which maps to the host user; the image's default
// non-root user would map to a subordinate uid and can't write the mount),
// under a rootful one it's the host uid:gid itself.
func (r *DockerRunner) buildUser(ctx context.Context) string {
	if r.docker.Rootless(ctx) {
		return "0:0"
	}
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}

// Run implements orchestrator.Runner.
func (r *DockerRunner) Run(ctx context.Context, spec orchestrator.RunSpec, out io.Writer) error {
	// HOME defaults to a writable scratch path: the build user is rarely the
	// image's own user, so its home may not exist or be writable.
	env := []string{"HOME=/tmp"}
	var mounts []docker.BuildMount

	var cfg *docker.DevcontainerConfig
	switch {
	case spec.Image != "":
		// A manifest-declared image is the repo's explicit contract and beats
		// devcontainer discovery.
		cfg = &docker.DevcontainerConfig{Image: spec.Image}
	case spec.Devcontainer.Image != "":
		cfg = &docker.DevcontainerConfig{Image: spec.Devcontainer.Image}
		// Mount the devcontainer's own named cache volumes and point HOME at
		// the remote user's home, so go/npm resolve their caches onto those
		// volumes and repeat builds start warm — the same volumes the
		// interactive session container uses.
		if spec.Devcontainer.Home != "" {
			env = []string{"HOME=" + spec.Devcontainer.Home}
		}
		for _, m := range spec.Devcontainer.Mounts {
			mounts = append(mounts, docker.BuildMount{Source: m.Source, Target: m.Target})
		}
	default:
		loaded, err := docker.LoadDevcontainer(spec.ScratchDir)
		if err != nil {
			return fmt.Errorf("load devcontainer config: %w", err)
		}
		cfg = loaded
	}

	image, err := r.docker.EnsureBuildImage(ctx, cfg, spec.ScratchDir, "preview-"+spec.RepoName)
	if err != nil {
		return fmt.Errorf("resolve build image: %w", err)
	}
	fmt.Fprintf(out, "[devcontainer image: %s]\n", image)
	if err := r.docker.RunBuildStep(ctx, docker.BuildStep{
		Image:   image,
		HostDir: spec.ScratchDir,
		Dir:     spec.Dir,
		Argv:    spec.Argv,
		Env:     env,
		User:    r.buildUser(ctx),
		Mounts:  mounts,
	}, out); err != nil {
		return fmt.Errorf("devcontainer build: %w", err)
	}
	return nil
}
