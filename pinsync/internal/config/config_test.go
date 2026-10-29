package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/lockstep/internal/config"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		content string
		want    []config.Pin
		noFile  bool
	}{
		{
			name: "defaults, sorted",
			content: `
[pins.uv]
source = "pre-commit"
repo = "https://github.com/astral-sh/uv-pre-commit"
action = "astral-sh/setup-uv"

[pins.mypkg]
source = "uv.lock"
package = "My_Pkg"
`,
			want: []config.Pin{
				{Name: "mypkg", Source: "uv.lock", Package: "My_Pkg"},
				{Name: "uv", Source: "pre-commit", Package: "uv", Repo: "https://github.com/astral-sh/uv-pre-commit", Action: "astral-sh/setup-uv", Input: "version"},
			},
		},
		{name: "missing", noFile: true, wantErr: config.ErrNoConfig},
		{name: "not toml", content: "[pins", wantErr: config.ErrBadConfig},
		{name: "unknown key", content: "[pins.uv]\nversion = \"1\"\nrepos = \"x\"\n", wantErr: config.ErrBadConfig},
		{name: "no source", content: "[pins.uv]\nrepo = \"x\"\n", wantErr: config.ErrBadConfig},
		{name: "source and version", content: "[pins.uv]\nsource = \"uv.lock\"\nversion = \"1\"\n", wantErr: config.ErrBadConfig},
		{name: "unknown source", content: "[pins.uv]\nsource = \"go.mod\"\n", wantErr: config.ErrBadConfig},
		{name: "pre-commit without repo", content: "[pins.uv]\nsource = \"pre-commit\"\n", wantErr: config.ErrBadConfig},
		{name: "input without action", content: "[pins.uv]\nversion = \"1\"\ninput = \"v\"\n", wantErr: config.ErrBadConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if !tt.noFile {
				p := filepath.Join(root, config.File)
				if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := config.Load(root)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Load() error = %v, want %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Load() = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Load()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
