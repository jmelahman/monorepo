package session

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/git"
	"github.com/jmelahman/kanban/internal/harness"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/kanbantoml"
)

type Manager struct {
	store  *db.Store
	docker *docker.Client
	hooks  *hooks.Runner

	proxies              *docker.ProxyManager
	brokers              *brokerSet
	apiBase              string
	claudeConfigOverride *bool

	// containerRunning answers "is this container still up?" for Reconcile.
	// Defaults to the docker client; tests swap in a stub via
	// SetContainerProbe so a dead container can be simulated without a daemon.
	containerRunning func(ctx context.Context, containerID string) (bool, error)

	// endPTY ends the process behind a closed PTY broker, found by its
	// $KANBAN_PTY_ID marker. Defaults to running endPTYScript in the
	// container; tests swap in a stub.
	endPTY func(ctx context.Context, containerID, marker string) error
}

func NewManager(store *db.Store, dc *docker.Client, h *hooks.Runner) *Manager {
	m := &Manager{
		store:   store,
		docker:  dc,
		hooks:   h,
		proxies: docker.NewProxyManager(context.Background(), dc),
		brokers: newBrokerSet(dc),
	}
	if dc != nil {
		m.containerRunning = dc.ContainerRunning
		m.endPTY = m.execEndPTY
	}
	return m
}

// SetContainerProbe overrides how Reconcile checks whether a session's
// container is still running. Pass nil to disable the check.
func (m *Manager) SetContainerProbe(probe func(ctx context.Context, containerID string) (bool, error)) {
	m.containerRunning = probe
}

// SetAPIBase configures the URL session containers should use to call back
// into the kanban API (e.g. http://kanban:7474).
func (m *Manager) SetAPIBase(base string) { m.apiBase = base }

// SetClaudeConfigOverride forces the built-in claude_config bind regardless of
// .kanban.toml. Pass nil to defer to the toml setting (default true).
func (m *Manager) SetClaudeConfigOverride(b *bool) { m.claudeConfigOverride = b }

// Ensure creates a session row for a ticket if missing, allocating a worktree
// only when the board is associated with a real git repo. For repo-less boards
// the session's "worktree path" is the board's mount path (or repo_path
// fallback) so downstream tools that need a host-side directory still have one.
func (m *Manager) Ensure(ctx context.Context, board *db.Board, ticket *db.Ticket) (*db.Session, error) {
	// Resolve paths against an empty session so we use the board defaults.
	paths := ResolvePaths(board, &db.Session{})

	if sess, err := m.store.GetSessionByTicket(ctx, ticket.ID); err == nil {
		// Settings go where the agent will actually launch — for a board
		// scoped to a subproject that is the subproject, not the worktree
		// root, or Claude Code never reads the status hooks.
		//
		// Gate the write on the project directory being real. writeClaudeSettings
		// MkdirAlls its parents, so an edited-to-garbage project_dir would have
		// this path fabricate `<worktree>/<project_dir>/.claude` inside the
		// worktree every time someone merely opens the ticket. Reconcile still
		// runs: the session row exists and its container may well be healthy on
		// the workspace folder it was created with, so squaring the row with the
		// daemon stays useful. Start reports the real error.
		// Worktrees created before kanban locked them at creation time get
		// locked here, so `git worktree prune` inside a session container
		// can't orphan them.
		if paths.HasRepo && isGitRepo(sess.WorktreePath) {
			if err := git.LockWorktree(sess.WorktreePath); err != nil {
				log.Printf("lock worktree %s: %v", sess.WorktreePath, err)
			}
		}
		if err := checkProjectRoot(board, paths, sess.WorktreePath); err != nil {
			log.Printf("skip claude settings for ticket %d: %v", ticket.ID, err)
		} else if err := writeClaudeSettings(paths.ProjectRoot(sess.WorktreePath)); err != nil {
			log.Printf("write claude settings for ticket %d: %v", ticket.ID, err)
		}
		return m.Reconcile(ctx, sess)
	}

	containerName := fmt.Sprintf("kanban-%s-%s", board.Slug, ticket.Slug)

	var worktreePath, branch string
	if paths.HasRepo {
		branch = resolveBranchPrefix(board, paths.RepoPath) + "/" + ticket.Slug
		worktreeRoot := board.WorktreeRoot
		if worktreeRoot == "" {
			return nil, fmt.Errorf("board %q has a repo but no worktree_root configured", board.Slug)
		}
		worktreePath = filepath.Join(worktreeRoot, ticket.Slug)
		if _, statErr := os.Stat(worktreePath); statErr == nil {
			// Worktree directory already exists. Trust it only when it actually
			// is a git worktree (has a .git entry) — dockerd will auto-create a
			// missing bind-mount source as an empty directory, and silently
			// trusting that empty dir is how we'd end up mounting nothing into
			// the session container.
			if !isGitRepo(worktreePath) {
				return nil, fmt.Errorf("worktree path %q exists but is not a git worktree (likely a stale empty directory from a prior failed start); remove it and try again", worktreePath)
			}
			if err := git.LockWorktree(worktreePath); err != nil {
				log.Printf("lock worktree %s: %v", worktreePath, err)
			}
		} else {
			base := git.ResolveLatestBase(paths.RepoPath, board.BaseBranch)
			if err := git.AddWorktree(paths.RepoPath, branch, worktreePath, base); err != nil {
				// Branch may already exist (orphaned). Try attaching it to a fresh worktree.
				if err2 := git.AddWorktreeFromExisting(paths.RepoPath, branch, worktreePath); err2 != nil {
					return nil, fmt.Errorf("create worktree at %s from %s base %q: %w", worktreePath, paths.RepoPath, base, err)
				}
			}
		}
	} else if board.RepoPath != "" {
		// User configured a repo path but kanban can't see it as a git repo —
		// most often because the host path isn't bind-mounted into the kanban
		// container. Fail loudly rather than silently degrading to a mount-only
		// session with an empty /workspace.
		return nil, fmt.Errorf("repo_path %q is not a git repository visible to kanban (no .git found); make sure the path exists and is bind-mounted into the kanban container", board.RepoPath)
	} else {
		// No repo — use the resolved mount as the session's "worktree" so things
		// like task discovery and claude settings have a host directory to act on.
		worktreePath = paths.MountPath
		if worktreePath == "" {
			return nil, fmt.Errorf("board %q has neither repo_path nor mount_path configured", board.Slug)
		}
	}

	if err := checkProjectRoot(board, paths, worktreePath); err != nil {
		return nil, err
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
	if err := writeClaudeSettings(paths.ProjectRoot(worktreePath)); err != nil {
		log.Printf("write claude settings for ticket %d: %v", ticket.ID, err)
	}
	return sess, nil
}

// Start brings up the devcontainer for a session. onPullProgress, if non-nil,
// receives throttled image-pull progress while the devcontainer image is
// being fetched (no-op when the image is already cached).
func (m *Manager) Start(ctx context.Context, sessionID int64, onPullProgress docker.PullProgressFunc) (*db.Session, error) {
	sess, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	// A session whose container died underneath it still reads as running;
	// square the row with the daemon first so the switch below restarts it
	// instead of returning the stale row as a no-op.
	if sess, err = m.Reconcile(ctx, sess); err != nil {
		return nil, err
	}
	switch sess.Status {
	case db.SessionStatusStopped, db.SessionStatusError:
		// proceed
	default:
		return sess, nil
	}

	// Stale container from a prior run (e.g. host reboot): clear the reference
	// so we don't try to reuse a vanished container ID below.
	if sess.ContainerID != nil && *sess.ContainerID != "" {
		cleared := ""
		sess.ContainerID = &cleared
	}

	board, _ := m.boardForSession(ctx, sess)
	paths := ResolvePaths(board, sess)
	// Re-checked on every start, not just at Ensure: the board's project_dir
	// can be edited (or the subproject deleted on a branch) between the
	// session row being created and the container being built.
	if err := checkProjectRoot(board, paths, sess.WorktreePath); err != nil {
		_ = m.store.UpdateSessionStatus(ctx, sess.ID, db.SessionStatusError)
		return nil, err
	}
	projectRoot := paths.ProjectRoot(sess.WorktreePath)

	// Subproject config wins, the worktree root is the fallback. LoadDevcontainerFrom
	// dedupes, so a whole-repo board (where projectRoot == WorktreePath) still
	// probes each path once.
	cfg, err := docker.LoadDevcontainerFrom(projectRoot, sess.WorktreePath)
	if err != nil {
		_ = m.store.UpdateSessionStatus(ctx, sess.ID, db.SessionStatusError)
		return nil, err
	}
	applyKanbanDevcontainerOverrides(cfg, kanbantoml.LoadFrom(sess.WorktreePath, projectRoot).Devcontainer, m.claudeConfigOverride)

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

	// Remove any pre-existing container with this name (e.g. left over after a
	// host reboot). Docker would otherwise reject ContainerCreate with a name
	// conflict.
	if containerName != "" {
		_ = m.docker.RemoveContainer(ctx, containerName)
	}

	worktreeMount := ""
	if paths.HasRepo {
		worktreeMount = sess.WorktreePath
	}

	// Board-level env vars (decrypted from the DB) ride along in ExtraEnv so
	// they land in the container environment. A failure here must not block
	// the launch — the session just starts without them.
	boardEnv := map[string]string{}
	if board != nil {
		if vars, err := m.store.GetBoardEnvVars(ctx, board.ID); err != nil {
			log.Printf("session %d: load board %d env vars: %v", sess.ID, board.ID, err)
		} else {
			boardEnv = vars
		}
	}

	res, err := m.docker.Spawn(ctx, cfg, docker.SpawnOptions{
		WorktreePath:     sess.WorktreePath,
		MountPath:        paths.MountPath,
		RepoWorktreePath: worktreeMount,
		SourceRepoPath:   paths.RepoPath,
		ProjectDir:       paths.ProjectDir,
		ContainerName:    containerName,
		Ports:            mappings,
		// mergeEnv keeps the KANBAN_* system vars authoritative even if a
		// board var collides (patchBoardEnv rejects the prefix, but rows may
		// predate that check or come from other writers).
		ExtraEnv: mergeEnv(boardEnv, map[string]string{
			"KANBAN_SESSION_ID": fmt.Sprintf("%d", sess.ID),
			"KANBAN_API_URL":    m.apiBase,
		}),
		AttachNetwork:  docker.KanbanNetworkName,
		OnPullProgress: onPullProgress,
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
	sess.WorkspaceFolder = res.WorkspaceFolder
	if err := m.store.UpdateSessionLifecycle(ctx, sess.ID, sess.Status, sess.ContainerID, sess.StartedAt, sess.StoppedAt, &res.WorkspaceFolder); err != nil {
		return nil, err
	}
	// Refresh from DB so any columns written concurrently (e.g. the github
	// poller's pr_* fields) are reflected in the returned value and in any
	// session_updated event published from it.
	if fresh, err := m.store.GetSession(ctx, sess.ID); err == nil {
		sess = fresh
	}

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

// resolveBranchPrefix picks the literal branch prefix for new sessions on a
// board. Precedence: board.BranchPrefix, then [branches].prefix from the
// merged kanban.toml (project file at repoPath plus user file), then the
// hardcoded "kanban/<slug>" default. The returned value is concatenated with
// "/" + ticket.Slug to form the full branch name.
func resolveBranchPrefix(board *db.Board, repoPath string) string {
	if p := strings.TrimSpace(board.BranchPrefix); p != "" {
		return p
	}
	cfg := kanbantoml.Load(repoPath)
	if cfg.Branches != nil && cfg.Branches.Prefix != nil {
		if p := strings.TrimSpace(*cfg.Branches.Prefix); p != "" {
			return p
		}
	}
	return "kanban/" + board.Slug
}

// applyKanbanDevcontainerOverrides layers the [devcontainer] section from
// .kanban.toml (project + user) onto the parsed devcontainer.json: run_args
// and mounts append, container_env merges with kanban values winning.
//
// For built-in configs the docker_socket flag (default false) and
// claude_config flag (default true) control whether the host docker
// socket and Claude Code config get bind-mounted. Hand-written
// devcontainer.json files manage their own mounts and ignore the flags.
// The image key likewise swaps only the built-in config's image; a
// hand-written devcontainer.json keeps the image or build it declares.
// claudeConfigOverride, when non-nil, wins over .kanban.toml — it's set
// by the --claude-config flag / $KANBAN_CLAUDE_CONFIG env so a single
// server invocation can disable forwarding without editing config files.
//
// The docker socket defaults off because mounting it grants the session
// agent the equivalent of root on the host. Users who want sessions to
// drive Docker (e.g. `docker compose up` in an agent task) can opt in
// with `[devcontainer].docker_socket = true` in .kanban.toml.
// mergeEnv combines base with protect; protect entries win on collision.
// buildContainerConfig appends ExtraEnv after the devcontainer/.kanban.toml
// ContainerEnv and Docker resolves duplicates last-value-wins, so the overall
// precedence is: container_env < board env vars < KANBAN_* system vars.
func mergeEnv(base, protect map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(protect))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range protect {
		out[k] = v
	}
	return out
}

func applyKanbanDevcontainerOverrides(cfg *docker.DevcontainerConfig, dev *kanbantoml.DevcontainerSection, claudeConfigOverride *bool) {
	if cfg == nil {
		return
	}
	if cfg.BuiltIn {
		if dev != nil && dev.Image != nil && *dev.Image != "" {
			cfg.Image = *dev.Image
		}
		mountSocket := false
		if dev != nil && dev.DockerSocket != nil {
			mountSocket = *dev.DockerSocket
		}
		if mountSocket {
			if mount := docker.DockerSocketMount(); mount != "" {
				cfg.Mounts = append(cfg.Mounts, mount)
			}
		}
		mountClaude := true
		if dev != nil && dev.ClaudeConfig != nil {
			mountClaude = *dev.ClaudeConfig
		}
		if claudeConfigOverride != nil {
			mountClaude = *claudeConfigOverride
		}
		if mountClaude {
			cfg.Mounts = append(cfg.Mounts, docker.ClaudeConfigMounts()...)
		}
	}
	if dev == nil {
		return
	}
	cfg.RunArgs = append(cfg.RunArgs, dev.RunArgs...)
	cfg.Mounts = append(cfg.Mounts, dev.Mounts...)
	if len(dev.ContainerEnv) > 0 && cfg.ContainerEnv == nil {
		cfg.ContainerEnv = map[string]string{}
	}
	for k, v := range dev.ContainerEnv {
		cfg.ContainerEnv[k] = v
	}
}

// Reconcile squares a session row with the docker daemon. The row's status
// and container_id are only ever written by kanban's own lifecycle calls,
// so a container that dies underneath us — a host reboot, a docker restart,
// an OOM kill, a manual `docker rm` — leaves the row claiming a live
// session with nothing behind it. Every consumer of that row then acts on
// the lie: `kanban ticket attach` skips the start and the exec fails with
// "container … is not running".
//
// When the recorded container is gone or not running, the session is
// stopped through the normal Stop path (which clears the container id and
// tears down brokers and port proxies), and the refreshed row is returned so
// callers see a stopped session they can start. Rows that don't claim a live
// container (stopped, error, no container id) and in-flight starts are left
// alone, as is any row whose container can't be inspected for a reason other
// than "not found": the daemon being unreachable is not evidence the
// container is gone, and whatever the caller does next fails loudly anyway.
func (m *Manager) Reconcile(ctx context.Context, sess *db.Session) (*db.Session, error) {
	if m.containerRunning == nil || sess.ContainerID == nil || *sess.ContainerID == "" {
		return sess, nil
	}
	switch sess.Status {
	case db.SessionStatusStopped, db.SessionStatusError, db.SessionStatusStarting:
		return sess, nil
	}
	running, err := m.containerRunning(ctx, *sess.ContainerID)
	if err != nil {
		log.Printf("session %d: inspect container %s: %v", sess.ID, *sess.ContainerID, err)
		return sess, nil
	}
	if running {
		return sess, nil
	}
	log.Printf("session %d: container %s is no longer running; marking the session stopped", sess.ID, *sess.ContainerID)
	if err := m.Stop(ctx, sess.ID); err != nil {
		return nil, err
	}
	return m.store.GetSession(ctx, sess.ID)
}

// endPTYTimeout bounds how long a harness switch waits for the old agent to
// go away: endPTYScript gives it about five seconds after SIGHUP.
const endPTYTimeout = 15 * time.Second

// StopAgentUnless stops the session's running agent (the harness CLI) unless
// it was launched with harnessID, so the next agent attach starts that
// harness instead. The container, worktree and shell are left alone. Reports
// whether an agent was stopped.
//
// Closing the broker only drops kanban's end of the exec: Docker doesn't end
// a TTY exec when its client goes away, so the old agent would otherwise keep
// running unseen. Its process is sent SIGHUP (as a closed terminal would) and
// then SIGKILL if it lingers. A working/awaiting_perm status it reported is
// reset to idle, since it will never send the matching idle.
func (m *Manager) StopAgentUnless(ctx context.Context, sessionID int64, harnessID string) bool {
	b := m.brokers.closeAgentUnless(sessionID, harnessID)
	if b == nil {
		return false
	}
	// Finish the job even if the request that asked for it goes away.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), endPTYTimeout)
	defer cancel()
	if m.endPTY != nil {
		if err := m.endPTY(ctx, b.containerID, b.marker); err != nil {
			log.Printf("session %d: stop %s agent: %v", sessionID, b.harness, err)
		}
	}
	if _, err := m.store.ResetSessionActivity(ctx, sessionID); err != nil {
		log.Printf("session %d: reset status after stopping agent: %v", sessionID, err)
	}
	return true
}

// endPTYScript signals the exec tagged KANBAN_PTY_ID=$1: SIGHUP, then
// SIGKILL if it is still there about five seconds later. Only the exec's
// own process (the one whose parent is outside the container, PPid 0) is
// signalled; its foreground job gets SIGHUP from the kernel when it exits,
// the same as closing a terminal. Prints the pids it signalled.
const endPTYScript = `m="KANBAN_PTY_ID=$1"
tagged() { tr '\0' '\n' 2>/dev/null <"/proc/$1/environ" | grep -qxF "$m"; }
for s in /proc/[0-9]*/status; do
	grep -q '^PPid:[[:space:]]*0$' "$s" 2>/dev/null || continue
	p=${s#/proc/}
	p=${p%/status}
	tagged "$p" || continue
	echo "$p"
	kill -HUP "$p" 2>/dev/null
	i=0
	while tagged "$p" && [ "$i" -lt 10 ]; do
		sleep 0.5 2>/dev/null || sleep 1
		i=$((i + 1))
	done
	if tagged "$p"; then kill -KILL "$p" 2>/dev/null; fi
done
exit 0
`

// execEndPTY is the default endPTY: it runs endPTYScript in the container.
func (m *Manager) execEndPTY(ctx context.Context, containerID, marker string) error {
	out, err := m.docker.ExecRun(ctx, containerID, []string{"sh", "-c", endPTYScript, "sh", marker})
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return fmt.Errorf("no process tagged %s=%s in container %s", ptyMarkerEnv, marker, containerID)
	}
	return nil
}

// Stop tears down the devcontainer; worktree is preserved.
func (m *Manager) Stop(ctx context.Context, sessionID int64) error {
	sess, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	// Tear down any persistent PTY broker so its hijacked exec connection is
	// closed before we kill the container underneath it.
	m.brokers.closeFor(sessionID)
	if sess.ContainerID != nil && *sess.ContainerID != "" {
		_ = m.docker.StopContainer(ctx, *sess.ContainerID, 10*time.Second)
		_ = m.docker.RemoveContainer(ctx, *sess.ContainerID)
	}
	now := time.Now().Unix()
	sess.Status = db.SessionStatusStopped
	sess.StoppedAt = &now
	cleared := ""
	sess.ContainerID = &cleared
	if err := m.store.UpdateSessionLifecycle(ctx, sess.ID, sess.Status, sess.ContainerID, sess.StartedAt, sess.StoppedAt, nil); err != nil {
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

// Restart stops the session's container (if any) and starts it again. The
// session row, worktree, branch, and port allocations are preserved. Returns
// the refreshed session.
func (m *Manager) Restart(ctx context.Context, sessionID int64, onPullProgress docker.PullProgressFunc) (*db.Session, error) {
	if err := m.Stop(ctx, sessionID); err != nil {
		return nil, err
	}
	return m.Start(ctx, sessionID, onPullProgress)
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
	paths := ResolvePaths(board, sess)
	if paths.HasRepo && sess.WorktreePath != "" {
		_ = git.RemoveWorktree(paths.RepoPath, sess.WorktreePath)
		_ = os.RemoveAll(sess.WorktreePath)
		if sess.BranchName != "" {
			_ = git.DeleteBranch(paths.RepoPath, sess.BranchName)
		}
	}
	return m.store.DeleteSession(ctx, sess.ID)
}

// Sync brings the session's branch up to date with the board's base branch
// using either "rebase" or "merge". Aborts on conflict and surfaces the error.
func (m *Manager) Sync(ctx context.Context, sessionID int64, strategy string) error {
	sess, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	board, err := m.boardForSession(ctx, sess)
	if err != nil {
		return err
	}
	paths := ResolvePaths(board, sess)
	if !paths.HasRepo {
		return fmt.Errorf("session has no associated repository")
	}
	if sess.WorktreePath == "" {
		return fmt.Errorf("session has no worktree")
	}
	clean, err := git.IsClean(sess.WorktreePath)
	if err != nil {
		return fmt.Errorf("check worktree clean: %w", err)
	}
	if !clean {
		return fmt.Errorf("worktree has uncommitted changes; commit or stash before syncing")
	}
	switch strategy {
	case "rebase":
		if err := git.Rebase(sess.WorktreePath, board.BaseBranch); err != nil {
			git.RebaseAbort(sess.WorktreePath)
			return fmt.Errorf("rebase aborted: %w", err)
		}
	case "merge":
		if err := git.Merge(sess.WorktreePath, board.BaseBranch); err != nil {
			git.MergeAbort(sess.WorktreePath)
			return fmt.Errorf("merge aborted: %w", err)
		}
	default:
		return fmt.Errorf("unknown strategy %q (want rebase or merge)", strategy)
	}
	return nil
}

// Merge integrates the session's branch into the board's base branch in the
// source repo. The source repo must be clean and have base_branch checked out.
// On any git failure the source repo and worktree are restored to their
// pre-merge state. Strategy is one of "merge-commit", "squash", "rebase".
func (m *Manager) Merge(ctx context.Context, sessionID int64, strategy string) error {
	sess, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	board, err := m.boardForSession(ctx, sess)
	if err != nil {
		return err
	}
	paths := ResolvePaths(board, sess)
	if !paths.HasRepo {
		return fmt.Errorf("session has no associated repository")
	}
	if sess.WorktreePath == "" || sess.BranchName == "" {
		return fmt.Errorf("session has no worktree")
	}
	ticket, err := m.store.GetTicket(ctx, sess.TicketID)
	if err != nil {
		return err
	}

	id := git.Identity{Name: board.GitAuthorName, Email: board.GitAuthorEmail}
	if clean, err := git.IsClean(sess.WorktreePath); err != nil {
		return fmt.Errorf("check worktree clean: %w", err)
	} else if !clean {
		if err := git.AddAll(sess.WorktreePath); err != nil {
			return fmt.Errorf("stage pending changes: %w", err)
		}
		msg := ticket.Title
		if mc := kanbantoml.Load(paths.RepoPath).Merge; mc != nil && mc.AICommitMessage != nil && *mc.AICommitMessage {
			h := harness.ForSession(sess.Harness, paths.RepoPath)
			if generated, err := m.generateCommitMessage(ctx, sess, h, ticket.Title); err == nil {
				msg = generated
			} else {
				log.Printf("merge: ai commit message unavailable, using ticket title: %v", err)
			}
		}
		if err := git.Commit(sess.WorktreePath, msg, id); err != nil {
			return fmt.Errorf("commit pending changes: %w", err)
		}
	}
	// Tracked files only: this gate exists because MergeSquash/ResetHard can
	// discard uncommitted work, and neither touches untracked files. An
	// untracked scratch dir is no reason to refuse the merge — if it would
	// collide with an incoming path, git itself refuses with a clear error.
	if clean, err := git.IsCleanTracked(paths.RepoPath); err != nil {
		return fmt.Errorf("check source repo clean: %w", err)
	} else if !clean {
		return fmt.Errorf("source repo has uncommitted changes; commit or stash before merging")
	}
	cur, err := git.CurrentBranch(paths.RepoPath)
	if err != nil {
		return fmt.Errorf("read source repo branch: %w", err)
	}
	if cur != board.BaseBranch {
		return fmt.Errorf("source repo must have %s checked out (currently on %q)", board.BaseBranch, cur)
	}
	baseHead, err := git.CurrentHead(paths.RepoPath, "HEAD")
	if err != nil {
		return fmt.Errorf("read base head: %w", err)
	}

	switch strategy {
	case "merge-commit":
		if err := git.MergeNoFF(paths.RepoPath, sess.BranchName, id); err != nil {
			git.MergeAbort(paths.RepoPath)
			return fmt.Errorf("merge aborted: %w", err)
		}
	case "squash":
		msg := fmt.Sprintf("%s (#%d)", ticket.Title, ticket.ID)
		if err := git.MergeSquash(paths.RepoPath, sess.BranchName, msg, id); err != nil {
			git.MergeAbort(paths.RepoPath)
			git.ResetHard(paths.RepoPath, baseHead)
			return fmt.Errorf("squash aborted: %w", err)
		}
	case "rebase":
		if err := git.Rebase(sess.WorktreePath, board.BaseBranch); err != nil {
			git.RebaseAbort(sess.WorktreePath)
			return fmt.Errorf("rebase aborted: %w", err)
		}
		if err := git.MergeFFOnly(paths.RepoPath, sess.BranchName); err != nil {
			return fmt.Errorf("fast-forward aborted: %w", err)
		}
	default:
		return fmt.Errorf("unknown strategy %q (want merge-commit, squash, or rebase)", strategy)
	}
	return nil
}

// generateCommitMessage renders the harness's commit-message script, runs it
// inside the session's container with the staged diff piped via stdin, and
// returns the trimmed first line of stdout. Returns an error (so the caller
// can fall back to the ticket title) when the container is not running, the
// harness has no template, or the script fails.
func (m *Manager) generateCommitMessage(ctx context.Context, sess *db.Session, h harness.Harness, ticketTitle string) (string, error) {
	if sess.ContainerID == nil || *sess.ContainerID == "" {
		return "", fmt.Errorf("container not running")
	}
	prompt := fmt.Sprintf(
		"Write a one-line git commit message in imperative mood for the staged diff piped via stdin. The change is for the ticket %q. Output only the commit message text - no preamble, no quotes, no markdown, no code fences.",
		ticketTitle,
	)
	script, err := h.RenderCommitScript(prompt, sess.WorkspaceDir())
	if err != nil {
		return "", err
	}
	if script == "" {
		return "", fmt.Errorf("harness %q has no commit-message template", h.ID)
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	out, err := m.docker.ExecRun(cctx, *sess.ContainerID, []string{"sh", "-lc", script})
	if err != nil {
		return "", err
	}
	msg := strings.TrimSpace(out)
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	msg = strings.Trim(msg, "\"' \t")
	if msg == "" {
		return "", fmt.Errorf("empty message")
	}
	return msg, nil
}

func (m *Manager) Proxies() *docker.ProxyManager { return m.proxies }

func (m *Manager) Docker() *docker.Client { return m.docker }

// checkProjectRoot validates a board's project_dir and confirms it names a
// real directory inside the worktree.
//
// The existence check is load-bearing: dockerd creates a missing WorkingDir
// at container-create time, and since the workspace is a bind mount it
// creates it on the host inside the worktree, owned by the container user.
// Every later `docker exec` still fails, because exec does not create
// directories — so the failure surfaces far from its cause.
//
// The validation re-runs against the raw column rather than the resolved
// paths. ResolvePaths clamps an escaping value to "" so it can never reach
// dockerd, but silently working from the repo root is not what the board
// asked for; a row written by an older binary or edited by hand should say so.
func checkProjectRoot(board *db.Board, paths ResolvedPaths, worktreePath string) error {
	// Callers reach here with a nil board when boardForSession failed, so
	// every message below goes through slug rather than board.Slug.
	slug := "<unknown>"
	if board != nil {
		slug = board.Slug
		if _, err := ValidateProjectDir(board.ProjectDir, board.RepoPath, board.MountPath); err != nil {
			return fmt.Errorf("board %q: %w", slug, err)
		}
	}
	projectRoot := paths.ProjectRoot(worktreePath)
	if projectRoot == worktreePath {
		return nil
	}
	if info, err := os.Stat(projectRoot); err != nil || !info.IsDir() {
		return fmt.Errorf("board %q has project_dir %q, but %s is not a directory in the worktree; fix the board's project directory or create it in the repo", slug, paths.ProjectDir, projectRoot)
	}
	return nil
}

func (m *Manager) boardForSession(ctx context.Context, sess *db.Session) (*db.Board, error) {
	t, err := m.store.GetTicket(ctx, sess.TicketID)
	if err != nil {
		return nil, err
	}
	return m.store.GetBoard(ctx, t.BoardID)
}
