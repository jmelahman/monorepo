// Package kanbantoml reads `.kanban.toml` from two layers — the user config
// at $XDG_CONFIG_HOME/kanban/config.toml (falling back to
// $HOME/.config/kanban/config.toml) and the per-repo file at
// <repoPath>/.kanban.toml or <repoPath>/.config/kanban/config.toml — and
// merges them with user-wins semantics.
//
// All scalar fields use *T so an absent key is distinguishable from a zero
// value: the user file only overrides keys it actually sets. Tasks merge by
// label — a user entry with the same label replaces the project entry;
// user-only labels are appended.
package kanbantoml

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type File struct {
	Board        *BoardSection        `toml:"board"`
	Harness      *HarnessSection      `toml:"harness"`
	Sync         *SyncSection         `toml:"sync"`
	Merge        *MergeSection        `toml:"merge"`
	GitHub       *GitHubSection       `toml:"github"`
	Worktrees    *WorktreesSection    `toml:"worktrees"`
	Git          *GitSection          `toml:"git"`
	Plans        *PlansSection        `toml:"plans"`
	Branches     *BranchesSection     `toml:"branches"`
	Devcontainer *DevcontainerSection `toml:"devcontainer"`
	Errors       *ErrorsSection       `toml:"errors"`
	BuildCop     *BuildCopSection     `toml:"buildcop"`
	DevToolbar   *DevToolbarSection   `toml:"dev_toolbar"`
	Tasks        []TaskEntry          `toml:"task"`
}

// BoardSection configures which board a CLI command means when it isn't given
// one. Default is a board id, slug, or name, and takes priority over inferring
// the board from the current directory.
type BoardSection struct {
	Default *string `toml:"default"`
}

type HarnessSection struct {
	ID *string `toml:"id"`
}

type WorktreesSection struct {
	Root *string `toml:"root"`
}

// GitSection configures how kanban makes its own commits. SignCommits, when
// true, lets the ambient git config's commit.gpgsign decide whether kanban's
// merge/squash commits are signed; when false (the default) kanban forces
// signing off, since the container usually has no signing key.
type GitSection struct {
	SignCommits *bool `toml:"sign_commits"`
}

// PlansSection configures the directory the in-app Plans tab reads. An
// absolute path (the default `~/.claude/plans` after `~`-expansion) is
// shared across every ticket. A relative path (e.g. `./plans`) is
// resolved against each session's worktree, so each ticket sees its own
// plans.
type PlansSection struct {
	Dir *string `toml:"dir"`
}

// BranchesSection sets defaults for the branch name new sessions get. Prefix
// is a literal string concatenated with "/" + ticket.Slug to form the full
// branch name. Per-board overrides in the boards table take precedence.
type BranchesSection struct {
	Prefix *string `toml:"prefix"`
}

// ErrorsSection toggles the in-process error-to-ticket reporter. Disabled by
// default — only developers maintaining the app typically want application
// errors mixed into their kanban boards.
type ErrorsSection struct {
	Enabled   *bool   `toml:"enabled"`
	BoardName *string `toml:"board_name"`
}

// BuildCopSection configures the GitHub Actions failure poller. Disabled by
// default. Each entry in Boards produces one auto-managed board scoped to a
// branch filter; the poller files a ticket when a job's failure rate over
// the rolling window exceeds the threshold.
type BuildCopSection struct {
	Enabled  *bool           `toml:"enabled"`
	Interval *string         `toml:"interval"`
	Boards   []BuildCopBoard `toml:"boards"`
}

// BuildCopBoard configures a single Build Cop board. RepoPath is the only
// required field; everything else has a default applied at resolve time.
// Branch == "" or "*" matches every branch.
type BuildCopBoard struct {
	Name                *string  `json:"name,omitempty" toml:"name"`
	RepoPath            *string  `json:"repo_path,omitempty" toml:"repo_path"`
	Branch              *string  `json:"branch,omitempty" toml:"branch"`
	FailureThreshold    *float64 `json:"failure_threshold,omitempty" toml:"failure_threshold"`
	MinRuns             *int     `json:"min_runs,omitempty" toml:"min_runs"`
	GreenStreakRequired *int     `json:"green_streak_required,omitempty" toml:"green_streak_required"`
	WindowDays          *int     `json:"window_days,omitempty" toml:"window_days"`
	FlakyThreshold      *int     `json:"flaky_threshold,omitempty" toml:"flaky_threshold"`
}

// DevToolbarSection toggles the in-app developer metrics widget — a small
// corner overlay showing FPS, JS heap, DOM/React-Query activity, and live
// connection state. Off by default: only developers debugging the frontend
// want it, so it stays hidden (no header button) for everyone else.
type DevToolbarSection struct {
	Enabled *bool `toml:"enabled"`
}

type SyncSection struct {
	AllowRebase *bool `toml:"allow_rebase"`
	AllowMerge  *bool `toml:"allow_merge"`
}

type MergeSection struct {
	AllowMergeCommit *bool `toml:"allow_merge_commit"`
	AllowSquash      *bool `toml:"allow_squash"`
	AllowRebase      *bool `toml:"allow_rebase"`
	// DefaultStrategy is the strategy used when a merge request omits one.
	// Unset means no default: the caller must name a strategy, unless the
	// board enables exactly one (then that one is implied).
	DefaultStrategy *string `toml:"default_strategy"`
	// AICommitMessage opts in to harness-generated commit messages for the
	// auto-commit kanban makes when a session has uncommitted changes at merge
	// time. Default false: kanban uses the ticket title.
	AICommitMessage *bool `toml:"ai_commit_message"`
}

// DevcontainerSection augments the loaded devcontainer.json. Mounts and
// run_args append to whatever the devcontainer.json already declares;
// container_env merges key-by-key with user values winning over both the
// project file and the devcontainer.json.
//
// DockerSocket and ClaudeConfig toggle host bind mounts on the *built-in*
// devcontainer only — hand-written devcontainer.json files are unaffected
// and manage their own mounts. Both default to true when unset.
//
// Image likewise only applies to the built-in devcontainer: it replaces the
// bundled image reference, while a hand-written devcontainer.json keeps
// whatever image or build it declares.
type DevcontainerSection struct {
	Image        *string           `toml:"image"`
	RunArgs      []string          `toml:"run_args"`
	Mounts       []string          `toml:"mounts"`
	ContainerEnv map[string]string `toml:"container_env"`
	DockerSocket *bool             `toml:"docker_socket"`
	ClaudeConfig *bool             `toml:"claude_config"`
}

type GitHubSection struct {
	AutoMove     *bool   `toml:"auto_move"`
	DraftColumn  *string `toml:"draft_column"`
	ReviewColumn *string `toml:"review_column"`
	DoneColumn   *string `toml:"done_column"`
	ClosedColumn *string `toml:"closed_column"`
}

type TaskEntry struct {
	Label         string `json:"label" toml:"label"`
	ContainerPort int    `json:"container_port" toml:"container_port"`
}

// UserPath returns the user-level config path. $KANBAN_CONFIG, when set,
// overrides the default location so callers (e.g. the `serve --config` flag)
// can point at an arbitrary file without threading the path through every
// caller of Load.
func UserPath() (string, error) {
	if path := os.Getenv("KANBAN_CONFIG"); path != "" {
		return path, nil
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "kanban", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kanban", "config.toml"), nil
}

// The per-repo config locations, relative to the repo (or monorepo
// subproject) directory. dotConfigProjectFile mirrors the user file's layout
// for repos that keep their dotfiles under .config/ (e.g. with undot, which
// then needs `nolink kanban` so it doesn't link the directory back into the
// root). A .kanban.toml that undot moved to .config/.kanban.toml is still
// found through its root symlink.
const (
	rootProjectFile      = ".kanban.toml"
	dotConfigProjectFile = ".config/kanban/config.toml"
)

// ProjectFiles returns the per-repo config locations, relative to the repo
// directory and slash-separated, in lookup order.
func ProjectFiles() []string {
	return []string{rootProjectFile, dotConfigProjectFile}
}

// ProjectPath returns the per-repo config path: the first of ProjectFiles that
// exists under repoPath. A dangling symlink counts as existing, so an undot
// link whose target was emptied away is written through rather than shadowed
// by a second file. When none exists, it names the file a write should create
// — the .config/ form if repoPath already has a .config directory, so a repo
// that tidied its root doesn't grow a new dotfile there, otherwise
// .kanban.toml. Empty repoPath returns "".
func ProjectPath(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	for _, name := range ProjectFiles() {
		path := filepath.Join(repoPath, filepath.FromSlash(name))
		if _, err := os.Lstat(path); err == nil {
			return path
		}
	}
	if fi, err := os.Stat(filepath.Join(repoPath, ".config")); err == nil && fi.IsDir() {
		return filepath.Join(repoPath, filepath.FromSlash(dotConfigProjectFile))
	}
	return filepath.Join(repoPath, rootProjectFile)
}

// ProjectFile is ProjectPath for a caller about to write the file: it
// resolves any symlinks and refuses a result outside repoPath. A repo's
// contents (upstream commits, or an agent's merged branch) can plant
// `.kanban.toml -> /anywhere`, and EditFile writes through links. See
// REGRESSIONS.md: "Project config links must stay inside the repo".
func ProjectFile(repoPath string) (string, error) {
	path := ProjectPath(repoPath)
	if path == "" {
		return "", nil
	}
	return resolveWithin(repoPath, path)
}

// resolveWithin returns path with every symlink resolved — including a
// dangling final link, whose target a write would create — and errors when
// the result isn't inside root (itself resolved, so a repo under a symlinked
// directory still contains its own files).
func resolveWithin(root, path string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := realPath(path)
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(realRoot, resolved); err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%s resolves to %s, outside %s", path, resolved, root)
	}
	return resolved, nil
}

// realPath is filepath.EvalSymlinks for a path that may not exist yet: it
// follows a dangling final link, then resolves the deepest existing ancestor
// so a symlinked parent directory (.config -> elsewhere) is accounted for.
func realPath(path string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}
	path, err := filepath.Abs(resolveLink(path))
	if err != nil {
		return "", err
	}
	missing := ""
	for dir := path; ; {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, missing), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path, nil
		}
		missing = filepath.Join(filepath.Base(dir), missing)
		dir = parent
	}
}

// readProjectAt reads the project file under dir, treating one whose
// symlinks lead outside root as absent: a worktree's agent controls its
// files, and the read happens on the host.
func readProjectAt(root, dir string) File {
	path := ProjectPath(dir)
	if path == "" {
		return File{}
	}
	resolved, err := resolveWithin(root, path)
	if err != nil {
		return File{}
	}
	return readFileAt(resolved)
}

// Load reads the user file and the project file (either may be missing or
// unparseable, in which case it is treated as empty) and returns a merged
// File where user values win.
func Load(repoPath string) File {
	return LoadFrom(repoPath, "")
}

// LoadFrom is Load with a monorepo subproject layered between the repo root
// and the user file, so precedence runs repo root < subproject < user.
//
// The subproject layer matters because a board scoped to a subdirectory runs
// its tasks from there: .vscode/tasks.json is discovered in the subproject,
// so the label -> port mapping that pairs with it has to come from the same
// place. Merging rather than replacing keeps repo-wide settings — a root
// [devcontainer] section, say — applying to every subproject that doesn't
// override them. An empty or repo-root-equal projectPath is a no-op.
func LoadFrom(repoPath, projectPath string) File {
	out := readProjectAt(repoPath, repoPath)
	if projectPath != "" && projectPath != repoPath {
		out = merge(out, readProjectAt(repoPath, projectPath))
	}
	user := File{}
	if path, err := UserPath(); err == nil {
		user = readFileAt(path)
	}
	return merge(out, user)
}

// DefaultBoard returns the board.default that applies to a command run in dir,
// or "" when none does. The user file wins, then the project file nearest dir,
// walking up to the repo at root. An empty value in any of them means "no
// default" and stops the lookup, so a subproject can opt out of its repo root's
// default and a user can opt out of a checked-in one. The board isn't known yet
// here, so unlike LoadFrom there's no project_dir to name the subproject layer.
// An empty root means dir is outside any repo: only the user file applies.
func DefaultBoard(root, dir string) string {
	if path, err := UserPath(); err == nil {
		if b := readFileAt(path).Board; b != nil && b.Default != nil {
			return *b.Default
		}
	}
	if root == "" {
		return ""
	}
	for {
		if b := readProjectAt(root, dir).Board; b != nil && b.Default != nil {
			return *b.Default
		}
		parent := filepath.Dir(dir)
		if dir == root || parent == dir {
			return ""
		}
		dir = parent
	}
}

func readFileAt(path string) File {
	if path == "" {
		return File{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}
	}
	var f File
	if err := toml.Unmarshal(data, &f); err != nil {
		return File{}
	}
	return f
}

// merge combines project (low priority) and user (high priority) into one
// File. Each scalar field falls through unless the user set it; tasks merge
// by label.
func merge(project, user File) File {
	out := File{}

	out.Board = mergeBoard(project.Board, user.Board)
	out.Harness = mergeHarness(project.Harness, user.Harness)
	out.Sync = mergeSync(project.Sync, user.Sync)
	out.Merge = mergeMerge(project.Merge, user.Merge)
	out.GitHub = mergeGitHub(project.GitHub, user.GitHub)
	out.Worktrees = mergeWorktrees(project.Worktrees, user.Worktrees)
	out.Git = mergeGit(project.Git, user.Git)
	out.Plans = mergePlans(project.Plans, user.Plans)
	out.Branches = mergeBranches(project.Branches, user.Branches)
	out.Devcontainer = mergeDevcontainer(project.Devcontainer, user.Devcontainer)
	out.Errors = mergeErrors(project.Errors, user.Errors)
	out.BuildCop = mergeBuildCop(project.BuildCop, user.BuildCop)
	out.DevToolbar = mergeDevToolbar(project.DevToolbar, user.DevToolbar)
	out.Tasks = mergeTasks(project.Tasks, user.Tasks)

	return out
}

// mergeSinglePtrField merges a config section that carries a single pointer
// field, with the user value winning over the project value when set. Returns
// nil when both inputs are nil. get/set access the lone field; a nil field
// (== zero) means "unset", matching the per-section merge rules.
func mergeSinglePtrField[S any, F comparable](p, u *S, zero F, get func(*S) F, set func(*S, F)) *S {
	if p == nil && u == nil {
		return nil
	}
	out := new(S)
	if p != nil {
		set(out, get(p))
	}
	if u != nil {
		if v := get(u); v != zero {
			set(out, v)
		}
	}
	return out
}

func mergeBoard(p, u *BoardSection) *BoardSection {
	return mergeSinglePtrField(p, u, (*string)(nil),
		func(s *BoardSection) *string { return s.Default },
		func(s *BoardSection, v *string) { s.Default = v })
}

func mergeWorktrees(p, u *WorktreesSection) *WorktreesSection {
	return mergeSinglePtrField(p, u, (*string)(nil),
		func(s *WorktreesSection) *string { return s.Root },
		func(s *WorktreesSection, v *string) { s.Root = v })
}

func mergeGit(p, u *GitSection) *GitSection {
	return mergeSinglePtrField(p, u, (*bool)(nil),
		func(s *GitSection) *bool { return s.SignCommits },
		func(s *GitSection, v *bool) { s.SignCommits = v })
}

func mergeDevToolbar(p, u *DevToolbarSection) *DevToolbarSection {
	return mergeSinglePtrField(p, u, (*bool)(nil),
		func(s *DevToolbarSection) *bool { return s.Enabled },
		func(s *DevToolbarSection, v *bool) { s.Enabled = v })
}

func mergePlans(p, u *PlansSection) *PlansSection {
	return mergeSinglePtrField(p, u, (*string)(nil),
		func(s *PlansSection) *string { return s.Dir },
		func(s *PlansSection, v *string) { s.Dir = v })
}

func mergeBranches(p, u *BranchesSection) *BranchesSection {
	return mergeSinglePtrField(p, u, (*string)(nil),
		func(s *BranchesSection) *string { return s.Prefix },
		func(s *BranchesSection, v *string) { s.Prefix = v })
}

func mergeErrors(p, u *ErrorsSection) *ErrorsSection {
	if p == nil && u == nil {
		return nil
	}
	out := ErrorsSection{}
	if p != nil {
		out.Enabled = p.Enabled
		out.BoardName = p.BoardName
	}
	if u != nil {
		if u.Enabled != nil {
			out.Enabled = u.Enabled
		}
		if u.BoardName != nil {
			out.BoardName = u.BoardName
		}
	}
	return &out
}

// mergeBuildCop overlays user-set fields atop project-set fields. The Boards
// slice is replaced wholesale when the user file sets it — there's no
// stable identity to merge entries by (board names can change), and the
// "user wins" rule is simpler than per-entry diffing. Project entries
// remain when the user file doesn't declare any.
func mergeBuildCop(p, u *BuildCopSection) *BuildCopSection {
	if p == nil && u == nil {
		return nil
	}
	out := BuildCopSection{}
	if p != nil {
		out.Enabled = p.Enabled
		out.Interval = p.Interval
		out.Boards = p.Boards
	}
	if u != nil {
		if u.Enabled != nil {
			out.Enabled = u.Enabled
		}
		if u.Interval != nil {
			out.Interval = u.Interval
		}
		if len(u.Boards) > 0 {
			out.Boards = u.Boards
		}
	}
	return &out
}

func mergeHarness(p, u *HarnessSection) *HarnessSection {
	return mergeSinglePtrField(p, u, (*string)(nil),
		func(s *HarnessSection) *string { return s.ID },
		func(s *HarnessSection, v *string) { s.ID = v })
}

func mergeSync(p, u *SyncSection) *SyncSection {
	if p == nil && u == nil {
		return nil
	}
	out := SyncSection{}
	if p != nil {
		out.AllowRebase = p.AllowRebase
		out.AllowMerge = p.AllowMerge
	}
	if u != nil {
		if u.AllowRebase != nil {
			out.AllowRebase = u.AllowRebase
		}
		if u.AllowMerge != nil {
			out.AllowMerge = u.AllowMerge
		}
	}
	return &out
}

func mergeMerge(p, u *MergeSection) *MergeSection {
	if p == nil && u == nil {
		return nil
	}
	out := MergeSection{}
	if p != nil {
		out.AllowMergeCommit = p.AllowMergeCommit
		out.AllowSquash = p.AllowSquash
		out.AllowRebase = p.AllowRebase
		out.AICommitMessage = p.AICommitMessage
		out.DefaultStrategy = p.DefaultStrategy
	}
	if u != nil {
		if u.AllowMergeCommit != nil {
			out.AllowMergeCommit = u.AllowMergeCommit
		}
		if u.AllowSquash != nil {
			out.AllowSquash = u.AllowSquash
		}
		if u.AllowRebase != nil {
			out.AllowRebase = u.AllowRebase
		}
		if u.AICommitMessage != nil {
			out.AICommitMessage = u.AICommitMessage
		}
		if u.DefaultStrategy != nil {
			out.DefaultStrategy = u.DefaultStrategy
		}
	}
	return &out
}

func mergeGitHub(p, u *GitHubSection) *GitHubSection {
	if p == nil && u == nil {
		return nil
	}
	out := GitHubSection{}
	if p != nil {
		out = *p
	}
	if u != nil {
		if u.AutoMove != nil {
			out.AutoMove = u.AutoMove
		}
		if u.DraftColumn != nil {
			out.DraftColumn = u.DraftColumn
		}
		if u.ReviewColumn != nil {
			out.ReviewColumn = u.ReviewColumn
		}
		if u.DoneColumn != nil {
			out.DoneColumn = u.DoneColumn
		}
		if u.ClosedColumn != nil {
			out.ClosedColumn = u.ClosedColumn
		}
	}
	return &out
}

func mergeDevcontainer(p, u *DevcontainerSection) *DevcontainerSection {
	if p == nil && u == nil {
		return nil
	}
	out := DevcontainerSection{}
	if p != nil {
		out.RunArgs = append(out.RunArgs, p.RunArgs...)
		out.Mounts = append(out.Mounts, p.Mounts...)
		for k, v := range p.ContainerEnv {
			if out.ContainerEnv == nil {
				out.ContainerEnv = map[string]string{}
			}
			out.ContainerEnv[k] = v
		}
		out.Image = p.Image
		out.DockerSocket = p.DockerSocket
		out.ClaudeConfig = p.ClaudeConfig
	}
	if u != nil {
		out.RunArgs = append(out.RunArgs, u.RunArgs...)
		out.Mounts = append(out.Mounts, u.Mounts...)
		for k, v := range u.ContainerEnv {
			if out.ContainerEnv == nil {
				out.ContainerEnv = map[string]string{}
			}
			out.ContainerEnv[k] = v
		}
		if u.Image != nil {
			out.Image = u.Image
		}
		if u.DockerSocket != nil {
			out.DockerSocket = u.DockerSocket
		}
		if u.ClaudeConfig != nil {
			out.ClaudeConfig = u.ClaudeConfig
		}
	}
	return &out
}

func mergeTasks(project, user []TaskEntry) []TaskEntry {
	if len(project) == 0 && len(user) == 0 {
		return nil
	}
	byLabel := make(map[string]int, len(project))
	out := make([]TaskEntry, 0, len(project)+len(user))
	for _, t := range project {
		byLabel[t.Label] = len(out)
		out = append(out, t)
	}
	for _, t := range user {
		if idx, ok := byLabel[t.Label]; ok {
			out[idx] = t
			continue
		}
		byLabel[t.Label] = len(out)
		out = append(out, t)
	}
	return out
}

// PortFor returns the container_port for the named task, if any.
func (f File) PortFor(label string) (int, bool) {
	for _, t := range f.Tasks {
		if t.Label == label && t.ContainerPort > 0 {
			return t.ContainerPort, true
		}
	}
	return 0, false
}

// WriteUserHarness sets (or clears, when id == "") the [harness].id key in
// the user config, preserving any other top-level keys already in the file.
// Creates the file (and parent directories) when needed; deletes the file
// when clearing leaves it empty. Thin wrapper over EditFile so it shares the
// per-file write lock and atomic write with the generic config API.
func WriteUserHarness(id string) error {
	path, err := UserPath()
	if err != nil {
		return err
	}
	if id == "" {
		return EditFile(path, nil, []string{"harness.id"})
	}
	return EditFile(path, map[string]any{"harness.id": id}, nil)
}

// WriteUserWorktreesRoot sets (or clears, when root == "") the
// [worktrees].root key in the user config.
func WriteUserWorktreesRoot(root string) error {
	path, err := UserPath()
	if err != nil {
		return err
	}
	if root == "" {
		return EditFile(path, nil, []string{"worktrees.root"})
	}
	return EditFile(path, map[string]any{"worktrees.root": root}, nil)
}

// WriteUserSignCommits sets the [git].sign_commits key in the user config when
// enabled, or clears the [git] section when disabled (the default), keeping the
// file minimal.
func WriteUserSignCommits(enabled bool) error {
	path, err := UserPath()
	if err != nil {
		return err
	}
	if !enabled {
		return EditFile(path, nil, []string{"git.sign_commits"})
	}
	return EditFile(path, map[string]any{"git.sign_commits": true}, nil)
}
