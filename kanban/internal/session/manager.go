package session

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
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

	mu        sync.Mutex
	bridgeIPs map[int64]string // sessionID → container bridge IP
	proxies   *docker.ProxyManager
}

func NewManager(store *db.Store, dc *docker.Client, h *hooks.Runner) *Manager {
	return &Manager{
		store:     store,
		docker:    dc,
		hooks:     h,
		bridgeIPs: map[int64]string{},
		proxies:   docker.NewProxyManager(context.Background()),
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

	m.mu.Lock()
	m.bridgeIPs[sess.ID] = res.BridgeIP
	m.mu.Unlock()

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

	m.mu.Lock()
	delete(m.bridgeIPs, sess.ID)
	m.mu.Unlock()

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

func (m *Manager) BridgeIP(sessionID int64) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ip, ok := m.bridgeIPs[sessionID]
	return ip, ok
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
