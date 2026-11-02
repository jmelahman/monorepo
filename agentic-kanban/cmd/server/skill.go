package server

import (
	"fmt"
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
	var force bool
	install := &cobra.Command{
		Use:   "install",
		Short: "Install kanban's agent skills for your coding agents",
		Long: `Install kanban's agent skills to <dir>/skills/<name>/SKILL.md. The default,
~/.agents, is the location shared between agents. Claude Code doesn't read it, so
pass --dir ~/.claude for Claude Code, or link the skill there.

The skills only run when you invoke them, such as with /kanban-init, in agents
that honor disable-model-invocation.

A file that differs from this version's skill is left alone unless you pass
--force, which also updates it after upgrading kanban.`,
		Example: `  kanban skill install
  kanban skill install --dir ~/.claude
  kanban skill install --dir ~/dotfiles/.agents`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			skillsDir, err := resolveSkillsDir(dir)
			if err != nil {
				return err
			}
			results, err := skills.Install(skillsDir, force)
			if err != nil {
				return err
			}
			for _, r := range results {
				if r.Unchanged {
					fmt.Fprintf(cmd.OutOrStdout(), "%s is up to date at %s\n", r.Name, r.Path)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "installed %s to %s\n", r.Name, r.Path)
				}
			}
			return nil
		},
	}
	install.Flags().StringVar(&dir, "dir", "", "Agent directory to install into, such as ~/.claude (default: ~/.agents)")
	install.Flags().BoolVar(&force, "force", false, "Replace an installed skill that differs from this version's")

	parent.AddCommand(install)
	return parent
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
