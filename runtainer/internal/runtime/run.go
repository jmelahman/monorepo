package runtime

import (
	"fmt"
	"github.com/jmelahman/runtainer/internal/paths"
	"os"
	"os/exec"
	"path/filepath"
)

func RunCommand(imageID string, argv []string) error {
	containerID := generateShortID()
	containerDir := paths.ContainerDir(containerID)
	upper := filepath.Join(containerDir, "upper")
	work := filepath.Join(containerDir, "work")
	merged := filepath.Join(containerDir, "merged")
	lower := paths.RootfsDir()

	// Prepare directories
	for _, dir := range []string{upper, work, merged} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Mount overlay using fuse-overlayfs or similar (if available)
	// OR use syscall.Mount if privileged.
	// Placeholder: this is not implemented yet.
	fmt.Printf("Would mount overlay: lower=%s, upper=%s, work=%s → merged=%s\n", lower, upper, work, merged)

	// Unshare + chroot + exec
	// cmd := exec.Command("unshare", "--mount", "--pid", "--fork", "--user", "--map-root-user", "chroot", merged, argv[0], argv[1:]...)
	cmd := exec.Command("unshare", argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func generateShortID() string {
	// Use first 6 chars of a random uuid or sha
	return fmt.Sprintf("c%x", os.Getpid()) // quick placeholder
}
