package releases

import (
	"regexp"
	"slices"
)

// BumpNone is the suggestion for commits that don't call for a release.
const BumpNone = "none"

// bumpOrder ranks the suggestions, least first.
var bumpOrder = []string{BumpNone, "patch", "minor", "major"}

// typeBumps is what each Conventional Commits type suggests.
var typeBumps = map[string]string{
	"feat":     "minor",
	"fix":      "patch",
	"perf":     "patch",
	"revert":   "patch",
	"docs":     BumpNone,
	"style":    BumpNone,
	"refactor": BumpNone,
	"test":     BumpNone,
	"build":    BumpNone,
	"ci":       BumpNone,
	"chore":    BumpNone,
}

var conventionalSubject = regexp.MustCompile(`^([a-z]+)(?:\([^()]+\))?(!)?: \S`)

// SuggestBump returns the bump that commits with these subjects call for: the
// largest any of them does. Subjects that aren't Conventional Commits are
// skipped.
func SuggestBump(subjects []string) string {
	bump := BumpNone
	for _, subject := range subjects {
		m := conventionalSubject.FindStringSubmatch(subject)
		if m == nil {
			continue
		}
		one, ok := typeBumps[m[1]]
		if !ok {
			continue
		}
		if m[2] != "" {
			one = "major"
		}
		if slices.Index(bumpOrder, one) > slices.Index(bumpOrder, bump) {
			bump = one
		}
	}
	return bump
}
