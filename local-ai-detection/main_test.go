package main

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

// These cases all stop before the models are needed; binoculars' tests cover
// scoring.
func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantErr    error
		wantAnyErr bool
	}{
		// main() turns this sentinel into a zero exit status: asking for help
		// and getting it is not a failure.
		{name: "help", args: []string{"-h"}, wantErr: flag.ErrHelp},
		{name: "unknown flag", args: []string{"-nope"}, wantAnyErr: true},
		{name: "two files", args: []string{"a", "b"}, wantAnyErr: true},
		{name: "no cache", args: []string{"-cache", ""}, wantAnyErr: true},
		{name: "missing file", args: []string{"-cache", "unused", "does-not-exist"}, wantAnyErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder
			err := run(t.Context(), tt.args, strings.NewReader(""), &stdout, &stderr)
			// flag's parse errors are unexported and formatted for humans, so
			// those cases assert only that the command failed.
			if tt.wantAnyErr {
				if err == nil {
					t.Fatalf("run(%q) = nil, want an error", tt.args)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("run(%q) error = %v, want %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if err := run(t.Context(), []string{"-version"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("run(-version) = %v", err)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Error("run(-version) printed nothing")
	}
}
