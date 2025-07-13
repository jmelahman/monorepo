package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type OCIConfig struct {
	Config struct {
		Entrypoint []string `json:"Entrypoint"`
		Cmd        []string `json:"Cmd"`
		WorkingDir string   `json:"WorkingDir"`
		Env        []string `json:"Env"`
		User       string   `json:"User"`
	} `json:"config"`
}

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

func parseOCIConfig(imagePath string) (*OCIConfig, error) {
	configPath := filepath.Join(imagePath, "config.json")
	
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config.json not found at %s", configPath)
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config.json: %w", err)
	}
	
	var config OCIConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config.json: %w", err)
	}
	
	return &config, nil
}

func getCommandToRun(config *OCIConfig, shellFlag string) []string {
	var cmd []string
	
	// Start with Entrypoint if it exists
	if len(config.Config.Entrypoint) > 0 {
		cmd = append(cmd, config.Config.Entrypoint...)
	}
	
	// Append Cmd if it exists
	if len(config.Config.Cmd) > 0 {
		cmd = append(cmd, config.Config.Cmd...)
	}
	
	// If neither Entrypoint nor Cmd are specified, use the shell flag
	if len(cmd) == 0 {
		cmd = []string{shellFlag}
	}
	
	return cmd
}

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

	// Parse OCI configuration
	fmt.Println("[porter] Parsing OCI configuration")
	ociConfig, err := parseOCIConfig(*imagePath)
	if err != nil {
		log.Printf("[porter] Warning: %v, using default shell", err)
		ociConfig = &OCIConfig{}
	}

	mergedRoot := "/tmp/porter-merged"
	fmt.Println("[porter] Mounting overlay FS")
	if err := mountOverlay(*imagePath, mergedRoot); err != nil {
		log.Fatalf("Overlay mount failed: %v", err)
	}

	// Get command to run based on OCI config
	cmdParts := getCommandToRun(ociConfig, *shell)
	if len(cmdParts) == 0 {
		log.Fatal("No command specified to run")
	}
	
	fmt.Printf("[porter] Starting container with command: %v\n", cmdParts)
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Dir = mergedRoot
	if ociConfig.Config.WorkingDir != "" {
		cmd.Dir = filepath.Join(mergedRoot, ociConfig.Config.WorkingDir)
	}
	
	// Set environment variables from OCI config
	if len(ociConfig.Config.Env) > 0 {
		cmd.Env = append(os.Environ(), ociConfig.Config.Env...)
	}

	if err := cmd.Run(); err != nil {
		log.Fatalf("Execution failed: %v", err)
	}
}
