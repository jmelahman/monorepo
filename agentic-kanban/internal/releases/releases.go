// Package releases keeps release reminders and releases subtrees of a
// git-orchard monorepo, through the same library as `git orchard release`.
package releases

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/orchard"
	"github.com/jmelahman/git-orchard/semver"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/github"
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
	// that would happen, or another is under way.
	ErrConflict = errors.New("release conflict")
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
	// busy holds the repos with a run under way.
	busy map[string]bool
	// live is closed when the run with that ID has finished writing its log.
	live map[int64]chan struct{}
	// projects is what the latest refresh of each repo found.
	projects map[string]Project
	// refreshing holds the repos with a refresh under way.
	refreshing map[string]bool
}

// New returns a Service keeping run logs under dataDir. Any run still marked
// running was cut off by the last shutdown, so it's marked interrupted.
func New(store *db.Store, dataDir string) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	if err := store.InterruptReleaseRuns(ctx); err != nil {
		log.Printf("releases: interrupt runs: %v", err)
	}
	return &Service{
		store:      store,
		logDir:     filepath.Join(dataDir, "release-runs"),
		ctx:        ctx,
		cancel:     cancel,
		busy:       map[string]bool{},
		live:       map[int64]chan struct{}{},
		projects:   map[string]Project{},
		refreshing: map[string]bool{},
	}
}

// Close cancels what's under way and waits for it, so runs are recorded as
// interrupted before the store closes.
func (s *Service) Close() {
	s.cancel()
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
	// Only a tag is pushed, for commits already on the base branch, and a
	// server has no terminal for a hook to talk to.
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
	return []string{
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=" + key,
		"GIT_CONFIG_VALUE_0=git@github.com:",
		"GIT_CONFIG_KEY_1=" + key,
		"GIT_CONFIG_VALUE_1=https://github.com/",
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
		return nil, config.Subtree{}, "", err
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
		return semver.Patch, nil
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
