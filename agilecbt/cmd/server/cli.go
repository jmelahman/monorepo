package server

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/client"
	"github.com/jmelahman/agilecbt/internal/db"
)

// addClientCommands attaches the user-facing CLI subcommands (`today`,
// `goal`, `step`, `export`, `import`) to root. They're thin wrappers over the
// HTTP API of a running `agilecbt serve`.
func addClientCommands(root *cobra.Command) {
	root.AddCommand(todayCmd(), goalCmd(), stepCmd(), exportCmd(), importCmd())
}

// resolveURL returns the effective server URL for a leaf command. APP_URL
// wins only when the user didn't explicitly pass --server.
func resolveURL(cmd *cobra.Command, serverURL string) string {
	if env := os.Getenv("APP_URL"); env != "" && !cmd.Flags().Changed("server") {
		return env
	}
	return serverURL
}

// addServerFlag registers --server as a persistent flag on the parent so
// every leaf inherits it.
func addServerFlag(parent *cobra.Command, dst *string) {
	parent.PersistentFlags().StringVar(dst, "server", "http://localhost:8080",
		"Base URL of the HTTP server")
}

// newClient builds an API client for a leaf, authenticating with
// $APP_SECRET when set.
func newClient(cmd *cobra.Command, serverURL string) *client.Client {
	return client.New(resolveURL(cmd, serverURL), os.Getenv("APP_SECRET"), nil)
}

func todayCmd() *cobra.Command {
	var serverURL string
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Show today's check-ins and steps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			snap, err := newClient(cmd, serverURL).Today(cmd.Context())
			if err != nil {
				return err
			}
			return printToday(cmd.OutOrStdout(), snap)
		},
	}
	addServerFlag(cmd, &serverURL)
	return cmd
}

func printToday(out io.Writer, s app.Snapshot) error {
	fmt.Fprintf(out, "Today, %s\n", s.Date)
	if s.Week.Intention != "" {
		fmt.Fprintf(out, "Week intention: %s\n", s.Week.Intention)
	}
	for _, c := range []*db.Checkin{s.Morning, s.Evening} {
		if c != nil {
			fmt.Fprintf(out, "%s check-in: mood %s, energy %s, anxiety %s\n",
				c.Kind, intStr(c.Mood), intStr(c.Energy), intStr(c.Anxiety))
		}
	}
	fmt.Fprintln(out, "\nToday:")
	if len(s.Today) == 0 {
		fmt.Fprintln(out, "  (nothing planned — that's okay)")
	}
	for _, st := range s.Today {
		carried := ""
		if st.CarriedOver {
			carried = " (carried over)"
		}
		fmt.Fprintf(out, "  #%d %s [energy %d]%s\n", st.ID, st.Title, st.EnergyCost, carried)
	}
	if len(s.DoneToday) > 0 {
		fmt.Fprintln(out, "\nDone:")
		for _, st := range s.DoneToday {
			fmt.Fprintf(out, "  ✓ #%d %s\n", st.ID, st.Title)
		}
	}
	return nil
}

func goalCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{Use: "goal", Short: "Manage roadmap goals"}
	addServerFlag(parent, &serverURL)

	var status string
	list := &cobra.Command{
		Use:   "list",
		Short: "List goals",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			goals, err := newClient(cmd, serverURL).ListGoals(cmd.Context(), status)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tSTATUS\tTITLE\tWHY")
			for _, g := range goals {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", g.ID, g.Status, g.Title, g.Why)
			}
			return tw.Flush()
		},
	}
	list.Flags().StringVar(&status, "status", "", "Filter by status (active, resting, done)")

	var why string
	var valueID int64
	add := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a goal",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			p := db.GoalPatch{Title: &title, Why: &why}
			if valueID != 0 {
				p.ValueID = &valueID
			}
			g, err := newClient(cmd, serverURL).CreateGoal(cmd.Context(), p)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "goal #%d %s\n", g.ID, g.Title)
			return nil
		},
	}
	add.Flags().StringVar(&why, "why", "", "Why this goal matters to you")
	add.Flags().Int64Var(&valueID, "value", 0, "Value id this goal serves")

	parent.AddCommand(list, add)
	return parent
}

func stepCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{Use: "step", Short: "Manage steps on the board"}
	addServerFlag(parent, &serverURL)

	var lanes []string
	list := &cobra.Command{
		Use:   "list",
		Short: "List steps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			steps, err := newClient(cmd, serverURL).ListSteps(cmd.Context(), lanes...)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tLANE\tENERGY\tTITLE")
			for _, s := range steps {
				fmt.Fprintf(tw, "%d\t%s\t%d\t%s\n", s.ID, s.Lane, s.EnergyCost, s.Title)
			}
			return tw.Flush()
		},
	}
	list.Flags().StringSliceVar(&lanes, "lane", nil, "Lanes to show (someday, week, today, done, let_go)")

	var lane string
	var energy int
	var goalID int64
	add := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a step",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			p := db.StepPatch{Title: &title}
			if energy != 0 {
				p.EnergyCost = &energy
			}
			if goalID != 0 {
				p.GoalID = &goalID
			}
			s, err := newClient(cmd, serverURL).CreateStep(cmd.Context(), lane, p)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "step #%d %s (%s)\n", s.ID, s.Title, s.Lane)
			return nil
		},
	}
	add.Flags().StringVar(&lane, "lane", "week", "Lane (someday, week, today)")
	add.Flags().IntVar(&energy, "energy", 0, "Energy cost 1-3 (default 1)")
	add.Flags().Int64Var(&goalID, "goal", 0, "Goal id this step serves")

	move := &cobra.Command{
		Use:   "move <id> <lane>",
		Short: "Move a step to a lane",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "step id")
			if err != nil {
				return err
			}
			s, err := newClient(cmd, serverURL).MoveStep(cmd.Context(), id, args[1])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "step #%d → %s\n", s.ID, s.Lane)
			return nil
		},
	}

	var mastery, pleasure int
	done := &cobra.Command{
		Use:   "done <id>",
		Short: "Complete a step, optionally rating mastery and pleasure (0-10)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "step id")
			if err != nil {
				return err
			}
			var m, p *int
			if cmd.Flags().Changed("mastery") {
				m = &mastery
			}
			if cmd.Flags().Changed("pleasure") {
				p = &pleasure
			}
			s, err := newClient(cmd, serverURL).CompleteStep(cmd.Context(), id, m, p)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ #%d %s\n", s.ID, s.Title)
			return nil
		},
	}
	done.Flags().IntVar(&mastery, "mastery", 0, "Sense of accomplishment, 0-10")
	done.Flags().IntVar(&pleasure, "pleasure", 0, "Enjoyment, 0-10")

	parent.AddCommand(list, add, move, done)
	return parent
}

func exportCmd() *cobra.Command {
	var serverURL, outPath string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all data as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := newClient(cmd, serverURL).Export(cmd.Context())
			if err != nil {
				return err
			}
			if outPath == "" || outPath == "-" {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return err
			}
			return os.WriteFile(outPath, raw, 0o600)
		},
	}
	cmd.Flags().StringVarP(&outPath, "output", "o", "", "Write to this file instead of stdout")
	addServerFlag(cmd, &serverURL)
	return cmd
}

func importCmd() *cobra.Command {
	var serverURL string
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import an export into an empty instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			if !json.Valid(raw) {
				return fmt.Errorf("%s is not valid JSON", args[0])
			}
			if err := newClient(cmd, serverURL).Import(cmd.Context(), raw); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "imported")
			return nil
		},
	}
	addServerFlag(cmd, &serverURL)
	return cmd
}

func intStr(p *int) string {
	if p == nil {
		return "–"
	}
	return strconv.Itoa(*p)
}

// parseInt64 wraps strconv.ParseInt with a descriptive error.
func parseInt64(s, label string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", label, s, err)
	}
	return id, nil
}
