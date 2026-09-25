// Package client is a thin HTTP client for the kanban REST API. It's used
// by both the MCP server (internal/mcp) and the kanban CLI subcommands
// (cmd/server) so the request shapes stay in one place.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Client wraps an *http.Client and a base URL.
type Client struct {
	baseURL string
	hc      *http.Client
}

// New returns a Client. If hc is nil, http.DefaultClient is used.
func New(baseURL string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), hc: hc}
}

// Board mirrors the subset of fields callers need from /api/boards.
type Board struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	RepoPath string `json:"repo_path"`
	// ProjectDir is the repo-relative subdirectory the board is scoped to, ""
	// for a whole-repo board. Board auto-detection uses it to pick between
	// several boards sharing one monorepo repo_path.
	ProjectDir string `json:"project_dir"`
}

// Ticket mirrors the subset of fields callers need from ticket responses.
type Ticket struct {
	ID         int64  `json:"id"`
	BoardID    int64  `json:"board_id"`
	ColumnID   int64  `json:"column_id"`
	Title      string `json:"title"`
	Slug       string `json:"slug"`
	Body       string `json:"body"`
	Position   int    `json:"position"`
	CreatedAt  int64  `json:"created_at"`
	ArchivedAt *int64 `json:"archived_at,omitempty"`
}

// Session mirrors the subset of fields callers need from session responses.
type Session struct {
	ID              int64   `json:"id"`
	TicketID        int64   `json:"ticket_id"`
	WorktreePath    string  `json:"worktree_path"`
	BranchName      string  `json:"branch_name"`
	ContainerID     *string `json:"container_id,omitempty"`
	ContainerName   *string `json:"container_name,omitempty"`
	Status          string  `json:"status"`
	StartedAt       *int64  `json:"started_at,omitempty"`
	StoppedAt       *int64  `json:"stopped_at,omitempty"`
	PRState         string  `json:"pr_state,omitempty"`
	PRNumber        *int64  `json:"pr_number,omitempty"`
	PRURL           string  `json:"pr_url,omitempty"`
	PRTitle         string  `json:"pr_title,omitempty"`
	MountPath       string  `json:"mount_path,omitempty"`
	RepoPath        string  `json:"repo_path,omitempty"`
	ClaudeSessionID string  `json:"claude_session_id,omitempty"`
	Harness         string  `json:"harness,omitempty"`
}

// Harness mirrors one entry of GET /api/harnesses. Default is set on the
// harness a board's sessions fall back to when the listing is scoped to a
// board.
type Harness struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Default bool   `json:"default,omitempty"`
}

// Port mirrors one entry of GET /api/sessions/{id}/ports.
type Port struct {
	ID            int64  `json:"id"`
	SessionID     int64  `json:"session_id"`
	Label         string `json:"label"`
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	ProxyActive   bool   `json:"proxy_active"`
}

// Task mirrors one entry of GET /api/sessions/{id}/discover-tasks: a
// .vscode/tasks.json or launch.json entry, plus the container port
// .kanban.toml maps its label to (HasPort false when none).
type Task struct {
	Label         string   `json:"label"`
	Command       string   `json:"command"`
	Args          []string `json:"args,omitempty"`
	Cwd           string   `json:"cwd,omitempty"`
	ContainerPort int      `json:"container_port,omitempty"`
	HasPort       bool     `json:"has_port"`
}

// TaskRun mirrors one execution of a task in a session container.
type TaskRun struct {
	ID        int64  `json:"id"`
	SessionID int64  `json:"session_id"`
	TaskLabel string `json:"task_label"`
	Command   string `json:"command"`
	Status    string `json:"status"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	StartedAt int64  `json:"started_at"`
	StoppedAt *int64 `json:"stopped_at,omitempty"`
}

// CreateBoardArgs is the request body for POST /api/boards. Name plus one of
// repo_path or mount_path are required by the server; everything else is
// optional and falls back to server defaults.
type CreateBoardArgs struct {
	Name           string `json:"name"`
	RepoPath       string `json:"repo_path,omitempty"`
	MountPath      string `json:"mount_path,omitempty"`
	ProjectDir     string `json:"project_dir,omitempty"`
	WorktreeRoot   string `json:"worktree_root,omitempty"`
	BaseBranch     string `json:"base_branch,omitempty"`
	BranchPrefix   string `json:"branch_prefix,omitempty"`
	GitAuthorName  string `json:"git_author_name,omitempty"`
	GitAuthorEmail string `json:"git_author_email,omitempty"`
}

// UpdateBoardArgs is the request body for PATCH /api/boards/{id}. Pointer
// fields signal "include in patch"; nil leaves the value untouched.
type UpdateBoardArgs struct {
	Name           *string `json:"name,omitempty"`
	RepoPath       *string `json:"repo_path,omitempty"`
	MountPath      *string `json:"mount_path,omitempty"`
	ProjectDir     *string `json:"project_dir,omitempty"`
	WorktreeRoot   *string `json:"worktree_root,omitempty"`
	BaseBranch     *string `json:"base_branch,omitempty"`
	BranchPrefix   *string `json:"branch_prefix,omitempty"`
	GitAuthorName  *string `json:"git_author_name,omitempty"`
	GitAuthorEmail *string `json:"git_author_email,omitempty"`
}

// CreateTicketArgs is the input shape for CreateTicket. Only Board and Title
// are required; the server defaults Column to the leftmost column.
type CreateTicketArgs struct {
	Board  string `json:"-"`
	Title  string `json:"title"`
	Body   string `json:"body,omitempty"`
	Column string `json:"column,omitempty"`
}

// UpdateTicketArgs is the request body for PATCH /api/tickets/{id}.
type UpdateTicketArgs struct {
	Title *string `json:"title,omitempty"`
	Body  *string `json:"body,omitempty"`
}

// MoveTicketArgs is the request body for PATCH /api/tickets/{id}/move.
type MoveTicketArgs struct {
	ColumnID int64 `json:"column_id"`
	Position int   `json:"position"`
}

// ListBoards calls GET /api/boards.
func (c *Client) ListBoards(ctx context.Context) ([]Board, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/boards", nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var rawBoards []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawBoards); err != nil {
		return nil, err
	}
	out := make([]Board, 0, len(rawBoards))
	for _, b := range rawBoards {
		var s Board
		_ = json.Unmarshal(b["id"], &s.ID)
		_ = json.Unmarshal(b["name"], &s.Name)
		_ = json.Unmarshal(b["slug"], &s.Slug)
		_ = json.Unmarshal(b["repo_path"], &s.RepoPath)
		_ = json.Unmarshal(b["project_dir"], &s.ProjectDir)
		out = append(out, s)
	}
	return out, nil
}

// CreateBoard calls POST /api/boards and returns the raw board JSON.
func (c *Client) CreateBoard(ctx context.Context, a CreateBoardArgs) (json.RawMessage, error) {
	if strings.TrimSpace(a.Name) == "" {
		return nil, fmt.Errorf("name required")
	}
	if a.RepoPath == "" && a.MountPath == "" {
		return nil, fmt.Errorf("repo-path or mount-path required")
	}
	return c.do(ctx, http.MethodPost, "/api/boards", a, http.StatusCreated)
}

// GetBoard calls GET /api/boards/{id}.
func (c *Client) GetBoard(ctx context.Context, id int64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/boards/"+strconv.FormatInt(id, 10), nil, http.StatusOK)
}

// UpdateBoard calls PATCH /api/boards/{id}.
func (c *Client) UpdateBoard(ctx context.Context, id int64, a UpdateBoardArgs) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPatch, "/api/boards/"+strconv.FormatInt(id, 10), a, http.StatusOK)
}

// DeleteBoard calls DELETE /api/boards/{id}.
func (c *Client) DeleteBoard(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/boards/"+strconv.FormatInt(id, 10), nil, http.StatusNoContent)
	return err
}

// PatchBoardEnvArgs is the request body for PATCH /api/boards/{id}/env.
// Values are write-only: responses only ever carry key names.
type PatchBoardEnvArgs struct {
	Set   map[string]string `json:"set,omitempty"`
	Unset []string          `json:"unset,omitempty"`
}

// ListBoardEnv calls GET /api/boards/{id}/env. The response contains env var
// key names only, never values.
func (c *Client) ListBoardEnv(ctx context.Context, id int64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/boards/"+strconv.FormatInt(id, 10)+"/env", nil, http.StatusOK)
}

// PatchBoardEnv calls PATCH /api/boards/{id}/env to set and/or unset board
// env vars. Returns the updated key list.
func (c *Client) PatchBoardEnv(ctx context.Context, id int64, a PatchBoardEnvArgs) (json.RawMessage, error) {
	if len(a.Set) == 0 && len(a.Unset) == 0 {
		return nil, fmt.Errorf("nothing to set or unset")
	}
	return c.do(ctx, http.MethodPatch, "/api/boards/"+strconv.FormatInt(id, 10)+"/env", a, http.StatusOK)
}

// BoardState calls GET /api/boards/{id}/state.
func (c *Client) BoardState(ctx context.Context, id int64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/boards/"+strconv.FormatInt(id, 10)+"/state", nil, http.StatusOK)
}

// ListArchived calls GET /api/boards/{id}/archived.
func (c *Client) ListArchived(ctx context.Context, id int64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/boards/"+strconv.FormatInt(id, 10)+"/archived", nil, http.StatusOK)
}

// DeleteArchived calls DELETE /api/boards/{id}/archived.
func (c *Client) DeleteArchived(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/boards/"+strconv.FormatInt(id, 10)+"/archived", nil, http.StatusNoContent)
	return err
}

// CreateTicket calls POST /api/boards/{board}/tickets and returns the raw
// JSON the server responded with. Callers that want a typed value can
// re-decode; we keep the raw form here so both MCP (which forwards to the
// agent) and CLI (which re-prints) can use it.
func (c *Client) CreateTicket(ctx context.Context, a CreateTicketArgs) (json.RawMessage, error) {
	if a.Board == "" {
		return nil, fmt.Errorf("board required")
	}
	if a.Title == "" {
		return nil, fmt.Errorf("title required")
	}
	body := map[string]any{"title": a.Title}
	if a.Body != "" {
		body["body"] = a.Body
	}
	if a.Column != "" {
		body["column"] = a.Column
	}
	return c.do(ctx, http.MethodPost, "/api/boards/"+url.PathEscape(a.Board)+"/tickets", body, http.StatusCreated)
}

// GetTicket calls GET /api/tickets/{id}. It's the only ticket lookup that
// needs no board context, so callers holding a bare ticket id can find the
// board it belongs to.
func (c *Client) GetTicket(ctx context.Context, id int64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/tickets/"+strconv.FormatInt(id, 10), nil, http.StatusOK)
}

// UpdateTicket calls PATCH /api/tickets/{id}.
func (c *Client) UpdateTicket(ctx context.Context, id int64, a UpdateTicketArgs) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPatch, "/api/tickets/"+strconv.FormatInt(id, 10), a, http.StatusOK)
}

// MoveTicket calls PATCH /api/tickets/{id}/move.
func (c *Client) MoveTicket(ctx context.Context, id int64, a MoveTicketArgs) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/tickets/"+strconv.FormatInt(id, 10)+"/move", a, http.StatusNoContent)
	return err
}

// ArchiveTicket calls POST /api/tickets/{id}/archive.
func (c *Client) ArchiveTicket(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodPost, "/api/tickets/"+strconv.FormatInt(id, 10)+"/archive", nil, http.StatusNoContent)
	return err
}

// UnarchiveTicket calls POST /api/tickets/{id}/unarchive.
func (c *Client) UnarchiveTicket(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodPost, "/api/tickets/"+strconv.FormatInt(id, 10)+"/unarchive", nil, http.StatusNoContent)
	return err
}

// DeleteTicket calls DELETE /api/tickets/{id}. The server requires the
// ticket to be archived first; callers should propagate the resulting error.
func (c *Client) DeleteTicket(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/tickets/"+strconv.FormatInt(id, 10), nil, http.StatusNoContent)
	return err
}

// DoneTicket calls POST /api/tickets/{id}/done.
func (c *Client) DoneTicket(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodPost, "/api/tickets/"+strconv.FormatInt(id, 10)+"/done", nil, http.StatusNoContent)
	return err
}

// SyncTicket calls POST /api/tickets/{id}/sync. Strategy is "rebase" or "merge".
func (c *Client) SyncTicket(ctx context.Context, id int64, strategy string) error {
	_, err := c.do(ctx, http.MethodPost, "/api/tickets/"+strconv.FormatInt(id, 10)+"/sync",
		map[string]string{"strategy": strategy}, http.StatusNoContent)
	return err
}

// MergeTicket calls POST /api/tickets/{id}/merge. Strategy is one of
// "merge-commit", "squash", or "rebase"; empty lets the server resolve the
// board's configured default.
func (c *Client) MergeTicket(ctx context.Context, id int64, strategy string) error {
	_, err := c.do(ctx, http.MethodPost, "/api/tickets/"+strconv.FormatInt(id, 10)+"/merge",
		map[string]string{"strategy": strategy}, http.StatusNoContent)
	return err
}

// ArchiveColumnTickets calls POST /api/columns/{id}/archive-all.
func (c *Client) ArchiveColumnTickets(ctx context.Context, columnID int64) error {
	_, err := c.do(ctx, http.MethodPost, "/api/columns/"+strconv.FormatInt(columnID, 10)+"/archive-all", nil, http.StatusNoContent)
	return err
}

// EnsureSession calls POST /api/tickets/{id}/session and returns the raw
// session JSON.
func (c *Client) EnsureSession(ctx context.Context, ticketID int64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/tickets/"+strconv.FormatInt(ticketID, 10)+"/session", nil, http.StatusCreated)
}

// StartSession calls POST /api/sessions/{id}/start.
func (c *Client) StartSession(ctx context.Context, id int64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/sessions/"+strconv.FormatInt(id, 10)+"/start", nil, http.StatusOK)
}

// StopSession calls POST /api/sessions/{id}/stop.
func (c *Client) StopSession(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodPost, "/api/sessions/"+strconv.FormatInt(id, 10)+"/stop", nil, http.StatusNoContent)
	return err
}

// RestartSession calls POST /api/sessions/{id}/restart.
func (c *Client) RestartSession(ctx context.Context, id int64) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/sessions/"+strconv.FormatInt(id, 10)+"/restart", nil, http.StatusOK)
}

// ListPorts calls GET /api/sessions/{id}/ports.
func (c *Client) ListPorts(ctx context.Context, sessionID int64) ([]Port, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/sessions/"+strconv.FormatInt(sessionID, 10)+"/ports", nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var ports []Port
	if err := json.Unmarshal(raw, &ports); err != nil {
		return nil, err
	}
	return ports, nil
}

// CreatePort calls POST /api/sessions/{id}/ports, allocating a host port for
// containerPort (or reusing the existing allocation) and opening its proxy.
// It returns the session's ports afterwards.
func (c *Client) CreatePort(ctx context.Context, sessionID int64, label string, containerPort int) ([]Port, error) {
	raw, err := c.do(ctx, http.MethodPost, "/api/sessions/"+strconv.FormatInt(sessionID, 10)+"/ports",
		map[string]any{"label": label, "container_port": containerPort}, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	var ports []Port
	if err := json.Unmarshal(raw, &ports); err != nil {
		return nil, err
	}
	return ports, nil
}

// DiscoverTasks calls GET /api/sessions/{id}/discover-tasks. The warnings
// describe task files that failed to parse.
func (c *Client) DiscoverTasks(ctx context.Context, sessionID int64) ([]Task, []string, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/sessions/"+strconv.FormatInt(sessionID, 10)+"/discover-tasks", nil, http.StatusOK)
	if err != nil {
		return nil, nil, err
	}
	var resp struct {
		Tasks    []Task   `json:"tasks"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, err
	}
	return resp.Tasks, resp.Warnings, nil
}

// ListTaskRuns calls GET /api/sessions/{id}/task-runs (newest first).
func (c *Client) ListTaskRuns(ctx context.Context, sessionID int64) ([]TaskRun, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/sessions/"+strconv.FormatInt(sessionID, 10)+"/task-runs", nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var runs []TaskRun
	if err := json.Unmarshal(raw, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// StartTaskRun calls POST /api/sessions/{id}/task-runs to run the task with
// the given label in the session's container.
func (c *Client) StartTaskRun(ctx context.Context, sessionID int64, label string) (TaskRun, error) {
	var tr TaskRun
	raw, err := c.do(ctx, http.MethodPost, "/api/sessions/"+strconv.FormatInt(sessionID, 10)+"/task-runs",
		map[string]string{"label": label}, http.StatusCreated)
	if err != nil {
		return tr, err
	}
	err = json.Unmarshal(raw, &tr)
	return tr, err
}

// StopTaskRun calls DELETE /api/task-runs/{id}, signalling the run's
// process tree. The run is marked exited once its output stream closes.
func (c *Client) StopTaskRun(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/task-runs/"+strconv.FormatInt(id, 10), nil, http.StatusNoContent)
	return err
}

// StreamTaskRunOutput follows GET /api/task-runs/{id}/output (SSE), calling
// onLine for each output line: first the buffered backlog, then live
// output. It returns nil once the run's output ends, or ctx's error if ctx
// is cancelled first.
func (c *Client) StreamTaskRunOutput(ctx context.Context, id int64, onLine func(string)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/task-runs/"+strconv.FormatInt(id, 10)+"/output", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	event := ""
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			event = ""
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if event == "end" {
				return nil
			}
			data := strings.TrimPrefix(line, "data:")
			onLine(strings.TrimPrefix(data, " "))
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("task output stream closed before the run ended")
}

// ListHarnesses calls GET /api/harnesses. A non-zero boardID flags the
// harness that board's sessions default to.
func (c *Client) ListHarnesses(ctx context.Context, boardID int64) ([]Harness, error) {
	path := "/api/harnesses"
	if boardID != 0 {
		path += "?board=" + strconv.FormatInt(boardID, 10)
	}
	raw, err := c.do(ctx, http.MethodGet, path, nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var hs []Harness
	if err := json.Unmarshal(raw, &hs); err != nil {
		return nil, err
	}
	return hs, nil
}

// SetSessionHarness calls PUT /api/sessions/{id}/harness. An empty id
// returns the session to the default harness. The server restarts a running
// agent when this changes which harness it launches.
func (c *Client) SetSessionHarness(ctx context.Context, sessionID int64, harnessID string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPut, "/api/sessions/"+strconv.FormatInt(sessionID, 10)+"/harness", map[string]string{"harness": harnessID}, http.StatusOK)
}

// ConfigPatchArgs is the request body for PATCH /api/config. Scope is "local"
// or "global"; Board (id or slug) is required for local scope. Set values are
// native typed values (bool/string/array/object); Unset names keys to clear.
type ConfigPatchArgs struct {
	Scope string         `json:"scope"`
	Board string         `json:"board,omitempty"`
	Set   map[string]any `json:"set,omitempty"`
	Unset []string       `json:"unset,omitempty"`
}

// Config calls GET /api/config. scope is "effective" (default when ""),
// "local", or "global"; board (id or slug) scopes the local layer.
func (c *Client) Config(ctx context.Context, scope, board string) (json.RawMessage, error) {
	q := url.Values{}
	if scope != "" {
		q.Set("scope", scope)
	}
	if board != "" {
		q.Set("board", board)
	}
	path := "/api/config"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return c.do(ctx, http.MethodGet, path, nil, http.StatusOK)
}

// ConfigPatch calls PATCH /api/config and returns the refreshed config view.
func (c *Client) ConfigPatch(ctx context.Context, a ConfigPatchArgs) (json.RawMessage, error) {
	if a.Scope == "" {
		return nil, fmt.Errorf("scope required (local or global)")
	}
	if len(a.Set) == 0 && len(a.Unset) == 0 {
		return nil, fmt.Errorf("nothing to set or unset")
	}
	return c.do(ctx, http.MethodPatch, "/api/config", a, http.StatusOK)
}

// ResolveBoardID resolves a board identifier (numeric id or slug) to a
// numeric id by listing boards. The REST endpoints under /api/boards/{id}
// (state, archived, etc.) require numeric ids; this helper lets callers
// accept slugs without each doing their own lookup.
func (c *Client) ResolveBoardID(ctx context.Context, ident string) (int64, error) {
	if ident == "" {
		return 0, fmt.Errorf("board identifier required")
	}
	if id, err := strconv.ParseInt(ident, 10, 64); err == nil {
		return id, nil
	}
	boards, err := c.ListBoards(ctx)
	if err != nil {
		return 0, err
	}
	for _, b := range boards {
		if b.Slug == ident || b.Name == ident {
			return b.ID, nil
		}
	}
	return 0, fmt.Errorf("board %q not found", ident)
}

// do is the shared request helper. body is JSON-encoded if non-nil; the
// response body is returned only when expectStatus matches and the server
// returned a body (HEAD/204 paths return nil, nil).
func (c *Client) do(ctx context.Context, method, path string, body any, expectStatus int) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectStatus {
		return nil, c.readError(resp)
	}
	if resp.StatusCode == http.StatusNoContent || resp.ContentLength == 0 {
		return nil, nil
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) readError(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(b, &e) == nil && e.Error != "" {
		return fmt.Errorf("%s: %s", resp.Status, e.Error)
	}
	return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
}
