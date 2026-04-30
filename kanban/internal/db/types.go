package db

type Board struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	SourceRepoPath string `json:"source_repo_path"`
	WorktreeRoot   string `json:"worktree_root"`
	BaseBranch     string `json:"base_branch"`
	CreatedAt      int64  `json:"created_at"`
}

type Column struct {
	ID       int64  `json:"id"`
	BoardID  int64  `json:"board_id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

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
	SessionStatusStopped       = "stopped"
	SessionStatusStarting      = "starting"
	SessionStatusIdle          = "idle"
	SessionStatusWorking       = "working"
	SessionStatusAwaitingPerm  = "awaiting_perm"
	SessionStatusError         = "error"

	TaskRunStatusRunning = "running"
	TaskRunStatusExited  = "exited"
	TaskRunStatusStopped = "stopped"
)
