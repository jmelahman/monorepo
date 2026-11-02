package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/internal/skills"
)

// newSkillCmd builds `kanban skill`.
func newSkillCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:   "skill",
		Short: "Manage the agent skills that ship with kanban",
	}

	var dir string
	var force, linkClaude bool
	install := &cobra.Command{
		Use:   "install",
		Short: "Install kanban's agent skills for your coding agents",
		Long: `Install kanban's agent skills to <dir>/skills/<name>/SKILL.md. The default,
~/.agents, is the location shared between agents. Claude Code doesn't read it, so
each skill is also symlinked into ~/.claude/skills when ~/.claude exists.

The skills only run when you invoke them, such as with /kanban-init, in agents
that honor disable-model-invocation.

A file that differs from this version's skill is left alone unless you pass
--force, which also updates it after upgrading kanban. --force also replaces a
link in ~/.claude/skills that points elsewhere, but never a real directory.

Installing never writes through a symlink, even with --force: a skill directory
or SKILL.md that's a symlink is refused, so remove it or pass --dir its real
location.`,
		Example: `  kanban skill install
  kanban skill install --dir ~/dotfiles/.agents
  kanban skill install --link-claude=false`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			skillsDir, err := resolveSkillsDir(dir)
			if err != nil {
				return err
			}
			linkDir := ""
			if linkClaude {
				if linkDir, err = claudeSkillsDir(); err != nil {
					return err
				}
			}
			results, err := skills.Install(skillsDir, linkDir, force)
			if err != nil {
				return err
			}
			for _, r := range results {
				if r.Unchanged {
					fmt.Fprintf(cmd.OutOrStdout(), "%s is up to date at %s\n", r.Name, r.Path)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "installed %s to %s\n", r.Name, r.Path)
				}
				if r.Link != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "linked %s to %s\n", r.Link, filepath.Dir(r.Path))
				}
			}
			return nil
		},
	}
	install.Flags().StringVar(&dir, "dir", "", "Agent directory to install into (default: ~/.agents)")
	install.Flags().BoolVar(&force, "force", false, "Replace an installed skill that differs from this version's, or a link that points elsewhere")
	install.Flags().BoolVar(&linkClaude, "link-claude", true, "Symlink each skill into ~/.claude/skills, if ~/.claude exists")

	parent.AddCommand(install)
	return parent
}

// claudeSkillsDir returns Claude Code's personal skills directory, or "" when
// ~/.claude doesn't exist, so that someone without Claude Code doesn't get one.
func claudeSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	claude := filepath.Join(home, ".claude")
	info, err := os.Stat(claude)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read ~/.claude: %w", err)
	}
	if !info.IsDir() {
		return "", nil
	}
	return filepath.Join(claude, "skills"), nil
}

// resolveSkillsDir returns the skills directory under an agent directory,
// defaulting to ~/.agents. A dir that's already named skills is used as is, so
// --dir ~/.claude/skills doesn't install to a skills/skills an agent won't read.
func resolveSkillsDir(dir string) (string, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		dir = filepath.Join(home, ".agents")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve --dir: %w", err)
	}
	if filepath.Base(abs) == "skills" {
		return abs, nil
	}
	return filepath.Join(abs, "skills"), nil
}
