package main

import (
	"strings"
	"testing"
)

// These cases all stop before the models are needed; binoculars' tests cover
// scoring. Cobra's errors are formatted for humans, so they assert only that
// the command failed.
func TestRun_Fails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--nope"}},
		{name: "two files", args: []string{"a", "b"}},
		{name: "no cache", args: []string{"--cache", ""}},
		{name: "missing file", args: []string{"--cache", "unused", "does-not-exist"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder
			if err := run(t.Context(), tt.args, strings.NewReader(""), &stdout, &stderr); err == nil {
				t.Fatalf("run(%q) = nil, want an error", tt.args)
			}
		})
	}
}

// Asking for help and getting it is not a failure, so main exits zero.
func TestRun_HelpPrintsFlags(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if err := run(t.Context(), []string{"-h"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("run(-h) = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "--threshold") {
		t.Errorf("run(-h) printed %q, want it to list --threshold", stdout.String())
	}
}

func TestRun_VersionPrintsVersion(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if err := run(t.Context(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("run(--version) = %v", err)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Error("run(--version) printed nothing")
	}
}
