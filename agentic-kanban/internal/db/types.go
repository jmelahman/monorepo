package db

type Board struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	RepoPath  string `json:"repo_path"`
	MountPath string `json:"mount_path"`
	// ProjectDir scopes the board to a subdirectory of RepoPath: the whole
	// repo is still checked out and mounted, but the agent works from this
	// subdirectory. Repo-relative, slash-separated, "" for whole-repo boards.
	// Requires RepoPath and an empty MountPath.
	ProjectDir     string `json:"project_dir"`
	WorktreeRoot   string `json:"worktree_root"`
	BaseBranch     string `json:"base_branch"`
	BranchPrefix   string `json:"branch_prefix"`
	GitAuthorName  string `json:"git_author_name"`
	GitAuthorEmail string `json:"git_author_email"`
	CreatedAt      int64  `json:"created_at"`
	Position       int    `json:"position"`
}

type Column struct {
	ID       int64  `json:"id"`
	BoardID  int64  `json:"board_id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type Ticket struct {
	ID          int64  `json:"id"`
	BoardID     int64  `json:"board_id"`
	ColumnID    int64  `json:"column_id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Body        string `json:"body"`
	Position    int    `json:"position"`
	CreatedAt   int64  `json:"created_at"`
	ArchivedAt  *int64 `json:"archived_at,omitempty"`
	Fingerprint string `json:"-"`
}

type Session struct {
	ID            int64   `json:"id"`
	TicketID      int64   `json:"ticket_id"`
	WorktreePath  string  `json:"worktree_path"`
	BranchName    string  `json:"branch_name"`
	ContainerID   *string `json:"container_id,omitempty"`
	ContainerName *string `json:"container_name,omitempty"`
	Status        string  `json:"status"`
	StartedAt     *int64  `json:"started_at,omitempty"`
	StoppedAt     *int64  `json:"stopped_at,omitempty"`
	PRState       string  `json:"pr_state,omitempty"`
	PRNumber      *int64  `json:"pr_number,omitempty"`
	PRURL         string  `json:"pr_url,omitempty"`
	PRTitle       string  `json:"pr_title,omitempty"`
	MountPath     string  `json:"mount_path,omitempty"`
	RepoPath      string  `json:"repo_path,omitempty"`
	// ClaudeSessionID is the UUID of the most recent Claude Code session
	// captured via its SessionStart hook. When non-empty, `claude` is
	// relaunched with `--resume <uuid>` so the conversation survives
	// container/Kanban restarts.
	ClaudeSessionID string `json:"claude_session_id,omitempty"`
	// Harness is the agent harness ID picked for this session, or "" to use
	// the user/project default (harness.Resolve). Written only by
	// UpdateSessionHarness; UpsertSession leaves it alone.
	Harness string `json:"harness,omitempty"`
	// WorkspaceFolder is the container path the agent's working directory was
	// created at, snapshotted at spawn time. Empty on sessions started before
	// the column existed; read it through WorkspaceDir.
	WorkspaceFolder string `json:"workspace_folder,omitempty"`
}

// DefaultWorkspaceFolder is the container path sessions used before
// workspaceFolder and project_dir were honored, and the fallback for rows
// that predate the sessions.workspace_folder column.
const DefaultWorkspaceFolder = "/workspace"

// WorkspaceDir is the container path execs against this session should use as
// their working directory. It is a snapshot of how the container was actually
// created, so it stays correct for a running container even if the board's
// project_dir is edited underneath it.
//
// See REGRESSIONS.md: "Exec sites hardcode /workspace".
func (s *Session) WorkspaceDir() string {
	if s == nil || s.WorkspaceFolder == "" {
		return DefaultWorkspaceFolder
	}
	return s.WorkspaceFolder
}

type PortAllocation struct {
	ID            int64  `json:"id"`
	SessionID     int64  `json:"session_id"`
	Label         string `json:"label"`
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	ProxyActive   bool   `json:"proxy_active"`
}

type TaskRun struct {
	ID        int64   `json:"id"`
	SessionID int64   `json:"session_id"`
	TaskLabel string  `json:"task_label"`
	Command   string  `json:"command"`
	ExecID    *string `json:"exec_id,omitempty"`
	Status    string  `json:"status"`
	ExitCode  *int    `json:"exit_code,omitempty"`
	StartedAt int64   `json:"started_at"`
	StoppedAt *int64  `json:"stopped_at,omitempty"`
}

type HookConfig struct {
	ID      int64  `json:"id"`
	BoardID *int64 `json:"board_id,omitempty"`
	Event   string `json:"event"`
	Command string `json:"command"`
	Enabled bool   `json:"enabled"`
}

const (
	SessionStatusStopped      = "stopped"
	SessionStatusStarting     = "starting"
	SessionStatusIdle         = "idle"
	SessionStatusWorking      = "working"
	SessionStatusAwaitingPerm = "awaiting_perm"
	SessionStatusError        = "error"

	TaskRunStatusRunning = "running"
	TaskRunStatusExited  = "exited"
	TaskRunStatusStopped = "stopped"
)
