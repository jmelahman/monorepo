package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/jmelahman/local-preview/internal/dockerapi"
)

// buildMount is where the scratch tree appears inside a build container.
const buildMount = "/preview-build"

// cacheMount holds per-image toolchain caches (npm's $HOME/.npm, Go's
// module/build caches) on a named volume so repeat container builds are
// warm.
const cacheMount = "/preview-cache"

// autoRunner is the default Runner: steps whose manifest side declares an
// image run in a one-shot container (when a Docker daemon is reachable);
// steps without one default into the repo's devcontainer when the commit
// carries a usable one; everything else execs on the host. The daemon is
// dialed lazily, once.
type autoRunner struct {
	host HostRunner

	mu     sync.Mutex
	probed bool
	client *dockerapi.Client
	err    error
}

func (r *autoRunner) docker(ctx context.Context) (*dockerapi.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.probed {
		r.probed = true
		r.client, r.err = dockerapi.Connect(ctx)
	}
	return r.client, r.err
}

// Run implements Runner.
func (r *autoRunner) Run(ctx context.Context, spec RunSpec, out io.Writer) error {
	image, devc := spec.Image, false
	if image == "" && spec.Devcontainer.Image != "" {
		// An explicit manifest image beats the devcontainer default.
		image, devc = spec.Devcontainer.Image, true
	}
	if image == "" {
		return r.host.Run(ctx, spec, out)
	}
	cli, err := r.docker(ctx)
	if err != nil {
		if devc {
			fmt.Fprintf(out, "warning: devcontainer %q: docker is unavailable (%v); building on the host instead\n", image, err)
		} else {
			fmt.Fprintf(out, "warning: build image %q requested but docker is unavailable (%v); building on the host instead\n", image, err)
		}
		return r.host.Run(ctx, spec, out)
	}

	// Under a rootless daemon container root maps to the daemon's host user
	// (the identity that owns the scratch dir); under a rootful daemon the
	// host uid:gid does.
	user := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	if cli.Rootless() {
		user = "0:0"
	}
	env := []string{"HOME=/tmp"}
	binds := []string{spec.ScratchDir + ":" + buildMount}
	switch {
	case devc:
		// The devcontainer's own named volumes keep toolchain caches warm —
		// and shared with the developer's interactive sandbox. HOME points at
		// the remote user's home so go/npm resolve their caches onto the
		// mounted targets.
		fmt.Fprintf(out, "[build image: %s (devcontainer)]\n", image)
		if spec.Devcontainer.Home != "" {
			env = []string{"HOME=" + spec.Devcontainer.Home}
		}
		for _, m := range spec.Devcontainer.Mounts {
			binds = append(binds, m.Source+":"+m.Target)
		}
	case cli.Rootless():
		// The per-image toolchain cache volume is only mounted when running
		// as container root — a non-root user can't write a fresh root-owned
		// volume.
		fmt.Fprintf(out, "[build image: %s]\n", image)
		env = []string{
			"HOME=" + cacheMount,
			"GOMODCACHE=" + cacheMount + "/gomod",
			"GOCACHE=" + cacheMount + "/gocache",
		}
		binds = append(binds, cacheVolume(image)+":"+cacheMount)
	default:
		fmt.Fprintf(out, "[build image: %s]\n", image)
	}
	err = cli.RunStep(ctx, dockerapi.Step{
		Image:   image,
		Cmd:     spec.Argv,
		WorkDir: path.Join(buildMount, filepath.ToSlash(spec.Dir)),
		Env:     env,
		User:    user,
		Binds:   binds,
	}, out)
	if err != nil {
		return fmt.Errorf("build step %q in %s: %w", strings.Join(spec.Argv, " "), image, err)
	}
	return nil
}

var volumeUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// cacheVolume names the per-image cache volume.
func cacheVolume(image string) string {
	return "local-preview-cache-" + volumeUnsafe.ReplaceAllString(image, "-")
}
