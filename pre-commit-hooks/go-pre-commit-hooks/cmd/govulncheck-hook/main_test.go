package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stream holds one advisory the code calls, one it only requires, and one
// whose package it only imports.
const stream = `
{"config": {"scanner_name": "govulncheck"}}
{"osv": {"id": "GO-1", "summary": "called"}}
{"osv": {"id": "GO-2", "summary": "required"}}
{"osv": {"id": "GO-3", "summary": "imported"}}
{"finding": {"osv": "GO-1", "fixed_version": "v1.1.0", "trace": [{"module": "example.com/a", "version": "v1.0.0"}]}}
{"finding": {"osv": "GO-2", "trace": [{"module": "example.com/b", "version": "v2.0.0"}]}}
{"finding": {"osv": "GO-3", "trace": [{"module": "example.com/c", "version": "v3.0.0", "package": "example.com/c/p"}]}}
{"finding": {"osv": "GO-1", "fixed_version": "v1.1.0", "trace": [{"module": "example.com/a", "version": "v1.0.0", "package": "example.com/a", "function": "F"}, {"module": "example.com/me", "function": "main"}]}}
`

func TestCalled(t *testing.T) {
	vulns, err := called(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	want := vuln{ID: "GO-1", Summary: "called", Module: "example.com/a", Version: "v1.0.0", FixedVersion: "v1.1.0"}
	if len(vulns) != 1 || vulns[0] != want {
		t.Errorf("called() = %+v, want [%+v]", vulns, want)
	}
}

func TestReport(t *testing.T) {
	vulns := []vuln{{ID: "GO-1", Summary: "called"}, {ID: "GO-4", Summary: "unfixed"}}
	var out strings.Builder
	if got := report(&out, vulns, map[string]bool{"GO-4": true}); got != 1 {
		t.Errorf("report() = %d, want 1", got)
	}
	for _, want := range []string{"ignored GO-4: unfixed", "GO-1: called", "fixed in: no fix available"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestReadIgnores(t *testing.T) {
	path := filepath.Join(t.TempDir(), ignoreFile)
	if ids, err := readIgnores(path); err != nil || len(ids) != 0 {
		t.Errorf("missing file: got %v, %v", ids, err)
	}

	content := "# why GO-1 is safe\nGO-1\n\n  GO-2  # trailing\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ids, err := readIgnores(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || !ids["GO-1"] || !ids["GO-2"] {
		t.Errorf("readIgnores() = %v, want GO-1 and GO-2", ids)
	}
}
