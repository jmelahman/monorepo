//go:build !windows

package supervise

// Host processes are supervised as a process group: a dev server routinely
// forks (a watcher, a bundler, a child runtime), so signalling the leader
// alone would leave the tree running and the port held.

import (
	"os/exec"
	"syscall"
)

// newProcessGroup makes cmd the leader of its own group, so its pid doubles
// as the group id the kill helpers below take.
func newProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateGroup asks every process in the group to exit.
func terminateGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGTERM)
}

// killGroup force-kills every process in the group.
func killGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// processGroupOf reports the group pid belongs to, erroring if it is gone.
func processGroupOf(pid int) (int, error) {
	return syscall.Getpgid(pid)
}
