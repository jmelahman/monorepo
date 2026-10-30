// Package git runs the git CLI. git-subtree has no library implementation,
// so everything goes through the binary, which also means credentials,
// url.<base>.insteadOf rewrites and SSH config all behave exactly as they do
// for a plain `git push`.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// waitDelay is how long a cancelled git gets to exit before its output pipes
// are closed, since a child such as ssh can outlive it and hold them open.
const waitDelay = 5 * time.Second

// Repo is a git working tree.
type Repo struct {
	Dir string
	// Env is added to git's environment, e.g. GIT_CONFIG_* to supply
	// credentials without writing them to disk.
	Env []string
	// Log is where Run sends git's output; it defaults to os.Stderr.
	Log io.Writer
	// Ctx bounds every git invocation; it defaults to no bound. A value in a
	// struct, unusually, since an embedder sets it once per operation and
	// nearly every method of Orchard runs git.
	Ctx context.Context
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
	ctx := r.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Dir
	cmd.WaitDelay = waitDelay
	if r.Env != nil {
		cmd.Env = append(os.Environ(), r.Env...)
	}
	return cmd
}

// Output runs git and returns its stdout with surrounding whitespace trimmed.
// On failure, the error carries git's stderr.
func (r Repo) Output(args ...string) (string, error) {
	return r.OutputEnv(nil, args...)
}

// OutputEnv is Output with env added to git's environment, e.g. a
// GIT_INDEX_FILE to build a tree in.
func (r Repo) OutputEnv(env []string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := r.command(args...)
	if env != nil {
		if cmd.Env == nil {
			cmd.Env = os.Environ()
		}
		cmd.Env = append(cmd.Env, env...)
	}
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return strings.TrimSpace(string(out)), nil
}

// Run runs git with its output going to r.Log, or else stderr, for commands
// whose progress the user should see.
func (r Repo) Run(args ...string) error {
	if r.Log != nil {
		return r.RunTo(r.Log, args...)
	}
	return r.RunTo(os.Stderr, args...)
}

// RunTo is Run with the output going to w.
func (r Repo) RunTo(w io.Writer, args ...string) error {
	cmd := r.command(args...)
	cmd.Stdout = w
	cmd.Stderr = w
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
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	return -1
}
