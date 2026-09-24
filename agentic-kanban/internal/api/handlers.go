package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/errreport"
	"github.com/jmelahman/kanban/internal/git"
	gh "github.com/jmelahman/kanban/internal/github"
	"github.com/jmelahman/kanban/internal/harness"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/kanbantoml"
	"github.com/jmelahman/kanban/internal/previews"
	"github.com/jmelahman/kanban/internal/session"
	"github.com/jmelahman/kanban/internal/slug"
	"github.com/jmelahman/kanban/internal/tasks"
)

type handlers struct {
	store    *db.Store
	docker   *docker.Client
	sessions *session.Manager
	hooks    *hooks.Runner
	config   *config.Config
	tasks    *tasks.Runner
	bus      *EventBus
	build    BuildInfo
	reporter *errreport.Reporter
	previews *orchestrator.Orchestrator
}

func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DB().PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"db": err.Error()})
		return
	}
	if err := h.docker.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"docker": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.build)
}

// Boards

type createBoardReq struct {
	Name           string `json:"name"`
	RepoPath       string `json:"repo_path"`
	MountPath      string `json:"mount_path"`
	ProjectDir     string `json:"project_dir"`
	WorktreeRoot   string `json:"worktree_root"`
	BaseBranch     string `json:"base_branch"`
	BranchPrefix   string `json:"branch_prefix"`
	GitAuthorName  string `json:"git_author_name"`
	GitAuthorEmail string `json:"git_author_email"`
}

func (h *handlers) listBoards(w http.ResponseWriter, r *http.Request) {
	boards, err := h.store.ListBoards(r.Context())
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, boards)
}

func (h *handlers) createBoard(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBody[createBoardReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.RepoPath = strings.TrimSpace(req.RepoPath)
	req.MountPath = strings.TrimSpace(req.MountPath)
	if req.Name == "" {
		h.httpError(w, fmt.Errorf("name required"), 400)
		return
	}
	if req.RepoPath == "" && req.MountPath == "" {
		h.httpError(w, fmt.Errorf("at least one of repo_path or mount_path required"), 400)
		return
	}
	projectDir, err := session.ValidateProjectDir(req.ProjectDir, req.RepoPath, req.MountPath)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if req.BaseBranch == "" && req.RepoPath != "" {
		if b, err := git.DefaultBranch(req.RepoPath); err == nil {
			req.BaseBranch = strings.TrimSpace(b)
		}
	}
	if req.BaseBranch == "" {
		req.BaseBranch = "main"
	}
	if req.WorktreeRoot == "" && req.RepoPath != "" {
		req.WorktreeRoot = filepath.Join(h.config.WorktreesDir(), slug.Make(req.Name, "x"))
	}
	board := &db.Board{
		Name:           req.Name,
		Slug:           slug.Make(req.Name, "x"),
		RepoPath:       req.RepoPath,
		MountPath:      req.MountPath,
		ProjectDir:     projectDir,
		WorktreeRoot:   req.WorktreeRoot,
		BaseBranch:     req.BaseBranch,
		BranchPrefix:   strings.TrimSpace(req.BranchPrefix),
		GitAuthorName:  strings.TrimSpace(req.GitAuthorName),
		GitAuthorEmail: strings.TrimSpace(req.GitAuthorEmail),
	}
	if err := h.store.CreateBoard(r.Context(), board); err != nil {
		if isUniqueViolation(err) {
			h.httpError(w, fmt.Errorf("a board named %q already exists", req.Name), http.StatusConflict)
			return
		}
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 201, board)
}

type updateBoardReq struct {
	Name           *string `json:"name"`
	RepoPath       *string `json:"repo_path"`
	MountPath      *string `json:"mount_path"`
	ProjectDir     *string `json:"project_dir"`
	WorktreeRoot   *string `json:"worktree_root"`
	BaseBranch     *string `json:"base_branch"`
	BranchPrefix   *string `json:"branch_prefix"`
	GitAuthorName  *string `json:"git_author_name"`
	GitAuthorEmail *string `json:"git_author_email"`
}

func (h *handlers) updateBoard(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	board, err := h.store.GetBoard(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	req, err := decodeBody[updateBoardReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			h.httpError(w, fmt.Errorf("name cannot be empty"), 400)
			return
		}
		board.Name = name
	}
	if req.RepoPath != nil {
		board.RepoPath = strings.TrimSpace(*req.RepoPath)
	}
	if req.MountPath != nil {
		board.MountPath = strings.TrimSpace(*req.MountPath)
	}
	if board.RepoPath == "" && board.MountPath == "" {
		h.httpError(w, fmt.Errorf("at least one of repo_path or mount_path required"), 400)
		return
	}
	if req.ProjectDir != nil {
		board.ProjectDir = strings.TrimSpace(*req.ProjectDir)
	}
	// Re-validate on every update, not just when project_dir itself changes:
	// setting mount_path on a board that already has a project_dir is the
	// same invalid pair arrived at from the other side.
	projectDir, err := session.ValidateProjectDir(board.ProjectDir, board.RepoPath, board.MountPath)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	board.ProjectDir = projectDir
	if req.WorktreeRoot != nil {
		board.WorktreeRoot = strings.TrimSpace(*req.WorktreeRoot)
	}
	if req.BaseBranch != nil {
		base := strings.TrimSpace(*req.BaseBranch)
		if base == "" {
			h.httpError(w, fmt.Errorf("base_branch cannot be empty"), 400)
			return
		}
		board.BaseBranch = base
	}
	if req.BranchPrefix != nil {
		board.BranchPrefix = strings.TrimSpace(*req.BranchPrefix)
	}
	if req.GitAuthorName != nil {
		board.GitAuthorName = strings.TrimSpace(*req.GitAuthorName)
	}
	if req.GitAuthorEmail != nil {
		board.GitAuthorEmail = strings.TrimSpace(*req.GitAuthorEmail)
	}
	if err := h.store.UpdateBoard(r.Context(), board); err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, board)
}

type moveBoardReq struct {
	Position int `json:"position"`
}

func (h *handlers) moveBoard(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	req, err := decodeBody[moveBoardReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if err := h.store.MoveBoard(r.Context(), id, req.Position); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			h.httpError(w, err, 404)
			return
		}
		h.httpError(w, err, 500)
		return
	}
	w.WriteHeader(204)
}

func (h *handlers) deleteBoard(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	board, err := h.store.GetBoard(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	sessions, err := h.store.ListSessionsByBoard(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	for _, sess := range sessions {
		if err := h.sessions.Destroy(r.Context(), sess.ID); err != nil {
			log.Printf("delete board %d: destroy session %d: %v", id, sess.ID, err)
		}
	}
	if err := h.store.DeleteBoard(r.Context(), id); err != nil {
		h.httpError(w, err, 500)
		return
	}
	// The board's preview deployments outlive its row otherwise: running
	// backends, the mirror clone, artifacts, state dirs, and build logs.
	// Best-effort — the board is already gone, and a board that never
	// deployed has no repo registered (ErrNotFound).
	if h.previews != nil {
		if err := h.previews.DeleteRepo(previews.RepoName(board)); err != nil && !errors.Is(err, orchestrator.ErrNotFound) {
			log.Printf("delete board %d: delete preview repo: %v", id, err)
		}
	}
	w.WriteHeader(204)
}

func (h *handlers) getBoard(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	board, err := h.store.GetBoard(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	writeJSON(w, 200, board)
}

// Board env vars — write-only secrets injected into session containers at
// launch. Responses carry key names only; values never leave the server.

// envKeyRe matches POSIX-style environment variable names.
var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reservedEnvPrefix guards the system-injected KANBAN_* variables
// (KANBAN_SESSION_ID, KANBAN_API_URL) from being shadowed by board vars.
const reservedEnvPrefix = "KANBAN_"

type boardEnvResp struct {
	Keys []string `json:"keys"`
}

type patchBoardEnvReq struct {
	Set   map[string]string `json:"set"`
	Unset []string          `json:"unset"`
}

func (h *handlers) listBoardEnv(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	if _, err := h.store.GetBoard(r.Context(), id); err != nil {
		h.httpError(w, err, 404)
		return
	}
	keys, err := h.store.ListBoardEnvVarKeys(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, boardEnvResp{Keys: keys})
}

func (h *handlers) patchBoardEnv(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	if _, err := h.store.GetBoard(r.Context(), id); err != nil {
		h.httpError(w, err, 404)
		return
	}
	req, err := decodeBody[patchBoardEnvReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if len(req.Set) == 0 && len(req.Unset) == 0 {
		h.httpError(w, fmt.Errorf("nothing to set or unset"), 400)
		return
	}
	for key := range req.Set {
		if err := validateEnvKey(key); err != nil {
			h.httpError(w, err, 400)
			return
		}
	}
	for _, key := range req.Unset {
		if err := validateEnvKey(key); err != nil {
			h.httpError(w, err, 400)
			return
		}
	}
	for key, value := range req.Set {
		if err := h.store.SetBoardEnvVar(r.Context(), id, key, value); err != nil {
			h.httpError(w, err, 500)
			return
		}
	}
	for _, key := range req.Unset {
		if err := h.store.DeleteBoardEnvVar(r.Context(), id, key); err != nil {
			h.httpError(w, err, 500)
			return
		}
	}
	keys, err := h.store.ListBoardEnvVarKeys(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, boardEnvResp{Keys: keys})
}

func validateEnvKey(key string) error {
	if !envKeyRe.MatchString(key) {
		return fmt.Errorf("invalid env var key %q", key)
	}
	if strings.HasPrefix(strings.ToUpper(key), reservedEnvPrefix) {
		return fmt.Errorf("key %q is reserved (%s prefix)", key, reservedEnvPrefix)
	}
	return nil
}

type boardStateResp struct {
	Board       *db.Board    `json:"board"`
	Columns     []db.Column  `json:"columns"`
	Tickets     []db.Ticket  `json:"tickets"`
	Sessions    []db.Session `json:"sessions"`
	MergeConfig MergeConfig  `json:"merge_config"`
	SyncConfig  SyncConfig   `json:"sync_config"`
}

func (h *handlers) boardState(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	board, err := h.store.GetBoard(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	cols, err := h.store.ListColumns(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	tickets, err := h.store.ListTickets(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	sessions, err := h.store.ListSessionsByBoard(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, boardStateResp{
		Board:       board,
		Columns:     cols,
		Tickets:     tickets,
		Sessions:    sessions,
		MergeConfig: loadMergeConfig(board.RepoPath),
		SyncConfig:  loadSyncConfig(board.RepoPath),
	})
}

// Tickets

type createTicketReq struct {
	ColumnID int64  `json:"column_id,omitempty"`
	Column   string `json:"column,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

func (h *handlers) createTicket(w http.ResponseWriter, r *http.Request) {
	board, err := h.resolveBoardIdent(r.Context(), r.PathValue("id"))
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	req, err := decodeBody[createTicketReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if req.Title == "" {
		h.httpError(w, fmt.Errorf("title required"), 400)
		return
	}
	columnID, err := h.resolveColumn(r.Context(), board.ID, req.ColumnID, req.Column)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	t := &db.Ticket{
		BoardID:  board.ID,
		ColumnID: columnID,
		Title:    req.Title,
		Slug:     slug.Make(req.Title, "x"),
		Body:     req.Body,
	}
	if err := h.store.CreateTicket(r.Context(), t); err != nil {
		h.httpError(w, err, 500)
		return
	}
	h.bus.Publish(board.ID, "ticket_created", t)
	h.hooks.Fire(&board.ID, hooks.EventTicketCreated, map[string]string{
		"ticket_id": fmt.Sprintf("%d", t.ID),
		"board":     board.Name,
	})
	writeJSON(w, 201, t)
}

// getTicket returns one ticket by id. Unlike the board-scoped listings it
// needs no board context, so callers holding only a ticket id (the CLI's
// `ticket info <id>`) can find the board it lives on.
func (h *handlers) getTicket(w http.ResponseWriter, r *http.Request) {
	t, err := h.store.GetTicket(r.Context(), pathID(r, "id"))
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	writeJSON(w, 200, t)
}

type updateTicketReq struct {
	Title *string `json:"title"`
	Body  *string `json:"body"`
}

func (h *handlers) updateTicket(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	t, err := h.store.GetTicket(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	req, err := decodeBody[updateTicketReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			h.httpError(w, fmt.Errorf("title cannot be empty"), 400)
			return
		}
		t.Title = title
	}
	if req.Body != nil {
		t.Body = *req.Body
	}
	if err := h.store.UpdateTicket(r.Context(), t); err != nil {
		h.httpError(w, err, 500)
		return
	}
	h.bus.Publish(t.BoardID, "ticket_updated", t)
	writeJSON(w, 200, t)
}

// resolveBoardIdent looks up a board by numeric id first, then by slug. The
// numeric path stays the canonical form for back-compat; slug is a fallback
// so programmatic callers don't need to pre-resolve IDs.
func (h *handlers) resolveBoardIdent(ctx context.Context, ident string) (*db.Board, error) {
	if ident == "" {
		return nil, fmt.Errorf("board identifier required")
	}
	if id, err := strconv.ParseInt(ident, 10, 64); err == nil {
		if b, err := h.store.GetBoard(ctx, id); err == nil {
			return b, nil
		}
	}
	return h.store.GetBoardBySlug(ctx, ident)
}

// resolveColumn picks a column for a new ticket. Precedence: numeric column_id
// > column (numeric string or case-insensitive name) > leftmost column.
func (h *handlers) resolveColumn(ctx context.Context, boardID int64, columnID int64, column string) (int64, error) {
	cols, err := h.store.ListColumns(ctx, boardID)
	if err != nil {
		return 0, err
	}
	if len(cols) == 0 {
		return 0, fmt.Errorf("board has no columns")
	}
	if columnID > 0 {
		for _, c := range cols {
			if c.ID == columnID {
				return c.ID, nil
			}
		}
		return 0, fmt.Errorf("column_id %d not found on this board", columnID)
	}
	if column != "" {
		if id, err := strconv.ParseInt(column, 10, 64); err == nil {
			for _, c := range cols {
				if c.ID == id {
					return c.ID, nil
				}
			}
		}
		for _, c := range cols {
			if strings.EqualFold(c.Name, column) {
				return c.ID, nil
			}
		}
		return 0, fmt.Errorf("column %q not found on this board", column)
	}
	return cols[0].ID, nil
}

type moveTicketReq struct {
	ColumnID int64 `json:"column_id"`
	Position int   `json:"position"`
}

func (h *handlers) moveTicket(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	req, err := decodeBody[moveTicketReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if err := h.store.MoveTicket(r.Context(), id, req.ColumnID, req.Position); err != nil {
		h.httpError(w, err, 500)
		return
	}
	t, _ := h.store.GetTicket(r.Context(), id)
	if t != nil {
		h.bus.Publish(t.BoardID, "ticket_moved", t)
	}
	w.WriteHeader(204)
}

func (h *handlers) archiveTicket(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	t, err := h.store.GetTicket(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	h.stopSessionForTicket(r.Context(), id)
	if err := h.store.ArchiveTicket(r.Context(), id); err != nil {
		h.httpError(w, err, 500)
		return
	}
	h.publishTicketArchived(t)
	w.WriteHeader(204)
}

func (h *handlers) unarchiveTicket(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	t, err := h.store.GetTicket(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	if t.ArchivedAt == nil {
		h.httpError(w, fmt.Errorf("ticket is not archived"), 400)
		return
	}
	if err := h.store.UnarchiveTicket(r.Context(), id); err != nil {
		h.httpError(w, err, 500)
		return
	}
	updated, _ := h.store.GetTicket(r.Context(), id)
	if updated != nil {
		h.bus.Publish(updated.BoardID, "ticket_unarchived", updated)
	}
	w.WriteHeader(204)
}

func (h *handlers) listArchivedTickets(w http.ResponseWriter, r *http.Request) {
	boardID := pathID(r, "id")
	tickets, err := h.store.ListArchivedTickets(r.Context(), boardID)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, tickets)
}

func (h *handlers) deleteTicket(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	t, err := h.store.GetTicket(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	if t.ArchivedAt == nil {
		h.httpError(w, fmt.Errorf("ticket must be archived before deletion"), 400)
		return
	}
	h.destroySessionForTicket(r.Context(), id)
	if err := h.store.DeleteTicket(r.Context(), id); err != nil {
		h.httpError(w, err, 500)
		return
	}
	h.bus.Publish(t.BoardID, "ticket_deleted", t)
	w.WriteHeader(204)
}

// archiveColumnTickets archives every non-archived ticket in a column. Mirrors
// archiveTicket per-ticket: stops any running session, archives, then publishes
// a ticket_archived SSE + EventTicketArchived hook for each ticket affected.
func (h *handlers) archiveColumnTickets(w http.ResponseWriter, r *http.Request) {
	columnID := pathID(r, "id")
	tickets, err := h.store.ListTicketsInColumn(r.Context(), columnID)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	for _, t := range tickets {
		h.stopSessionForTicket(r.Context(), t.ID)
	}
	if _, err := h.store.ArchiveTicketsInColumn(r.Context(), columnID); err != nil {
		h.httpError(w, err, 500)
		return
	}
	for i := range tickets {
		h.publishTicketArchived(&tickets[i])
	}
	w.WriteHeader(204)
}

// deleteAllArchived permanently deletes every archived ticket on a board.
// Mirrors deleteTicket per-ticket: destroys any associated session, deletes
// the ticket, then publishes a ticket_deleted SSE for each.
func (h *handlers) deleteAllArchived(w http.ResponseWriter, r *http.Request) {
	boardID := pathID(r, "id")
	tickets, err := h.store.ListArchivedTickets(r.Context(), boardID)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	for _, t := range tickets {
		h.destroySessionForTicket(r.Context(), t.ID)
	}
	if _, err := h.store.DeleteAllArchivedTickets(r.Context(), boardID); err != nil {
		h.httpError(w, err, 500)
		return
	}
	for i := range tickets {
		t := tickets[i]
		h.bus.Publish(boardID, "ticket_deleted", &t)
	}
	w.WriteHeader(204)
}

type strategyReq struct {
	Strategy string `json:"strategy"`
}

func (h *handlers) syncTicket(w http.ResponseWriter, r *http.Request) {
	h.ticketStrategyAction(w, r,
		[]string{"rebase", "merge"},
		func(string) string { return "rebase" },
		func(repo, strat string) bool { return loadSyncConfig(repo).allows(strat) },
		h.sessions.Sync,
		true,
	)
}

func (h *handlers) mergeTicket(w http.ResponseWriter, r *http.Request) {
	h.ticketStrategyAction(w, r,
		kanbantoml.MergeStrategies,
		func(repo string) string { return loadMergeConfig(repo).DefaultStrategy },
		func(repo, strat string) bool { return loadMergeConfig(repo).allows(strat) },
		h.sessions.Merge,
		false,
	)
}

// ticketStrategyAction is the shared body of syncTicket and mergeTicket: parse
// strategy, validate against the allowed set and the per-board config, ensure
// a session exists, run the action, optionally publish session_updated.
func (h *handlers) ticketStrategyAction(
	w http.ResponseWriter, r *http.Request,
	allowed []string, defaultFor func(repoPath string) string,
	isAllowed func(repoPath, strategy string) bool,
	action func(ctx context.Context, sessID int64, strategy string) error,
	publishOnSuccess bool,
) {
	id := pathID(r, "id")
	req, err := decodeBody[strategyReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	// Resolve the board before validating: the per-board config decides which
	// of `allowed` the caller can actually pick, and an error that advertises
	// a strategy the next request will reject just costs a round trip.
	_, board, code, err := h.ticketBoard(r.Context(), id)
	if err != nil {
		h.httpError(w, err, code)
		return
	}
	enabled := make([]string, 0, len(allowed))
	for _, s := range allowed {
		if isAllowed(board.RepoPath, s) {
			enabled = append(enabled, s)
		}
	}
	if len(enabled) == 0 {
		h.httpError(w, fmt.Errorf("every strategy is disabled for this board"), 400)
		return
	}
	explicit := req.Strategy != ""
	if !explicit {
		req.Strategy = defaultFor(board.RepoPath)
		// A default the board disables is worth no more than no default at
		// all, and when there's exactly one way to do it there's nothing for
		// the caller to choose.
		if !slices.Contains(enabled, req.Strategy) && len(enabled) == 1 {
			req.Strategy = enabled[0]
		}
		if req.Strategy == "" {
			h.httpError(w, fmt.Errorf("strategy is required; enabled: %s", joinStrategies(enabled)), 400)
			return
		}
	}
	if !slices.Contains(enabled, req.Strategy) {
		// Three ways to be wrong, and the caller can only act on the right
		// one: their own typo, their own disabled pick, or a configured
		// default that the same config turns off.
		switch {
		case !explicit:
			h.httpError(w, fmt.Errorf("default strategy %s is disabled for this board; enabled: %s",
				req.Strategy, joinStrategies(enabled)), 400)
		case slices.Contains(allowed, req.Strategy):
			h.httpError(w, fmt.Errorf("strategy %s is disabled for this board; enabled: %s",
				req.Strategy, joinStrategies(enabled)), 400)
		default:
			h.httpError(w, fmt.Errorf("strategy must be %s", joinStrategies(enabled)), 400)
		}
		return
	}
	sess, err := h.store.GetSessionByTicket(r.Context(), id)
	if err != nil || sess == nil {
		h.httpError(w, fmt.Errorf("no session for ticket"), 404)
		return
	}
	if err := action(r.Context(), sess.ID, req.Strategy); err != nil {
		h.httpError(w, err, 409)
		return
	}
	if publishOnSuccess {
		h.publishSessionUpdated(r.Context(), sess.ID)
	}
	w.WriteHeader(204)
}

// joinStrategies renders an allowed-strategy slice as English ("a or b",
// "a, b, or c") for the strategy-validation error message.
func joinStrategies(s []string) string {
	switch len(s) {
	case 0:
		return ""
	case 1:
		return s[0]
	case 2:
		return s[0] + " or " + s[1]
	default:
		return strings.Join(s[:len(s)-1], ", ") + ", or " + s[len(s)-1]
	}
}

func (h *handlers) doneTicket(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	t, err := h.store.GetTicket(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	cols, err := h.store.ListColumns(r.Context(), t.BoardID)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	if len(cols) == 0 {
		h.httpError(w, fmt.Errorf("board has no columns"), 409)
		return
	}
	doneCol := cols[len(cols)-1]
	if sess, err := h.store.GetSessionByTicket(r.Context(), id); err == nil && sess != nil {
		if err := h.sessions.Stop(r.Context(), sess.ID); err != nil {
			log.Printf("done: stop session %d: %v", sess.ID, err)
		}
	}
	maxPos, err := h.store.MaxTicketPosition(r.Context(), doneCol.ID)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	if err := h.store.MoveTicket(r.Context(), t.ID, doneCol.ID, maxPos+1); err != nil {
		h.httpError(w, err, 500)
		return
	}
	if updated, _ := h.store.GetTicket(r.Context(), id); updated != nil {
		h.bus.Publish(updated.BoardID, "ticket_moved", updated)
	}
	w.WriteHeader(204)
}

// Sessions

func (h *handlers) ensureSession(w http.ResponseWriter, r *http.Request) {
	ticketID := pathID(r, "id")
	t, board, code, err := h.ticketBoard(r.Context(), ticketID)
	if err != nil {
		h.httpError(w, err, code)
		return
	}
	sess, err := h.sessions.Ensure(r.Context(), board, t)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	h.publishSessionUpdated(r.Context(), sess.ID)
	writeJSON(w, 201, sess)
}

func (h *handlers) startSession(w http.ResponseWriter, r *http.Request) {
	h.sessionStartOrRestart(w, r, h.sessions.Start)
}

func (h *handlers) restartSession(w http.ResponseWriter, r *http.Request) {
	h.sessionStartOrRestart(w, r, h.sessions.Restart)
}

func (h *handlers) sessionStartOrRestart(
	w http.ResponseWriter, r *http.Request,
	action func(context.Context, int64, docker.PullProgressFunc) (*db.Session, error),
) {
	id := pathID(r, "id")
	sess, err := action(r.Context(), id, h.makePullProgressCb(r.Context(), id))
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	h.publishSessionUpdated(r.Context(), sess.ID)
	writeJSON(w, 200, sess)
}

// makePullProgressCb returns a callback that publishes session_pull_progress
// SSE events to the session's board. Returns nil if the session or its ticket
// can't be resolved — Start will surface that as its own error.
func (h *handlers) makePullProgressCb(ctx context.Context, sessionID int64) docker.PullProgressFunc {
	sess, err := h.store.GetSession(ctx, sessionID)
	if err != nil || sess == nil {
		return nil
	}
	t, err := h.store.GetTicket(ctx, sess.TicketID)
	if err != nil || t == nil {
		return nil
	}
	boardID := t.BoardID
	return func(p docker.PullProgress) {
		h.bus.Publish(boardID, "session_pull_progress", map[string]any{
			"session_id": sessionID,
			"image":      p.Image,
			"current":    p.Current,
			"total":      p.Total,
			"layers":     p.Layers,
			"status":     p.Status,
			"done":       p.Done,
		})
	}
}

func (h *handlers) stopSession(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	if err := h.sessions.Stop(r.Context(), id); err != nil {
		h.httpError(w, err, 500)
		return
	}
	h.publishSessionUpdated(r.Context(), id)
	w.WriteHeader(204)
}

// stopSessionForTicket best-effort stops any session attached to ticketID.
// No-op when no session exists; lookup or stop errors are swallowed since
// callers use it as fire-and-forget before archiving.
func (h *handlers) stopSessionForTicket(ctx context.Context, ticketID int64) {
	if sess, err := h.store.GetSessionByTicket(ctx, ticketID); err == nil && sess != nil {
		_ = h.sessions.Stop(ctx, sess.ID)
	}
}

// destroySessionForTicket is the delete-time counterpart of
// stopSessionForTicket: it tears down container + worktree rather than
// just stopping the agent.
func (h *handlers) destroySessionForTicket(ctx context.Context, ticketID int64) {
	if sess, err := h.store.GetSessionByTicket(ctx, ticketID); err == nil && sess != nil {
		_ = h.sessions.Destroy(ctx, sess.ID)
	}
}

// publishTicketArchived emits the "ticket_archived" SSE and fires the
// EventTicketArchived hook in one go — the standard pair after any
// archive-ticket store mutation.
func (h *handlers) publishTicketArchived(t *db.Ticket) {
	h.bus.Publish(t.BoardID, "ticket_archived", t)
	h.hooks.Fire(&t.BoardID, hooks.EventTicketArchived, map[string]string{
		"ticket_id": fmt.Sprintf("%d", t.ID),
	})
}

// publishSessionUpdated emits a "session_updated" event on the board's
// channel. Always refetches the session from the DB before publishing
// so concurrent writes (e.g. the GitHub PR poller writing pr_number /
// pr_url while a handler is mid-mutation) are reflected on the wire.
// Publishing a stale in-memory snapshot is what caused PR fields to
// disappear from the live UI until the page was refreshed.
// Best-effort: silently no-ops if the session or its ticket can't be
// resolved so callers can use it as fire-and-forget.
func (h *handlers) publishSessionUpdated(ctx context.Context, sessionID int64) {
	sess, err := h.store.GetSession(ctx, sessionID)
	if err != nil || sess == nil {
		return
	}
	t, err := h.store.GetTicket(ctx, sess.TicketID)
	if err != nil || t == nil {
		return
	}
	h.bus.Publish(t.BoardID, "session_updated", sess)
}

// sessionSummaryResp is an instance-wide count of running sessions, broken
// down by status. Running is the sum of the four active states; stopped and
// error sessions are excluded.
type sessionSummaryResp struct {
	Running      int `json:"running"`
	Working      int `json:"working"`
	AwaitingPerm int `json:"awaiting_perm"`
	Idle         int `json:"idle"`
	Starting     int `json:"starting"`
}

// sessionSummary returns the count of currently-running containers across all
// boards, grouped by status. The frontend header polls this for an at-a-glance
// indicator, so it stays a single cheap GROUP BY rather than a per-board scan.
func (h *handlers) sessionSummary(w http.ResponseWriter, r *http.Request) {
	counts, err := h.store.CountSessionsByStatus(r.Context())
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	resp := sessionSummaryResp{
		Working:      counts[db.SessionStatusWorking],
		AwaitingPerm: counts[db.SessionStatusAwaitingPerm],
		Idle:         counts[db.SessionStatusIdle],
		Starting:     counts[db.SessionStatusStarting],
	}
	resp.Running = resp.Working + resp.AwaitingPerm + resp.Idle + resp.Starting
	writeJSON(w, 200, resp)
}

type updateSessionStatusReq struct {
	Status string `json:"status"`
}

// updateSessionStatus is called by Claude Code hooks running inside the session
// container to report the active state of the agent (working/idle/awaiting_perm).
// Other statuses (stopped/starting/error) are owned by the session manager and
// rejected here.
func (h *handlers) updateSessionStatus(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	req, err := decodeBody[updateSessionStatusReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	var hookEvent string
	switch req.Status {
	case db.SessionStatusWorking:
		hookEvent = hooks.EventSessionWorking
	case db.SessionStatusIdle:
		hookEvent = hooks.EventSessionIdle
	case db.SessionStatusAwaitingPerm:
		hookEvent = hooks.EventSessionAwaitingPerm
	default:
		h.httpError(w, fmt.Errorf("status must be working, idle, or awaiting_perm"), 400)
		return
	}
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	if err := h.store.UpdateSessionStatus(r.Context(), id, req.Status); err != nil {
		h.httpError(w, err, 500)
		return
	}
	h.publishSessionUpdated(r.Context(), sess.ID)
	if t, _ := h.store.GetTicket(r.Context(), sess.TicketID); t != nil {
		boardID := t.BoardID
		h.hooks.Fire(&boardID, hookEvent, map[string]string{
			"session_id": fmt.Sprintf("%d", sess.ID),
			"ticket_id":  fmt.Sprintf("%d", sess.TicketID),
		})
	}
	// An idle transition means the agent finished a burst of work — the
	// moment its latest commit is worth a preview.
	if req.Status == db.SessionStatusIdle {
		h.maybeAutoDeployPreview(sess)
	}
	w.WriteHeader(204)
}

type updateClaudeSessionIDReq struct {
	ClaudeSessionID string `json:"claude_session_id"`
}

// claudeSessionIDPattern matches the UUID format Claude Code writes for its
// per-conversation transcript filenames (`~/.claude/projects/.../<uuid>.jsonl`).
// Strict-ish: hyphenated 8-4-4-4-12 hex, case-insensitive. Anything else is a
// hook misconfiguration and we reject it so we never persist a value we can't
// later pass to `claude --resume`.
var claudeSessionIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// updateClaudeSessionID is called by the Claude Code SessionStart hook running
// inside the session container to report the UUID of the active conversation.
// Stored so subsequent `claude` launches resume the same transcript across
// container/Kanban restarts. Same trust model as updateSessionStatus — the
// session is identified by URL path, no auth.
func (h *handlers) updateClaudeSessionID(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	req, err := decodeBody[updateClaudeSessionIDReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	uuid := strings.TrimSpace(req.ClaudeSessionID)
	if !claudeSessionIDPattern.MatchString(uuid) {
		h.httpError(w, fmt.Errorf("claude_session_id must be a UUID"), 400)
		return
	}
	if err := h.store.UpdateClaudeSessionID(r.Context(), id, uuid); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			h.httpError(w, err, 404)
			return
		}
		h.httpError(w, err, 500)
		return
	}
	// Push the updated session out over SSE so the InfoPanel's "Claude
	// session" row reflects the UUID without a page reload. Without this,
	// stores seeded from a stale boardState fetch keep displaying the
	// pre-UUID session and the "resume" plumbing looks broken to users.
	h.publishSessionUpdated(r.Context(), id)
	w.WriteHeader(204)
}

type updateSessionBranchReq struct {
	BranchName string `json:"branch_name"`
}

// updateSessionBranch re-points a session at a different git branch. The use
// case is a session that pivots onto a new branch (the agent pushes a new
// branch and opens a PR from it) — re-pointing makes kanban surface that PR's
// GitHub events on the ticket. This is a metadata-only re-point of
// branch_name, not a git rename; kanban never touches GitHub. Because the
// GitHub poller associates sessions to PRs purely by branch name, the DB layer
// clears the cached pr_* fields so the stale old-branch PR disappears and the
// poller repopulates fresh data for the new branch on its next tick.
func (h *handlers) updateSessionBranch(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	req, err := decodeBody[updateSessionBranchReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	branch := strings.TrimSpace(req.BranchName)
	if branch == "" {
		h.httpError(w, fmt.Errorf("branch_name cannot be empty"), 400)
		return
	}
	// Lightweight format guard. We don't require the branch to exist locally or
	// remotely (the agent may not have pushed yet), but reject values git would
	// never accept as a ref name so a typo can't wedge the poller's matching.
	if strings.ContainsAny(branch, " \t") || strings.HasPrefix(branch, "-") || strings.Contains(branch, "..") {
		h.httpError(w, fmt.Errorf("branch_name is not a valid branch name"), 400)
		return
	}
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	// No-op when unchanged: re-saving the same value shouldn't needlessly clear
	// the cached pr_* fields and trigger a poll round-trip to repopulate them.
	if branch == sess.BranchName {
		writeJSON(w, 200, sess)
		return
	}
	if err := h.store.RepointSessionBranch(r.Context(), id, branch); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			h.httpError(w, err, 404)
			return
		}
		h.httpError(w, err, 500)
		return
	}
	// Push the re-pointed session out over SSE so other clients' InfoPanels
	// reflect the new branch (and the now-cleared PR section) without a reload.
	h.publishSessionUpdated(r.Context(), id)
	fresh, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, fresh)
}

// Tasks

type taskInfo struct {
	tasks.VSCodeTask
	ContainerPort int  `json:"container_port,omitempty"`
	HasPort       bool `json:"has_port"`
}

type discoverTasksResp struct {
	Tasks    []taskInfo `json:"tasks"`
	Warnings []string   `json:"warnings"`
}

func (h *handlers) discoverTasks(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	projectRoot := h.sessionProjectRoot(r.Context(), sess)
	found, warnings, _ := tasks.Discover(projectRoot)
	out := make([]taskInfo, 0, len(found))
	for _, t := range found {
		info := taskInfo{VSCodeTask: t}
		if port, ok := tasks.PortFor(sess.WorktreePath, projectRoot, t.Label); ok {
			info.ContainerPort = port
			info.HasPort = true
		}
		out = append(out, info)
	}
	if warnings == nil {
		warnings = []string{}
	}
	writeJSON(w, 200, discoverTasksResp{Tasks: out, Warnings: warnings})
}

func (h *handlers) listTaskRuns(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	runs, err := h.store.ListTaskRuns(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, runs)
}

type createTaskRunReq struct {
	Label string `json:"label"`
}

func (h *handlers) createTaskRun(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	req, err := decodeBody[createTaskRunReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	projectRoot := h.sessionProjectRoot(r.Context(), sess)
	found, _, err := tasks.Discover(projectRoot)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	var task tasks.VSCodeTask
	for _, t := range found {
		if t.Label == req.Label {
			task = t
			break
		}
	}
	if task.Label == "" {
		h.httpError(w, fmt.Errorf("task %q not found", req.Label), 404)
		return
	}
	tr, err := h.tasks.Start(r.Context(), sess, task)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}

	if port, ok := tasks.PortFor(sess.WorktreePath, projectRoot, task.Label); ok {
		_ = h.ensurePortProxy(r.Context(), sess, task.Label, port)
	}

	writeJSON(w, 201, tr)
}

func (h *handlers) stopTaskRun(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	tr, err := h.store.GetTaskRun(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	sess, err := h.store.GetSession(r.Context(), tr.SessionID)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	if err := h.tasks.Stop(r.Context(), sess, tr); err != nil {
		h.httpError(w, err, 500)
		return
	}
	w.WriteHeader(204)
}

func (h *handlers) taskRunOutput(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.httpError(w, fmt.Errorf("streaming unsupported"), 500)
		return
	}
	writeSSEHeaders(w)

	ch, cancel := h.tasks.Subscribe(id)
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case s, ok := <-ch:
			if !ok {
				fmt.Fprintf(w, "event: end\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", strings.TrimRight(s, "\n"))
			flusher.Flush()
		}
	}
}

// Ports

func (h *handlers) listPorts(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	ports, err := h.store.ListPorts(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, ports)
}

type createPortReq struct {
	Label         string `json:"label"`
	ContainerPort int    `json:"container_port"`
}

func (h *handlers) createPort(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	req, err := decodeBody[createPortReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if req.ContainerPort <= 0 {
		h.httpError(w, fmt.Errorf("container_port required"), 400)
		return
	}
	if err := h.ensurePortProxy(r.Context(), sess, req.Label, req.ContainerPort); err != nil {
		h.httpError(w, err, 500)
		return
	}
	ports, _ := h.store.ListPorts(r.Context(), id)
	writeJSON(w, 201, ports)
}

func (h *handlers) deletePort(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	if p, err := h.store.GetPort(r.Context(), id); err == nil && p.ProxyActive {
		h.sessions.Proxies().Close(p.HostPort)
	}
	if err := h.store.DeletePort(r.Context(), id); err != nil {
		h.httpError(w, err, 500)
		return
	}
	w.WriteHeader(204)
}

func (h *handlers) prDetail(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	if sess.PRNumber == nil || *sess.PRNumber == 0 {
		h.httpError(w, fmt.Errorf("session has no pull request"), 404)
		return
	}
	repoPath := sess.RepoPath
	if repoPath == "" {
		board, err := h.boardForSession(r.Context(), sess)
		if err != nil {
			h.httpError(w, err, 500)
			return
		}
		repoPath = board.RepoPath
	}
	if repoPath == "" {
		h.httpError(w, fmt.Errorf("no repo path for session"), 400)
		return
	}
	detail, err := gh.FetchPRDetail(r.Context(), repoPath, *sess.PRNumber)
	if err != nil {
		h.httpError(w, err, 502)
		return
	}
	writeJSON(w, 200, detail)
}

func (h *handlers) ensurePortProxy(ctx context.Context, sess *db.Session, label string, containerPort int) error {
	existing, _ := h.store.ListPorts(ctx, sess.ID)
	for _, p := range existing {
		if p.ContainerPort == containerPort {
			if !p.ProxyActive {
				if err := h.startProxy(ctx, sess, p); err != nil {
					return err
				}
			}
			return nil
		}
	}
	hostPort, err := h.store.AllocateHostPort(ctx, h.config.PortRangeStart, h.config.PortRangeEnd)
	if err != nil {
		return err
	}
	p := &db.PortAllocation{SessionID: sess.ID, Label: label, ContainerPort: containerPort, HostPort: hostPort}
	if err := h.store.CreatePort(ctx, p); err != nil {
		return err
	}
	return h.startProxy(ctx, sess, *p)
}

func (h *handlers) startProxy(ctx context.Context, sess *db.Session, p db.PortAllocation) error {
	if sess.ContainerID == nil || *sess.ContainerID == "" {
		return fmt.Errorf("session not running")
	}
	if err := h.sessions.Proxies().Open(p.HostPort, *sess.ContainerID, p.ContainerPort); err != nil {
		return err
	}
	if err := h.store.SetPortActive(ctx, p.ID, true); err != nil {
		return err
	}
	board, _ := h.boardForSession(ctx, sess)
	var boardID *int64
	if board != nil {
		boardID = &board.ID
	}
	h.hooks.Fire(boardID, hooks.EventPortExposed, map[string]string{
		"session_id":     fmt.Sprintf("%d", sess.ID),
		"label":          p.Label,
		"container_port": fmt.Sprintf("%d", p.ContainerPort),
		"host_port":      fmt.Sprintf("%d", p.HostPort),
	})
	return nil
}

func (h *handlers) boardForSession(ctx context.Context, sess *db.Session) (*db.Board, error) {
	_, board, _, err := h.ticketBoard(ctx, sess.TicketID)
	return board, err
}

// sessionProjectRoot is the host directory the agent actually works in: the
// worktree root descended into the board's project_dir. Host-side discovery
// (tasks, plans) has to agree with the container's working directory or it
// reads the wrong tree. Falls back to the worktree root when the board can't
// be loaded, which is what every caller did before project_dir existed.
func (h *handlers) sessionProjectRoot(ctx context.Context, sess *db.Session) string {
	board, err := h.boardForSession(ctx, sess)
	if err != nil {
		return sess.WorktreePath
	}
	return session.ResolvePaths(board, sess).ProjectRoot(sess.WorktreePath)
}

// ticketBoard fetches a ticket and its parent board in one go. The returned
// status is the HTTP code callers should pass to httpError when err != nil:
// 404 if the ticket lookup failed (typically ErrNotFound), 500 otherwise.
func (h *handlers) ticketBoard(ctx context.Context, id int64) (*db.Ticket, *db.Board, int, error) {
	t, err := h.store.GetTicket(ctx, id)
	if err != nil {
		return nil, nil, 404, err
	}
	board, err := h.store.GetBoard(ctx, t.BoardID)
	if err != nil {
		return nil, nil, 500, err
	}
	return t, board, 0, nil
}

// PTY

func (h *handlers) wsPTY(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	repoPath := ""
	if board, err := h.boardForSession(r.Context(), sess); err == nil && board != nil {
		repoPath = board.RepoPath
	}
	resolved := harness.ForSession(sess.Harness, repoPath)
	cmd := resolved.PTYCommand
	// Resume the prior Claude Code conversation when we have its UUID. The
	// SessionStart hook in .claude/settings.local.json captures the UUID on
	// every launch, so this preserves the conversation across container
	// restarts. Other harnesses don't have an equivalent flag, so this is
	// gated on the resolved harness ID.
	//
	// SessionStart fires the moment `claude` boots — before any prompt is
	// submitted and before claude flushes a transcript file. If the user
	// restarts the container in that window, the DB holds a UUID that has
	// no matching `~/.claude/projects/*/<uuid>.jsonl`, and `claude --resume`
	// bombs out with "No conversation found with session ID". Verify the
	// transcript exists on disk before resuming; clear the dead UUID so the
	// UI stops advertising it and the next launch starts cleanly.
	if resolved.ID == "claude" && sess.ClaudeSessionID != "" {
		if claudeTranscriptExists(sess.ClaudeSessionID) {
			cmd = append(append([]string(nil), cmd...), "--resume", sess.ClaudeSessionID)
		} else {
			if err := h.store.UpdateClaudeSessionID(r.Context(), id, ""); err != nil {
				log.Printf("clear stale claude_session_id for session %d: %v", id, err)
			} else {
				h.publishSessionUpdated(r.Context(), id)
			}
		}
	}
	if err := h.sessions.AttachAgent(r.Context(), sess, w, r, resolved.ID, cmd, sess.WorkspaceDir()); err != nil {
		h.reconcileAfterAttach(r.Context(), sess)
	}
}

// reconcileAfterAttach runs when attaching to a session's container failed.
// The usual cause is a container that died while the row still claims a live
// session; Reconcile flips such rows to stopped, and the board is told so its
// cards stop advertising a session that isn't there.
func (h *handlers) reconcileAfterAttach(ctx context.Context, sess *db.Session) {
	fresh, err := h.sessions.Reconcile(ctx, sess)
	if err != nil {
		log.Printf("reconcile session %d after attach failure: %v", sess.ID, err)
		return
	}
	if fresh.Status != sess.Status {
		h.publishSessionUpdated(ctx, sess.ID)
	}
}

// claudeTranscriptExists reports whether Claude Code has a persisted JSONL
// transcript for the given session UUID on the host's `~/.claude/projects/`
// tree (bind-mounted into the container by ClaudeConfigMounts). The
// project subdirectory name encodes the cwd Claude saw at launch, which
// from inside the session container is just `/workspace` — but rather
// than couple to that encoding, glob across all project directories
// since the UUID is unique. Returns false on any I/O error so callers
// fall back to launching `claude` fresh.
func claudeTranscriptExists(uuid string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", uuid+".jsonl"))
	return len(matches) > 0
}

func (h *handlers) wsShell(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	if err := h.sessions.AttachShell(r.Context(), sess, w, r, sess.WorkspaceDir()); err != nil {
		h.reconcileAfterAttach(r.Context(), sess)
	}
}

// Settings — backed by the user-level config file at
// $XDG_CONFIG_HOME/kanban/config.toml. Empty values mean "no user override".

type settingsResp struct {
	Harness               string `json:"harness"`
	WorktreesRoot         string `json:"worktrees_root"`
	WorktreesRootResolved string `json:"worktrees_root_resolved"`
	WorktreesRootLocked   bool   `json:"worktrees_root_locked"`
	SignCommits           bool   `json:"sign_commits"`
}

func (h *handlers) settingsResponse() settingsResp {
	id, _ := harness.ReadUserHarness()
	return settingsResp{
		Harness:               id,
		WorktreesRoot:         readUserWorktreesRoot(),
		WorktreesRootResolved: h.config.WorktreesDir(),
		WorktreesRootLocked:   h.config.HasWorktreesDirOverride(),
		SignCommits:           readUserSignCommits(),
	}
}

// readUserSignCommits reports whether the user opted kanban into commit signing
// ([git].sign_commits in the user config). Defaults false.
func readUserSignCommits() bool {
	g := kanbantoml.Load("").Git
	return g != nil && g.SignCommits != nil && *g.SignCommits
}

func (h *handlers) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.settingsResponse())
}

type updateSettingsReq struct {
	Harness       *string `json:"harness"`
	WorktreesRoot *string `json:"worktrees_root"`
	SignCommits   *bool   `json:"sign_commits"`
}

func (h *handlers) updateSettings(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBody[updateSettingsReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if req.Harness != nil {
		id := *req.Harness
		if id != "" && !harness.IsKnown(id) {
			h.httpError(w, fmt.Errorf("unknown harness %q", id), 400)
			return
		}
		if err := harness.WriteUserHarness(id); err != nil {
			h.httpError(w, err, 500)
			return
		}
	}
	if req.WorktreesRoot != nil {
		if h.config.HasWorktreesDirOverride() {
			h.httpError(w, fmt.Errorf("worktrees root is locked by --worktrees-dir or $KANBAN_WORKTREES_DIR"), http.StatusConflict)
			return
		}
		root := strings.TrimSpace(*req.WorktreesRoot)
		if err := kanbantoml.WriteUserWorktreesRoot(root); err != nil {
			h.httpError(w, err, 500)
			return
		}
	}
	if req.SignCommits != nil {
		if err := kanbantoml.WriteUserSignCommits(*req.SignCommits); err != nil {
			h.httpError(w, err, 500)
			return
		}
		git.SetCommitSigning(*req.SignCommits)
	}
	writeJSON(w, 200, h.settingsResponse())
}

func readUserWorktreesRoot() string {
	f := kanbantoml.Load("")
	if f.Worktrees == nil || f.Worktrees.Root == nil {
		return ""
	}
	return *f.Worktrees.Root
}

// harnessEntry is one row of GET /api/harnesses. Default marks the harness
// a session without its own choice launches for the requested board.
type harnessEntry struct {
	harness.Harness
	Default bool `json:"default,omitempty"`
}

// listHarnesses returns the harness registry. With ?board=<id> the entry the
// board's sessions fall back to (user config, then the repo's .kanban.toml,
// then the built-in default) is flagged "default": true.
func (h *handlers) listHarnesses(w http.ResponseWriter, r *http.Request) {
	defaultID := ""
	if raw := r.URL.Query().Get("board"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			h.httpError(w, fmt.Errorf("board must be a numeric id"), 400)
			return
		}
		board, err := h.store.GetBoard(r.Context(), id)
		if err != nil {
			h.httpError(w, err, 404)
			return
		}
		defaultID = harness.Resolve(board.RepoPath).ID
	}
	out := make([]harnessEntry, len(harness.Registry))
	for i, hr := range harness.Registry {
		out[i] = harnessEntry{Harness: hr, Default: hr.ID == defaultID}
	}
	writeJSON(w, 200, out)
}

type updateSessionHarnessReq struct {
	Harness string `json:"harness"`
}

// updateSessionHarness picks the agent harness for one session ("" returns
// it to the user/project default). If the session's agent is running a
// different harness than the one it now resolves to, that agent is stopped
// so the next attach starts the new harness; the container, worktree and
// shell stay up.
func (h *handlers) updateSessionHarness(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	req, err := decodeBody[updateSessionHarnessReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	next := strings.TrimSpace(req.Harness)
	if next != "" && !harness.IsKnown(next) {
		h.httpError(w, fmt.Errorf("unknown harness %q", next), 400)
		return
	}
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	repoPath := ""
	if board, err := h.boardForSession(r.Context(), sess); err == nil && board != nil {
		repoPath = board.RepoPath
	}
	changed := next != sess.Harness
	if changed {
		if err := h.store.UpdateSessionHarness(r.Context(), id, next); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				h.httpError(w, err, 404)
				return
			}
			h.httpError(w, err, 500)
			return
		}
	}
	// Compare against the harness the running agent was launched with, not
	// the session's previous setting: with no explicit pick, the default it
	// launched under may have changed since.
	stopped := h.sessions.StopAgentUnless(r.Context(), id, harness.ForSession(next, repoPath).ID)
	if changed || stopped {
		h.publishSessionUpdated(r.Context(), id)
	}
	fresh, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, fresh)
}

// helpers

func pathID(r *http.Request, name string) int64 {
	v := r.PathValue(name)
	id, _ := strconv.ParseInt(v, 10, 64)
	return id
}

// decodeBody is the single funnel for JSON request bodies. Centralizing it
// keeps any future hardening (size limits, content-type checks, strict
// field validation) in one place rather than 12 handlers.
func decodeBody[T any](r *http.Request) (T, error) {
	var v T
	err := json.NewDecoder(r.Body).Decode(&v)
	return v, err
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeSSEHeaders sets the standard Server-Sent Events response headers.
// Call before the first Flush on any SSE handler.
func writeSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}

// httpError writes a JSON error response and, for 5xx codes, files a kanban
// ticket via the reporter (a no-op when the reporter is disabled or nil).
// 4xx codes are user/client errors and are not reported.
//
// 502 Bad Gateway is the one 5xx we skip: it means an upstream dependency (the
// GitHub API) was unreachable — a transient outage or the operator being
// offline, not a server bug. Reporting it just spams the errors board with a
// recurring ticket every time GitHub hiccups.
func (h *handlers) httpError(w http.ResponseWriter, err error, code int) {
	// Client disconnected mid-request — nobody to respond to, and not a
	// server bug worth reporting.
	if errors.Is(err, context.Canceled) {
		return
	}
	log.Printf("http %d: %v", code, err)
	writeJSON(w, code, map[string]string{"error": err.Error()})
	if code >= 500 && code != http.StatusBadGateway && h.reporter != nil {
		h.reporter.Capture(context.Background(), "http", err)
	}
}

type reportFrontendErrorReq struct {
	Message   string            `json:"message"`
	Stack     string            `json:"stack"`
	Source    string            `json:"source"`
	URL       string            `json:"url"`
	UserAgent string            `json:"user_agent"`
	Meta      map[string]string `json:"meta"`
}

// reportFrontendError accepts error reports from the React app's
// ErrorBoundary, global React Query onError, window error/unhandledrejection
// listeners, and the long-task observer. Always returns 204.
//
// Every accepted report is also logged to the server's default logger so it
// shows up in `docker logs` regardless of whether the errreport ticket
// reporter is enabled — that's the path operators rely on to see UI errors
// without having to ask the user to open DevTools.
func (h *handlers) reportFrontendError(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBody[reportFrontendErrorReq](r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req.Message != "" {
		source := strings.TrimSpace(req.Source)
		if source == "" {
			source = "frontend"
		}
		logClientError(source, req)
		if h.reporter != nil {
			meta := map[string]string{
				"url":        req.URL,
				"user_agent": req.UserAgent,
			}
			for k, v := range req.Meta {
				if _, taken := meta[k]; taken {
					continue
				}
				meta[k] = v
			}
			h.reporter.Report(r.Context(), source, req.Message, req.Stack, meta)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// Caps applied before logging so a malicious or runaway client can't fill the
// container's log volume with a single multi-megabyte payload.
const (
	clientLogMessageMax = 1024
	clientLogStackMax   = 8192
	clientLogFieldMax   = 512
)

// logClientError writes a single-line summary plus the (truncated) stack to
// the default logger. Free-text fields are emitted via %q so embedded
// newlines/quotes can't break the one-line-per-event grep contract.
func logClientError(source string, req reportFrontendErrorReq) {
	msg := truncate(req.Message, clientLogMessageMax)
	url := truncate(req.URL, clientLogFieldMax)
	ua := truncate(req.UserAgent, clientLogFieldMax)
	stack := truncate(req.Stack, clientLogStackMax)
	if stack == "" {
		log.Printf("client error: source=%s url=%s ua=%q message=%q", source, url, ua, msg)
		return
	}
	log.Printf("client error: source=%s url=%s ua=%q message=%q\n%s", source, url, ua, msg, stack)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}

// fsCheck reports whether a host path is visible to the kanban server and, if
// so, whether it looks like a git repo. Paths outside the kanban container's
// mounts return "unknown" — they may still be valid host paths that dockerd
// can mount; we just can't see them from here.
func (h *handlers) fsCheck(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		h.httpError(w, fmt.Errorf("path required"), 400)
		return
	}
	state := "unknown"
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			if g, err := os.Stat(filepath.Join(path, ".git")); err == nil && (g.IsDir() || g.Mode().IsRegular()) {
				state = "git"
			} else {
				state = "not_git"
			}
		} else {
			state = "not_git"
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": state})
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// Plans
//
// resolvePlansDir gives a session-scoped absolute path for the configured
// plans directory. An absolute config value (the default `~/.claude/plans`
// post-expansion) is shared globally; a relative one (e.g. `./plans`) is
// joined onto the agent's working directory so each ticket gets its own
// plans — and so a monorepo board finds the plans the agent wrote from
// inside its project_dir rather than looking at the repo root.
func (h *handlers) resolvePlansDir(ctx context.Context, sess *db.Session) string {
	dir := h.config.PlansDir()
	if sess.WorktreePath == "" {
		return dir
	}
	return plansDirUnder(dir, h.sessionProjectRoot(ctx, sess))
}

// plansDirUnder joins a relative plans dir onto root, leaving an absolute one
// (and an empty root) alone.
func plansDirUnder(configured, root string) string {
	if filepath.IsAbs(configured) || root == "" {
		return configured
	}
	return filepath.Join(root, configured)
}

type planMeta struct {
	Name    string    `json:"name"`
	ModTime time.Time `json:"mod_time"`
	Size    int64     `json:"size"`
}

// listSessionPlans returns the markdown files in the session's plans
// directory. A missing directory is treated as "no plans" and returns an
// empty list — the frontend uses an empty response as its "hide the tab"
// signal, so 404-on-missing-dir would be hostile.
func (h *handlers) listSessionPlans(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	dir := h.resolvePlansDir(r.Context(), sess)
	plans := []planMeta{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, 200, plans)
			return
		}
		h.httpError(w, err, 500)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		plans = append(plans, planMeta{
			Name:    e.Name(),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}
	writeJSON(w, 200, plans)
}

// getSessionPlan returns one plan file's raw markdown bytes. Rejects
// names containing path separators or `..` so the handler can never
// escape resolvePlansDir.
func (h *handlers) getSessionPlan(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	name := r.PathValue("name")
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		h.httpError(w, fmt.Errorf("invalid plan name"), 400)
		return
	}
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	full := filepath.Join(h.resolvePlansDir(r.Context(), sess), name)
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			h.httpError(w, fmt.Errorf("plan not found"), 404)
			return
		}
		h.httpError(w, err, 500)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}

type sessionDiff struct {
	Base  string `json:"base"`
	Patch string `json:"patch"`
}

// getSessionDiff returns a unified diff of the session worktree against the
// point where its branch diverged from the board's base branch (committed plus
// uncommitted tracked changes — see git.DiffAgainstBase). A session without a
// worktree returns an empty patch rather than an error so the frontend can
// render a "no changes" state.
func (h *handlers) getSessionDiff(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	if sess.WorktreePath == "" {
		writeJSON(w, 200, sessionDiff{})
		return
	}
	board, err := h.boardForSession(r.Context(), sess)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	patch, err := git.DiffAgainstBase(sess.WorktreePath, board.BaseBranch)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, sessionDiff{Base: board.BaseBranch, Patch: patch})
}

type sessionFile struct {
	Path     string `json:"path"`
	Contents string `json:"contents"`
}

// getSessionFile returns the current working-tree contents of a single file in
// the session worktree — the "new" side of GET /api/sessions/{id}/diff — so the
// diff viewer can show a whole-file "View file" view (like GitHub's) instead of
// just the hunks. The `path` query parameter must name a file inside the
// worktree; anything missing or outside it returns 404.
func (h *handlers) getSessionFile(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	path := r.URL.Query().Get("path")
	if path == "" {
		h.httpError(w, fmt.Errorf("path query parameter is required"), 400)
		return
	}
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	if sess.WorktreePath == "" {
		h.httpError(w, fmt.Errorf("session has no worktree"), 404)
		return
	}
	contents, err := git.ReadWorktreeFile(sess.WorktreePath, path)
	if err != nil {
		h.httpError(w, fmt.Errorf("file not found: %s", path), 404)
		return
	}
	writeJSON(w, 200, sessionFile{Path: path, Contents: contents})
}

type sessionFileDiff struct {
	Path        string `json:"path"`
	OldContents string `json:"old_contents"`
	NewContents string `json:"new_contents"`
}

// getSessionFileDiff returns both sides of a single changed file — the base
// version (from the merge-base the patch is computed against) and the current
// working-tree version — so the diff viewer can render expandable surrounding
// context (GitHub's "expand up/down") instead of just the patch's few context
// lines. `path` names the current file; `old_path` is the pre-rename path when
// it differs (the diff's `prevName`), defaulting to `path`. A side that doesn't
// exist at its ref comes back empty — an empty `old_contents` means a new file,
// an empty `new_contents` a deleted one. Only when *both* are empty (the path is
// in neither tree) is it a 404.
func (h *handlers) getSessionFileDiff(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	path := r.URL.Query().Get("path")
	if path == "" {
		h.httpError(w, fmt.Errorf("path query parameter is required"), 400)
		return
	}
	oldPath := r.URL.Query().Get("old_path")
	if oldPath == "" {
		oldPath = path
	}
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	if sess.WorktreePath == "" {
		h.httpError(w, fmt.Errorf("session has no worktree"), 404)
		return
	}
	board, err := h.boardForSession(r.Context(), sess)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}

	// A missing side is expected (new/deleted file) and maps to empty contents;
	// only a path present in neither tree is a genuine 404.
	newContents, newErr := git.ReadWorktreeFile(sess.WorktreePath, path)
	oldContents, oldErr := git.ReadBaseFile(sess.WorktreePath, board.BaseBranch, oldPath)
	if newErr != nil && oldErr != nil {
		h.httpError(w, fmt.Errorf("file not found: %s", path), 404)
		return
	}
	writeJSON(w, 200, sessionFileDiff{Path: path, OldContents: oldContents, NewContents: newContents})
}
