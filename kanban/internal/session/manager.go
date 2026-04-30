package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/git"
	"github.com/jmelahman/kanban/internal/hooks"
)

type Manager struct {
	store  *db.Store
	docker *docker.Client
	hooks  *hooks.Runner

	proxies *docker.ProxyManager
}

func NewManager(store *db.Store, dc *docker.Client, h *hooks.Runner) *Manager {
	return &Manager{
		store:   store,
		docker:  dc,
		hooks:   h,
		proxies: docker.NewProxyManager(context.Background(), dc),
	}
}

// Ensure creates a session row for a ticket if missing, allocating a worktree.
func (m *Manager) Ensure(ctx context.Context, board *db.Board, ticket *db.Ticket) (*db.Session, error) {
	if sess, err := m.store.GetSessionByTicket(ctx, ticket.ID); err == nil {
		return sess, nil
	}

	branch := fmt.Sprintf("kanban/%s/%s", board.Slug, ticket.Slug)
	worktreePath := filepath.Join(board.WorktreeRoot, ticket.Slug)
	containerName := fmt.Sprintf("kanban-%s-%s", board.Slug, ticket.Slug)

	if err := git.AddWorktree(board.SourceRepoPath, branch, worktreePath, board.BaseBranch); err != nil {
		return nil, fmt.Errorf("create worktree: %w", err)
	}

	sess := &db.Session{
		TicketID:      ticket.ID,
		WorktreePath:  worktreePath,
		BranchName:    branch,
		ContainerName: &containerName,
		Status:        db.SessionStatusStopped,
	}
	if err := m.store.UpsertSession(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// Start brings up the devcontainer for a session.
func (m *Manager) Start(ctx context.Context, sessionID int64) (*db.Session, error) {
	sess, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status != db.SessionStatusStopped {
		return sess, nil
	}

	cfg, err := docker.LoadDevcontainer(sess.WorktreePath)
	if err != nil {
		_ = m.store.UpdateSessionStatus(ctx, sess.ID, db.SessionStatusError)
		return nil, err
	}

	_ = m.store.UpdateSessionStatus(ctx, sess.ID, db.SessionStatusStarting)

	ports, _ := m.store.ListPorts(ctx, sess.ID)
	mappings := make([]docker.PortMapping, 0, len(ports))
	for _, p := range ports {
		mappings = append(mappings, docker.PortMapping{HostPort: p.HostPort, ContainerPort: p.ContainerPort})
	}

	containerName := ""
	if sess.ContainerName != nil {
		containerName = *sess.ContainerName
	}

	res, err := m.docker.Spawn(ctx, cfg, docker.SpawnOptions{
		WorktreePath:  sess.WorktreePath,
		ContainerName: containerName,
		Ports:         mappings,
	})
	if err != nil {
		_ = m.store.UpdateSessionStatus(ctx, sess.ID, db.SessionStatusError)
		return nil, err
	}

	now := time.Now().Unix()
	sess.ContainerID = &res.ContainerID
	sess.Status = db.SessionStatusIdle
	sess.StartedAt = &now
	sess.StoppedAt = nil
	if err := m.store.UpsertSession(ctx, sess); err != nil {
		return nil, err
	}

	board, _ := m.boardForSession(ctx, sess)
	var boardID *int64
	if board != nil {
		boardID = &board.ID
	}
	m.hooks.Fire(boardID, hooks.EventSessionStarted, map[string]string{
		"session_id": fmt.Sprintf("%d", sess.ID),
		"ticket_id":  fmt.Sprintf("%d", sess.TicketID),
	})

	return sess, nil
}

// Stop tears down the devcontainer; worktree is preserved.
func (m *Manager) Stop(ctx context.Context, sessionID int64) error {
	sess, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.ContainerID != nil && *sess.ContainerID != "" {
		_ = m.docker.StopContainer(ctx, *sess.ContainerID, 10*time.Second)
		_ = m.docker.RemoveContainer(ctx, *sess.ContainerID)
	}
	now := time.Now().Unix()
	sess.Status = db.SessionStatusStopped
	sess.StoppedAt = &now
	cleared := ""
	sess.ContainerID = &cleared
	if err := m.store.UpsertSession(ctx, sess); err != nil {
		return err
	}

	// Close any active proxies for this session.
	ports, _ := m.store.ListPorts(ctx, sess.ID)
	for _, p := range ports {
		if p.ProxyActive {
			m.proxies.Close(p.HostPort)
			_ = m.store.SetPortActive(ctx, p.ID, false)
		}
	}

	board, _ := m.boardForSession(ctx, sess)
	var boardID *int64
	if board != nil {
		boardID = &board.ID
	}
	m.hooks.Fire(boardID, hooks.EventSessionStopped, map[string]string{
		"session_id": fmt.Sprintf("%d", sess.ID),
	})
	return nil
}

// Destroy fully tears down a session: stops the container, removes the
// worktree directory, deletes the branch, and removes the session row.
// Errors from filesystem/git cleanup are non-fatal and reported via the
// returned error only when the DB row removal itself fails.
func (m *Manager) Destroy(ctx context.Context, sessionID int64) error {
	sess, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	_ = m.Stop(ctx, sessionID)

	board, _ := m.boardForSession(ctx, sess)
	if board != nil && sess.WorktreePath != "" {
		_ = git.RemoveWorktree(board.SourceRepoPath, sess.WorktreePath)
	}
	if sess.WorktreePath != "" {
		_ = os.RemoveAll(sess.WorktreePath)
	}
	if board != nil && sess.BranchName != "" {
		_ = git.DeleteBranch(board.SourceRepoPath, sess.BranchName)
	}
	return m.store.DeleteSession(ctx, sess.ID)
}

func (m *Manager) Proxies() *docker.ProxyManager { return m.proxies }

func (m *Manager) Docker() *docker.Client { return m.docker }

func (m *Manager) boardForSession(ctx context.Context, sess *db.Session) (*db.Board, error) {
	t, err := m.store.GetTicket(ctx, sess.TicketID)
	if err != nil {
		return nil, err
	}
	return m.store.GetBoard(ctx, t.BoardID)
}
