package share

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/git-orchard/config"
)

func TestReplaceBlock(t *testing.T) {
	tests := []struct {
		name, content, block, want, err string
	}{
		{
			name:    "hash comments",
			content: "repos:\n  # BEGIN orchard:go\n  - old\n  # END orchard:go\n  - local\n",
			block:   "  - new\n  - newer\n",
			want:    "repos:\n  # BEGIN orchard:go\n  - new\n  - newer\n  # END orchard:go\n  - local\n",
		},
		{
			name:    "slash comments, block without trailing newline",
			content: "// BEGIN orchard:web\n// END orchard:web\n",
			block:   "{}",
			want:    "// BEGIN orchard:web\n{}\n// END orchard:web\n",
		},
		{
			name:    "other names are left alone",
			content: "# BEGIN orchard:go-cli\nkeep\n# END orchard:go-cli\n# BEGIN orchard:go\n# END orchard:go\n",
			block:   "x\n",
			want:    "# BEGIN orchard:go-cli\nkeep\n# END orchard:go-cli\n# BEGIN orchard:go\nx\n# END orchard:go\n",
		},
		{
			name:    "text after the name",
			content: "<!-- BEGIN orchard:go -->\n<!-- END orchard:go -->\n",
			block:   "x\n",
			want:    "<!-- BEGIN orchard:go -->\nx\n<!-- END orchard:go -->\n",
		},
		{name: "missing", content: "a\n", err: "no BEGIN"},
		{name: "unclosed", content: "# BEGIN orchard:go\n", err: "has no END"},
		{name: "end first", content: "# END orchard:go\n# BEGIN orchard:go\n", err: "has no BEGIN"},
		{name: "duplicate begin", content: "# BEGIN orchard:go\n# BEGIN orchard:go\n# END orchard:go\n", err: "duplicate BEGIN"},
		{name: "duplicate end", content: "# BEGIN orchard:go\n# END orchard:go\n# END orchard:go\n", err: "duplicate END"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := "go"
			if strings.Contains(tt.content, "orchard:web") {
				name = "web"
			}
			got, err := ReplaceBlock([]byte(tt.content), name, []byte(tt.block))
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("got error %v, want one containing %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sync(t *testing.T, root string, subtrees ...config.Subtree) []Change {
	t.Helper()
	cfg := config.Config{SharedDir: config.DefaultSharedDir, Subtrees: subtrees}
	changes, err := Plan(root, cfg, subtrees)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, changes); err != nil {
		t.Fatal(err)
	}
	return changes
}

func TestPlanAndApply(t *testing.T) {
	root := t.TempDir()
	shared := config.DefaultSharedDir
	write(t, root, shared+"/base/.github/dependabot.yml", "version: 2\n")
	write(t, root, shared+"/base/.pre-commit-config.yaml", "  - base\n")
	write(t, root, shared+"/go/.pre-commit-config.yaml", "  - go\n")
	write(t, root, "a/.pre-commit-config.yaml",
		"repos:\n  # BEGIN orchard:base\n  # END orchard:base\n  # BEGIN orchard:go\n  - stale\n  # END orchard:go\n  - local\n")
	write(t, root, "b/.github/dependabot.yml", "version: 1\n")

	a := config.Subtree{Prefix: "a", Shared: []string{"base", "go"}}
	b := config.Subtree{Prefix: "b", Shared: []string{"base"}}
	write(t, root, "b/.pre-commit-config.yaml", "repos:\n  # BEGIN orchard:base\n  - base\n  # END orchard:base\n")

	changes := sync(t, root, a, b)
	var paths []string
	for _, c := range changes {
		paths = append(paths, c.Path)
	}
	if got, want := strings.Join(paths, " "), "a/.github/dependabot.yml a/.pre-commit-config.yaml b/.github/dependabot.yml"; got != want {
		t.Errorf("changed %s, want %s", got, want)
	}
	if got := read(t, root, "a/.pre-commit-config.yaml"); got != "repos:\n  # BEGIN orchard:base\n  - base\n  # END orchard:base\n  # BEGIN orchard:go\n  - go\n  # END orchard:go\n  - local\n" {
		t.Errorf("a/.pre-commit-config.yaml:\n%s", got)
	}
	for _, p := range []string{"a/.github/dependabot.yml", "b/.github/dependabot.yml"} {
		if got := read(t, root, p); got != "version: 2\n" {
			t.Errorf("%s: %q", p, got)
		}
	}

	if changes := sync(t, root, a, b); len(changes) != 0 {
		t.Errorf("a second sync should change nothing, got %+v", changes)
	}
}

func TestPlanKeepsSourceMode(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, config.DefaultSharedDir, "base", "run.sh")
	write(t, root, config.DefaultSharedDir+"/base/run.sh", "#!/bin/sh\n")
	if err := os.Chmod(src, 0o755); err != nil {
		t.Fatal(err)
	}
	sync(t, root, config.Subtree{Prefix: "a", Shared: []string{"base"}})
	info, err := os.Stat(filepath.Join(root, "a", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode is %v, want 0755", info.Mode().Perm())
	}
}

func TestPlanSyncsModeOfWholeFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, config.DefaultSharedDir+"/base/run.sh", "#!/bin/sh\n")
	write(t, root, "a/run.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(root, config.DefaultSharedDir, "base", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	changes := sync(t, root, config.Subtree{Prefix: "a", Shared: []string{"base"}})
	if len(changes) != 1 || changes[0].OldMode != 0o644 || changes[0].Mode != 0o755 {
		t.Fatalf("want one change from 0644 to 0755, got %+v", changes)
	}
	info, err := os.Stat(filepath.Join(root, "a", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode is %v, want 0755", info.Mode().Perm())
	}
}

func TestPlanKeepsModeOfBlockFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, config.DefaultSharedDir+"/base/run.sh", "echo shared\n")
	write(t, root, "a/run.sh", "#!/bin/sh\n# BEGIN orchard:base\n# END orchard:base\n")
	if err := os.Chmod(filepath.Join(root, "a", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := config.Subtree{Prefix: "a", Shared: []string{"base"}}
	sync(t, root, s)
	info, err := os.Stat(filepath.Join(root, "a", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode is %v, want the subtree's 0755", info.Mode().Perm())
	}
	if changes := sync(t, root, s); len(changes) != 0 {
		t.Errorf("the profile's mode shouldn't apply to a block, got %+v", changes)
	}
}

func TestPlanErrors(t *testing.T) {
	root := t.TempDir()
	shared := config.DefaultSharedDir
	write(t, root, shared+"/base/f", "base\n")
	write(t, root, shared+"/go/f", "go\n")
	write(t, root, "partial/f", "# BEGIN orchard:base\n# END orchard:base\n")

	tests := []struct {
		name string
		s    config.Subtree
		err  string
	}{
		{"missing profile", config.Subtree{Prefix: "a", Shared: []string{"nope"}}, "does not exist"},
		{"two profiles, no markers", config.Subtree{Prefix: "a", Shared: []string{"base", "go"}}, "all provide it"},
		{"one profile unmarked", config.Subtree{Prefix: "partial", Shared: []string{"base", "go"}}, "no BEGIN orchard:go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{SharedDir: shared}
			_, err := Plan(root, cfg, []config.Subtree{tt.s})
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("got error %v, want one containing %q", err, tt.err)
			}
		})
	}
}
