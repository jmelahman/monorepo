package project_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/jmelahman/vinder/internal/project"
)

func TestDetectPackageManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		files     map[string]string
		wantErr   error
		name      string
		want      string
		wantBerry bool
	}{
		{
			name:  "packageManager field",
			files: map[string]string{"package.json": `{"packageManager": "pnpm@10.4.1+sha512.abc"}`},
			want:  "pnpm",
		},
		{
			name: "packageManager field wins over the lockfile",
			files: map[string]string{
				"package.json": `{"packageManager": "nub@0.9.6"}`,
				"bun.lock":     "",
			},
			want: "nub",
		},
		{
			name:  "devEngines object",
			files: map[string]string{"package.json": `{"devEngines": {"packageManager": {"name": "npm", "version": "11.0.0"}}}`},
			want:  "npm",
		},
		{
			name:  "devEngines list takes the first",
			files: map[string]string{"package.json": `{"devEngines": {"packageManager": [{"name": "yarn"}, {"name": "npm"}]}}`},
			want:  "yarn",
		},
		{
			name:  "yarn 1 range",
			files: map[string]string{"package.json": `{"devEngines": {"packageManager": {"name": "yarn", "version": "^1.22.0"}}}`},
			want:  "yarn",
		},
		{
			name:  "yarn 1 major",
			files: map[string]string{"package.json": `{"devEngines": {"packageManager": {"name": "yarn", "version": ">=1"}}}`},
			want:  "yarn",
		},
		{
			name:      "yarn berry",
			files:     map[string]string{"package.json": `{"packageManager": "yarn@4.5.0+sha512.abc"}`},
			want:      "yarn",
			wantBerry: true,
		},
		{
			name: "yarn wildcard falls back to yarnrc",
			files: map[string]string{
				"package.json": `{"devEngines": {"packageManager": {"name": "yarn", "version": "*"}}}`,
				".yarnrc.yml":  "",
			},
			want:      "yarn",
			wantBerry: true,
		},
		{
			name:  "bun lockfile",
			files: map[string]string{"package.json": `{}`, "bun.lock": ""},
			want:  "bun",
		},
		{
			name:  "npm shrinkwrap",
			files: map[string]string{"package.json": `{}`, "npm-shrinkwrap.json": ""},
			want:  "npm",
		},
		{
			name:  "nub lockfile",
			files: map[string]string{"package.json": `{}`, "nub.lock": ""},
			want:  "nub",
		},
		{
			name:  "yarn lockfile",
			files: map[string]string{"package.json": `{}`, "yarn.lock": ""},
			want:  "yarn",
		},
		{
			name:    "nothing to go on",
			files:   map[string]string{"package.json": `{}`},
			wantErr: project.ErrNoPackageManager,
		},
		{
			name:    "two lockfiles",
			files:   map[string]string{"package.json": `{}`, "bun.lock": "", "pnpm-lock.yaml": ""},
			wantErr: project.ErrAmbiguousPackageManager,
		},
		{
			name:    "unsupported",
			files:   map[string]string{"package.json": `{"packageManager": "deno@2.0.0"}`},
			wantErr: project.ErrUnknownPackageManager,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, content := range tt.files {
				write(t, filepath.Join(dir, name), content)
			}
			pm, err := project.DetectPackageManager(dir)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DetectPackageManager() error = %v, want %v", err, tt.wantErr)
			}
			if pm.Name != tt.want {
				t.Errorf("DetectPackageManager() = %q, want %q", pm.Name, tt.want)
			}
			if pm.Berry() != tt.wantBerry {
				t.Errorf("DetectPackageManager().Berry() = %v, want %v", pm.Berry(), tt.wantBerry)
			}
		})
	}
}
