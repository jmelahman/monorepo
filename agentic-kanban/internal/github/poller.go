// Package github polls the GitHub REST API per board and moves tickets
// between columns according to the PR's state, mirroring how the session
// manager reflects Claude session state on a ticket.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/eventbus"
	"github.com/jmelahman/kanban/internal/kanbantoml"
	"github.com/jmelahman/kanban/internal/metrics"
)

const (
	PRStateDraft  = "draft"
	PRStateOpen   = "open"
	PRStateMerged = "merged"
	PRStateClosed = "closed"

	defaultAPIBase = "https://api.github.com"
	prPageSize     = 100
	prMaxPages     = 2
)

// Config captures the [github] section of the merged kanban config (user
// file layered over the project's .kanban.toml).
type Config struct {
	AutoMove     bool
	DraftColumn  string
	ReviewColumn string
	DoneColumn   string
	ClosedColumn string
}

func defaultConfig() Config {
	return Config{
		AutoMove:     true,
		DraftColumn:  "In Progress",
		ReviewColumn: "Review",
		DoneColumn:   "Done",
		ClosedColumn: "",
	}
}

// LoadConfig returns the [github] section from the merged kanban config
// (user file layered over <repoPath>/.kanban.toml). Missing keys keep their
// defaults; auto_move is on by default.
func LoadConfig(repoPath string) Config {
	cfg := defaultConfig()
	g := kanbantoml.Load(repoPath).GitHub
	if g == nil {
		return cfg
	}
	if g.AutoMove != nil {
		cfg.AutoMove = *g.AutoMove
	}
	if g.DraftColumn != nil {
		cfg.DraftColumn = *g.DraftColumn
	}
	if g.ReviewColumn != nil {
		cfg.ReviewColumn = *g.ReviewColumn
	}
	if g.DoneColumn != nil {
		cfg.DoneColumn = *g.DoneColumn
	}
	if g.ClosedColumn != nil {
		cfg.ClosedColumn = *g.ClosedColumn
	}
	return cfg
}

// SessionStopper is the subset of *session.Manager the poller needs to halt
// a session's container after a PR is merged externally. The worktree,
// branch, and session row are preserved so the user can still inspect or
// restart the work.
type SessionStopper interface {
	Stop(ctx context.Context, sessionID int64) error
}

type Poller struct {
	store    *db.Store
	bus      eventbus.Publisher
	sessions SessionStopper
	interval time.Duration
	http     *http.Client
	// prCache memoises each `/pulls` page response keyed by request URL.
	// GitHub returns 304 Not Modified when If-None-Match matches, and 304s
	// don't count against the core rate limit — so a warm cache turns a
	// no-change poll into a free request.
	prCache sync.Map // url string -> prCacheEntry
}

type prCacheEntry struct {
	etag string
	data []ghPR
	next string // rel="next" Link from the cached response, or ""
}

// NewPoller constructs a poller. interval is the global tick rate.
func NewPoller(store *db.Store, bus eventbus.Publisher, sessions SessionStopper, interval time.Duration) *Poller {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Poller{
		store:    store,
		bus:      bus,
		sessions: sessions,
		interval: interval,
		http: &http.Client{
			Timeout:   20 * time.Second,
			Transport: metrics.WrapGitHubTransport(nil),
		},
	}
}

// Start blocks until ctx is done, ticking once per interval.
func (p *Poller) Start(ctx context.Context) {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	p.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	boards, err := p.store.ListBoards(ctx)
	if err != nil {
		log.Printf("github poller: list boards: %v", err)
		return
	}
	for i := range boards {
		b := &boards[i]
		if b.RepoPath == "" {
			continue
		}
		cfg := LoadConfig(b.RepoPath)
		if !cfg.AutoMove {
			continue
		}
		if err := p.syncBoard(ctx, b, cfg); err != nil {
			log.Printf("github poller: board %d: %v", b.ID, err)
		}
	}
}

type ghPR struct {
	Number   int64   `json:"number"`
	HTMLURL  string  `json:"html_url"`
	Title    string  `json:"title"`
	State    string  `json:"state"` // "open" or "closed"
	Draft    bool    `json:"draft"`
	MergedAt *string `json:"merged_at"` // null until merged
	Head     struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

func (p *Poller) syncBoard(ctx context.Context, board *db.Board, cfg Config) error {
	sessions, err := p.store.ListSessionsByBoard(ctx, board.ID)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	branches := make(map[string]struct{}, len(sessions))
	for i := range sessions {
		if b := sessions[i].BranchName; b != "" {
			branches[b] = struct{}{}
		}
	}
	if len(branches) == 0 {
		return nil
	}
	prs, err := p.listPRs(ctx, board.RepoPath)
	if err != nil {
		return err
	}
	// API returns most-recent-first when sorted by updated desc, so the first
	// hit per branch wins.
	byBranch := make(map[string]ghPR, len(prs))
	for _, pr := range prs {
		if _, ok := branches[pr.Head.Ref]; !ok {
			continue
		}
		if _, seen := byBranch[pr.Head.Ref]; seen {
			continue
		}
		byBranch[pr.Head.Ref] = pr
	}
	cols, err := p.store.ListColumns(ctx, board.ID)
	if err != nil {
		return fmt.Errorf("list columns: %w", err)
	}
	colByName := make(map[string]*db.Column, len(cols))
	for i := range cols {
		colByName[cols[i].Name] = &cols[i]
	}
	for i := range sessions {
		sess := &sessions[i]
		pr, ok := byBranch[sess.BranchName]
		if !ok {
			continue
		}
		newState := classify(pr)
		stateChanged := newState != sess.PRState
		numberChanged := sess.PRNumber == nil || *sess.PRNumber != pr.Number
		urlChanged := sess.PRURL != pr.HTMLURL
		titleChanged := sess.PRTitle != pr.Title
		if !stateChanged && !numberChanged && !urlChanged && !titleChanged {
			continue
		}
		if err := p.applyTransition(ctx, board, sess, sess.PRState, newState, pr, cfg, colByName); err != nil {
			log.Printf("github poller: ticket %d: %v", sess.TicketID, err)
		}
	}
	return nil
}

func classify(pr ghPR) string {
	switch pr.State {
	case "open":
		if pr.Draft {
			return PRStateDraft
		}
		return PRStateOpen
	case "closed":
		if pr.MergedAt != nil {
			return PRStateMerged
		}
		return PRStateClosed
	}
	return ""
}

func columnFor(state string, cfg Config) string {
	switch state {
	case PRStateDraft:
		return cfg.DraftColumn
	case PRStateOpen:
		return cfg.ReviewColumn
	case PRStateMerged:
		return cfg.DoneColumn
	case PRStateClosed:
		return cfg.ClosedColumn
	}
	return ""
}

func (p *Poller) applyTransition(ctx context.Context, board *db.Board, sess *db.Session, prior, next string, pr ghPR, cfg Config, colByName map[string]*db.Column) error {
	// Always persist the new observation, even when we decide not to move.
	defer func() {
		number := pr.Number
		if err := p.store.UpdateSessionPR(ctx, sess.ID, next, &number, pr.HTMLURL, pr.Title); err != nil {
			log.Printf("github poller: persist pr_state %d: %v", sess.ID, err)
			return
		}
		// Refetch from the DB before publishing so concurrent writes to
		// other columns (status / container_id from the session manager)
		// land on the wire instead of the in-memory snapshot we entered
		// the transition with. Without this the UI flickers back to a
		// stale status whenever the poller and a Start/Stop overlap.
		fresh, err := p.store.GetSession(ctx, sess.ID)
		if err != nil || fresh == nil {
			sess.PRState = next
			sess.PRNumber = &number
			sess.PRURL = pr.HTMLURL
			sess.PRTitle = pr.Title
			p.bus.Publish(board.ID, "session_updated", sess)
			return
		}
		*sess = *fresh
		p.bus.Publish(board.ID, "session_updated", fresh)
	}()

	target := columnFor(next, cfg)
	if target == "" {
		return nil
	}
	targetCol, ok := colByName[target]
	if !ok {
		log.Printf("github poller: board %d has no column %q (target for state %q)", board.ID, target, next)
		return nil
	}
	t, err := p.store.GetTicket(ctx, sess.TicketID)
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}
	if t.ArchivedAt != nil {
		return nil
	}
	// Don't fight a manual move: only act when the ticket sits in the column
	// the prior state mapped to. First observation (prior=="") always acts.
	if prior != "" {
		if priorCol := columnFor(prior, cfg); priorCol != "" && priorCol != target {
			if c, ok := colByName[priorCol]; ok && c.ID != t.ColumnID {
				return nil
			}
		}
	}
	if t.ColumnID == targetCol.ID {
		return nil
	}
	maxPos, err := p.store.MaxTicketPosition(ctx, targetCol.ID)
	if err != nil {
		return fmt.Errorf("max position: %w", err)
	}
	if err := p.store.MoveTicket(ctx, t.ID, targetCol.ID, maxPos+1); err != nil {
		return fmt.Errorf("move ticket: %w", err)
	}
	if updated, err := p.store.GetTicket(ctx, t.ID); err == nil {
		p.bus.Publish(board.ID, "ticket_moved", updated)
	}
	if next == PRStateMerged && p.sessions != nil {
		if err := p.sessions.Stop(ctx, sess.ID); err != nil {
			log.Printf("github poller: stop session %d after merge: %v", sess.ID, err)
		}
	}
	return nil
}

func (p *Poller) listPRs(ctx context.Context, repoPath string) ([]ghPR, error) {
	owner, repo, host, err := ParseGitHubRepo(repoPath)
	if err != nil {
		return nil, err
	}
	apiBase := APIBaseFor(host)
	tok := Token(host)

	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	next := fmt.Sprintf("%s/repos/%s/%s/pulls?state=all&per_page=%d&sort=updated&direction=desc",
		apiBase, url.PathEscape(owner), url.PathEscape(repo), prPageSize)

	var all []ghPR
	for page := 0; page < prMaxPages && next != ""; page++ {
		pageURL := next
		var cached prCacheEntry
		if v, ok := p.prCache.Load(pageURL); ok {
			cached = v.(prCacheEntry)
		}

		req, err := http.NewRequestWithContext(cctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		if cached.etag != "" {
			req.Header.Set("If-None-Match", cached.etag)
		}
		resp, err := p.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("github api: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("github api: read body: %w", err)
		}
		switch {
		case resp.StatusCode == http.StatusNotModified:
			all = append(all, cached.data...)
			next = cached.next
		case resp.StatusCode/100 == 2:
			var pageData []ghPR
			if err := json.Unmarshal(body, &pageData); err != nil {
				return nil, fmt.Errorf("github api: parse: %w", err)
			}
			all = append(all, pageData...)
			nextLink := ParseNextLink(resp.Header.Get("Link"))
			if etag := resp.Header.Get("ETag"); etag != "" {
				p.prCache.Store(pageURL, prCacheEntry{
					etag: etag,
					data: pageData,
					next: nextLink,
				})
			} else {
				p.prCache.Delete(pageURL)
			}
			next = nextLink
		default:
			return nil, fmt.Errorf("github api %s: %s: %s",
				pageURL, resp.Status, strings.TrimSpace(string(body)))
		}
	}
	return all, nil
}

// ParseGitHubRepo resolves the owner, repo, and host from the origin remote
// of the git checkout at repoPath. Shared by the PR poller and the build-cop
// poller — both need the same translation from on-disk repo to GitHub coords.
func ParseGitHubRepo(repoPath string) (owner, repo, host string, err error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	return parseRemoteURL(strings.TrimSpace(string(out)))
}

// parseRemoteURL handles the common GitHub remote shapes:
//
//	git@github.com:owner/repo(.git)
//	ssh://git@github.com/owner/repo(.git)
//	https://github.com/owner/repo(.git)
func parseRemoteURL(remote string) (owner, repo, host string, err error) {
	r := strings.TrimSuffix(remote, ".git")
	if !strings.Contains(r, "://") && strings.Contains(r, ":") {
		// SCP-style: [user@]host:owner/repo
		at := strings.LastIndex(r, "@")
		colon := strings.Index(r, ":")
		if colon > at {
			host = r[at+1 : colon]
			path := strings.TrimPrefix(r[colon+1:], "/")
			parts := strings.SplitN(path, "/", 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				return parts[0], parts[1], host, nil
			}
		}
		return "", "", "", fmt.Errorf("unrecognized git remote: %s", remote)
	}
	u, err := url.Parse(r)
	if err != nil {
		return "", "", "", fmt.Errorf("parse remote %q: %w", remote, err)
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("unrecognized git remote: %s", remote)
	}
	return parts[0], parts[1], u.Host, nil
}

// APIBaseFor returns the REST API root for a given git host. Honors
// GITHUB_API_URL (matching gh's convention) and falls back to the GitHub
// Enterprise Server convention of https://<host>/api/v3 for unknown hosts.
func APIBaseFor(host string) string {
	if v := os.Getenv("GITHUB_API_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	if host == "" || host == "github.com" {
		return defaultAPIBase
	}
	return "https://" + host + "/api/v3"
}

var (
	ghTokenCache    sync.Map // host -> string
	ghTokenLookPath = exec.LookPath
	ghTokenRun      = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

// Token returns the GitHub API token for host, preferring $GH_TOKEN then
// $GITHUB_TOKEN, then shelling out to `gh auth token` (cached per host).
// Returns "" when no token is available — callers fall back to unauthenticated
// requests (rate-limited at 60/hr per IP).
func Token(host string) string {
	if v := os.Getenv("GH_TOKEN"); v != "" {
		return v
	}
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		return v
	}
	key := host
	if key == "" {
		key = "github.com"
	}
	if v, ok := ghTokenCache.Load(key); ok {
		return v.(string)
	}
	tok := ""
	if _, err := ghTokenLookPath("gh"); err == nil {
		args := []string{"auth", "token"}
		if key != "github.com" {
			args = append(args, "--hostname", key)
		}
		if out, err := ghTokenRun("gh", args...); err == nil {
			tok = strings.TrimSpace(string(out))
		}
	}
	ghTokenCache.Store(key, tok)
	return tok
}

// ParseNextLink extracts the URL of rel="next" from a GitHub Link header.
func ParseNextLink(h string) string {
	if h == "" {
		return ""
	}
	for _, part := range strings.Split(h, ",") {
		seg := strings.TrimSpace(part)
		end := strings.Index(seg, ">")
		if end < 0 || !strings.HasPrefix(seg, "<") {
			continue
		}
		u := seg[1:end]
		params := seg[end+1:]
		if strings.Contains(params, `rel="next"`) {
			return u
		}
	}
	return ""
}
