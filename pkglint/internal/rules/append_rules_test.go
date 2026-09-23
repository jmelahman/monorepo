package rules

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// The PKGBUILDs below share a header and differ only in how the source array
// (and its checksums) is spelled: one writes every entry in a single
// `source=(...)`, the other appends with `source+=(...)`. The `+=` variants
// keep the first `source=` on the same line as the reference, which is where a
// merged Var anchors its position, so findings land on the same line either
// way.
const appendDiffHeader = `pkgname=demo
pkgver=1.0.0
pkgrel=1
pkgdesc='A demo'
arch=('x86_64')
url='https://github.com/example/demo'
license=('MIT')
`

const (
	goodSource = `"https://github.com/example/demo/archive/v1.0.0.tar.gz"`
	// Plaintext http:// on a host that is not the project url: trips PB104
	// (insecure-transport) and PB105 (source-url-mismatch).
	badSource = `"http://mirror.example.net/extra.patch"`
	sumA      = `'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'`
	sumB      = `'beefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdead'`
)

// findingsAt strips the temp-dir prefix from Path so findings produced in two
// different package dirs are directly comparable, and drops the column.
//
// Columns cannot match across the two spellings: a merged `+=` Var keeps only
// the first assignment's Assign, so an appended element has no AST element to
// take a position from and falls back to the array's own position (the column
// of `source=`). The reference spelling writes that same entry inside the base
// array, where it gets its own column. Line still matches, because the
// fallback position is the base `source=` line and the reference lists every
// source on that line. Which findings are reported — the property this test
// exists to pin — is compared exactly.
func findingsAt(fs []Finding) []Finding {
	out := make([]Finding, 0, len(fs))
	for _, f := range fs {
		f.Path = filepath.Base(f.Path)
		f.Col = 0
		out = append(out, f)
	}
	// Run's total order includes the column we just dropped, so two spellings
	// can interleave same-line findings differently; re-sort on what remains.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Message < b.Message
	})
	return out
}

// TestAppendedSourceLintsIdenticallyToBaseArray is the load-bearing regression
// test for `+=` support. `extractTopLevel` used to overwrite on collision, so
// `source+=(...)` threw the base `source=(...)` array away and the rules saw a
// different set of sources than makepkg fetches. Either half could be hidden:
// writing a malicious source as an append made it invisible when the base
// array won, and appending a benign source hid a malicious base array.
//
// Each case pairs a reference PKGBUILD that spells the whole array at once
// with a `+=` variant containing exactly the same sources. Every rule must
// report identical findings for both, lines included (see findingsAt for why
// columns are exempt).
func TestAppendedSourceLintsIdenticallyToBaseArray(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reference string // single `source=(...)` array
		appended  string // same sources, split across `=` and `+=`
	}{
		{
			// Pre-fix this hid the *base* array: only the appended entry
			// survived, so the http:// source disappeared and PB104/PB105
			// never fired at all.
			name:      "problematic source in the base array",
			reference: "source=(" + badSource + " " + goodSource + ")\nsha256sums=(" + sumA + " " + sumB + ")\n",
			appended:  "source=(" + badSource + ")\nsource+=(" + goodSource + ")\nsha256sums=(" + sumA + ")\nsha256sums+=(" + sumB + ")\n",
		},
		{
			// The mirror image: the problematic source is the appended one.
			name:      "problematic source appended",
			reference: "source=(" + goodSource + " " + badSource + ")\nsha256sums=(" + sumA + " " + sumB + ")\n",
			appended:  "source=(" + goodSource + ")\nsource+=(" + badSource + ")\nsha256sums=(" + sumA + ")\nsha256sums+=(" + sumB + ")\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reference := findingsAt(lint(t, map[string]string{"PKGBUILD": appendDiffHeader + tc.reference}))
			appended := findingsAt(lint(t, map[string]string{"PKGBUILD": appendDiffHeader + tc.appended}))

			if len(reference) == 0 {
				t.Fatal("reference PKGBUILD produced no findings; the differential test proves nothing")
			}
			if !reflect.DeepEqual(reference, appended) {
				t.Errorf("findings differ between base-array and += spellings:\n reference: %+v\n appended:  %+v", reference, appended)
			}

			// Name the rule explicitly rather than relying on set equality
			// alone: the plaintext http:// source must trip
			// PB104 (insecure-transport) in both spellings.
			for name, fs := range map[string][]Finding{"reference": reference, "appended": appended} {
				var got int
				for _, f := range fs {
					if f.RuleID == "PB104" {
						got++
					}
				}
				if got != 1 {
					t.Errorf("%s: got %d PB104 (insecure-transport) findings, want 1: %+v", name, got, fs)
				}
			}
		})
	}
}

// TestAppendedSourceCountsForChecksumPairing pins the other half of the hole:
// once both halves of the source array are visible, an appended source with no
// corresponding checksum is an unverified source and must be reported. Before
// append support the arrays were never compared at full length, so this
// PKGBUILD linted clean.
func TestAppendedSourceCountsForChecksumPairing(t *testing.T) {
	files := map[string]string{"PKGBUILD": appendDiffHeader +
		"source=(" + goodSource + ")\nsource+=(\"https://mirror.example.net/extra.patch\")\nsha256sums=(" + sumA + ")\n"}
	got := ruleIDs(lint(t, files))
	if got["PB110"] != 1 {
		t.Errorf("got %d PB110 (checksum-count-mismatch) findings, want 1: %v", got["PB110"], got)
	}
}
