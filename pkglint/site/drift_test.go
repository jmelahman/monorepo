package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/rules"
)

// stateFor loads a PKGBUILD from a string and fingerprints it.
func stateFor(t *testing.T, content string, lastModified int64) sourceState {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := pkgbuild.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return extractState(pkg, lastModified)
}

const driftBase = `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
source=("https://example.com/demo-1.0.0.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')
`

func TestDriftNotes(t *testing.T) {
	t.Run("no previous state means no drift", func(t *testing.T) {
		cur := stateFor(t, driftBase, 200)
		if notes := driftNotes(sourceState{}, cur); notes != nil {
			t.Errorf("expected no drift on first sighting, got %v", notes)
		}
	})
	t.Run("same snapshot means no drift", func(t *testing.T) {
		prev := stateFor(t, driftBase, 100)
		cur := stateFor(t, driftBase, 100)
		if notes := driftNotes(prev, cur); notes != nil {
			t.Errorf("expected no drift for identical snapshot, got %v", notes)
		}
	})
	t.Run("checksum change under an unchanged URL is drift", func(t *testing.T) {
		prev := stateFor(t, driftBase, 100)
		tampered := strings.Replace(driftBase,
			"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			"2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae", 1)
		cur := stateFor(t, tampered, 200)
		notes := driftNotes(prev, cur)
		if len(notes) != 1 || !strings.Contains(notes[0], "checksum for https://example.com/demo-1.0.0.tar.gz changed") {
			t.Errorf("expected checksum drift note, got %v", notes)
		}
	})
	t.Run("version bump with a new URL is not drift", func(t *testing.T) {
		prev := stateFor(t, driftBase, 100)
		bumped := strings.ReplaceAll(strings.Replace(driftBase,
			"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			"2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae", 1),
			"1.0.0", "1.1.0")
		cur := stateFor(t, bumped, 200)
		if notes := driftNotes(prev, cur); notes != nil {
			t.Errorf("expected no drift for a version bump, got %v", notes)
		}
	})
	t.Run("commit pin moving without a pkgver change is drift", func(t *testing.T) {
		vcs := `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
source=("git+https://example.com/demo.git#commit=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
sha256sums=('SKIP')
`
		prev := stateFor(t, vcs, 100)
		moved := strings.Replace(vcs, "commit=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"commit=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1)
		cur := stateFor(t, moved, 200)
		notes := driftNotes(prev, cur)
		if len(notes) != 1 || !strings.Contains(notes[0], "pinned commit") {
			t.Errorf("expected commit drift note, got %v", notes)
		}
	})
	t.Run("commit pin moving with a pkgver bump is not drift", func(t *testing.T) {
		vcs := `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
source=("git+https://example.com/demo.git#commit=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
sha256sums=('SKIP')
`
		prev := stateFor(t, vcs, 100)
		moved := strings.Replace(strings.Replace(vcs, "commit=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"commit=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1), "pkgver=1.0.0", "pkgver=1.1.0", 1)
		cur := stateFor(t, moved, 200)
		if notes := driftNotes(prev, cur); notes != nil {
			t.Errorf("expected no drift for a pinned bump, got %v", notes)
		}
	})
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	st := map[string]stateRecord{"demo": {
		Base: "demo", LastModified: 123, Grade: "B",
		Findings:    []rules.Finding{{RuleID: "PB101", Path: "PKGBUILD", Line: 3}},
		Drift:       []string{"a note"},
		Fingerprint: stateFor(t, driftBase, 123),
	}}
	if err := saveState(path, st); err != nil {
		t.Fatal(err)
	}
	got, err := loadState(path)
	if err != nil {
		t.Fatal(err)
	}
	rec := got["demo"]
	if rec.LastModified != 123 || rec.Grade != "B" || rec.Fingerprint.Pkgver != "1.0.0" {
		t.Errorf("state round trip lost data: %+v", rec)
	}
	if len(rec.Findings) != 1 || rec.Findings[0].RuleID != "PB101" {
		t.Errorf("state round trip lost findings: %+v", rec.Findings)
	}
	if len(rec.Drift) != 1 {
		t.Errorf("state round trip lost drift: %+v", rec.Drift)
	}
	if len(rec.Fingerprint.Sums) != 1 {
		t.Errorf("expected one fingerprinted source, got %+v", rec.Fingerprint.Sums)
	}
}

// TestLoadStateMissingIsEmpty pins the first-run case: no state file is the
// ordinary way this starts, not a failure.
func TestLoadStateMissingIsEmpty(t *testing.T) {
	got, err := loadState(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil {
		t.Fatalf("loadState on a missing file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected an empty state, got %v", got)
	}
}

// TestLoadStateRejectsMalformed keeps a damaged state file from being read as
// an empty one. Treating it as empty would rescan the whole corpus and then
// overwrite the file that was merely unparseable.
func TestLoadStateRejectsMalformed(t *testing.T) {
	for name, line := range map[string]string{
		"truncated JSON": `{"base":"demo","last_`,
		"unsafe base":    `{"base":"../../evil","last_modified":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.jsonl")
			if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := loadState(path); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestRenderDrift(t *testing.T) {
	results := []siteResult{{
		Name: "demo", Base: "demo", Version: "1.0-1", Grade: "A",
		Drift: []string{"sha256 checksum for https://example.com/demo.tar.gz changed (aaa → bbb) while the URL stayed the same"},
	}}
	out := t.TempDir()
	for _, sub := range []string{"rules", "package", "badge"} {
		if err := os.MkdirAll(filepath.Join(out, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSite(out, results); err != nil {
		t.Fatalf("renderSite: %v", err)
	}
	pkg, err := os.ReadFile(filepath.Join(out, "package", "demo.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pkg), "Source drift since the previous scan") {
		t.Errorf("package page missing drift section:\n%s", pkg)
	}
	index, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "drifted") {
		t.Errorf("index missing drifted tile:\n%s", index)
	}
}
