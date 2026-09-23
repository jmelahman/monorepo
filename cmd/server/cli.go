package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/internal/client"
)

// addClientCommands attaches the user-facing CLI subcommand groups
// (`board`, `ticket`, `column`, `session`) to root. They're thin wrappers
// over the kanban HTTP API, useful for scripting and remote control of a
// running `kanban serve`. For dockerized deploys curl against the REST API
// is friendlier (no host install).
func addClientCommands(root *cobra.Command) {
	root.AddCommand(boardCmd(), ticketCmd(), columnCmd(), sessionCmd(), configCmd(), envCmd())
}

// resolveURL returns the effective server URL for a leaf command. KANBAN_URL
// wins only when the user didn't explicitly pass --server.
func resolveURL(cmd *cobra.Command, serverURL string) string {
	if env := os.Getenv("KANBAN_URL"); env != "" && !cmd.Flags().Changed("server") {
		return env
	}
	return serverURL
}

// addServerFlag registers --server as a persistent flag on the parent group
// so every leaf inherits it. The same string variable is reused across the
// group's subcommands.
func addServerFlag(parent *cobra.Command, dst *string) {
	parent.PersistentFlags().StringVar(dst, "server", "http://localhost:7474",
		"Base URL of the kanban HTTP server")
}

// ---------- board ----------

func boardCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{
		Use:   "board",
		Short: "Manage kanban boards",
	}
	addServerFlag(parent, &serverURL)

	list := &cobra.Command{
		Use:   "list",
		Short: "List kanban boards as a table",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoardList(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout())
		},
	}

	var (
		bcName, bcRepo, bcMount, bcWorktreeRoot, bcBaseBranch, bcBranchPrefix string
		bcJSON                                                                bool
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a new board",
		Long: `Create a new board.

Run inside a git repository, every flag is optional: --repo-path defaults
to the repo containing the current directory (resolved to the main working
tree when run from a linked worktree) and --name to that directory's name.
The server detects the base branch and worktree root from the repo.
Explicit flags always win; inference only fills in what you omit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := inferBoardCreateArgs(client.CreateBoardArgs{
				Name:         bcName,
				RepoPath:     bcRepo,
				MountPath:    bcMount,
				WorktreeRoot: bcWorktreeRoot,
				BaseBranch:   bcBaseBranch,
				BranchPrefix: bcBranchPrefix,
			})
			if err != nil {
				return err
			}
			return runBoardCreate(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), a, bcJSON)
		},
	}
	create.Flags().StringVar(&bcName, "name", "", "Board name (default: the repo directory's name)")
	create.Flags().StringVar(&bcRepo, "repo-path", "", "Path to the git repo on the host (default: the repo containing the current directory)")
	create.Flags().StringVar(&bcMount, "mount-path", "", "Mount path inside session containers (alternative to --repo-path)")
	create.Flags().StringVar(&bcWorktreeRoot, "worktree-root", "", "Override the parent directory for new session worktrees")
	create.Flags().StringVar(&bcBaseBranch, "base-branch", "", "Branch session worktrees fork from (default: detected from the repo)")
	create.Flags().StringVar(&bcBranchPrefix, "branch-prefix", "", "Optional prefix prepended to session branch names")
	create.Flags().BoolVar(&bcJSON, "json", false, "Print the full board JSON instead of a one-line summary")

	var bgJSON bool
	get := &cobra.Command{
		Use:   "get [id]",
		Short: "Print a single board (defaults to the board for the repo in the current directory)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := resolveURL(cmd, serverURL)
			ident, err := resolveBoardIdent(cmd.Context(), url, args)
			if err != nil {
				return err
			}
			return runBoardGet(cmd.Context(), url, cmd.OutOrStdout(), ident, bgJSON)
		},
	}
	get.Flags().BoolVar(&bgJSON, "json", false, "Print the full board JSON instead of a one-line summary")

	var (
		buName, buRepo, buMount, buWorktreeRoot, buBaseBranch, buBranchPrefix string
		buJSON                                                                bool
	)
	update := &cobra.Command{
		Use:   "update <id>",
		Short: "Update fields on a board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := client.UpdateBoardArgs{}
			if cmd.Flags().Changed("name") {
				a.Name = &buName
			}
			if cmd.Flags().Changed("repo-path") {
				a.RepoPath = &buRepo
			}
			if cmd.Flags().Changed("mount-path") {
				a.MountPath = &buMount
			}
			if cmd.Flags().Changed("worktree-root") {
				a.WorktreeRoot = &buWorktreeRoot
			}
			if cmd.Flags().Changed("base-branch") {
				a.BaseBranch = &buBaseBranch
			}
			if cmd.Flags().Changed("branch-prefix") {
				a.BranchPrefix = &buBranchPrefix
			}
			return runBoardUpdate(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0], a, buJSON)
		},
	}
	update.Flags().StringVar(&buName, "name", "", "Rename the board")
	update.Flags().StringVar(&buRepo, "repo-path", "", "Update repo path")
	update.Flags().StringVar(&buMount, "mount-path", "", "Update mount path")
	update.Flags().StringVar(&buWorktreeRoot, "worktree-root", "", "Update worktree root")
	update.Flags().StringVar(&buBaseBranch, "base-branch", "", "Update base branch")
	update.Flags().StringVar(&buBranchPrefix, "branch-prefix", "", "Update branch prefix")
	update.Flags().BoolVar(&buJSON, "json", false, "Print the full board JSON instead of a one-line summary")

	del := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a board (and destroy all its sessions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoardDelete(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0])
		},
	}

	state := &cobra.Command{
		Use:   "state [id]",
		Short: "Print full board state (board, columns, tickets, sessions) as JSON",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := resolveURL(cmd, serverURL)
			ident, err := resolveBoardIdent(cmd.Context(), url, args)
			if err != nil {
				return err
			}
			return runBoardState(cmd.Context(), url, cmd.OutOrStdout(), ident)
		},
	}

	archived := &cobra.Command{
		Use:   "archived [id]",
		Short: "List archived tickets on a board",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := resolveURL(cmd, serverURL)
			ident, err := resolveBoardIdent(cmd.Context(), url, args)
			if err != nil {
				return err
			}
			return runBoardArchived(cmd.Context(), url, cmd.OutOrStdout(), ident)
		},
	}

	archivedClear := &cobra.Command{
		Use:   "archived-clear <id>",
		Short: "Permanently delete every archived ticket on a board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoardArchivedClear(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0])
		},
	}

	parent.AddCommand(list, create, get, update, del, state, archived, archivedClear)
	return parent
}

func runBoardList(ctx context.Context, url string, out io.Writer) error {
	boards, err := client.New(url, nil).ListBoards(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSLUG\tNAME")
	for _, b := range boards {
		fmt.Fprintf(tw, "%d\t%s\t%s\n", b.ID, b.Slug, b.Name)
	}
	return tw.Flush()
}

func runBoardCreate(ctx context.Context, url string, out io.Writer, args client.CreateBoardArgs, asJSON bool) error {
	raw, err := client.New(url, nil).CreateBoard(ctx, args)
	if err != nil {
		return err
	}
	return printBoardSummary(out, raw, asJSON)
}

func runBoardGet(ctx context.Context, url string, out io.Writer, ident string, asJSON bool) error {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	raw, err := c.GetBoard(ctx, id)
	if err != nil {
		return err
	}
	return printBoardSummary(out, raw, asJSON)
}

func runBoardUpdate(ctx context.Context, url string, out io.Writer, ident string, args client.UpdateBoardArgs, asJSON bool) error {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	raw, err := c.UpdateBoard(ctx, id, args)
	if err != nil {
		return err
	}
	return printBoardSummary(out, raw, asJSON)
}

func runBoardDelete(ctx context.Context, url string, out io.Writer, ident string) error {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	if err := c.DeleteBoard(ctx, id); err != nil {
		return err
	}
	fmt.Fprintf(out, "deleted board %d\n", id)
	return nil
}

func runBoardState(ctx context.Context, url string, out io.Writer, ident string) error {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	raw, err := c.BoardState(ctx, id)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(raw))
	return err
}

func runBoardArchived(ctx context.Context, url string, out io.Writer, ident string) error {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	raw, err := c.ListArchived(ctx, id)
	if err != nil {
		return err
	}
	var tickets []client.Ticket
	if err := json.Unmarshal(raw, &tickets); err != nil {
		_, perr := fmt.Fprintln(out, string(raw))
		return perr
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSLUG\tTITLE")
	for _, t := range tickets {
		fmt.Fprintf(tw, "%d\t%s\t%s\n", t.ID, t.Slug, t.Title)
	}
	return tw.Flush()
}

func runBoardArchivedClear(ctx context.Context, url string, out io.Writer, ident string) error {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	if err := c.DeleteArchived(ctx, id); err != nil {
		return err
	}
	fmt.Fprintf(out, "cleared archived tickets on board %d\n", id)
	return nil
}

// printBoardSummary prints either the raw JSON or a one-line summary derived
// from id/slug/name.
func printBoardSummary(out io.Writer, raw json.RawMessage, asJSON bool) error {
	if asJSON {
		_, err := fmt.Fprintln(out, string(raw))
		return err
	}
	var b struct {
		ID         int64  `json:"id"`
		Slug       string `json:"slug"`
		Name       string `json:"name"`
		RepoPath   string `json:"repo_path"`
		MountPath  string `json:"mount_path"`
		BaseBranch string `json:"base_branch"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		_, perr := fmt.Fprintln(out, string(raw))
		return perr
	}
	// Echo the resolved repo and base branch so inferred values are visible.
	line := fmt.Sprintf("#%d %s (%s)", b.ID, b.Name, b.Slug)
	switch {
	case b.RepoPath != "":
		line += " repo=" + b.RepoPath
	case b.MountPath != "":
		line += " mount=" + b.MountPath
	}
	if b.BaseBranch != "" {
		line += " base=" + b.BaseBranch
	}
	fmt.Fprintln(out, line)
	return nil
}

// inferBoardCreateArgs fills the gaps a bare `kanban board create` leaves.
// With neither --repo-path nor --mount-path it resolves the git repo
// containing the current directory, and with no --name it uses the repo
// directory's basename. Explicit flags always win; inference only fills
// what's missing. The inferred path is only meaningful when the CLI and
// the server share a filesystem — same as a hand-typed --repo-path.
func inferBoardCreateArgs(a client.CreateBoardArgs) (client.CreateBoardArgs, error) {
	if a.RepoPath == "" && a.MountPath == "" {
		repo, err := cwdRepoRoot()
		if err != nil {
			return a, fmt.Errorf("cannot infer --repo-path: %w (run inside a git repo or pass --repo-path/--mount-path)", err)
		}
		a.RepoPath = repo
	}
	if strings.TrimSpace(a.Name) == "" {
		if a.RepoPath == "" {
			return a, fmt.Errorf("--name required when --mount-path is set without --repo-path")
		}
		a.Name = filepath.Base(a.RepoPath)
	}
	return a, nil
}

// cwdRepoRoot returns the root of the git working tree containing the
// current directory. From a linked worktree it resolves to the main working
// tree instead: session worktrees fork from the board's repo_path, and a
// board pointed at a disposable worktree breaks when that worktree is
// removed.
func cwdRepoRoot() (string, error) {
	top, err := gitRevParse("--show-toplevel")
	if err != nil {
		return "", err
	}
	common, err := gitRevParse("--git-common-dir")
	if err != nil {
		return "", err
	}
	// The common dir ends in "/.git" for a normal repo (shared by all its
	// worktrees); anything else (e.g. a bare repo) keeps the cwd's toplevel.
	if abs, err := filepath.Abs(common); err == nil && filepath.Base(abs) == ".git" {
		return filepath.Dir(abs), nil
	}
	return top, nil
}

func gitRevParse(flag string) (string, error) {
	out, err := exec.Command("git", "rev-parse", flag).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("git rev-parse %s: %s", flag, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git rev-parse %s: %w", flag, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveBoardIdent returns the board identifier for a command whose [id]
// arg is optional: the explicit arg when given, otherwise the id of the
// board whose repo_path is the git repo containing the current directory.
// Zero or several matching boards is an error, never a guess — boards may
// legitimately share a repo (e.g. Build Cop boards).
func resolveBoardIdent(ctx context.Context, url string, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	repo, err := cwdRepoRoot()
	if err != nil {
		return "", fmt.Errorf("no board id given and cannot infer one: %w (run inside a board's git repo or pass an id/slug)", err)
	}
	boards, err := client.New(url, nil).ListBoards(ctx)
	if err != nil {
		return "", err
	}
	var matches []client.Board
	for _, b := range boards {
		if b.RepoPath != "" && samePath(b.RepoPath, repo) {
			matches = append(matches, b)
		}
	}
	switch len(matches) {
	case 1:
		return strconv.FormatInt(matches[0].ID, 10), nil
	case 0:
		return "", fmt.Errorf("no board has repo path %s; pass an id or slug", repo)
	default:
		slugs := make([]string, len(matches))
		for i, b := range matches {
			slugs[i] = b.Slug
		}
		return "", fmt.Errorf("%d boards use repo %s (%s); pass an id or slug", len(matches), repo, strings.Join(slugs, ", "))
	}
}

// samePath reports whether two paths name the same location, tolerating
// unresolved symlinks on either side (e.g. a board created with a symlinked
// repo path).
func samePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}

// ---------- ticket ----------

func ticketCmd() *cobra.Command {
	var serverURL, boardIdent string
	parent := &cobra.Command{
		Use:   "ticket",
		Short: "Manage tickets on a kanban board",
		Long: `Manage tickets on a kanban board.

Every subcommand takes the ticket id as an optional positional argument.
Without one, the board's tickets are listed for you to pick from: the
board is inferred from the git repo containing the current directory (an
error if zero or several boards use that repo) unless --board names one.
Typing narrows the list; Enter runs the subcommand on the highlighted
ticket, Esc cancels. The list needs a real terminal, so in scripts and
pipes the id is required.`,
	}
	addServerFlag(parent, &serverURL)
	// --board is persistent so every subcommand's picker can be pointed at a
	// board from outside its repo with the same flag.
	parent.PersistentFlags().StringVar(&boardIdent, "board", "",
		"Board id or slug (default: the board for the repo in the current directory)")

	var (
		tcTitle, tcBody, tcColumn, tcDetachKeys, tcHarness string
		tcJSON, tcAttach                                   bool
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a ticket, interactively when run without --title, and drop into its agent",
		Long: `Create a ticket on a board.

Without --board the board is inferred from the git repo containing the
current directory (an error if zero or several boards use that repo).

Without --title, a terminal form asks for a title and an optional
description; on submit the ticket is created, its session is started, and
your terminal attaches to the agent running inside the devcontainer (see
"kanban ticket attach"). The form's Harness row picks which agent CLI the
session launches: Tab to it and use Left/Right to cycle. With --title the
command is non-interactive and only prints the created ticket unless
--attach is also given; --harness picks the agent there.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, serverURL)
			out := cmd.OutOrStdout()

			board, err := resolveBoardIdent(ctx, url, boardArgs(boardIdent))
			if err != nil {
				return err
			}

			interactive := strings.TrimSpace(tcTitle) == ""
			attach := tcAttach
			if !cmd.Flags().Changed("attach") {
				attach = interactive
			}
			// Both the form and the attach need a real terminal; say so
			// before creating anything rather than half-way through.
			if interactive && !stdinIsTerminal() {
				return errors.New("--title is required when not running in an interactive terminal")
			}
			if attach && !stdinIsTerminal() {
				return errors.New("--attach needs an interactive terminal on stdin and stdout")
			}
			if tcHarness != "" && !attach {
				return errors.New("--harness needs --attach: the harness is picked when the ticket's session starts")
			}
			if err := validateHarnessFlag(ctx, url, tcHarness); err != nil {
				return err
			}
			a := client.CreateTicketArgs{Board: board, Title: tcTitle, Body: tcBody, Column: tcColumn}
			harnessID := tcHarness
			if interactive {
				label, err := boardLabel(ctx, url, board)
				if err != nil {
					return err
				}
				// The harness row only means something when we go on to
				// start the session and attach to its agent.
				var opts *harnessOptions
				if attach {
					if opts, err = loadHarnessOptions(ctx, url, board); err != nil {
						return err
					}
				}
				res, ok, err := promptTicketForm(label, tcBody, opts, tcHarness)
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("cancelled; no ticket created")
				}
				a.Title, a.Body = res.Title, res.Body
				harnessID = formHarness(opts, tcHarness, res.Harness)
			}

			id, err := runTicketCreate(ctx, url, out, a, tcJSON)
			if err != nil {
				return err
			}
			if !attach {
				return nil
			}
			return runTicketAttach(ctx, url, out, id, "agent", tcDetachKeys, harnessID)
		},
	}
	create.Flags().StringVar(&tcTitle, "title", "", "Ticket title (omit to be prompted)")
	create.Flags().StringVar(&tcBody, "body", "", "Ticket body / description (pre-fills the prompt when --title is omitted)")
	create.Flags().StringVar(&tcColumn, "column", "", "Column name or id (default: leftmost column)")
	create.Flags().BoolVar(&tcJSON, "json", false, "Print the full ticket JSON instead of a one-line summary")
	create.Flags().BoolVar(&tcAttach, "attach", false, "Start the ticket's session and attach to its agent after creating (default: true when prompted, false with --title)")
	create.Flags().StringVar(&tcDetachKeys, "detach-keys", defaultDetachKeys, "Key sequence that detaches from the agent, docker-style (e.g. ctrl-p,ctrl-q or ctrl-])")
	create.Flags().StringVar(&tcHarness, "harness", "", "Agent harness for the ticket's session, e.g. claude or pi (default: the user/project harness; needs --attach)")

	var tiJSON bool
	info := &cobra.Command{
		Use:   "info [id]",
		Short: "Show a ticket's details, with each value copyable to the clipboard",
		Long: `Show everything the web UI's Info tab does for a ticket: its description
and board placement, its session and container, the worktree and branch,
any pull request, and the ports its session has allocated.

On a terminal this opens a full-screen view. Up/Down move between values,
Enter (or "c" / "y") copies the highlighted one to the clipboard, and Esc
closes. Piped or redirected it prints the same fields as plain text, and
--json prints them as a single JSON object.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, serverURL)
			id, err := ticketArg(ctx, url, args, boardIdent, pickerAction{"Ticket info", "show"}, false)
			if err != nil {
				return err
			}
			return runTicketInfo(ctx, url, cmd.OutOrStdout(), id, tiJSON)
		},
	}
	info.Flags().BoolVar(&tiJSON, "json", false, "Print the ticket's info as JSON instead of opening the viewer")

	var (
		taDetachKeys, taHarness string
		taShell                 bool
	)
	attach := &cobra.Command{
		Use:   "attach [id]",
		Short: "Attach your terminal to a ticket's agent (starting its session if needed)",
		Long: `Attach the current terminal to the agent running in a ticket's session
container, the same PTY the web UI shows. The session is created and
started first if it isn't running. Input is forwarded as typed; the
terminal size follows your window.

Without an id, the board's open tickets are listed for you to pick from:
the board is inferred from the git repo containing the current directory
(an error if zero or several boards use that repo) unless --board names
one. Typing narrows the list; Enter attaches, Esc cancels. Left/Right
cycle the agent harness for the highlighted ticket.

Picking a different harness (in the list, or with --harness) stores it on
the ticket's session. If its agent is already running under another
harness, that agent is stopped (hung up, then killed if it lingers) and the
new one launched; the container, worktree and shell are left alone.

Detaching (default ctrl-p,ctrl-q) leaves the agent running — reattach any
time, or keep using it from the web UI. With --shell an interactive login
shell in the container is attached instead of the agent.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, serverURL)
			// A bad --detach-keys would only fail after the list; reject it
			// before anything is fetched or drawn.
			if _, err := parseDetachKeys(taDetachKeys); err != nil {
				return err
			}
			if taShell && taHarness != "" {
				return errors.New("--harness picks the agent; it can't be combined with --shell")
			}
			if err := validateHarnessFlag(ctx, url, taHarness); err != nil {
				return err
			}
			action := pickerAction{"Attach to ticket", "attach"}
			id, harnessID := int64(0), taHarness
			var err error
			if len(args) > 0 || taShell || taHarness != "" {
				// Nothing to pick a harness for: an explicit id, the shell,
				// or --harness already decided it.
				id, err = ticketArg(ctx, url, args, boardIdent, action, false)
			} else {
				var item pickerItem
				item, err = pickTicketItem(ctx, url, boardIdent, action, false, true)
				id, harnessID = item.ID, item.Harness
			}
			if err != nil {
				return err
			}
			kind := "agent"
			if taShell {
				kind = "shell"
			}
			return runTicketAttach(ctx, url, cmd.OutOrStdout(), id, kind, taDetachKeys, harnessID)
		},
	}
	attach.Flags().BoolVar(&taShell, "shell", false, "Attach an interactive shell in the session container instead of the agent")
	attach.Flags().StringVar(&taDetachKeys, "detach-keys", defaultDetachKeys, "Key sequence that detaches, docker-style (e.g. ctrl-p,ctrl-q or ctrl-])")
	attach.Flags().StringVar(&taHarness, "harness", "", "Switch the ticket's agent to this harness, e.g. claude or pi (restarts a running agent on another harness)")

	var (
		tuTitle, tuBody string
		tuJSON          bool
	)
	update := &cobra.Command{
		Use:   "update [id]",
		Short: "Update title/body of a ticket",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, serverURL)
			id, err := ticketArg(ctx, url, args, boardIdent, pickerAction{"Update ticket", "update"}, false)
			if err != nil {
				return err
			}
			a := client.UpdateTicketArgs{}
			if cmd.Flags().Changed("title") {
				a.Title = &tuTitle
			}
			if cmd.Flags().Changed("body") {
				a.Body = &tuBody
			}
			return runTicketUpdate(ctx, url, cmd.OutOrStdout(), id, a, tuJSON)
		},
	}
	update.Flags().StringVar(&tuTitle, "title", "", "New title")
	update.Flags().StringVar(&tuBody, "body", "", "New body")
	update.Flags().BoolVar(&tuJSON, "json", false, "Print the full ticket JSON instead of a one-line summary")

	var (
		tmColumnID int64
		tmPosition int
	)
	move := &cobra.Command{
		Use:   "move [id]",
		Short: "Move a ticket to a different column / position",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, serverURL)
			id, err := ticketArg(ctx, url, args, boardIdent, pickerAction{"Move ticket", "move"}, false)
			if err != nil {
				return err
			}
			return runTicketMove(ctx, url, cmd.OutOrStdout(), id,
				client.MoveTicketArgs{ColumnID: tmColumnID, Position: tmPosition})
		},
	}
	move.Flags().Int64Var(&tmColumnID, "column-id", 0, "Target column id (numeric, required)")
	move.Flags().IntVar(&tmPosition, "position", 0, "Target position within the column (0-indexed)")
	_ = move.MarkFlagRequired("column-id")

	archive := simpleTicketCmd("archive", "Archive a ticket", pickerAction{"Archive ticket", "archive"}, false,
		&serverURL, &boardIdent,
		func(c *client.Client, ctx context.Context, id int64) error { return c.ArchiveTicket(ctx, id) })
	unarchive := simpleTicketCmd("unarchive", "Unarchive a ticket", pickerAction{"Unarchive ticket", "unarchive"}, true,
		&serverURL, &boardIdent,
		func(c *client.Client, ctx context.Context, id int64) error { return c.UnarchiveTicket(ctx, id) })
	delTicket := simpleTicketCmd("delete", "Permanently delete a ticket (must be archived first)", pickerAction{"Delete ticket", "delete"}, true,
		&serverURL, &boardIdent,
		func(c *client.Client, ctx context.Context, id int64) error { return c.DeleteTicket(ctx, id) })
	doneCmd := simpleTicketCmd("done", "Move a ticket to the rightmost column and stop its session", pickerAction{"Finish ticket", "finish"}, false,
		&serverURL, &boardIdent,
		func(c *client.Client, ctx context.Context, id int64) error { return c.DoneTicket(ctx, id) })

	var syncStrategy string
	sync := &cobra.Command{
		Use:   "sync [id]",
		Short: "Sync a ticket branch from base (rebase|merge)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, serverURL)
			id, err := ticketArg(ctx, url, args, boardIdent, pickerAction{"Sync ticket branch", "sync"}, false)
			if err != nil {
				return err
			}
			return runTicketSync(ctx, url, cmd.OutOrStdout(), id, syncStrategy)
		},
	}
	sync.Flags().StringVar(&syncStrategy, "strategy", "rebase", "Sync strategy: rebase or merge")

	var mergeStrategy string
	mergeCmd := &cobra.Command{
		Use:   "merge [id]",
		Short: "Merge a ticket branch into base (merge-commit|squash|rebase)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, serverURL)
			id, err := ticketArg(ctx, url, args, boardIdent, pickerAction{"Merge ticket branch", "merge"}, false)
			if err != nil {
				return err
			}
			return runTicketMerge(ctx, url, cmd.OutOrStdout(), id, mergeStrategy)
		},
	}
	mergeCmd.Flags().StringVar(&mergeStrategy, "strategy", "", "Merge strategy: merge-commit, squash, or rebase (default: merge.default_strategy)")

	parent.AddCommand(create, info, attach, update, move, archive, unarchive, delTicket, doneCmd, sync, mergeCmd)
	// A cancelled picker or a failed API call isn't a usage error; don't
	// bury any of them under the flag table.
	for _, c := range parent.Commands() {
		c.SilenceUsage = true
	}
	return parent
}

// boardArgs adapts an optional --board value to the positional-arg slice
// resolveBoardIdent takes.
func boardArgs(ident string) []string {
	if ident == "" {
		return nil
	}
	return []string{ident}
}

// boardLabel returns "Name (slug)" for the board the ticket form header
// names, so the user can tell an inferred board was the right one before
// typing anything.
func boardLabel(ctx context.Context, url, ident string) (string, error) {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return "", err
	}
	raw, err := c.GetBoard(ctx, id)
	if err != nil {
		return "", err
	}
	var b struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return "", fmt.Errorf("decode board: %w", err)
	}
	return formatBoardLabel(b.Name, b.Slug), nil
}

// ticketArg resolves the optional ticket id every `kanban ticket`
// subcommand accepts: the explicit argument when given, otherwise the
// ticket the user picks out of the board's list.
func ticketArg(ctx context.Context, url string, args []string, boardIdent string, action pickerAction, archived bool) (int64, error) {
	if len(args) > 0 {
		return parseInt64(args[0], "ticket id")
	}
	return pickTicket(ctx, url, boardIdent, action, archived)
}

// pickTicket resolves the board (an explicit ident, else the cwd repo),
// lists its tickets, and returns the one the user picks. archived selects
// the board's archived tickets, for the subcommands that only act on those.
// The terminal check comes first so nothing is fetched or drawn for a
// picker that can't be shown.
func pickTicket(ctx context.Context, url, boardIdent string, action pickerAction, archived bool) (int64, error) {
	item, err := pickTicketItem(ctx, url, boardIdent, action, archived, false)
	return item.ID, err
}

// pickTicketItem is pickTicket returning the whole picked row. withHarness
// adds the picker's harness row; the item's Harness is then the harness to
// switch the ticket's session to, or "" to leave it as is.
func pickTicketItem(ctx context.Context, url, boardIdent string, action pickerAction, archived, withHarness bool) (pickerItem, error) {
	if !stdinIsTerminal() {
		return pickerItem{}, errors.New("a ticket id is required when not running in an interactive terminal")
	}
	ident, err := resolveBoardIdent(ctx, url, boardArgs(boardIdent))
	if err != nil {
		return pickerItem{}, err
	}
	label, items, err := loadBoardTickets(ctx, url, ident, archived)
	if err != nil {
		return pickerItem{}, err
	}
	if len(items) == 0 {
		kind := "open"
		if archived {
			kind = "archived"
		}
		return pickerItem{}, fmt.Errorf("board %s has no %s tickets", label, kind)
	}
	var opts *harnessOptions
	if withHarness {
		if opts, err = loadHarnessOptions(ctx, url, ident); err != nil {
			return pickerItem{}, err
		}
	}
	item, ok, err := promptTicketPicker(action, label, items, opts)
	if err != nil {
		return pickerItem{}, err
	}
	if !ok {
		return pickerItem{}, errors.New("cancelled; no ticket selected")
	}
	return item, nil
}

// runTicketCreate creates the ticket, prints its summary, and returns the
// new ticket's id so callers can act on it (attach a terminal, etc).
func runTicketCreate(ctx context.Context, url string, out io.Writer, args client.CreateTicketArgs, asJSON bool) (int64, error) {
	raw, err := client.New(url, nil).CreateTicket(ctx, args)
	if err != nil {
		return 0, err
	}
	var tk struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &tk); err != nil {
		return 0, fmt.Errorf("decode ticket: %w", err)
	}
	return tk.ID, printTicketSummary(out, raw, asJSON)
}

func runTicketUpdate(ctx context.Context, url string, out io.Writer, id int64, args client.UpdateTicketArgs, asJSON bool) error {
	raw, err := client.New(url, nil).UpdateTicket(ctx, id, args)
	if err != nil {
		return err
	}
	return printTicketSummary(out, raw, asJSON)
}

func runTicketMove(ctx context.Context, url string, out io.Writer, id int64, args client.MoveTicketArgs) error {
	if err := client.New(url, nil).MoveTicket(ctx, id, args); err != nil {
		return err
	}
	fmt.Fprintf(out, "moved ticket %d to column %d position %d\n", id, args.ColumnID, args.Position)
	return nil
}

func runTicketSync(ctx context.Context, url string, out io.Writer, id int64, strategy string) error {
	if err := client.New(url, nil).SyncTicket(ctx, id, strategy); err != nil {
		return err
	}
	fmt.Fprintf(out, "synced ticket %d (%s)\n", id, strategy)
	return nil
}

func runTicketMerge(ctx context.Context, url string, out io.Writer, id int64, strategy string) error {
	if err := client.New(url, nil).MergeTicket(ctx, id, strategy); err != nil {
		return err
	}
	// An omitted strategy is resolved server-side from the board config, and
	// the 204 doesn't say which one won — don't invent a name for it.
	if strategy == "" {
		fmt.Fprintf(out, "merged ticket %d\n", id)
		return nil
	}
	fmt.Fprintf(out, "merged ticket %d (%s)\n", id, strategy)
	return nil
}

// simpleTicketCmd builds an archive/unarchive/delete/done style subcommand
// (optional positional ticket id, no body, no flags) that maps to a one-line
// client call. The shape is the same for all four commands; this avoids four
// near-identical blocks in ticketCmd(). pick names the picker opened when no
// id is given, and archived points it at the board's archived tickets for
// the commands that only ever act on those.
func simpleTicketCmd(use, short string, pick pickerAction, archived bool, serverURL, boardIdent *string, action func(c *client.Client, ctx context.Context, id int64) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " [id]",
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, *serverURL)
			id, err := ticketArg(ctx, url, args, *boardIdent, pick, archived)
			if err != nil {
				return err
			}
			if err := action(client.New(url, nil), ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s ticket %d\n", use, id)
			return nil
		},
	}
}

func printTicketSummary(out io.Writer, raw json.RawMessage, asJSON bool) error {
	if asJSON {
		_, err := fmt.Fprintln(out, string(raw))
		return err
	}
	var t struct {
		ID    int64  `json:"id"`
		Slug  string `json:"slug"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &t); err != nil {
		_, perr := fmt.Fprintln(out, string(raw))
		return perr
	}
	fmt.Fprintf(out, "#%d %s (%s)\n", t.ID, t.Title, t.Slug)
	return nil
}

// ---------- column ----------

func columnCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{
		Use:   "column",
		Short: "Column-level operations",
	}
	addServerFlag(parent, &serverURL)

	archiveAll := &cobra.Command{
		Use:   "archive-all <id>",
		Short: "Archive every ticket in a column (numeric column id)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "column id")
			if err != nil {
				return err
			}
			return runColumnArchiveAll(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), id)
		},
	}

	parent.AddCommand(archiveAll)
	return parent
}

func runColumnArchiveAll(ctx context.Context, url string, out io.Writer, columnID int64) error {
	if err := client.New(url, nil).ArchiveColumnTickets(ctx, columnID); err != nil {
		return err
	}
	fmt.Fprintf(out, "archived all tickets in column %d\n", columnID)
	return nil
}

// ---------- session ----------

func sessionCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{
		Use:   "session",
		Short: "Manage agent sessions",
	}
	addServerFlag(parent, &serverURL)

	var (
		seTicket int64
		seJSON   bool
	)
	ensure := &cobra.Command{
		Use:   "ensure",
		Short: "Ensure a session exists for a ticket (create if absent)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionEnsure(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), seTicket, seJSON)
		},
	}
	ensure.Flags().Int64Var(&seTicket, "ticket", 0, "Ticket id (required)")
	ensure.Flags().BoolVar(&seJSON, "json", false, "Print the full session JSON instead of a one-line summary")
	_ = ensure.MarkFlagRequired("ticket")

	start := sessionLifecycleCmd("start", "Start a stopped session", &serverURL,
		func(c *client.Client, ctx context.Context, id int64) (json.RawMessage, error) {
			return c.StartSession(ctx, id)
		})
	restart := sessionLifecycleCmd("restart", "Restart a session", &serverURL,
		func(c *client.Client, ctx context.Context, id int64) (json.RawMessage, error) {
			return c.RestartSession(ctx, id)
		})

	stop := &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "session id")
			if err != nil {
				return err
			}
			if err := client.New(resolveURL(cmd, serverURL), nil).StopSession(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stopped session %d\n", id)
			return nil
		},
	}

	parent.AddCommand(ensure, start, stop, restart)
	return parent
}

func runSessionEnsure(ctx context.Context, url string, out io.Writer, ticketID int64, asJSON bool) error {
	raw, err := client.New(url, nil).EnsureSession(ctx, ticketID)
	if err != nil {
		return err
	}
	return printSessionSummary(out, raw, asJSON)
}

// sessionLifecycleCmd builds start/restart subcommands (positional session
// id, --json flag) that return the session body.
func sessionLifecycleCmd(use, short string, serverURL *string, action func(c *client.Client, ctx context.Context, id int64) (json.RawMessage, error)) *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   use + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "session id")
			if err != nil {
				return err
			}
			raw, err := action(client.New(resolveURL(cmd, *serverURL), nil), cmd.Context(), id)
			if err != nil {
				return err
			}
			return printSessionSummary(cmd.OutOrStdout(), raw, asJSON)
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "Print the full session JSON instead of a one-line summary")
	return c
}

func printSessionSummary(out io.Writer, raw json.RawMessage, asJSON bool) error {
	if asJSON {
		_, err := fmt.Fprintln(out, string(raw))
		return err
	}
	var s struct {
		ID       int64  `json:"id"`
		TicketID int64  `json:"ticket_id"`
		Status   string `json:"status"`
		Branch   string `json:"branch_name"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		_, perr := fmt.Fprintln(out, string(raw))
		return perr
	}
	fmt.Fprintf(out, "session #%d ticket=%d status=%s branch=%s\n", s.ID, s.TicketID, s.Status, s.Branch)
	return nil
}

// ---------- env ----------

// envCmd manages per-board environment variables. Values are write-only
// secrets: the server encrypts them at rest and only ever returns key names,
// so no subcommand here can print a value.
func envCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{
		Use:   "env",
		Short: "Manage board environment variables (injected into session containers)",
		Long: "Manage per-board environment variables. They are injected into the board's\n" +
			"session containers at the next session start/restart. Values are encrypted\n" +
			"at rest and write-only: they can be set or removed, never read back.",
	}
	addServerFlag(parent, &serverURL)

	list := &cobra.Command{
		Use:   "list <board>",
		Short: "List env var key names on a board (values are never shown)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvList(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0])
		},
	}

	set := &cobra.Command{
		Use:   "set <board> KEY=VALUE [KEY=VALUE...]",
		Short: "Set env vars on a board",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvSet(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0], args[1:])
		},
	}

	unset := &cobra.Command{
		Use:   "unset <board> KEY [KEY...]",
		Short: "Remove env vars from a board",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvUnset(cmd.Context(), resolveURL(cmd, serverURL), cmd.OutOrStdout(), args[0], args[1:])
		},
	}

	parent.AddCommand(list, set, unset)
	return parent
}

func runEnvList(ctx context.Context, url string, out io.Writer, ident string) error {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	raw, err := c.ListBoardEnv(ctx, id)
	if err != nil {
		return err
	}
	return printEnvKeys(out, raw)
}

func runEnvSet(ctx context.Context, url string, out io.Writer, ident string, pairs []string) error {
	set := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return fmt.Errorf("expected KEY=VALUE, got %q", pair)
		}
		set[key] = value
	}
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	raw, err := c.PatchBoardEnv(ctx, id, client.PatchBoardEnvArgs{Set: set})
	if err != nil {
		return err
	}
	return printEnvKeys(out, raw)
}

func runEnvUnset(ctx context.Context, url string, out io.Writer, ident string, keys []string) error {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return err
	}
	raw, err := c.PatchBoardEnv(ctx, id, client.PatchBoardEnvArgs{Unset: keys})
	if err != nil {
		return err
	}
	return printEnvKeys(out, raw)
}

func printEnvKeys(out io.Writer, raw json.RawMessage) error {
	var resp struct {
		Keys []string `json:"keys"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		_, perr := fmt.Fprintln(out, string(raw))
		return perr
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY")
	for _, k := range resp.Keys {
		fmt.Fprintln(tw, k)
	}
	return tw.Flush()
}

// parseInt64 wraps strconv.ParseInt with a descriptive error.
func parseInt64(s, label string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", label, s, err)
	}
	return id, nil
}
