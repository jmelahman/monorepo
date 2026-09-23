package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDockerSocketMount_HostOverride(t *testing.T) {
	// Point both local probe paths at an empty dir so they can't accidentally
	// satisfy the Stat fallback.
	empty := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", empty)

	t.Run("override wins even when no local socket exists", func(t *testing.T) {
		t.Setenv(envHostDockerSock, "/var/run/user/1000/docker.sock")
		got := DockerSocketMount()
		want := "type=bind,source=/var/run/user/1000/docker.sock,target=/var/run/docker.sock"
		if got != want {
			t.Errorf("DockerSocketMount() = %q; want %q", got, want)
		}
	})

	t.Run("override wins over an existing local socket", func(t *testing.T) {
		runDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(runDir, "docker.sock"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("XDG_RUNTIME_DIR", runDir)
		t.Setenv(envHostDockerSock, "/host/path/docker.sock")
		got := DockerSocketMount()
		want := "type=bind,source=/host/path/docker.sock,target=/var/run/docker.sock"
		if got != want {
			t.Errorf("DockerSocketMount() = %q; want %q", got, want)
		}
	})

	t.Run("unset override falls back to local probe", func(t *testing.T) {
		// Run the test only when no local socket can satisfy the probe, so we
		// can assert the override-absent path returns "" (i.e. no mount).
		// The devcontainer has /var/run/docker.sock present, so skip there.
		if _, err := os.Stat("/var/run/docker.sock"); err == nil {
			t.Skip("/var/run/docker.sock exists; can't isolate probe fallback")
		}
		t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
		t.Setenv(envHostDockerSock, "")
		if got := DockerSocketMount(); got != "" {
			t.Errorf("DockerSocketMount() = %q; want empty when no socket exists", got)
		}
	})
}
