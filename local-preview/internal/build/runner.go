package build

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmelahman/local-preview/internal/devcontainer"
)

// RunSpec describes one build step. ScratchDir is the extracted commit tree
// on the host; Dir is the working directory relative to it. A containerized
// runner mounts ScratchDir and maps Dir inside the mount, so builds that
// reference sibling files (lockfiles, workspace configs) keep working.
// Image is the manifest-declared build image for this step's side ("" =
// unspecified; the default runner then uses Devcontainer, or the host).
type RunSpec struct {
	RepoName   string
	SHA        string
	ScratchDir string
	Dir        string
	Argv       []string
	Image      string
	// Devcontainer is the repo's devcontainer resolved at the deployed
	// commit (zero when absent or opted out) — the default environment for
	// steps whose side declares no explicit Image.
	Devcontainer devcontainer.Config
}

// Runner executes build steps. The default (autoRunner) honors the
// manifest image, then the repo devcontainer, then the host; embedders can
// inject an implementation that takes over execution entirely.
type Runner interface {
	Run(ctx context.Context, spec RunSpec, output io.Writer) error
}

// HostRunner runs build steps as plain host processes.
type HostRunner struct{}

// Run implements Runner.
func (HostRunner) Run(ctx context.Context, spec RunSpec, output io.Writer) error {
	cmd := exec.CommandContext(ctx, spec.Argv[0], spec.Argv[1:]...)
	cmd.Dir = filepath.Join(spec.ScratchDir, spec.Dir)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build step %q: %w", strings.Join(spec.Argv, " "), err)
	}
	return nil
}
