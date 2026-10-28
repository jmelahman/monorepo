package main

import (
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/jmelahman/golang-template/greeter"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		want       string
		args       []string
		wantErr    error
		wantAnyErr bool
	}{
		{name: "no arguments greets the world", want: "Hello, world!\n"},
		{name: "positional name", args: []string{"gopher"}, want: "Hello, gopher!\n"},
		{name: "multi-word name", args: []string{"the", "gopher"}, want: "Hello, the gopher!\n"},
		{name: "flags", args: []string{"-greeting", "Howdy", "-shout", "gopher"}, want: "HOWDY, GOPHER!\n"},
		{name: "blank name", args: []string{"  "}, wantErr: greeter.ErrEmptyName},
		// main() turns this sentinel into a zero exit status: asking for help
		// and getting it is not a failure.
		{name: "help", args: []string{"-h"}, wantErr: flag.ErrHelp},
		{name: "unknown flag", args: []string{"-nope"}, wantAnyErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder
			err := run(tt.args, &stdout, &stderr)
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
			if got := stdout.String(); got != tt.want {
				t.Errorf("run(%q) stdout = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if err := run([]string{"-version"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(-version) = %v", err)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Error("run(-version) printed nothing")
	}
}
