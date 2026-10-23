package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// sourceState is the provenance fingerprint of one package base, persisted
// across site runs so the next run can notice drift: a checksum changing
// under an unchanged URL, or a pinned commit moving without a version bump —
// classic signs of a hijacked upstream release or a bad-faith update. This is
// deliberately a site-generator concern, not a lint rule: a single PKGBUILD
// snapshot cannot see its own history.
type sourceState struct {
	LastModified int64                        `json:"last_modified"`
	Pkgver       string                       `json:"pkgver,omitempty"`
	Sums         map[string]map[string]string `json:"sums,omitempty"` // source URL -> algo -> digest
	Pins         map[string]string            `json:"pins,omitempty"` // VCS URL -> commit
}

// extractState fingerprints a loaded package: its pkgver, every remote
// source's declared digests keyed by URL, and every VCS source's commit pin.
func extractState(pkg *pkgbuild.Package, lastModified int64) sourceState {
	st := sourceState{
		LastModified: lastModified,
		Sums:         map[string]map[string]string{},
		Pins:         map[string]string{},
	}
	if v, ok := pkg.Scalar("pkgver"); ok {
		st.Pkgver = v
	}
	for _, e := range pkg.Sources() {
		if e.VCS != "" {
			if c, ok := e.Fragment["commit"]; ok {
				st.Pins[e.URL] = strings.ToLower(c)
			}
			continue
		}
		if e.Local {
			continue
		}
		sums := map[string]string{}
		for algo, vals := range pkg.Checksums(e.Arch) {
			if e.Index < len(vals) && !strings.EqualFold(vals[e.Index], "SKIP") {
				sums[algo] = strings.ToLower(vals[e.Index])
			}
		}
		if len(sums) > 0 {
			st.Sums[e.URL] = sums
		}
	}
	return st
}

// driftNotes compares two fingerprints of the same package base from
// different snapshots and describes suspicious movement. It only speaks at
// an unchanged pkgver, where the comparison is apples-to-apples: a bumped
// release legitimately changes everything else.
//
// That holds for checksums under a URL that did not change, too. A URL can
// look unchanged across a release without being so — a rolling one
// (releases/download/nightly/…) serves each new build at the same address,
// and one whose version comes from a variable the parser cannot resolve
// (`$pkgver_amd64`, a function-set `_ver`) loses the version from its text —
// and both would otherwise raise a note on every upload. The case the note
// exists for, an artifact swapped out under a release already reviewed,
// keeps its pkgver, a pkgrel bump included.
func driftNotes(prev, cur sourceState) []string {
	if prev.LastModified == 0 || prev.LastModified == cur.LastModified {
		return nil // first sighting, or the very same snapshot
	}
	if prev.Pkgver == "" || prev.Pkgver != cur.Pkgver {
		return nil // a release, or a version that cannot be compared
	}
	var notes []string
	for url, sums := range cur.Sums {
		psums, ok := prev.Sums[url]
		if !ok {
			continue
		}
		for algo, digest := range sums {
			if pd, ok := psums[algo]; ok && pd != digest {
				notes = append(notes, fmt.Sprintf(
					"%s checksum for %s changed (%s → %s) while the URL stayed the same: the artifact is no longer the one previously reviewed",
					algo, url, shortDigest(pd), shortDigest(digest)))
			}
		}
	}
	for url, pin := range cur.Pins {
		if ppin, ok := prev.Pins[url]; ok && ppin != pin {
			notes = append(notes, fmt.Sprintf(
				"pinned commit for %s moved (%s → %s) without a pkgver change",
				url, shortDigest(ppin), shortDigest(pin)))
		}
	}
	sort.Strings(notes)
	return notes
}

func shortDigest(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}
