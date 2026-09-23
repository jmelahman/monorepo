//go:build windows

package supervise

// Windows has no POSIX process groups. A child is started in its own
// console process group so console signals don't leak into it, but teardown
// can only reach the pid we started — grandchildren it forked are left to
// the OS. Graceful termination has no equivalent either, so both kill
// helpers terminate outright.

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func newProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func terminateGroup(pid int) error { return killGroup(pid) }

func killGroup(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// processGroupOf is unsupported here, so orphan reclamation skips host
// processes: the records are still cleared, but a leftover from an unclean
// exit is not killed.
func processGroupOf(int) (int, error) { return 0, errors.ErrUnsupported }
