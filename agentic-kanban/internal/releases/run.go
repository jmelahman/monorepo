package releases

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/ghrun"
	"github.com/jmelahman/git-orchard/orchard"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/github"
)

const (
	previewTimeout = 5 * time.Minute
	// runTimeout bounds a release from when its turn comes, including watching
	// what it publishes.
	runTimeout = 30 * time.Minute
	// releaseJobs bounds the runs tagging in a repo at once: each may run git
	// subtree split, and reach two remotes. Watching what a tag publishes only
	// polls GitHub, so it doesn't count.
	releaseJobs = 4
	// followInterval is how often a followed log is checked for more output.
	followInterval = 200 * time.Millisecond
)

// Preview is the release that would happen now.
type Preview struct {
	RepoPath string `json:"repo_path"`
	Prefix   string `json:"prefix"`
	Version  string `json:"version"`
	Rev      string `json:"rev"`
	// Released means Rev already has Version, so a release finishes publishing it.
	Released bool `json:"released"`
	Ahead    int  `json:"ahead"`
	Behind   int  `json:"behind"`
	// Blocked says why it can't be released without RunRequest.Upstream, or is
	// empty if it can.
	Blocked string `json:"blocked,omitempty"`
}

// Preview works out the release of a subtree that a Run would do. It fetches,
// so it can take a while.
func (s *Service) Preview(ctx context.Context, repo, prefix, bump, suffix string) (Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, previewTimeout)
	defer cancel()
	p, err := s.preview(ctx, repo, prefix, bump, suffix)
	return p, runError(ctx, err)
}

// runError says when err is ctx ending: git reports that as being killed.
func runError(ctx context.Context, err error) error {
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func (s *Service) preview(ctx context.Context, repo, prefix, bump, suffix string) (Preview, error) {
	if err := validate(bump, suffix); err != nil {
		return Preview{}, err
	}
	inc, err := increment(bump)
	if err != nil {
		return Preview{}, err
	}
	o, sub, base, err := s.lookup(ctx, repo, prefix, io.Discard)
	if err != nil {
		return Preview{}, err
	}
	p := Preview{RepoPath: repo, Prefix: sub.Prefix}
	if p.Rev, err = tip(o, base); err != nil {
		return p, redactErr(err)
	}
	p.Version, p.Released, err = o.NextVersion(sub, orchard.NextOptions{Rev: p.Rev, Remote: remote, Increment: inc, Suffix: suffix})
	if err != nil {
		return p, redactErr(err)
	}
	status, err := s.status(o, repo, sub, p.Rev)
	if err != nil {
		return p, redactErr(err)
	}
	p.Ahead, p.Behind = status.Ahead, status.Behind
	// See REGRESSIONS.md: "A release waits for the mirror". Orchard.Release
	// refuses this, so say so before anyone confirms.
	if status.Ahead > 0 {
		p.Blocked = fmt.Sprintf("%s's upstream lacks %d commit(s) from %s; wait for the mirror to publish them, or push to the upstream directly", sub.Prefix, status.Ahead, base)
	}
	return p, nil
}

// gitError is a failure of git, whose text has no token. It doesn't wrap the
// original, which might.
type gitError struct{ text string }

func (e gitError) Error() string { return e.text }

func (e gitError) Is(target error) bool { return target == ErrGit }

// redactErr marks a git error as one, keeping its meaning without its token.
func redactErr(err error) error {
	return gitError{text: redact(err.Error())}
}

// RunRequest pins the release the user confirmed.
type RunRequest struct {
	RepoPath   string
	Prefix     string
	Bump       string
	Suffix     string
	Version    string
	Rev        string
	ReminderID *int64
	// Upstream pushes the subtree's branch and tag to its upstream directly,
	// instead of leaving them to the mirror.
	Upstream bool
}

// Run queues the release req confirmed, if it's still the one that would happen
// and the subtree has no other queued or under way: that one's tag would make
// this version out of date. It returns once the run is queued; Follow streams
// its output. The confirmed rev is what gets released, however long the wait.
func (s *Service) Run(ctx context.Context, req RunRequest) (*db.ReleaseRun, error) {
	key := subtreeKey{repo: req.RepoPath, prefix: config.Clean(req.Prefix)}
	if err := s.acquire(key); err != nil {
		return nil, err
	}
	run, sub, err := s.prepare(ctx, req)
	if err != nil {
		s.release(key)
		return nil, err
	}
	done := make(chan struct{})
	s.mu.Lock()
	// Close may have begun while this prepared, and a WaitGroup can't grow once
	// it's being waited on.
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		s.finish(run, err)
		s.release(key)
		return nil, fmt.Errorf("%w: the server is stopping", ErrConflict)
	}
	s.live[run.ID] = done
	s.wg.Go(func() {
		defer s.release(key)
		defer func() {
			s.mu.Lock()
			delete(s.live, run.ID)
			s.mu.Unlock()
			close(done)
		}()
		s.execute(run, sub, req.Upstream)
	})
	s.mu.Unlock()
	return run, nil
}

// subtreeKey names a subtree across repos.
type subtreeKey struct{ repo, prefix string }

func (s *Service) acquire(key subtreeKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return fmt.Errorf("%w: the server is stopping", ErrConflict)
	}
	if s.busy[key] {
		return fmt.Errorf("%w: %s already has a release queued or running in %s", ErrConflict, key.prefix, key.repo)
	}
	s.busy[key] = true
	return nil
}

func (s *Service) release(key subtreeKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.busy, key)
}

// repoQueue is what a repo's runs take turns on. Each is a semaphore, and
// channels wake their waiters in order, so runs start in the order they queued.
type repoQueue struct {
	jobs chan struct{}
	// checkout is held by a run whose subtree has a pre-release hook.
	// See REGRESSIONS.md: "Pre-release hooks share the checkout".
	checkout chan struct{}
}

func (s *Service) queue(repo string) *repoQueue {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queues[repo]
	if q == nil {
		q = &repoQueue{jobs: make(chan struct{}, releaseJobs), checkout: make(chan struct{}, 1)}
		s.queues[repo] = q
	}
	return q
}

// admit waits for run's turn and marks it running. The returned func gives up
// the turn, and may be called more than once. A hook's turn comes first, so
// waiting for it doesn't keep a job from anyone else.
func (s *Service) admit(run *db.ReleaseRun, sub config.Subtree, output io.Writer) (func(), error) {
	q := s.queue(run.RepoPath)
	sems := []chan struct{}{q.jobs}
	if sub.PreRelease != "" {
		sems = []chan struct{}{q.checkout, q.jobs}
	}
	var held []chan struct{}
	free := sync.OnceFunc(func() {
		for _, sem := range held {
			<-sem
		}
	})
	for _, sem := range sems {
		select {
		case sem <- struct{}{}:
		default:
			_, _ = fmt.Fprintln(output, "Queued behind other releases in this repo...")
			select {
			case sem <- struct{}{}:
			case <-s.ctx.Done():
				free()
				return nil, s.ctx.Err()
			}
		}
		held = append(held, sem)
	}
	if err := s.store.StartReleaseRun(s.ctx, run.ID); err != nil {
		free()
		return nil, err
	}
	return free, nil
}

// prepare checks req against what would happen now and records the run.
func (s *Service) prepare(ctx context.Context, req RunRequest) (*db.ReleaseRun, config.Subtree, error) {
	if req.ReminderID != nil {
		r, err := s.store.GetReleaseReminder(ctx, *req.ReminderID)
		if err != nil {
			return nil, config.Subtree{}, fmt.Errorf("reminder %d: %w", *req.ReminderID, err)
		}
		if r.RepoPath != req.RepoPath || r.Prefix != config.Clean(req.Prefix) || r.Status != db.ReleaseReminderOpen {
			return nil, config.Subtree{}, fmt.Errorf("%w: reminder %d isn't open for %s", ErrInvalid, r.ID, req.Prefix)
		}
	}
	p, err := s.Preview(ctx, req.RepoPath, req.Prefix, req.Bump, req.Suffix)
	if err != nil {
		return nil, config.Subtree{}, err
	}
	if p.Blocked != "" && !req.Upstream {
		return nil, config.Subtree{}, fmt.Errorf("%w: %s", ErrConflict, p.Blocked)
	}
	if p.Version != req.Version || p.Rev != req.Rev {
		return nil, config.Subtree{}, fmt.Errorf("%w: the release is now %s at %.12s, not %s at %.12s", ErrConflict, p.Version, p.Rev, req.Version, req.Rev)
	}
	o, err := open(ctx, req.RepoPath, io.Discard)
	if err != nil {
		return nil, config.Subtree{}, err
	}
	sub, ok := o.Config.Lookup(req.Prefix)
	if !ok {
		return nil, config.Subtree{}, fmt.Errorf("%w: %s has no subtree %s", ErrUnknown, req.RepoPath, req.Prefix)
	}
	if err := os.MkdirAll(s.logDir, 0o755); err != nil {
		return nil, config.Subtree{}, err
	}
	run := &db.ReleaseRun{
		ReminderID: req.ReminderID, RepoPath: req.RepoPath, Prefix: sub.Prefix,
		Rev: req.Rev, Version: req.Version, Bump: req.Bump, Suffix: req.Suffix,
	}
	if run.Bump == "" {
		run.Bump = "patch"
	}
	if err := s.store.CreateReleaseRun(ctx, run); err != nil {
		return nil, config.Subtree{}, err
	}
	return run, sub, nil
}

func (s *Service) logPath(id int64) string {
	return filepath.Join(s.logDir, strconv.FormatInt(id, 10)+".log")
}

// redactWriter keeps the token out of a run's log. git writes whole lines, so
// a token isn't split across writes.
type redactWriter struct{ w io.Writer }

func (r redactWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(r.w, redact(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// execute waits for run's turn, releases and records how it went. The server's
// context is what bounds it: the request that started it is long gone.
func (s *Service) execute(run *db.ReleaseRun, sub config.Subtree, upstream bool) {
	file, err := os.OpenFile(s.logPath(run.ID), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		s.finish(run, err)
		return
	}
	defer file.Close()
	output := redactWriter{w: file}
	free, err := s.admit(run, sub, output)
	if err != nil {
		_, _ = fmt.Fprintf(output, "%v\n", err)
		s.finish(run, err)
		return
	}
	defer free()

	ctx, cancel := context.WithTimeout(s.ctx, runTimeout)
	defer cancel()
	boards, _, err := s.repos(ctx)
	if err == nil {
		err = release(ctx, run, sub, upstream, identityEnv(boards[run.RepoPath]), output, free)
	}
	err = runError(ctx, err)
	if err != nil {
		_, _ = fmt.Fprintf(output, "%v\n", err)
	}
	s.finish(run, err)
}

// release tags run and watches what the tag publishes, calling tagged between
// the two.
func release(ctx context.Context, run *db.ReleaseRun, sub config.Subtree, toUpstream bool, identity []string, output io.Writer, tagged func()) error {
	o, err := open(ctx, run.RepoPath, output)
	if err != nil {
		return err
	}
	o.Repo.Env = append(o.Repo.Env, identity...)
	_, _ = fmt.Fprintf(output, "Releasing %s %s at %.12s...\n", sub.Prefix, run.Version, run.Rev)
	opts := orchard.ReleaseOptions{Rev: run.Rev, Remote: remote}
	if toUpstream {
		// See REGRESSIONS.md: "A release waits for the mirror". With a Remote,
		// Release would push the rev to whichever branch the checkout is on.
		opts = orchard.ReleaseOptions{Rev: run.Rev, Upstream: true}
	}
	tag, err := o.Release(sub, run.Version, opts)
	if err == nil && toUpstream {
		err = o.Repo.Run("push", "--no-verify", remote, "refs/tags/"+tag)
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(output, "Tagged %s.\n", tag)
	tagged()

	// The tag is out; only the watch is skipped.
	if github.Token("github.com") == "" {
		_, _ = fmt.Fprintln(output, "The server has no GitHub token; not watching what the tag publishes.")
		return nil
	}
	_, _, mirrored := ghrun.ParseGitHubRepo(o.RemoteURL(remote))
	_, _, upstream := ghrun.ParseGitHubRepo(sub.Remote)
	if !mirrored && !upstream {
		_, _ = fmt.Fprintln(output, "Neither the monorepo nor the upstream is on GitHub; nothing to watch.")
		return nil
	}
	return o.WatchRelease(ctx, sub, run.Version, tag, orchard.WatchOptions{Remote: remote, Upstream: toUpstream, Output: output})
}

// finish records the outcome. The context may be why it ended, so writes don't
// use it.
func (s *Service) finish(run *db.ReleaseRun, err error) {
	ctx := context.Background()
	status, text := db.ReleaseRunReleased, ""
	switch {
	case err == nil:
	case s.ctx.Err() != nil:
		status, text = db.ReleaseRunInterrupted, "the server stopped"
	case errors.Is(err, context.DeadlineExceeded):
		status, text = db.ReleaseRunFailed, fmt.Sprintf("stopped after %s; %s may still be publishing", runTimeout, run.Version)
	default:
		status, text = db.ReleaseRunFailed, redact(err.Error())
	}
	if err := s.store.FinishReleaseRun(ctx, run.ID, status, text); err != nil {
		log.Printf("releases: finish run %d: %v", run.ID, err)
	}
	if status != db.ReleaseRunReleased || run.ReminderID == nil {
		return
	}
	if _, err := s.store.CloseReleaseReminder(ctx, *run.ReminderID, db.ReleaseReminderReleased); err != nil {
		log.Printf("releases: close reminder %d: %v", *run.ReminderID, err)
	}
}

// Follow calls emit with each line of a run's log, from the start, and returns
// when the run has finished and the log is exhausted or ctx ends. Reading the
// file means a late or reconnecting reader misses nothing.
func (s *Service) Follow(ctx context.Context, id int64, emit func(line string)) error {
	s.mu.Lock()
	done := s.live[id]
	s.mu.Unlock()

	file, err := os.Open(s.logPath(id))
	for errors.Is(err, os.ErrNotExist) && done != nil {
		// The run has yet to create its log.
		if !wait(ctx, done) {
			return nil
		}
		s.mu.Lock()
		done = s.live[id]
		s.mu.Unlock()
		file, err = os.Open(s.logPath(id))
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var partial strings.Builder
	for {
		chunk, err := reader.ReadString('\n')
		partial.WriteString(chunk)
		if err == nil {
			emit(strings.TrimRight(partial.String(), "\r\n"))
			partial.Reset()
			continue
		}
		if !errors.Is(err, io.EOF) {
			return err
		}
		if done == nil {
			if partial.Len() > 0 {
				emit(partial.String())
			}
			return nil
		}
		if !wait(ctx, done) {
			return nil
		}
		select {
		case <-done:
			// Read once more: the log is complete now.
			done = nil
		default:
		}
	}
}

// wait pauses for followInterval or until done, reporting false if ctx ended.
func wait(ctx context.Context, done <-chan struct{}) bool {
	timer := time.NewTimer(followInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-done:
	case <-timer.C:
	}
	return true
}
