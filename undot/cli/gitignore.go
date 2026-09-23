package cli

import (
	"solod.dev/so/bytes"
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
	"solod.dev/so/strings"
)

const gitignoreFile = ".gitignore"
const gitignoreHeader = "# Symlinks into .config/, recreated by `undot link`."

// ensureGitignore makes sure every root symlink is ignored via an anchored
// `/name` pattern. Anchored, because the tracked copy lives at .config/name;
// without a trailing slash, because a symlink never matches directory-only
// patterns like `name/`. Returns the number of added lines, or -1 on error.
func ensureGitignore(a mem.Allocator, names []string, dryRun bool) int {
	old := ""
	data, err := os.ReadFile(a, gitignoreFile)
	if err == nil {
		old = string(data)
	}
	lines := strings.Split(a, old, "\n")

	var missing [maxEntries]string
	nMissing := 0
	for _, name := range names {
		pattern := "/" + name
		if hasExactLine(lines, pattern) {
			continue
		}
		if nMissing < maxEntries {
			missing[nMissing] = pattern
			nMissing++
		}
	}

	// Pre-existing patterns that reach inside a migrated entry (".yarn/cache")
	// contain a slash and are therefore root-anchored, so after the move they
	// no longer match the content at .config/. Add a companion pattern for
	// each so `git add -A` doesn't start tracking previously ignored files.
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		neg := strings.HasPrefix(t, "!")
		core := strings.TrimPrefix(t, "!")
		core = strings.TrimPrefix(core, "/")
		if !refersInsideAny(core, names) {
			continue
		}
		companion := "/.config/" + core
		if neg {
			companion = "!" + companion
		}
		if hasExactLine(lines, companion) || containsString(missing[:nMissing], companion) {
			continue
		}
		if nMissing < maxEntries {
			missing[nMissing] = companion
			nMissing++
		}
	}

	if nMissing == 0 {
		return 0
	}
	if dryRun {
		for i := range nMissing {
			fmt.Printf("undot: [dry-run] would add %s to %s\n", missing[i], gitignoreFile)
		}
		return nMissing
	}

	buf := bytes.NewBuffer(a, []byte{})
	buf.WriteString(old)
	if len(old) > 0 && !strings.HasSuffix(old, "\n") {
		buf.WriteByte('\n')
	}
	if !hasExactLine(lines, gitignoreHeader) {
		if len(old) > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(gitignoreHeader)
		buf.WriteByte('\n')
	}
	for i := range nMissing {
		buf.WriteString(missing[i])
		buf.WriteByte('\n')
	}
	if werr := os.WriteFile(gitignoreFile, buf.Bytes(), 0o644); werr != nil {
		fmt.Fprintf(os.Stderr, "undot: cannot write %s: %s\n", gitignoreFile, werr.Error())
		return -1
	}
	fmt.Printf("undot: added %d entries to %s\n", nMissing, gitignoreFile)
	return nMissing
}

// refersInsideAny reports whether the gitignore pattern core (leading "!"
// and "/" already stripped) points inside one of the managed entries, like
// ".yarn/cache" does for ".yarn".
func refersInsideAny(core string, names []string) bool {
	for _, name := range names {
		if len(core) > len(name) && strings.HasPrefix(core, name) && core[len(name)] == '/' {
			return true
		}
	}
	return false
}
