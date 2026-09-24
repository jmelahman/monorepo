// Package mcp implements a minimal Model Context Protocol server (stdio
// transport) that lets AI agents drive kanban via the existing HTTP API.
// It speaks the JSON-RPC 2.0 dialect described in the MCP spec
// (https://modelcontextprotocol.io) — initialize, tools/list, tools/call,
// plus the standard cancellation / ping flow. Only the methods we need are
// implemented; unknown methods return JSON-RPC error -32601.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/jmelahman/kanban/internal/client"
)

// Protocol version we advertise. Clients can still negotiate; we just echo
// back whatever they asked for if it's something we recognize, otherwise we
// pin to this. Kept current with the published MCP spec.
const protocolVersion = "2025-06-18"

// Run starts the MCP server on stdin/stdout and blocks until ctx is done or
// stdin closes. serverURL is the base URL of a running `kanban serve`.
func Run(ctx context.Context, serverURL string) error {
	return run(ctx, serverURL, os.Stdin, os.Stdout, http.DefaultClient)
}

func run(ctx context.Context, serverURL string, in io.Reader, out io.Writer, hc *http.Client) error {
	srv := &server{client: client.New(serverURL, hc), out: out}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		srv.handle(ctx, line)
	}
	return scanner.Err()
}

type server struct {
	client *client.Client
	out    io.Writer
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (s *server) handle(ctx context.Context, line []byte) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		s.write(rpcResponse{JSONRPC: "2.0", ID: nil, Error: &rpcError{Code: -32700, Message: "parse error: " + err.Error()}})
		return
	}
	// Notifications (no id) must not get a response.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"
	switch req.Method {
	case "initialize":
		if isNotification {
			return
		}
		s.write(s.success(req.ID, s.initialize(req.Params)))
	case "notifications/initialized", "notifications/cancelled":
		// fire-and-forget
	case "ping":
		if isNotification {
			return
		}
		s.write(s.success(req.ID, map[string]any{}))
	case "tools/list":
		if isNotification {
			return
		}
		s.write(s.success(req.ID, map[string]any{"tools": toolDefinitions()}))
	case "tools/call":
		if isNotification {
			return
		}
		result, err := s.callTool(ctx, req.Params)
		if err != nil {
			s.write(rpcResponse{JSONRPC: "2.0", ID: rawID(req.ID), Error: &rpcError{Code: -32603, Message: err.Error()}})
			return
		}
		s.write(s.success(req.ID, result))
	default:
		if isNotification {
			return
		}
		s.write(rpcResponse{JSONRPC: "2.0", ID: rawID(req.ID), Error: &rpcError{Code: -32601, Message: "method not found: " + req.Method}})
	}
}

func (s *server) success(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: rawID(id), Result: result}
}

// rawID returns the id as a typed value so json.Marshal preserves number vs
// string vs null. Without this we'd quote numeric ids.
func rawID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	return id
}

func (s *server) write(resp rpcResponse) {
	buf, err := json.Marshal(resp)
	if err != nil {
		return
	}
	buf = append(buf, '\n')
	_, _ = s.out.Write(buf)
}

func (s *server) initialize(params json.RawMessage) map[string]any {
	var req struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &req)
	version := req.ProtocolVersion
	if version == "" {
		version = protocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "kanban",
			"version": "0.1.0",
		},
	}
}

// schemaProp is shorthand for one inputSchema property entry.
func schemaProp(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

func boardIdentSchema() map[string]any {
	return schemaProp("string", "Board id (numeric) or slug")
}

func ticketIDSchema() map[string]any {
	return schemaProp("integer", "Ticket id (numeric)")
}

func sessionIDSchema() map[string]any {
	return schemaProp("integer", "Session id (numeric)")
}

func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "list_boards",
			"description": "List all kanban boards as {id, name, slug}.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "create_board",
			"description": "Create a new kanban board. Either repo_path or mount_path must be set; the server seeds the four default columns (Backlog, In Progress, Review, Done).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":             schemaProp("string", "Board name"),
					"repo_path":        schemaProp("string", "Path to the host git repo (one of repo_path/mount_path is required)"),
					"mount_path":       schemaProp("string", "Mount path inside session containers (alternative to repo_path)"),
					"project_dir":      schemaProp("string", "Repo-relative subdirectory the agent works from, for a monorepo subproject (e.g. \"services/api\"). Requires repo_path and cannot be combined with mount_path."),
					"worktree_root":    schemaProp("string", "Override the parent directory for new session worktrees"),
					"base_branch":      schemaProp("string", "Branch session worktrees fork from (default: main)"),
					"branch_prefix":    schemaProp("string", "Optional prefix prepended to session branch names"),
					"git_author_name":  schemaProp("string", "Author name used for merge commits (e.g. squash merges)"),
					"git_author_email": schemaProp("string", "Author email used for merge commits"),
				},
				"required": []string{"name"},
			},
		},
		{
			"name":        "get_board",
			"description": "Fetch a single board by id or slug.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board": boardIdentSchema(),
				},
				"required": []string{"board"},
			},
		},
		{
			"name":        "update_board",
			"description": "Patch fields on a board. Only fields you supply are updated.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board":            boardIdentSchema(),
					"name":             schemaProp("string", "New name"),
					"repo_path":        schemaProp("string", "New repo path"),
					"mount_path":       schemaProp("string", "New mount path"),
					"project_dir":      schemaProp("string", "New repo-relative project directory; empty string clears it"),
					"worktree_root":    schemaProp("string", "New worktree root"),
					"base_branch":      schemaProp("string", "New base branch"),
					"branch_prefix":    schemaProp("string", "New branch prefix"),
					"git_author_name":  schemaProp("string", "New commit author name"),
					"git_author_email": schemaProp("string", "New commit author email"),
				},
				"required": []string{"board"},
			},
		},
		{
			"name":        "delete_board",
			"description": "Delete a board. Destroys every associated session first.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board": boardIdentSchema(),
				},
				"required": []string{"board"},
			},
		},
		{
			"name":        "board_state",
			"description": "Return a single-shot snapshot of a board: {board, columns, tickets, sessions, merge_config, sync_config}.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board": boardIdentSchema(),
				},
				"required": []string{"board"},
			},
		},
		{
			"name":        "list_board_env",
			"description": "List the environment variable KEY NAMES configured on a board. Values are write-only secrets and are never returned.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board": boardIdentSchema(),
				},
				"required": []string{"board"},
			},
		},
		{
			"name":        "set_board_env",
			"description": "Set one or more environment variables on a board. They are injected into the board's session containers at the next session start/restart. Values are encrypted at rest and write-only: they cannot be read back through any API.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board": boardIdentSchema(),
					"vars": map[string]any{
						"type":                 "object",
						"description":          "Map of NAME to value. Names must match [A-Za-z_][A-Za-z0-9_]*; the KANBAN_ prefix is reserved.",
						"additionalProperties": map[string]any{"type": "string"},
					},
				},
				"required": []string{"board", "vars"},
			},
		},
		{
			"name":        "unset_board_env",
			"description": "Remove one or more environment variables from a board by key name. Removing a missing key is a no-op.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board": boardIdentSchema(),
					"keys": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Key names to remove",
					},
				},
				"required": []string{"board", "keys"},
			},
		},
		{
			"name":        "list_archived",
			"description": "List archived tickets on a board.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board": boardIdentSchema(),
				},
				"required": []string{"board"},
			},
		},
		{
			"name":        "delete_archived",
			"description": "Permanently delete every archived ticket on a board (destructive).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board": boardIdentSchema(),
				},
				"required": []string{"board"},
			},
		},
		{
			"name":        "create_ticket",
			"description": "Create a new ticket on a kanban board. Board accepts numeric id or slug. Column is optional and defaults to the leftmost column (typically Backlog); when set it accepts a column name (case-insensitive) or numeric id.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"board":  boardIdentSchema(),
					"title":  schemaProp("string", "Ticket title"),
					"body":   schemaProp("string", "Optional ticket body / description"),
					"column": schemaProp("string", "Optional column name or id"),
				},
				"required": []string{"board", "title"},
			},
		},
		{
			"name":        "update_ticket",
			"description": "Patch the title and/or body of a ticket.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket": ticketIDSchema(),
					"title":  schemaProp("string", "New title"),
					"body":   schemaProp("string", "New body"),
				},
				"required": []string{"ticket"},
			},
		},
		{
			"name":        "move_ticket",
			"description": "Move a ticket to a different column / position.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket":    ticketIDSchema(),
					"column_id": schemaProp("integer", "Target column id"),
					"position":  schemaProp("integer", "Target 0-indexed position within the column"),
				},
				"required": []string{"ticket", "column_id"},
			},
		},
		{
			"name":        "archive_ticket",
			"description": "Archive a ticket. Stops its session if running.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket": ticketIDSchema(),
				},
				"required": []string{"ticket"},
			},
		},
		{
			"name":        "unarchive_ticket",
			"description": "Unarchive a ticket; placed at the end of its original column.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket": ticketIDSchema(),
				},
				"required": []string{"ticket"},
			},
		},
		{
			"name":        "delete_ticket",
			"description": "Permanently delete a ticket. The ticket must be archived first.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket": ticketIDSchema(),
				},
				"required": []string{"ticket"},
			},
		},
		{
			"name":        "done_ticket",
			"description": "Move a ticket to the board's rightmost column (last position) and stop its session.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket": ticketIDSchema(),
				},
				"required": []string{"ticket"},
			},
		},
		{
			"name":        "sync_ticket",
			"description": "Sync a ticket branch from base. Strategy is 'rebase' or 'merge'.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket":   ticketIDSchema(),
					"strategy": schemaProp("string", "rebase or merge (default: rebase)"),
				},
				"required": []string{"ticket"},
			},
		},
		{
			"name":        "merge_ticket",
			"description": "Merge a ticket branch into base. Strategy is 'merge-commit', 'squash', or 'rebase'; omit it to use the board's configured default.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket":   ticketIDSchema(),
					"strategy": schemaProp("string", "merge-commit, squash, or rebase (default: merge.default_strategy)"),
				},
				"required": []string{"ticket"},
			},
		},
		{
			"name":        "archive_column_tickets",
			"description": "Archive every non-archived ticket in a column. Stops their sessions.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"column_id": schemaProp("integer", "Column id (numeric)"),
				},
				"required": []string{"column_id"},
			},
		},
		{
			"name":        "ensure_session",
			"description": "Ensure a session exists for a ticket (creates one if absent).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ticket": ticketIDSchema(),
				},
				"required": []string{"ticket"},
			},
		},
		{
			"name":        "start_session",
			"description": "Start a stopped session.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session": sessionIDSchema(),
				},
				"required": []string{"session"},
			},
		},
		{
			"name":        "stop_session",
			"description": "Stop a running session.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session": sessionIDSchema(),
				},
				"required": []string{"session"},
			},
		},
		{
			"name":        "restart_session",
			"description": "Restart a session.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session": sessionIDSchema(),
				},
				"required": []string{"session"},
			},
		},
		{
			"name":        "list_config",
			"description": "List kanban config keys with their values and source. scope is 'effective' (default, merged + read-only runtime values), 'local', or 'global'; board (id or slug) selects the local layer for the effective/local view.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope": schemaProp("string", "effective (default), local, or global"),
					"board": boardIdentSchema(),
				},
			},
		},
		{
			"name":        "get_config",
			"description": "Get one config value by dotted key (e.g. sync.allow_rebase, github.draft_column, devcontainer.container_env.FOO). scope/board as in list_config.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":   schemaProp("string", "Dotted config key"),
					"scope": schemaProp("string", "effective (default), local, or global"),
					"board": boardIdentSchema(),
				},
				"required": []string{"key"},
			},
		},
		{
			"name":        "set_config",
			"description": "Set a config key. scope is 'local' (requires board) or 'global'. value is a native JSON value matching the key's type: bool/string for scalars, array for run_args/mounts, object for container_env, array-of-objects for task/buildcop.boards.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":   schemaProp("string", "Dotted config key"),
					"value": map[string]any{"description": "New value (type depends on the key)"},
					"scope": schemaProp("string", "local (requires board) or global"),
					"board": boardIdentSchema(),
				},
				"required": []string{"key", "value", "scope"},
			},
		},
		{
			"name":        "unset_config",
			"description": "Remove a config key. scope is 'local' (requires board) or 'global'.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":   schemaProp("string", "Dotted config key"),
					"scope": schemaProp("string", "local (requires board) or global"),
					"board": boardIdentSchema(),
				},
				"required": []string{"key", "scope"},
			},
		},
	}
}

func (s *server) callTool(ctx context.Context, params json.RawMessage) (map[string]any, error) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// unmarshal args into a single big shape; tools only read the fields
	// they care about.
	var a struct {
		Board          string            `json:"board"`
		Name           string            `json:"name"`
		RepoPath       string            `json:"repo_path"`
		MountPath      string            `json:"mount_path"`
		ProjectDir     string            `json:"project_dir"`
		WorktreeRoot   string            `json:"worktree_root"`
		BaseBranch     string            `json:"base_branch"`
		BranchPrefix   string            `json:"branch_prefix"`
		GitAuthorName  string            `json:"git_author_name"`
		GitAuthorEmail string            `json:"git_author_email"`
		NamePtr        *string           `json:"-"`
		Title          string            `json:"title"`
		Body           string            `json:"body"`
		Column         string            `json:"column"`
		Ticket         int64             `json:"ticket"`
		ColumnID       int64             `json:"column_id"`
		Position       int               `json:"position"`
		Strategy       string            `json:"strategy"`
		Session        int64             `json:"session"`
		Key            string            `json:"key"`
		Value          json.RawMessage   `json:"value"`
		Scope          string            `json:"scope"`
		Vars           map[string]string `json:"vars"`
		Keys           []string          `json:"keys"`
	}
	if len(p.Arguments) > 0 {
		if err := json.Unmarshal(p.Arguments, &a); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	// raw is also re-decoded for tools whose patch semantics need
	// "field present?" detection (update_board, update_ticket).
	var rawArgs map[string]json.RawMessage
	if len(p.Arguments) > 0 {
		_ = json.Unmarshal(p.Arguments, &rawArgs)
	}
	hasField := func(k string) bool { _, ok := rawArgs[k]; return ok }

	switch p.Name {
	case "list_boards":
		boards, err := s.client.ListBoards(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		buf, _ := json.Marshal(boards)
		return textResult(string(buf)), nil

	case "create_board":
		raw, err := s.client.CreateBoard(ctx, client.CreateBoardArgs{
			Name:           a.Name,
			RepoPath:       a.RepoPath,
			MountPath:      a.MountPath,
			ProjectDir:     a.ProjectDir,
			WorktreeRoot:   a.WorktreeRoot,
			BaseBranch:     a.BaseBranch,
			BranchPrefix:   a.BranchPrefix,
			GitAuthorName:  a.GitAuthorName,
			GitAuthorEmail: a.GitAuthorEmail,
		})
		return rawOrError(raw, err), nil

	case "get_board":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		raw, err := s.client.GetBoard(ctx, id)
		return rawOrError(raw, err), nil

	case "update_board":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		patch := client.UpdateBoardArgs{}
		if hasField("name") {
			patch.Name = &a.Name
		}
		if hasField("repo_path") {
			patch.RepoPath = &a.RepoPath
		}
		if hasField("mount_path") {
			patch.MountPath = &a.MountPath
		}
		if hasField("project_dir") {
			patch.ProjectDir = &a.ProjectDir
		}
		if hasField("worktree_root") {
			patch.WorktreeRoot = &a.WorktreeRoot
		}
		if hasField("base_branch") {
			patch.BaseBranch = &a.BaseBranch
		}
		if hasField("branch_prefix") {
			patch.BranchPrefix = &a.BranchPrefix
		}
		if hasField("git_author_name") {
			patch.GitAuthorName = &a.GitAuthorName
		}
		if hasField("git_author_email") {
			patch.GitAuthorEmail = &a.GitAuthorEmail
		}
		raw, err := s.client.UpdateBoard(ctx, id, patch)
		return rawOrError(raw, err), nil

	case "delete_board":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		if err := s.client.DeleteBoard(ctx, id); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "board_state":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		raw, err := s.client.BoardState(ctx, id)
		return rawOrError(raw, err), nil

	case "list_board_env":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		raw, err := s.client.ListBoardEnv(ctx, id)
		return rawOrError(raw, err), nil

	case "set_board_env":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		raw, err := s.client.PatchBoardEnv(ctx, id, client.PatchBoardEnvArgs{Set: a.Vars})
		return rawOrError(raw, err), nil

	case "unset_board_env":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		raw, err := s.client.PatchBoardEnv(ctx, id, client.PatchBoardEnvArgs{Unset: a.Keys})
		return rawOrError(raw, err), nil

	case "list_archived":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		raw, err := s.client.ListArchived(ctx, id)
		return rawOrError(raw, err), nil

	case "delete_archived":
		id, errRes := s.boardID(ctx, a.Board)
		if errRes != nil {
			return errRes, nil
		}
		if err := s.client.DeleteArchived(ctx, id); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "create_ticket":
		raw, err := s.client.CreateTicket(ctx, client.CreateTicketArgs{
			Board:  a.Board,
			Title:  a.Title,
			Body:   a.Body,
			Column: a.Column,
		})
		return rawOrError(raw, err), nil

	case "update_ticket":
		patch := client.UpdateTicketArgs{}
		if hasField("title") {
			patch.Title = &a.Title
		}
		if hasField("body") {
			patch.Body = &a.Body
		}
		raw, err := s.client.UpdateTicket(ctx, a.Ticket, patch)
		return rawOrError(raw, err), nil

	case "move_ticket":
		if err := s.client.MoveTicket(ctx, a.Ticket, client.MoveTicketArgs{
			ColumnID: a.ColumnID, Position: a.Position,
		}); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "archive_ticket":
		if err := s.client.ArchiveTicket(ctx, a.Ticket); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "unarchive_ticket":
		if err := s.client.UnarchiveTicket(ctx, a.Ticket); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "delete_ticket":
		if err := s.client.DeleteTicket(ctx, a.Ticket); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "done_ticket":
		if err := s.client.DoneTicket(ctx, a.Ticket); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "sync_ticket":
		strategy := a.Strategy
		if strategy == "" {
			strategy = "rebase"
		}
		if err := s.client.SyncTicket(ctx, a.Ticket, strategy); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "merge_ticket":
		if err := s.client.MergeTicket(ctx, a.Ticket, a.Strategy); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "archive_column_tickets":
		if err := s.client.ArchiveColumnTickets(ctx, a.ColumnID); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "ensure_session":
		raw, err := s.client.EnsureSession(ctx, a.Ticket)
		return rawOrError(raw, err), nil

	case "start_session":
		raw, err := s.client.StartSession(ctx, a.Session)
		return rawOrError(raw, err), nil

	case "stop_session":
		if err := s.client.StopSession(ctx, a.Session); err != nil {
			return errorResult(err.Error()), nil
		}
		return textResult("ok"), nil

	case "restart_session":
		raw, err := s.client.RestartSession(ctx, a.Session)
		return rawOrError(raw, err), nil

	case "list_config":
		raw, err := s.client.Config(ctx, a.Scope, a.Board)
		return rawOrError(raw, err), nil

	case "get_config":
		raw, err := s.client.Config(ctx, a.Scope, a.Board)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		v, ok := configValueFor(raw, a.Key)
		if !ok {
			return errorResult(fmt.Sprintf("no value set for %q", a.Key)), nil
		}
		buf, _ := json.Marshal(map[string]any{"key": a.Key, "value": v})
		return textResult(string(buf)), nil

	case "set_config":
		var v any
		if len(a.Value) > 0 {
			if err := json.Unmarshal(a.Value, &v); err != nil {
				return errorResult("invalid value: " + err.Error()), nil
			}
		}
		raw, err := s.client.ConfigPatch(ctx, client.ConfigPatchArgs{
			Scope: a.Scope,
			Board: a.Board,
			Set:   map[string]any{a.Key: v},
		})
		return rawOrError(raw, err), nil

	case "unset_config":
		raw, err := s.client.ConfigPatch(ctx, client.ConfigPatchArgs{
			Scope: a.Scope,
			Board: a.Board,
			Unset: []string{a.Key},
		})
		return rawOrError(raw, err), nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", p.Name)
	}
}

// configValueFor extracts a single key's value from a GET /api/config
// response, resolving a StringMap entry address (parent.key) by indexing the
// parent map when no exact key matches.
func configValueFor(raw json.RawMessage, key string) (any, bool) {
	var view struct {
		Entries []struct {
			Key   string `json:"key"`
			Value any    `json:"value"`
		} `json:"entries"`
	}
	if json.Unmarshal(raw, &view) != nil {
		return nil, false
	}
	for _, e := range view.Entries {
		if e.Key == key {
			return e.Value, true
		}
	}
	if i := strings.LastIndex(key, "."); i > 0 {
		parent, sub := key[:i], key[i+1:]
		for _, e := range view.Entries {
			if e.Key == parent {
				if m, ok := e.Value.(map[string]any); ok {
					if v, ok := m[sub]; ok {
						return v, true
					}
				}
			}
		}
	}
	return nil, false
}

// boardID resolves ident through the client, returning (id, nil) on success
// or (0, errorResult) on failure. Callers do `id, errRes := s.boardID(...)
// if errRes != nil { return errRes, nil }`.
func (s *server) boardID(ctx context.Context, ident string) (int64, map[string]any) {
	id, err := s.client.ResolveBoardID(ctx, ident)
	if err != nil {
		return 0, errorResult(err.Error())
	}
	return id, nil
}

func rawOrError(raw json.RawMessage, err error) map[string]any {
	if err != nil {
		return errorResult(err.Error())
	}
	if len(raw) == 0 {
		return textResult("ok")
	}
	return textResult(string(raw))
}

func textResult(text string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	}
}

func errorResult(text string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": true,
	}
}
