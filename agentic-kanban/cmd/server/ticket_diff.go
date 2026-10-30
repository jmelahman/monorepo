package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/internal/client"
)

func ticketDiffCmd(serverURL, boardIdent *string) *cobra.Command {
	var plain, asJSON bool
	cmd := &cobra.Command{
		Use:   "diff [id]",
		Short: "Show a ticket's changes against its base branch",
		Long: `Show what a ticket's branch changes, the same patch as the web UI's Diff
tab: everything since the branch left the board's base branch, including
uncommitted edits and new files. The server computes it, so this also works
when the worktree isn't on this machine.

In a terminal the patch opens in hunk (https://github.com/modem-dev/hunk) if
it's on your PATH, otherwise in $PAGER, falling back to less. When piped, or
with --plain, the raw patch is printed so it can be fed to "git apply" or
another tool.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			url := resolveURL(cmd, *serverURL)
			id, err := ticketArg(ctx, url, args, *boardIdent, pickerAction{"Diff ticket", "diff"})
			if err != nil {
				return err
			}
			interactive := !plain && stdinIsTerminal()
			return runTicketDiff(ctx, url, cmd.OutOrStdout(), cmd.ErrOrStderr(), id, interactive, asJSON)
		},
	}
	cmd.Flags().BoolVar(&plain, "plain", false, "Print the raw patch instead of opening a viewer")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the base branch and patch as JSON")
	return cmd
}

// runTicketDiff fetches the patch for the ticket's session and prints it, or
// opens it in a viewer when interactive. "No changes" goes to errOut so stdout
// is still a valid (empty) patch.
func runTicketDiff(ctx context.Context, url string, out, errOut io.Writer, id int64, interactive, asJSON bool) error {
	info, err := loadTicketInfo(ctx, url, id)
	if err != nil {
		return err
	}
	if info.Session == nil {
		return fmt.Errorf("ticket %d has no session (and so no branch) to diff; start one with `kanban ticket attach %d`", id, id)
	}
	// The API answers this case with an empty patch, which would read as a clean
	// branch when nothing was compared.
	if info.Session.WorktreePath == "" {
		return fmt.Errorf("ticket %d's session has no worktree to diff; create one with `kanban ticket attach %d`", id, id)
	}
	raw, err := client.New(url, nil).SessionDiff(ctx, info.Session.ID)
	if err != nil {
		return err
	}
	if asJSON {
		_, err := fmt.Fprintln(out, string(raw))
		return err
	}
	var diff struct {
		Base  string `json:"base"`
		Patch string `json:"patch"`
	}
	if err := json.Unmarshal(raw, &diff); err != nil {
		return fmt.Errorf("decode diff: %w", err)
	}
	if diff.Patch == "" {
		if diff.Base == "" {
			fmt.Fprintf(errOut, "ticket %d has no changes\n", id)
		} else {
			fmt.Fprintf(errOut, "ticket %d has no changes against %s\n", id, diff.Base)
		}
		return nil
	}
	if !interactive {
		_, err := io.WriteString(out, diff.Patch)
		return err
	}
	return viewPatch(ctx, diff.Patch, out, errOut)
}

// viewPatch shows patch in hunk when it's installed, otherwise colored in the
// user's pager, otherwise colored on out.
func viewPatch(ctx context.Context, patch string, out, errOut io.Writer) error {
	run := func(input, name string, env []string, args ...string) error {
		// Ctrl-C reaches the whole foreground process group. The viewer decides
		// what it means (less just stops searching); exiting here would hand the
		// terminal back to the shell with the viewer still reading from it.
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt)
		defer signal.Stop(sigs)

		c := exec.CommandContext(ctx, name, args...)
		c.Stdin = strings.NewReader(input)
		c.Stdout = out
		c.Stderr = errOut
		c.Env = append(os.Environ(), env...)
		return c.Run()
	}

	if path, err := exec.LookPath("hunk"); err == nil {
		if err := run(patch, path, nil, "patch", "-"); err != nil {
			return fmt.Errorf("hunk: %w (--plain skips it)", err)
		}
		return nil
	}

	colored := colorizePatch(escapeControls(patch))
	argv := pagerCommand()
	if argv == nil {
		_, err := io.WriteString(out, colored)
		return err
	}
	// Same default git gives less: keep the colors, and don't page a short diff.
	var env []string
	if _, set := os.LookupEnv("LESS"); !set {
		env = []string{"LESS=FRX"}
	}
	if err := run(colored, argv[0], env, argv[1:]...); err != nil {
		return fmt.Errorf("pager: %w (--plain skips it)", err)
	}
	return nil
}

// pagerCommand returns the argv of $PAGER, falling back to less, or nil when
// the one it would use isn't installed. $PAGER is a shell command, as it is for
// git, so quoting works. Only without a shell (Windows) is it split on spaces.
func pagerCommand() []string {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	} else if _, err := exec.LookPath("sh"); err == nil {
		return []string{"sh", "-c", pager}
	}
	argv := strings.Fields(pager)
	if len(argv) == 0 {
		return nil
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return nil
	}
	return argv
}

// escapeControls makes agent-written file contents safe to show on a terminal,
// where an escape sequence could set the clipboard or the title, move the cursor
// to hide lines, or fake diff colors. Control characters become visible, in
// caret notation as git and less show them, leaving only tabs and newlines.
func escapeControls(s string) string {
	var b strings.Builder
	for _, r := range strings.ToValidUTF8(s, "\uFFFD") {
		switch {
		case r == '\n', r == '\t', !unicode.IsControl(r):
			b.WriteRune(r)
		case r < 0x20:
			b.WriteByte('^')
			b.WriteRune(r + 0x40)
		case r == 0x7f:
			b.WriteString("^?")
		default:
			b.WriteRune('\uFFFD')
		}
	}
	return b.String()
}

// colorizePatch adds git's default diff colors to a unified diff.
func colorizePatch(patch string) string {
	const (
		bold  = "\x1b[1m"
		red   = "\x1b[31m"
		green = "\x1b[32m"
		cyan  = "\x1b[36m"
		reset = "\x1b[0m"
	)
	var b strings.Builder
	// Between "diff --git" and the first hunk, "--- a/x" is a file header. Past
	// it, the same text is a removed line.
	inHeader := false
	for _, line := range strings.SplitAfter(patch, "\n") {
		text, newline := strings.CutSuffix(line, "\n")
		color := ""
		switch {
		case strings.HasPrefix(text, "diff --git "):
			inHeader = true
			color = bold
		case strings.HasPrefix(text, "@@"):
			inHeader = false
			color = cyan
		case inHeader:
			color = bold
		case strings.HasPrefix(text, "+"):
			color = green
		case strings.HasPrefix(text, "-"):
			color = red
		}
		if color == "" || text == "" {
			b.WriteString(line)
			continue
		}
		b.WriteString(color + text + reset)
		if newline {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
