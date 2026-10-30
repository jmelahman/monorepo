package project_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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

func TestAgeGate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.January, 8, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		wantErr  error
		name     string
		files    map[string]string
		version  string
		wantArgs []string
		wantEnv  []string
	}{
		{
			name:     "bun",
			files:    map[string]string{"package.json": `{"packageManager": "bun"}`},
			version:  "1.3.0",
			wantArgs: []string{"--minimum-release-age=604800"},
		},
		{
			name:    "old bun would ignore it",
			files:   map[string]string{"package.json": `{"packageManager": "bun"}`},
			version: "1.2.23",
			wantErr: project.ErrNoCooldown,
		},
		{
			name:    "bun version that doesn't parse",
			files:   map[string]string{"package.json": `{"packageManager": "bun"}`},
			version: "unknown",
			wantErr: project.ErrNoCooldown,
		},
		{
			name:     "npm",
			files:    map[string]string{"package.json": `{"packageManager": "npm"}`},
			version:  "6.0.0",
			wantArgs: []string{"--before=2026-01-01T00:00:00Z"},
		},
		{
			name:     "nub",
			files:    map[string]string{"package.json": `{"packageManager": "nub"}`},
			version:  "0.9.7",
			wantArgs: []string{"--minimum-release-age=10080m"},
		},
		{
			name:     "pnpm",
			files:    map[string]string{"package.json": `{"packageManager": "pnpm"}`},
			version:  "10.16.0",
			wantArgs: []string{"--config.minimumReleaseAge=10080"},
		},
		{
			name:    "old pnpm would ignore it",
			files:   map[string]string{"package.json": `{"packageManager": "pnpm"}`},
			version: "10.15.1",
			wantErr: project.ErrNoCooldown,
		},
		{
			name:    "yarn berry",
			files:   map[string]string{"package.json": `{"packageManager": "yarn@4.10.1"}`},
			version: "4.10.1",
			wantEnv: []string{"YARN_NPM_MINIMAL_AGE_GATE=10080m"},
		},
		{
			name:    "yarn 1 would ignore it",
			files:   map[string]string{"package.json": `{"packageManager": "yarn@1.22.22"}`},
			version: "1.22.22",
			wantErr: project.ErrNoCooldown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, data := range tt.files {
				write(t, filepath.Join(dir, name), data)
			}
			pm, err := project.DetectPackageManager(dir)
			if err != nil {
				t.Fatal(err)
			}
			args, env, err := pm.AgeGate(tt.version, 7*24*time.Hour, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("AgeGate() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) || !reflect.DeepEqual(env, tt.wantEnv) {
				t.Errorf("AgeGate() = %q, %q, want %q, %q", args, env, tt.wantArgs, tt.wantEnv)
			}
		})
	}
}
