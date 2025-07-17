package paths

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func RuntimeDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "runtainer")
	}
	uid := os.Getuid()
	return filepath.Join("/run/user", fmt.Sprint(uid))
}

func StateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "runtainer")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("cannot determine home dir")
	}
	return filepath.Join(home, ".local", "state", "runtainer")
}

func ImageDir() string {
	return filepath.Join(StateDir(), "images")
}

func RootfsDir() string {
	return filepath.Join(StateDir(), "roots")
}

func ContainerDir(id string) string {
	return filepath.Join(StateDir(), "containers", id)
}
