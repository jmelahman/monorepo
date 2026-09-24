// Package semver parses and increments release tags of the form
// [<prefix>/]v<major>.<minor>.<patch>[-<pre-release>[.<num>]], as tag
// (github.com/jmelahman/tag) does.
package semver

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Version is a parsed release tag.
type Version struct {
	// Prefix is the path before the version, with its trailing slash.
	Prefix              string
	Major, Minor, Patch uint64
	// PreRelease is the pre-release name, e.g. "rc", or empty for a stable
	// release, and PreReleaseNum its number; 0 is written without one.
	PreRelease    string
	PreReleaseNum uint64
}

func (v Version) String() string {
	s := fmt.Sprintf("%sv%d.%d.%d", v.Prefix, v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
		if v.PreReleaseNum > 0 {
			s += "." + strconv.FormatUint(v.PreReleaseNum, 10)
		}
	}
	return s
}

// Base is v without its pre-release.
func (v Version) Base() Version {
	return Version{Prefix: v.Prefix, Major: v.Major, Minor: v.Minor, Patch: v.Patch}
}

// Stable reports whether v is not a pre-release.
func (v Version) Stable() bool {
	return v.PreRelease == ""
}

// Compare orders versions, ignoring their prefixes: by major, minor and
// patch, then a stable release above its pre-releases, then pre-releases by
// name (ASCII, as SemVer orders them, so alpha < beta < rc) and number.
func Compare(a, b Version) int {
	for _, c := range [][2]uint64{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if c[0] != c[1] {
			return cmpUint(c[0], c[1])
		}
	}
	switch {
	case a.Stable() && b.Stable():
		return 0
	case a.Stable():
		return 1
	case b.Stable():
		return -1
	}
	if c := strings.Compare(a.PreRelease, b.PreRelease); c != 0 {
		return c
	}
	return cmpUint(a.PreReleaseNum, b.PreReleaseNum)
}

func cmpUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Parse finds the first version in tag. The search is unanchored, and
// trailing text such as "+<build metadata>" is ignored; the prefix is greedy,
// so a/b/v1.2.3 has the prefix "a/b/".
func Parse(tag string) (Version, error) {
	for i := range tag {
		if v, ok := matchAt(tag[i:]); ok {
			return v, nil
		}
	}
	return Version{}, fmt.Errorf("invalid semver tag: %s", tag)
}

// matchAt matches a version at the start of s, optionally preceded by a
// <prefix>/ path, preferring the longest prefix.
func matchAt(s string) (Version, bool) {
	// A prefix needs at least one character before its slash.
	for i := len(s) - 1; i > 0; i-- {
		if s[i] != '/' {
			continue
		}
		if v, ok := matchCore(s[i+1:]); ok {
			v.Prefix = s[:i+1]
			return v, true
		}
	}
	return matchCore(s)
}

// matchCore matches v<major>.<minor>.<patch>[-<pre-release>[.<num>]] at the
// start of s.
func matchCore(s string) (Version, bool) {
	var v Version
	rest, ok := strings.CutPrefix(s, "v")
	if !ok {
		return v, false
	}
	if v.Major, rest, ok = takeDigits(rest); !ok {
		return v, false
	}
	for _, n := range []*uint64{&v.Minor, &v.Patch} {
		if rest, ok = strings.CutPrefix(rest, "."); !ok {
			return v, false
		}
		if *n, rest, ok = takeDigits(rest); !ok {
			return v, false
		}
	}
	// The pre-release and its number are optional: failing to match either
	// leaves the version parsed so far.
	if rest, ok = strings.CutPrefix(rest, "-"); ok {
		if v.PreRelease, rest, ok = takeLetters(rest); ok {
			if rest, ok = strings.CutPrefix(rest, "."); ok {
				v.PreReleaseNum, _, _ = takeDigits(rest)
			}
		}
	}
	return v, true
}

// takeDigits splits off the leading ASCII digits of s. A number too large
// for a uint64 saturates, so an absurd version still parses as a large one.
func takeDigits(s string) (uint64, string, bool) {
	end := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if end < 0 {
		end = len(s)
	}
	if end == 0 {
		return 0, s, false
	}
	n, err := strconv.ParseUint(s[:end], 10, 64)
	if err != nil {
		n = math.MaxUint64
	}
	return n, s[end:], true
}

// takeLetters splits off the leading ASCII letters of s.
func takeLetters(s string) (string, string, bool) {
	end := strings.IndexFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	})
	if end < 0 {
		end = len(s)
	}
	if end == 0 {
		return "", s, false
	}
	return s[:end], s[end:], true
}

// Increment is the part of a version that Next increments.
type Increment int

const (
	// Auto increments the patch version, or with a suffix, continues that
	// pre-release.
	Auto Increment = iota
	Patch
	Minor
	Major
)

// Next is the version after v, with the pre-release suffix (empty for a
// stable release). Switching v to a different pre-release continues from
// the highest number among existing versions of the same base and suffix.
func (v Version) Next(inc Increment, suffix string, existing []Version) Version {
	next := v
	switch {
	case inc == Major:
		next.Major++
		next.Minor, next.Patch, next.PreReleaseNum = 0, 0, 0
	case inc == Minor:
		next.Minor++
		next.Patch, next.PreReleaseNum = 0, 0
	case inc == Patch || suffix == "":
		next.Patch++
		next.PreReleaseNum = 0
	case v.PreRelease != suffix:
		var largest uint64
		for _, e := range existing {
			if e.PreRelease == suffix && e.Base() == v.Base() {
				largest = max(largest, e.PreReleaseNum)
			}
		}
		if largest > 0 {
			next.PreReleaseNum = largest + 1
		}
	default:
		next.PreReleaseNum++
	}
	next.PreRelease = suffix
	return next
}

// Predecessor is the version expected immediately before v, or false if v is
// the first possible version:
//
//	v1.2.3-rc.2 → v1.2.3-rc.1
//	v1.2.3-rc.1 → v1.2.3-rc
//	v1.2.3-rc   → v1.2.2
//	v1.1.0      → v1.0.0
//	v1.0.0      → v0.0.0
func (v Version) Predecessor() (Version, bool) {
	if v.PreReleaseNum > 0 {
		v.PreReleaseNum--
		return v, true
	}
	v.PreRelease = ""
	switch {
	case v.Patch > 0:
		v.Patch--
	case v.Minor > 0:
		v.Minor--
	case v.Major > 0:
		v.Major--
	default:
		return v, false
	}
	return v, true
}

// Larger returns the stable versions among all with v's prefix that are
// greater than v's base.
func (v Version) Larger(all []Version) []Version {
	var larger []Version
	for _, a := range all {
		if a.Prefix == v.Prefix && a.Stable() && Compare(a, v.Base()) > 0 {
			larger = append(larger, a)
		}
	}
	return larger
}
