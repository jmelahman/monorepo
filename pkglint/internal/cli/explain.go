package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmelahman/pkglint/internal/report"
	"github.com/jmelahman/pkglint/internal/rules"
)

// newExplainCommand builds `pkglint explain`: the reference page for a rule,
// in the terminal, so the ID beside a finding can be looked up without going
// to the report card site. It reads nothing but the registry — no paths, no
// package, no network — which is why it takes no reportOpts beyond --color.
func newExplainCommand(stdout io.Writer) *cobra.Command {
	color := "auto"
	cmd := &cobra.Command{
		Use:   "explain [flags] <rule> ...",
		Short: "print a rule's documentation, example, and how to suppress it",
		Long: `explain prints the reference page for each named rule: what it reports and
what it is checked against, a flagged snippet beside the preferred spelling,
the flag that auto-fixes it when it has one, and the directive that suppresses
it — the same reference the report card site publishes.

A rule is named by ID or by short name, in any case: 'pkglint explain PB101',
'pkglint explain pb101' and 'pkglint explain skipped-checksum' all print the
same page. 'pkglint --rules' lists every rule. Note that 'pkglint explain' names
this command even when ./explain is a package directory; spell that one
'pkglint ./explain'.`,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeRules,
		SilenceUsage:      true,
		SilenceErrors:     true,
		RunE: func(_ *cobra.Command, args []string) error {
			// cobra's own arity error ("requires at least 1 arg(s)") says
			// nothing about what an argument is; a rule ID is not guessable.
			if len(args) == 0 {
				return fmt.Errorf("explain: name a rule, e.g. 'pkglint explain PB101' (run 'pkglint --rules' to list them)")
			}
			colorize, err := colorEnabled(color, stdout)
			if err != nil {
				return err
			}
			// Every argument is resolved before any page is printed, so a typo
			// in the second rule does not leave the first one's page on screen
			// above the error.
			found := make([]rules.Rule, 0, len(args))
			for _, arg := range args {
				r, ok := rules.Lookup(arg)
				if !ok {
					return unknownRuleError(arg)
				}
				found = append(found, r)
			}
			for i, r := range found {
				if i > 0 {
					fmt.Fprintln(stdout)
				}
				report.RenderRuleDetail(stdout, r, colorize)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&color, "color", "auto", "colorize output: auto, always, or never")
	return cmd
}

// completeRules offers the rules `explain` can still be given, accepting what
// Lookup accepts: an ID in any case, or a name. A rule is offered by ID, the
// way findings print it, unless the word typed so far can only be the start of
// its name. Rules already on the command line are left out.
func completeRules(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	named := make(map[string]bool, len(args))
	for _, arg := range args {
		if r, ok := rules.Lookup(arg); ok {
			named[r.ID] = true
		}
	}
	q := strings.ToLower(toComplete)
	var out []cobra.Completion
	for _, r := range rules.Registry() {
		switch {
		case named[r.ID]:
		case strings.HasPrefix(strings.ToLower(r.ID), q):
			out = append(out, cobra.CompletionWithDesc(r.ID, r.Name))
		case strings.HasPrefix(r.Name, q):
			out = append(out, cobra.CompletionWithDesc(r.Name, r.ID))
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// unknownRuleError reports an argument the registry has no rule for, offering
// the rules a partial or mistyped one plausibly meant.
func unknownRuleError(arg string) error {
	near := rules.Suggest(arg)
	if len(near) == 0 {
		return fmt.Errorf("unknown rule %q (run 'pkglint --rules' to list them)", arg)
	}
	names := make([]string, 0, len(near))
	for _, r := range near {
		names = append(names, r.ID+" "+r.Name)
	}
	return fmt.Errorf("unknown rule %q; did you mean %s? (run 'pkglint --rules' to list them)",
		arg, strings.Join(names, ", "))
}
