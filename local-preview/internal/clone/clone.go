// Package clone runs repo registrations' mirror clones in the background.
// POST /api/repos inserts the row with status "cloning" and returns
// immediately; a Cloner goroutine performs the clone and records the
// outcome on the row (ready/failed), exposing the transport's live progress
// for the API to merge into repo responses. Clones interrupted by a
// shutdown are restarted on the next Start, mirroring how the build queue
// re-enqueues unfinished deploys.
package clone

import (
	"bytes"
	"context"
	"errors"
	"log"
	"sync"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
)

// Cloner owns the background clone goroutines for repos being registered.
type Cloner struct {
	db  *db.Store
	git *gitrepo.Manager
	// onReady, when set, is called after a repo turns ready — the server
	// wires it to the watcher's Kick so a repo registered with watch=true
	// deploys its branch tips without waiting a poll interval.
	onReady func()
	ctx     context.Context

	mu     sync.Mutex
	active map[int64]*job
}

// job is one in-flight clone: its cancel handle and progress tail.
type job struct {
	cancel context.CancelFunc
	prog   *progressTail
}

// New returns a Cloner. onReady may be nil.
func New(database *db.Store, git *gitrepo.Manager, onReady func()) *Cloner {
	return &Cloner{
		db:      database,
		git:     git,
		onReady: onReady,
		ctx:     context.Background(),
		active:  map[int64]*job{},
	}
}

// Start binds clone goroutines to ctx (a shutdown cancels them; the rows
// stay "cloning" and are picked up again here) and restarts clones a
// previous run left unfinished.
func (c *Cloner) Start(ctx context.Context) {
	c.ctx = ctx
	repos, err := c.db.ListRepos()
	if err != nil {
		log.Printf("clone: list repos: %v", err)
		return
	}
	for _, repo := range repos {
		if repo.Status == db.RepoCloning {
			log.Printf("clone: resuming interrupted clone of %s", repo.Name)
			c.Begin(repo)
		}
	}
}

// Begin launches the background clone for a freshly created repo row
// (status "cloning") and returns immediately.
func (c *Cloner) Begin(repo db.Repo) {
	ctx, cancel := context.WithCancel(c.ctx)
	j := &job{cancel: cancel, prog: &progressTail{}}
	c.mu.Lock()
	c.active[repo.ID] = j
	c.mu.Unlock()
	go c.run(ctx, repo, j)
}

func (c *Cloner) run(ctx context.Context, repo db.Repo, j *job) {
	defer func() {
		c.mu.Lock()
		delete(c.active, repo.ID)
		c.mu.Unlock()
		j.cancel()
	}()
	_, err := c.git.Add(ctx, repo.Name, repo.Source, j.prog)
	if err != nil {
		log.Printf("clone: %s: %v", repo.Name, err)
		if serr := c.db.SetRepoStatus(repo.ID, db.RepoFailed, truncate(err.Error(), 500)); serr != nil && !errors.Is(serr, db.ErrNotFound) {
			log.Printf("clone: mark %s failed: %v", repo.Name, serr)
		}
		return
	}
	switch serr := c.db.SetRepoStatus(repo.ID, db.RepoReady, ""); {
	case errors.Is(serr, db.ErrNotFound):
		// Deleted while cloning: the delete's mirror cleanup may have run
		// before the clone landed, so remove the now-orphaned mirror.
		if rerr := c.git.Remove(repo.Name); rerr != nil {
			log.Printf("clone: remove mirror of deleted repo %s: %v", repo.Name, rerr)
		}
	case serr != nil:
		log.Printf("clone: mark %s ready: %v", repo.Name, serr)
	default:
		log.Printf("clone: %s ready", repo.Name)
		if c.onReady != nil {
			c.onReady()
		}
	}
}

// Cancel aborts the in-flight clone for a repo being deleted. A repo with
// no active clone is a no-op.
func (c *Cloner) Cancel(repoID int64) {
	c.mu.Lock()
	j := c.active[repoID]
	c.mu.Unlock()
	if j != nil {
		j.cancel()
	}
}

// Progress returns the clone's most recent progress line ("Counting
// objects: …"), or "" once it finished or when the transport reports none
// (local-path sources).
func (c *Cloner) Progress(repoID int64) string {
	c.mu.Lock()
	j := c.active[repoID]
	c.mu.Unlock()
	if j == nil {
		return ""
	}
	return j.prog.Last()
}

// progressTail retains the tail of a clone's progress stream. Git progress
// rewrites one line via carriage returns, so the last \r- or \n-delimited
// segment is the current line.
type progressTail struct {
	mu  sync.Mutex
	buf []byte
}

const progressTailCap = 1024

func (p *progressTail) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, b...)
	if len(p.buf) > progressTailCap {
		p.buf = append(p.buf[:0], p.buf[len(p.buf)-progressTailCap:]...)
	}
	return len(b), nil
}

func (p *progressTail) Last() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	tail := p.buf
	for len(tail) > 0 {
		i := bytes.LastIndexAny(tail, "\r\n")
		if i < 0 {
			break
		}
		if line := bytes.TrimSpace(tail[i+1:]); len(line) > 0 {
			return string(line)
		}
		tail = tail[:i]
	}
	return string(bytes.TrimSpace(tail))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
