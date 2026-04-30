package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

// Boards

func (s *Store) CreateBoard(ctx context.Context, b *Board) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO boards (name, slug, source_repo_path, worktree_root, base_branch) VALUES (?, ?, ?, ?, ?)`,
		b.Name, b.Slug, b.SourceRepoPath, b.WorktreeRoot, b.BaseBranch,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = id
	return s.createDefaultColumns(ctx, id)
}

func (s *Store) createDefaultColumns(ctx context.Context, boardID int64) error {
	defaults := []struct {
		name string
		pos  int
	}{{"Backlog", 0}, {"In Progress", 1}, {"Review", 2}, {"Done", 3}}
	for _, c := range defaults {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO columns (board_id, name, position) VALUES (?, ?, ?)`,
			boardID, c.name, c.pos,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListBoards(ctx context.Context) ([]Board, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, slug, source_repo_path, worktree_root, base_branch, created_at FROM boards ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	boards := []Board{}
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Name, &b.Slug, &b.SourceRepoPath, &b.WorktreeRoot, &b.BaseBranch, &b.CreatedAt); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func (s *Store) GetBoard(ctx context.Context, id int64) (*Board, error) {
	var b Board
	err := s.db.QueryRowContext(ctx, `SELECT id, name, slug, source_repo_path, worktree_root, base_branch, created_at FROM boards WHERE id=?`, id).
		Scan(&b.ID, &b.Name, &b.Slug, &b.SourceRepoPath, &b.WorktreeRoot, &b.BaseBranch, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// Columns

func (s *Store) ListColumns(ctx context.Context, boardID int64) ([]Column, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, board_id, name, position FROM columns WHERE board_id=? ORDER BY position`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := []Column{}
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.ID, &c.BoardID, &c.Name, &c.Position); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// Tickets

func (s *Store) CreateTicket(ctx context.Context, t *Ticket) error {
	var maxPos sql.NullInt64
	if err := s.db.QueryRowContext(ctx,
		`SELECT MAX(position) FROM tickets WHERE column_id=? AND archived_at IS NULL`, t.ColumnID,
	).Scan(&maxPos); err != nil {
		return err
	}
	t.Position = int(maxPos.Int64) + 1
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tickets (board_id, column_id, title, slug, body, position) VALUES (?, ?, ?, ?, ?, ?)`,
		t.BoardID, t.ColumnID, t.Title, t.Slug, t.Body, t.Position,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

func (s *Store) ListTickets(ctx context.Context, boardID int64) ([]Ticket, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, board_id, column_id, title, slug, body, position, created_at, archived_at
         FROM tickets WHERE board_id=? AND archived_at IS NULL ORDER BY column_id, position`,
		boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tickets := []Ticket{}
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.BoardID, &t.ColumnID, &t.Title, &t.Slug, &t.Body, &t.Position, &t.CreatedAt, &t.ArchivedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

func (s *Store) GetTicket(ctx context.Context, id int64) (*Ticket, error) {
	var t Ticket
	err := s.db.QueryRowContext(ctx,
		`SELECT id, board_id, column_id, title, slug, body, position, created_at, archived_at FROM tickets WHERE id=?`, id,
	).Scan(&t.ID, &t.BoardID, &t.ColumnID, &t.Title, &t.Slug, &t.Body, &t.Position, &t.CreatedAt, &t.ArchivedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) MoveTicket(ctx context.Context, ticketID, columnID int64, position int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE tickets SET column_id=?, position=? WHERE id=?`, columnID, position, ticketID)
	return err
}

func (s *Store) ArchiveTicket(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE tickets SET archived_at=unixepoch() WHERE id=?`, id)
	return err
}

// Sessions

func (s *Store) UpsertSession(ctx context.Context, sess *Session) error {
	if sess.ID == 0 {
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO sessions (ticket_id, worktree_path, branch_name, container_id, container_name, status, started_at, stopped_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			sess.TicketID, sess.WorktreePath, sess.BranchName, sess.ContainerID, sess.ContainerName, sess.Status, sess.StartedAt, sess.StoppedAt,
		)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		sess.ID = id
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET worktree_path=?, branch_name=?, container_id=?, container_name=?, status=?, started_at=?, stopped_at=? WHERE id=?`,
		sess.WorktreePath, sess.BranchName, sess.ContainerID, sess.ContainerName, sess.Status, sess.StartedAt, sess.StoppedAt, sess.ID,
	)
	return err
}

func (s *Store) UpdateSessionStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) GetSession(ctx context.Context, id int64) (*Session, error) {
	var sess Session
	err := s.db.QueryRowContext(ctx,
		`SELECT id, ticket_id, worktree_path, branch_name, container_id, container_name, status, started_at, stopped_at FROM sessions WHERE id=?`, id,
	).Scan(&sess.ID, &sess.TicketID, &sess.WorktreePath, &sess.BranchName, &sess.ContainerID, &sess.ContainerName, &sess.Status, &sess.StartedAt, &sess.StoppedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) GetSessionByTicket(ctx context.Context, ticketID int64) (*Session, error) {
	var sess Session
	err := s.db.QueryRowContext(ctx,
		`SELECT id, ticket_id, worktree_path, branch_name, container_id, container_name, status, started_at, stopped_at FROM sessions WHERE ticket_id=?`, ticketID,
	).Scan(&sess.ID, &sess.TicketID, &sess.WorktreePath, &sess.BranchName, &sess.ContainerID, &sess.ContainerName, &sess.Status, &sess.StartedAt, &sess.StoppedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) ListSessionsByBoard(ctx context.Context, boardID int64) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.ticket_id, s.worktree_path, s.branch_name, s.container_id, s.container_name, s.status, s.started_at, s.stopped_at
         FROM sessions s JOIN tickets t ON t.id=s.ticket_id WHERE t.board_id=?`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := []Session{}
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.TicketID, &sess.WorktreePath, &sess.BranchName, &sess.ContainerID, &sess.ContainerName, &sess.Status, &sess.StartedAt, &sess.StoppedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

func (s *Store) DeleteSession(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id=?`, id)
	return err
}

// Port allocations

func (s *Store) CreatePort(ctx context.Context, p *PortAllocation) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO port_allocations (session_id, label, container_port, host_port, proxy_active) VALUES (?, ?, ?, ?, ?)`,
		p.SessionID, p.Label, p.ContainerPort, p.HostPort, boolToInt(p.ProxyActive),
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

func (s *Store) ListPorts(ctx context.Context, sessionID int64) ([]PortAllocation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, label, container_port, host_port, proxy_active FROM port_allocations WHERE session_id=?`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ports := []PortAllocation{}
	for rows.Next() {
		var p PortAllocation
		var active int
		if err := rows.Scan(&p.ID, &p.SessionID, &p.Label, &p.ContainerPort, &p.HostPort, &active); err != nil {
			return nil, err
		}
		p.ProxyActive = active != 0
		ports = append(ports, p)
	}
	return ports, rows.Err()
}

func (s *Store) ListAllActivePorts(ctx context.Context) ([]PortAllocation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, label, container_port, host_port, proxy_active FROM port_allocations WHERE proxy_active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ports := []PortAllocation{}
	for rows.Next() {
		var p PortAllocation
		var active int
		if err := rows.Scan(&p.ID, &p.SessionID, &p.Label, &p.ContainerPort, &p.HostPort, &active); err != nil {
			return nil, err
		}
		p.ProxyActive = active != 0
		ports = append(ports, p)
	}
	return ports, rows.Err()
}

func (s *Store) SetPortActive(ctx context.Context, id int64, active bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE port_allocations SET proxy_active=? WHERE id=?`, boolToInt(active), id)
	return err
}

func (s *Store) DeletePort(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM port_allocations WHERE id=?`, id)
	return err
}

func (s *Store) AllocateHostPort(ctx context.Context, start, end int) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT host_port FROM port_allocations`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	taken := map[int]bool{}
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			return 0, err
		}
		taken[p] = true
	}
	for p := start; p <= end; p++ {
		if !taken[p] {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port in range %d-%d", start, end)
}

// Task runs

func (s *Store) CreateTaskRun(ctx context.Context, tr *TaskRun) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO task_runs (session_id, task_label, command, exec_id, status) VALUES (?, ?, ?, ?, ?)`,
		tr.SessionID, tr.TaskLabel, tr.Command, tr.ExecID, tr.Status,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	tr.ID = id
	return nil
}

func (s *Store) UpdateTaskRunStatus(ctx context.Context, id int64, status string, exitCode *int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE task_runs SET status=?, exit_code=?, stopped_at=unixepoch() WHERE id=?`, status, exitCode, id)
	return err
}

func (s *Store) GetTaskRun(ctx context.Context, id int64) (*TaskRun, error) {
	var tr TaskRun
	err := s.db.QueryRowContext(ctx,
		`SELECT id, session_id, task_label, command, exec_id, status, exit_code, started_at, stopped_at FROM task_runs WHERE id=?`, id,
	).Scan(&tr.ID, &tr.SessionID, &tr.TaskLabel, &tr.Command, &tr.ExecID, &tr.Status, &tr.ExitCode, &tr.StartedAt, &tr.StoppedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tr, nil
}

func (s *Store) ListTaskRuns(ctx context.Context, sessionID int64) ([]TaskRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, task_label, command, exec_id, status, exit_code, started_at, stopped_at FROM task_runs WHERE session_id=? ORDER BY id DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []TaskRun{}
	for rows.Next() {
		var tr TaskRun
		if err := rows.Scan(&tr.ID, &tr.SessionID, &tr.TaskLabel, &tr.Command, &tr.ExecID, &tr.Status, &tr.ExitCode, &tr.StartedAt, &tr.StoppedAt); err != nil {
			return nil, err
		}
		runs = append(runs, tr)
	}
	return runs, rows.Err()
}

// Hook configs

func (s *Store) ListHooks(ctx context.Context, boardID *int64, event string) ([]HookConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, board_id, event, command, enabled FROM hook_configs
         WHERE enabled=1 AND event=? AND (board_id IS NULL OR board_id=?)`,
		event, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hooks := []HookConfig{}
	for rows.Next() {
		var h HookConfig
		var enabled int
		if err := rows.Scan(&h.ID, &h.BoardID, &h.Event, &h.Command, &enabled); err != nil {
			return nil, err
		}
		h.Enabled = enabled != 0
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
