package project_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/vinder/internal/project"
)

// TestAddRejects covers what Add refuses before it touches anything.
func TestAddRejects(t *testing.T) {
	t.Parallel()

	const config = "[deps.lib]\npackage = \"lib\"\nbundle = { \"l.js\" = \"lib\" }\n\n[deps.at-other]\npackage = \"renamed\"\nbundle = { \"o.js\" = \"renamed\" }\n"
	tests := []struct {
		wantErr error
		name    string
		specs   []string
	}{
		{name: "path", specs: []string{"../lib"}, wantErr: project.ErrBadPackageSpec},
		{name: "url", specs: []string{"https://example.com/x.tgz"}, wantErr: project.ErrBadPackageSpec},
		{name: "quote", specs: []string{`a"b`}, wantErr: project.ErrBadPackageSpec},
		{name: "same package", specs: []string{"lib"}, wantErr: project.ErrAlreadyVendored},
		{name: "same package under another key", specs: []string{"renamed"}, wantErr: project.ErrAlreadyVendored},
		{name: "key in use", specs: []string{"@at/other"}, wantErr: project.ErrAlreadyVendored},
		{name: "key given twice", specs: []string{"new.pkg", "new-pkg"}, wantErr: project.ErrAlreadyVendored},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			write(t, filepath.Join(dir, project.ConfigFile), config)
			_, _, err := project.Add(context.Background(), dir, tt.specs, func(context.Context, string, string) (string, error) {
				t.Error("looked up a release")
				return "", nil
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add(%q) error = %v, want %v", tt.specs, err, tt.wantErr)
			}
			if _, err := os.Stat(filepath.Join(dir, "package.json")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("Add(%q) wrote package.json", tt.specs)
			}
		})
	}
}

// TestAddUnmanagedOutput refuses to replace what is already at a package's
// output path, and leaves it there.
func TestAddUnmanagedOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
		file   string
	}{
		{name: "directory", file: "vendor/lib/mine.js"},
		{name: "file", file: "vendor/lib"},
		{name: "custom out", config: "out = \"web/vendor\"\n", file: "web/vendor/lib/mine.js"},
		{name: "unreadable manifest", file: "vendor/lib/mine.js"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			write(t, filepath.Join(dir, tt.file), "mine\n")
			if tt.config != "" {
				write(t, filepath.Join(dir, project.ConfigFile), tt.config)
			}
			if tt.name == "unreadable manifest" {
				write(t, filepath.Join(dir, "vendor", project.ManifestFile), "<<<<<<<\n")
			}
			_, _, err := project.Add(context.Background(), dir, []string{"lib"}, func(context.Context, string, string) (string, error) {
				t.Error("looked up a release")
				return "", nil
			})
			if !errors.Is(err, project.ErrUnmanagedOutput) {
				t.Fatalf("Add() error = %v, want %v", err, project.ErrUnmanagedOutput)
			}
			if _, err := os.Stat(filepath.Join(dir, tt.file)); err != nil {
				t.Errorf("Add() removed %s: %v", tt.file, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "package.json")); !errors.Is(err, os.ErrNotExist) {
				t.Error("Add() wrote package.json")
			}
		})
	}
}
