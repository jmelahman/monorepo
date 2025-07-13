package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func isMounted(path string) bool {
	mounts, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	
	content := string(mounts)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}
	return false
}

func mountOverlay(containerRoot, mergedRoot string) error {
	if isMounted(mergedRoot) {
		fmt.Printf("[porter] Overlay already mounted at %s, skipping mount\n", mergedRoot)
		return nil
	}

	lowerdir := "/"
	upperdir := containerRoot
	workdir := "/tmp/porter-work"

	if err := os.MkdirAll(workdir, 0755); err != nil {
		return fmt.Errorf("creating workdir: %w", err)
	}
	if err := os.MkdirAll(mergedRoot, 0755); err != nil {
		return fmt.Errorf("creating merged dir: %w", err)
	}

	overlayOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerdir, upperdir, workdir)
	if err := syscall.Mount("overlay", mergedRoot, "overlay", 0, overlayOpts); err != nil {
		return fmt.Errorf("mounting overlay: %w", err)
	}
	return nil
}

// TODO: Parse config.json from OCI image to get Entrypoint and Cmd fields
// TODO: Validate and fallback to default shell if no entrypoint is specified
// TODO: Use unshare to enter a new mount and PID namespace for process isolation
// TODO: pivot_root or chroot into the overlay rootfs
// TODO: Drop privileges to non-root user if specified in image config
// TODO: Set hostname, env vars, working dir from OCI config
// TODO: Properly clean up mount points and temporary dirs on exit

func main() {
	imagePath := flag.String("image", "", "Path to unpacked OCI image rootfs")
	shell := flag.String("exec", "/bin/sh", "Command to run in the overlay")
	flag.Parse()

	if *imagePath == "" {
		log.Fatal("--image is required")
	}

	if _, err := os.Stat(*imagePath); os.IsNotExist(err) {
		log.Fatalf("Image path does not exist: %s", *imagePath)
	}

	mergedRoot := "/tmp/porter-merged"
	fmt.Println("[porter] Mounting overlay FS")
	if err := mountOverlay(*imagePath, mergedRoot); err != nil {
		log.Fatalf("Overlay mount failed: %v", err)
	}

	fmt.Println("[porter] Starting container")
	cmd := exec.Command(*shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Dir = mergedRoot
	if err := cmd.Run(); err != nil {
		log.Fatalf("Execution failed: %v", err)
	}
}
