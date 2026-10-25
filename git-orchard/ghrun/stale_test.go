package ghrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGH puts a gh on PATH that answers gh api calls from routes, by
// endpoint, and 404s the rest, logging each call's arguments to the
// returned file.
func fakeGH(t *testing.T, routes map[string]string) (calls string) {
	t.Helper()
	dir := t.TempDir()
	calls = filepath.Join(dir, "calls")
	var b strings.Builder
	b.WriteString("#!/bin/sh\necho \"$*\" >> " + calls + "\ncase \"$2\" in\n")
	for endpoint, out := range routes {
		b.WriteString("'" + endpoint + "') echo '" + out + "' ;;\n")
	}
	b.WriteString("*) echo 'gh: Not Found (HTTP 404)' >&2; exit 1 ;;\nesac\n")
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(b.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return calls
}

func TestDependabotStale(t *testing.T) {
	for _, tc := range []struct {
		name   string
		routes map[string]string
		stale  bool
	}{
		{"configured", map[string]string{
			"repos/o/r/contents/.github/dependabot.yml?ref=main": "{}",
			"repos/o/r/automated-security-fixes":                 "false",
		}, false},
		{"configured as .yaml", map[string]string{
			"repos/o/r/contents/.github/dependabot.yaml?ref=main": "{}",
			"repos/o/r/automated-security-fixes":                  "false",
		}, false},
		{"security updates on", map[string]string{
			"repos/o/r/automated-security-fixes": "true",
		}, false},
		{"neither", map[string]string{
			"repos/o/r/automated-security-fixes": "false",
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeGH(t, tc.routes)
			why, err := DependabotStale(context.Background(), "o/r", "main")
			if err != nil {
				t.Fatal(err)
			}
			if got := why != ""; got != tc.stale {
				t.Errorf("stale = %t (%q), want %t", got, why, tc.stale)
			}
		})
	}
}

func TestDependabotStaleCantTell(t *testing.T) {
	// No access to the security fixes setting: a 404, like no such setting.
	fakeGH(t, nil)
	if why, err := DependabotStale(context.Background(), "o/r", "main"); err == nil {
		t.Errorf("got %q, want an error", why)
	}
}

func TestDeleteRun(t *testing.T) {
	calls := fakeGH(t, map[string]string{"repos/o/r/actions/runs/1": ""})
	if err := DeleteRun(context.Background(), "o/r", 1); err != nil {
		t.Fatal(err)
	}
	// Gone already, as after a retry.
	if err := DeleteRun(context.Background(), "o/r", 2); err != nil {
		t.Errorf("deleting a deleted run: %v", err)
	}
	log, _ := os.ReadFile(calls)
	if want := "api repos/o/r/actions/runs/1 -X DELETE --silent\n"; !strings.HasPrefix(string(log), want) {
		t.Errorf("gh called with\n%s\nwant first\n%s", log, want)
	}
}
