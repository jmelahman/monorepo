package cli

import (
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
	"solod.dev/so/path"
	"solod.dev/so/strings"
)

// confFileName is the configuration read from .config/ in the repository
// and from ${XDG_CONFIG_HOME:-~/.config}/undot/ for the user. It is never
// symlinked into the repository root.
//
// Format, one rule per line:
//   - a root-level entry name or shell glob (".env", ".cla*") that
//     `undot migrate` should leave alone;
//   - "!name" to un-ignore an entry (overriding earlier rules and the
//     built-in skip list);
//   - "nolink name" for entries that live in .config/ but should not be
//     symlinked into the root (tools are pointed at the .config/ path
//     directly). Repository file only: a user-level nolink would silently
//     break repositories that expect their links.
//
// "#" starts a comment. The user file loads first, then the repository
// file; the last matching ignore rule wins.
const confFileName = "undot.conf"

type ignoreRule struct {
	pattern string
	negate  bool
}

// The CLI is single-threaded and rules must outlive the loading frame, so
// package-level storage is the simplest safe home for them. Patterns are
// views into arena-backed file contents.
var ignoreRules [maxEntries]ignoreRule
var nIgnoreRules int

var nolinkRules [maxEntries]string
var nNolinkRules int

// loadIgnoreRules reads the user-level and repository-level config files.
// Must be called with the repository root as the working directory.
func loadIgnoreRules(a mem.Allocator) {
	nIgnoreRules = 0
	nNolinkRules = 0
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home := os.Getenv("HOME")
		if home != "" {
			base = home + "/.config"
		}
	}
	if base != "" {
		loadConfFile(a, base+"/undot/"+confFileName, false)
	}
	loadConfFile(a, configDir+"/"+confFileName, true)
}

func loadConfFile(a mem.Allocator, fname string, repoLevel bool) {
	data, err := os.ReadFile(a, fname)
	if err != nil {
		return
	}
	lines := strings.Split(a, string(data), "\n")
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		nolink := false
		if strings.HasPrefix(t, "nolink ") || strings.HasPrefix(t, "nolink\t") {
			nolink = true
			t = strings.TrimSpace(t[6:])
			if !repoLevel {
				fmt.Fprintf(os.Stderr, "undot: %s: nolink is only honored in the repository config; ignoring %s\n", fname, t)
				continue
			}
		}
		neg := false
		if strings.HasPrefix(t, "!") {
			neg = true
			t = strings.TrimSpace(t[1:])
		}
		if t == "" {
			continue
		}
		if strings.IndexByte(t, '/') >= 0 {
			fmt.Fprintf(os.Stderr, "undot: %s: ignoring %s (rules match root-level names, not paths)\n", fname, t)
			continue
		}
		if _, merr := path.Match(t, "x"); merr != nil {
			fmt.Fprintf(os.Stderr, "undot: %s: ignoring malformed pattern %s\n", fname, t)
			continue
		}
		if nolink {
			if nNolinkRules < maxEntries {
				nolinkRules[nNolinkRules] = t
				nNolinkRules++
			}
			continue
		}
		if nIgnoreRules < maxEntries {
			ignoreRules[nIgnoreRules] = ignoreRule{pattern: t, negate: neg}
			nIgnoreRules++
		}
	}
}

// isNolink reports whether a .config/ entry should be left without a root
// symlink; tools are expected to reference the .config/ path directly.
func isNolink(name string) bool {
	for i := range nNolinkRules {
		ok, merr := path.Match(nolinkRules[i], name)
		if merr == nil && ok {
			return true
		}
	}
	return false
}

// isIgnored reports whether the automatic scan should skip a root entry.
// Config rules take precedence over the built-in skip list; among rules,
// the last match wins.
func isIgnored(name string) bool {
	matched := false
	ignored := false
	for i := range nIgnoreRules {
		ok, merr := path.Match(ignoreRules[i].pattern, name)
		if merr == nil && ok {
			matched = true
			ignored = !ignoreRules[i].negate
		}
	}
	if matched {
		return ignored
	}
	return containsString(skipByDefault, name)
}
