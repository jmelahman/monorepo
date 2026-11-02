// Package releases keeps release reminders and releases subtrees of a
// git-orchard monorepo, through the same library as `git orchard release`.
package releases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/ghrun"
	"github.com/jmelahman/git-orchard/orchard"
	"github.com/jmelahman/git-orchard/semver"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/github"
	"github.com/jmelahman/kanban/internal/metrics"
)

// remote is the monorepo remote releases are tagged on and read from.
const remote = "origin"

// Errors a caller distinguishes; the API maps them to status codes.
var (
	// ErrUnknown means a repo isn't a board's, or a prefix isn't in its manifest.
	ErrUnknown = errors.New("unknown repo or subtree")
	// ErrInvalid means a bump or suffix can't be released.
	ErrInvalid = errors.New("invalid release")
	// ErrConflict means the release shown to the user is no longer the one
	// that would happen, or the subtree already has one queued or under way.
	ErrConflict = errors.New("release conflict")
	// ErrGit is git or the network failing, rather than the server or the request.
	ErrGit = errors.New("git failed")
)

var suffixRE = regexp.MustCompile(`^[A-Za-z][0-9A-Za-z]*$`)

// Service is the release state of every board's repo.
type Service struct {
	store  *db.Store
	logDir string
	// ctx ends with the server, and runs with it.
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.Mutex
	// busy holds the subtrees with a run queued or under way.
	busy map[subtreeKey]bool
	// queues holds what each repo's runs take turns on.
	queues map[string]*repoQueue
	// live is closed when the run with that ID has finished writing its log.
	live map[int64]chan struct{}
	// projects is what the latest refresh of each repo found.
	projects map[string]Project
	// refreshing holds the repos with a refresh under way.
	refreshing map[string]bool
	// ci is each repo's CI health by prefix, and ciStarted when its latest
	// check began.
	ci        map[string]map[string]CIHealth
	ciStarted map[string]time.Time
}

// New returns a Service keeping run logs under dataDir. Any run still marked
// running was cut off by the last shutdown, so it's marked interrupted.
func New(store *db.Store, dataDir string) *Service {
	ghrun.SetTransport(metrics.WrapGitHubTransport(nil))
	ctx, cancel := context.WithCancel(context.Background())
	if err := store.InterruptReleaseRuns(ctx); err != nil {
		log.Printf("releases: interrupt runs: %v", err)
	}
	s := &Service{
		store:      store,
		logDir:     filepath.Join(dataDir, "release-runs"),
		ctx:        ctx,
		cancel:     cancel,
		busy:       map[subtreeKey]bool{},
		queues:     map[string]*repoQueue{},
		live:       map[int64]chan struct{}{},
		projects:   map[string]Project{},
		refreshing: map[string]bool{},
		ci:         map[string]map[string]CIHealth{},
		ciStarted:  map[string]time.Time{},
	}
	s.loadProjects()
	return s
}

// loadProjects restores each repo's last state, so the tab has something to
// show while the first refresh after a restart runs.
func (s *Service) loadProjects() {
	saved, err := s.store.ListReleaseProjects(s.ctx)
	if err != nil {
		log.Printf("releases: load saved state: %v", err)
		return
	}
	for repo, state := range saved {
		var p Project
		if err := json.Unmarshal(state, &p); err != nil || p.RepoPath != repo {
			continue
		}
		p.Refreshing = false
		s.projects[repo] = p
	}
}

// Close cancels what's under way and waits for it, so runs are recorded as
// interrupted before the store closes.
func (s *Service) Close() {
	s.mu.Lock()
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}

// Unavailable says why nothing here can work, or is empty if it can.
func Unavailable() string {
	if _, err := exec.LookPath("git"); err != nil {
		return "git isn't installed on the server"
	}
	return ""
}

// open returns a fresh Orchard for repo: one remembers which upstreams it has
// fetched for as long as it lives, and holds the manifest as it was read.
func open(ctx context.Context, repo string, output io.Writer) (*orchard.Orchard, error) {
	o, err := orchard.Open(repo)
	if err != nil {
		return nil, err
	}
	o.Repo.Ctx = ctx //nolint:fatcontext // this Orchard lasts one operation, and ctx bounds it
	o.Repo.Log = output
	o.Repo.Env = gitEnv()
	// Nothing but a tag goes to origin, for commits already on the base branch,
	// and a server has no terminal for a hook to talk to.
	o.NoVerify = true
	return o, nil
}

// gitEnv has git reach GitHub with the server's token, where there is one,
// since the server may have no ssh key or known_hosts. It's set through the
// environment so the token is never written to disk.
func gitEnv() []string {
	token := github.Token("github.com")
	if token == "" {
		return nil
	}
	key := "url.https://x-access-token:" + token + "@github.com/.insteadOf"
	// These go after any already in the environment, such as lfs.Install's.
	n, _ := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	return []string{
		"GIT_CONFIG_COUNT=" + strconv.Itoa(n+2),
		"GIT_CONFIG_KEY_" + strconv.Itoa(n) + "=" + key,
		"GIT_CONFIG_VALUE_" + strconv.Itoa(n) + "=git@github.com:",
		"GIT_CONFIG_KEY_" + strconv.Itoa(n+1) + "=" + key,
		"GIT_CONFIG_VALUE_" + strconv.Itoa(n+1) + "=https://github.com/",
	}
}

// redact removes the server's token from text bound for a log or the database.
func redact(text string) string {
	if token := github.Token("github.com"); token != "" {
		return strings.ReplaceAll(text, token, "***")
	}
	return text
}

// repos returns each distinct board repo's first board, and the repos in board
// order.
func (s *Service) repos(ctx context.Context) (map[string]db.Board, []string, error) {
	boards, err := s.store.ListBoards(ctx)
	if err != nil {
		return nil, nil, err
	}
	first := map[string]db.Board{}
	var order []string
	for _, b := range boards {
		if b.RepoPath == "" {
			continue
		}
		if _, ok := first[b.RepoPath]; !ok {
			first[b.RepoPath] = b
			order = append(order, b.RepoPath)
		}
	}
	return first, order, nil
}

// identityEnv names the tagger after the board's git author, where it has one:
// a server may have no git identity, and an annotated tag needs one.
func identityEnv(b db.Board) []string {
	var env []string
	if b.GitAuthorName != "" {
		env = append(env, "GIT_COMMITTER_NAME="+b.GitAuthorName)
	}
	if b.GitAuthorEmail != "" {
		env = append(env, "GIT_COMMITTER_EMAIL="+b.GitAuthorEmail)
	}
	return env
}

// lookup finds repo's base branch and the subtree at prefix in a fresh Orchard.
func (s *Service) lookup(ctx context.Context, repo, prefix string, output io.Writer) (*orchard.Orchard, config.Subtree, string, error) {
	boards, _, err := s.repos(ctx)
	if err != nil {
		return nil, config.Subtree{}, "", err
	}
	board, ok := boards[repo]
	if !ok {
		return nil, config.Subtree{}, "", fmt.Errorf("%w: no board has the repo %s", ErrUnknown, repo)
	}
	o, err := open(ctx, repo, output)
	if err != nil {
		return nil, config.Subtree{}, "", redactErr(err)
	}
	sub, ok := o.Config.Lookup(prefix)
	if !ok {
		return nil, config.Subtree{}, "", fmt.Errorf("%w: %s has no subtree %s", ErrUnknown, repo, prefix)
	}
	return o, sub, board.BaseBranch, nil
}

// tip fetches base and returns the commit at the tip of origin's copy, which
// is what gets released: the user's checkout may be anywhere.
func tip(o *orchard.Orchard, base string) (string, error) {
	if _, err := o.Repo.Output("fetch", "--quiet", remote, base); err != nil {
		return "", fmt.Errorf("fetching %s: %w", base, err)
	}
	return o.Repo.Output("rev-parse", "--verify", "refs/remotes/"+remote+"/"+base+"^{commit}")
}

func increment(bump string) (semver.Increment, error) {
	switch bump {
	case "patch", "":
		// As git orchard release does by default: with a suffix, this continues
		// that pre-release, and semver.Patch would skip to the next version.
		return semver.Auto, nil
	case "minor":
		return semver.Minor, nil
	case "major":
		return semver.Major, nil
	}
	return 0, fmt.Errorf("%w: unknown bump %q", ErrInvalid, bump)
}

func validate(bump, suffix string) error {
	if _, err := increment(bump); err != nil {
		return err
	}
	if suffix != "" && !suffixRE.MatchString(suffix) {
		return fmt.Errorf("%w: suffix %q must be letters and digits, starting with a letter", ErrInvalid, suffix)
	}
	return nil
}

// AddReminder records that a subtree should be released. A subtree with an
// open reminder can't have another: edit that one.
func (s *Service) AddReminder(ctx context.Context, r *db.ReleaseReminder) error {
	if r.Bump == "" {
		r.Bump = "patch"
	}
	if err := validate(r.Bump, r.Suffix); err != nil {
		return err
	}
	_, sub, _, err := s.lookup(ctx, r.RepoPath, r.Prefix, io.Discard)
	if err != nil {
		return err
	}
	r.Prefix = sub.Prefix
	if err := s.store.CreateReleaseReminder(ctx, r); err != nil {
		if db.IsUniqueViolation(err) {
			return fmt.Errorf("%w: %s already has an open reminder", ErrConflict, sub.Prefix)
		}
		return err
	}
	return nil
}

// UpdateReminder changes an open reminder.
func (s *Service) UpdateReminder(ctx context.Context, r *db.ReleaseReminder) error {
	if r.Bump == "" {
		r.Bump = "patch"
	}
	if err := validate(r.Bump, r.Suffix); err != nil {
		return err
	}
	return s.store.UpdateReleaseReminder(ctx, r)
}

// DismissReminder closes an open reminder without releasing.
func (s *Service) DismissReminder(ctx context.Context, id int64) error {
	ok, err := s.store.CloseReleaseReminder(ctx, id, db.ReleaseReminderDismissed)
	if err != nil {
		return err
	}
	if !ok {
		return db.ErrNotFound
	}
	return nil
}

// status is o.Status(sub, rev), reusing the saved split of a subtree that
// hasn't changed since it was last split, even across restarts. Only the
// upstream is fetched then.
func (s *Service) status(o *orchard.Orchard, repo string, sub config.Subtree, rev string) (orchard.Status, error) {
	change, err := o.LastChange(sub, rev)
	if err != nil {
		return orchard.Status{}, err
	}
	split, ok, err := s.store.GetReleaseSplit(s.ctx, repo, sub.Prefix, change)
	if err != nil {
		log.Printf("releases: read split of %s: %v", sub.Prefix, err)
	}
	if ok {
		// No ref holds a split, so git gc prunes it in time.
		_, err := o.Repo.Output("cat-file", "-e", split+"^{commit}")
		ok = err == nil
	}
	if !ok {
		if split, err = o.Split(sub, rev); err != nil {
			return orchard.Status{}, err
		}
		if err := s.store.SetReleaseSplit(s.ctx, repo, sub.Prefix, change, split); err != nil {
			log.Printf("releases: save split of %s: %v", sub.Prefix, err)
		}
	}
	return o.StatusOf(sub, split)
}
