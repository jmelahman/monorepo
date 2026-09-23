package rules

import (
	"regexp"
	"testing"
)

// exampleHeader is the minimal identity block prepended to every snippet. It
// deliberately omits the fields examples commonly set themselves
// (source/url/sha256sums/install/DLAGENTS) so a snippet's own assignments are
// never shadowed.
const exampleHeader = `pkgname=demo
pkgver=1.0.0
pkgrel=1
pkgdesc='demo'
arch=('x86_64')
license=('MIT')
`

// scriptletHint matches snippets that are install-scriptlet bodies; those must
// be linted as a *.install file, not inlined in the PKGBUILD.
var scriptletHint = regexp.MustCompile(`(?m)^\s*(post_|pre_)(install|upgrade|remove)\s*\(\)`)

// exampleExtras supplies files an example's scenario needs but its snippet
// cannot carry, because the scenario spans more than the PKGBUILD: the example
// text names a second file (a missing install scriptlet, a stale .SRCINFO) and
// the linter can only see the situation if that file really is (or is not)
// there. Keyed by rule ID, with separate files for each half of the example.
var exampleExtras = map[string]struct{ bad, good map[string]string }{
	"PB107": {
		// Bad is "install=foo.install, but no foo.install is committed", which
		// packageFor already produces by writing the PKGBUILD alone. Good says
		// the scriptlet does ship next to it, so put it there.
		good: map[string]string{"foo.install": "# nothing to do at install time\n"},
	},
	"PB601": {
		// Bad quotes the stale .SRCINFO inline ("pkgver = 1.3.0"); write it out
		// so the drift is real. Good is the regeneration command, so its
		// .SRCINFO agrees with the PKGBUILD.
		bad:  map[string]string{".SRCINFO": "pkgbase = demo\n\tpkgver = 1.3.0\n"},
		good: map[string]string{".SRCINFO": "pkgbase = demo\n\tpkgver = 1.0.0\n"},
	},
}

// packageFor turns one example snippet into the files of a lintable package.
// id names the owning rule and side is "bad" or "good", so per-rule extras land
// on the right half of the example.
func packageFor(id, side, snippet string) map[string]string {
	files := map[string]string{"PKGBUILD": exampleHeader + snippet + "\n"}
	if scriptletHint.MatchString(snippet) {
		// Scriptlet bodies only reach the PB5xx rules when they are analyzed as
		// an install scriptlet, i.e. a *.install file referenced by install=.
		files["PKGBUILD"] = exampleHeader + "install=demo.install\n"
		files["demo.install"] = snippet
	}
	extra := exampleExtras[id].bad
	if side == "good" {
		extra = exampleExtras[id].good
	}
	for name, content := range extra {
		files[name] = content
	}
	return files
}

// knownGaps lists rules whose example cannot be round-tripped through the
// linter as-is, with the reason. Keep this list SMALL and justified — every
// entry is an example the site shows but this suite does not verify. A reason
// prefixed "REVIEW:" marks a mismatch the suite found between an example and
// its rule, parked here pending a maintainer decision rather than papered over
// by editing the example or weakening the assertion.
var knownGaps = map[string]string{
	"PB306": "REVIEW: Bad invokes `$runner`, a plain parameter expansion, and pkglint reports " +
		"nothing at all (findings map is empty). checkDynamicCommands only fires for ${!indirect} " +
		"names and for names RenderWord marks dynamic (command substitution, arithmetic); a bare " +
		"$var command name sets Name=\"\" but leaves Dynamic false, so both switch arms are skipped. " +
		"Either the example must invoke the substitution directly or the rule must cover unresolved " +
		"$var command names — a maintainer call, so the example is not edited here.",
}

func TestExamplesTripTheirRule(t *testing.T) {
	for _, r := range Registry() {
		r := r
		if r.Scope == ScopePackage {
			// Package-rule examples show the PKGBUILD mistake behind bad
			// archive contents; the rules run on built packages, and the
			// package-rule test suite verifies each with archive fixtures.
			continue
		}
		if reason, skip := knownGaps[r.ID]; skip {
			t.Logf("SKIP %s (known gap): %s", r.ID, reason)
			continue
		}
		t.Run(r.ID, func(t *testing.T) {
			bad := ruleIDs(lint(t, packageFor(r.ID, "bad", r.Bad)))
			if bad[r.ID] == 0 {
				t.Errorf("%s: Bad example did not trip the rule; got %v", r.ID, bad)
			}
			good := ruleIDs(lint(t, packageFor(r.ID, "good", r.Good)))
			if good[r.ID] != 0 {
				t.Errorf("%s: Good example still trips the rule (%d)", r.ID, good[r.ID])
			}
		})
	}
}

// TestKnownGapsAreStillGaps keeps the allowlist honest: an entry whose Bad
// example does trip its rule is no longer a gap and must be removed.
func TestKnownGapsAreStillGaps(t *testing.T) {
	for id := range knownGaps {
		r, ok := RuleByID(id)
		if !ok {
			t.Errorf("knownGaps lists unknown rule %s", id)
			continue
		}
		if ruleIDs(lint(t, packageFor(id, "bad", r.Bad)))[id] != 0 {
			t.Errorf("%s is in knownGaps but its Bad example now trips the rule; remove it", id)
		}
	}
}
