package server

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestManWritesAPagePerCommand(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "man1")
	root := Root()
	root.SetArgs([]string{"man", "-o", dir})
	if err := root.Execute(); err != nil {
		t.Fatalf("kanban man: %v", err)
	}

	var check func(c *cobra.Command)
	check = func(c *cobra.Command) {
		if !c.IsAvailableCommand() && c.HasParent() {
			return
		}
		name := strings.ReplaceAll(c.CommandPath(), " ", "-")
		page, err := os.ReadFile(filepath.Join(dir, name+".1"))
		if err != nil {
			t.Errorf("%s: %v", c.CommandPath(), err)
		} else if !strings.Contains(string(page), `.TH "`+strings.ToUpper(name)+`" "1"`) {
			t.Errorf("%s: page header doesn't name the command:\n%s", c.CommandPath(), page)
		}
		for _, sub := range c.Commands() {
			check(sub)
		}
	}
	check(root)
}

func TestManReplacesOnlyKanbanPages(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"kanban-removed-command.1", "kanbanize.1", "git.1", "kanban-notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root := Root()
	root.SetArgs([]string{"man", "-o", dir})
	if err := root.Execute(); err != nil {
		t.Fatalf("kanban man: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "kanban-removed-command.1")); !os.IsNotExist(err) {
		t.Errorf("stale page: stat err = %v, want not exist", err)
	}
	for _, name := range []string{"kanbanize.1", "git.1", "kanban-notes.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s should be left alone: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "kanban.1")); err != nil {
		t.Errorf("kanban.1 should be regenerated: %v", err)
	}
}

func TestManRequiresOutputDir(t *testing.T) {
	root := Root()
	root.SetArgs([]string{"man"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "output-dir") {
		t.Fatalf("kanban man without -o: err = %v, want a missing output-dir error", err)
	}
}
