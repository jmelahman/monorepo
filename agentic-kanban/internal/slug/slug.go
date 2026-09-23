// Package slug converts arbitrary strings into lowercase, dash-separated
// slugs suitable for use as git ref segments, board/ticket slugs, and
// filesystem-safe identifiers.
//
// It is a dependency-free leaf package so that api, errreport, buildcop, and
// any future caller can share one implementation without the import cycles
// that previously forced each package to keep its own copy.
package slug

import "strings"

// MaxLen caps slugified output so that branch names built from a slug stay
// within git's ref-length limits.
const MaxLen = 80

// Make converts s into a lowercase, dash-separated slug of at most MaxLen
// bytes. Runs of non-alphanumeric characters collapse to a single dash, and
// leading/trailing dashes are trimmed (including any dash exposed by
// truncation). If the result would be empty, fallback is returned verbatim.
func Make(s, fallback string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteRune(c)
			prevDash = false
		case c == '-' || c == '_':
			b.WriteRune('-')
			prevDash = true
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > MaxLen {
		out = strings.TrimRight(out[:MaxLen], "-")
	}
	if out == "" {
		out = fallback
	}
	return out
}
