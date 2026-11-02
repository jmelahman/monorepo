package releases

import (
	"regexp"
	"slices"
)

// BumpNone is the suggestion for commits that don't call for a release.
const BumpNone = "none"

// bumpOrder ranks the suggestions, least first.
var bumpOrder = []string{BumpNone, "patch", "minor", "major"}

// typeBumps is what each Conventional Commits type suggests. Keep its types in
// step with tools/scripts/check-commit-subject.sh in the monorepo.
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
// largest any of them does. A subject that isn't a Conventional Commit could be
// anything, so it counts as a patch, and unclassified says how many there were.
func SuggestBump(subjects []string) (bump string, unclassified int) {
	bump = BumpNone
	for _, subject := range subjects {
		one, ok := "", false
		if m := conventionalSubject.FindStringSubmatch(subject); m != nil {
			one, ok = typeBumps[m[1]]
			if ok && m[2] != "" {
				one = "major"
			}
		}
		if !ok {
			one = "patch"
			unclassified++
		}
		if slices.Index(bumpOrder, one) > slices.Index(bumpOrder, bump) {
			bump = one
		}
	}
	return bump, unclassified
}
