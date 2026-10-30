package pins_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/pinsync/internal/config"
	"github.com/jmelahman/pinsync/internal/pins"
)

const orchard = "https://github.com/jmelahman/git-orchard"

func TestApply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		pin   config.Pin
		input string
		want  string
	}{
		{
			name: "frozen rev gets the tag's commit, and keeps its v",
			pin:  config.Pin{Name: "git-orchard", Package: "git-orchard", Version: "1.3.0", Repo: orchard},
			input: `repos:
  - repo: https://github.com/jmelahman/git-orchard.git
    rev: 50de16612d3ebcd745b8feb5bb4d3637b732140d # frozen: v1.1.0
    hooks:
      - id: git-orchard-sync
`,
			want: `repos:
  - repo: https://github.com/jmelahman/git-orchard.git
    rev: commit-of-v1.3.0 # frozen: v1.3.0
    hooks:
      - id: git-orchard-sync
`,
		},
		{
			name: "bare list of repos, as in a git-orchard profile",
			pin:  config.Pin{Name: "git-orchard", Package: "git-orchard", Version: "1.3.0", Repo: orchard},
			input: `  # Spliced under a subtree's repos.
  - repo: https://github.com/jmelahman/git-orchard
    rev: 50de16612d3ebcd745b8feb5bb4d3637b732140d  # frozen: v1.1.0
    hooks:
      - id: git-orchard-sync
`,
			want: `  # Spliced under a subtree's repos.
  - repo: https://github.com/jmelahman/git-orchard
    rev: commit-of-v1.3.0  # frozen: v1.3.0
    hooks:
      - id: git-orchard-sync
`,
		},
		{
			name: "go dependency",
			pin:  config.Pin{Name: "shfmt", Package: "mvdan.cc/sh/v3/cmd/shfmt", Version: "3.15.0"},
			input: `repos:
  - repo: local
    hooks:
      - id: shfmt
        additional_dependencies:
          - mvdan.cc/sh/v3/cmd/shfmt@v3.14.1
`,
			want: `repos:
  - repo: local
    hooks:
      - id: shfmt
        additional_dependencies:
          - mvdan.cc/sh/v3/cmd/shfmt@v3.15.0
`,
		},
		{
			name: "python dependency by normalized name, others untouched",
			pin:  config.Pin{Name: "ruamel", Package: "Ruamel.YAML", Version: "0.18.0"},
			input: `repos:
  - repo: local
    hooks:
      - id: x
        additional_dependencies: ["ruamel_yaml==0.17.1", "uv==0.17.1", "ruamel-yaml>=0.1"]
`,
			want: `repos:
  - repo: local
    hooks:
      - id: x
        additional_dependencies: ["ruamel_yaml==0.18.0", "uv==0.17.1", "ruamel-yaml>=0.1"]
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			const name = "nested/.pre-commit-config.yaml"
			write(t, root, name, tt.input)
			copies, err := pins.Find(root, []string{name}, []config.Pin{tt.pin})
			if err != nil {
				t.Fatal(err)
			}
			commit := func(_ context.Context, url, tag string) (string, error) {
				if url != orchard {
					t.Errorf("commit looked up %s, want %s", url, orchard)
				}
				return "commit-of-" + tag, nil
			}
			if err := pins.Apply(t.Context(), root, copies, commit); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestFindErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		files   map[string]string
		pin     config.Pin
	}{
		{
			name: "commit rev without a frozen comment",
			pin:  config.Pin{Name: "git-orchard", Version: "1.3.0", Repo: orchard},
			files: map[string]string{".pre-commit-config.yaml": `repos:
  - repo: https://github.com/jmelahman/git-orchard
    rev: 50de16612d3ebcd745b8feb5bb4d3637b732140d
`},
			wantErr: pins.ErrUnfrozen,
		},
		{
			name:    "pre-commit source missing from the root config",
			pin:     config.Pin{Name: "uv", Source: config.SourcePreCommit, Repo: orchard},
			files:   map[string]string{".pre-commit-config.yaml": "repos: []\n"},
			wantErr: pins.ErrNoSource,
		},
		{
			name:    "package missing from uv.lock",
			pin:     config.Pin{Name: "uv", Package: "uv", Source: config.SourceUVLock},
			files:   map[string]string{"uv.lock": "[[package]]\nname = \"ruff\"\nversion = \"1.0.0\"\n"},
			wantErr: pins.ErrNoSource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			var names []string
			for name, content := range tt.files {
				write(t, root, name, content)
				names = append(names, name)
			}
			if _, err := pins.Find(root, names, []config.Pin{tt.pin}); !errors.Is(err, tt.wantErr) {
				t.Errorf("Find() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
