package main

import (
	"strings"
	"testing"
)

// TestRenderDoc covers the backtick-to-<code> pass over rule documentation,
// which is authored for the terminal and reused verbatim in HTML.
func TestRenderDoc(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Pin every source.", "Pin every source."},
		{"one span", "Use `sha256sums` instead.", "Use <code>sha256sums</code> instead."},
		{"two spans", "`makedepends` not `depends`", "<code>makedepends</code> not <code>depends</code>"},
		// A lone backtick is far more likely to be prose than a broken span, so
		// it stays text rather than swallowing the rest of the sentence.
		{"unmatched", "A stray ` here", "A stray ` here"},
		{"unmatched after span", "`a` and a stray `", "<code>a</code> and a stray `"},
		// The escape has to happen inside the span too: rule docs quote shell.
		{"escapes outside", "a < b & c", "a &lt; b &amp; c"},
		{"escapes inside", "Avoid `curl | sh` and `a<b`", "Avoid <code>curl | sh</code> and <code>a&lt;b</code>"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(renderDoc(tc.in)); got != tc.want {
				t.Errorf("renderDoc(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestScanError checks that a parse failure is reduced to the part a reader can
// act on. The raw error names pkglint's snapshot cache twice, which is an
// internal path that means nothing to someone reading the report card.
func TestScanError(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			"snapshot path",
			"parse .cache/snapshots/mesa-git@1786811951/PKGBUILD: .cache/snapshots/mesa-git@1786811951/PKGBUILD:289:18: `[` must be followed by an expression",
			"PKGBUILD:289:18: `[` must be followed by an expression",
		},
		{
			"nested file",
			"parse .cache/snapshots/foo@1/PKGBUILD: .cache/snapshots/foo@1/src/build.sh:2:1: bad",
			"build.sh:2:1: bad",
		},
		// Errors that never went through the parse wrapper are left alone.
		{"no prefix", "PKGBUILD:1:1: bad", "PKGBUILD:1:1: bad"},
		{"unrelated", "fetch failed", "fetch failed"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := scanError(tc.in); got != tc.want {
				t.Errorf("scanError(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestBadgeSVG pins the properties the badge is meant to have: the site's grade
// ramp rather than the shields.io palette, and dark text on the coloured half,
// which is what makes one ramp work for every grade. The colours must track
// site.css; a badge that disagrees with the report card is worse than no badge.
func TestBadgeSVG(t *testing.T) {
	want := map[string]string{
		"A": "#57b87e", "B": "#a3c265", "C": "#d8b04a",
		"D": "#e08a48", "F": "#e05a55", "?": "#898994",
	}
	seen := map[string]bool{}
	for grade, color := range want {
		svg := badgeSVG(grade)
		if !strings.Contains(svg, color) {
			t.Errorf("badge %q missing ramp colour %s", grade, color)
		}
		if !strings.Contains(svg, `aria-label="pkglint: `+grade+`"`) {
			t.Errorf("badge %q missing accessible label", grade)
		}
		if !strings.Contains(svg, `fill="#0a0a0c"`) {
			t.Errorf("badge %q does not put dark text on the grade", grade)
		}
		seen[color] = true
	}
	// An unknown grade falls back to the unscannable slate, not to a colour
	// that would read as a passing grade.
	if got := badgeSVG("Z"); !strings.Contains(got, want["?"]) {
		t.Error("unknown grade did not fall back to the unscannable colour")
	}
	if len(seen) != len(want) {
		t.Errorf("ramp reuses a colour across grades: %d distinct for %d grades", len(seen), len(want))
	}
}
