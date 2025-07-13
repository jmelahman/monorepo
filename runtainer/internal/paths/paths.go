package paths

import (
	"os"
	"path/filepath"
)

func StateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "runtainer")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		panic("cannot determine home dir")
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
