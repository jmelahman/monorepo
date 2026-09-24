// Package git runs the git CLI. git-subtree has no library implementation,
// so everything goes through the binary, which also means credentials,
// url.<base>.insteadOf rewrites and SSH config all behave exactly as they do
// for a plain `git push`.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Repo is a git working tree.
type Repo struct {
	Dir string
}

// Open returns the repository containing dir.
func Open(dir string) (Repo, error) {
	top, err := Repo{Dir: dir}.Output("rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, err
	}
	return Repo{Dir: top}, nil
}

func (r Repo) command(args ...string) *exec.Cmd {
	log.Debugf("git %s", strings.Join(args, " "))
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	return cmd
}

// Output runs git and returns its stdout with surrounding whitespace trimmed.
// On failure, the error carries git's stderr.
func (r Repo) Output(args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := r.command(args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return strings.TrimSpace(string(out)), nil
}

// Run runs git with its output going to stderr, for commands whose progress
// the user should see.
func (r Repo) Run(args ...string) error {
	cmd := r.command(args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return &Error{Args: args, Err: err}
	}
	return nil
}

// Error is a failed git invocation.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCode is the exit status of the git invocation behind err, or -1 if err
// is not one.
func ExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
