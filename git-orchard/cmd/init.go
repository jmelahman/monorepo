package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/history"
)

// InitOptions holds options for the init command
type InitOptions struct {
	Remote string
	Branch string
}

// NewInitCommand creates a new init command
func NewInitCommand() *cobra.Command {
	opts := &InitOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write the manifest from the subtrees in git history",
		Long: `Write the manifest from the subtrees in git history.

The manifest is .gitsubtrees, or .config/git-orchard/subtrees if that exists.

Every directory named by a git-subtree-dir trailer that still exists and isn't
already in the manifest is added. Its remote is --remote with {name} replaced
by the prefix's last component; the default is origin's URL with its
repository name replaced, so a monorepo at github.com:owner/monorepo.git maps
tools/foo to github.com:owner/foo.git. Edit the manifest afterwards for any
upstream that is named differently.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Remote, "remote", "", "remote URL template, with {name} for the prefix's last component")
	cmd.Flags().StringVar(&opts.Branch, "branch", config.DefaultBranch, "upstream branch")

	return cmd
}

func runInit(opts *InitOptions) error {
	o, err := openOrchard()
	if err != nil {
		return err
	}

	template := opts.Remote
	if template == "" {
		origin, err := o.Repo.Output("config", "--get", "remote.origin.url")
		if err != nil {
			return fmt.Errorf("no --remote given and no origin to derive it from")
		}
		template = RemoteTemplate(origin)
	}

	found, err := history.NewGitHistoryReader().GetSubtreesFromHistory()
	if err != nil {
		return fmt.Errorf("failed to read git history: %w", err)
	}
	var prefixes []string
	for prefix := range found {
		if prefix == "" {
			continue
		}
		if _, ok := o.Config.Lookup(prefix); ok {
			continue
		}
		if info, err := os.Stat(filepath.Join(o.Repo.Dir, prefix)); err != nil || !info.IsDir() {
			continue
		}
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	for _, prefix := range prefixes {
		s := config.Subtree{
			Prefix: prefix,
			Remote: strings.ReplaceAll(template, "{name}", filepath.Base(prefix)),
			Branch: opts.Branch,
		}
		if err := config.Add(o.Repo, o.Config.Manifest, s); err != nil {
			return err
		}
		fmt.Printf("%s\t%s\n", s.Prefix, s.Remote)
	}
	if len(prefixes) == 0 {
		fmt.Fprintln(os.Stderr, "No new subtrees found in git history.")
	}
	return nil
}

// RemoteTemplate turns a repository URL into a template for its siblings:
// git@github.com:owner/monorepo.git becomes git@github.com:owner/{name}.git.
func RemoteTemplate(url string) string {
	i := strings.LastIndexAny(url, "/:")
	suffix := ""
	if strings.HasSuffix(url, ".git") {
		suffix = ".git"
	}
	return url[:i+1] + "{name}" + suffix
}
