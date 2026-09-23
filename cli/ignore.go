package cli

import (
	"solod.dev/so/bufio"
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
	"solod.dev/so/slices"
	"solod.dev/so/strings"
)

// shouldIgnore reports whether a path matches any ignore pattern.
func shouldIgnore(st *state, p string) bool {
	if len(st.patterns) == 0 {
		return false
	}
	rel := relTo(st.topLevel, p)
	base := baseName(rel)
	for i := range st.patterns {
		pat := strings.TrimSuffix(st.patterns[i], "/")
		if strings.HasPrefix(rel, pat) || base == pat {
			return true
		}
	}
	return false
}

// relTo strips the top-level prefix from an absolute path.
func relTo(top string, p string) string {
	if top == "" || len(p) <= len(top) {
		return p
	}
	if strings.HasPrefix(p, top) && p[len(top)] == '/' {
		return p[len(top)+1:]
	}
	return p
}

// loadIgnorePatterns loads patterns from .symlinkignore or
// .config/symlinkignore under the top-level directory.
func loadIgnorePatterns(st *state) {
	var buf [os.MaxPathLen]byte
	p := joinPath(buf[:], st.topLevel, ".symlinkignore")
	if loadPatternsFromFile(st, p) {
		return
	}
	p = joinPath(buf[:], st.topLevel, ".config/symlinkignore")
	loadPatternsFromFile(st, p)
}

// loadPatternsFromFile reads patterns from a file, one per line, and reports
// whether it found any.
func loadPatternsFromFile(st *state, name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	sc := bufio.NewScanner(mem.System, &f)
	defer sc.Free()
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		st.patterns = slices.Append(mem.System, st.patterns, strings.Clone(mem.System, line))
	}
	if st.debug && len(st.patterns) > 0 {
		fmt.Printf("Found ignore file: %s\n", name)
	}
	return len(st.patterns) > 0
}

// freePatterns releases the ignore patterns.
func freePatterns(st *state) {
	for i := range st.patterns {
		mem.FreeString(mem.System, st.patterns[i])
	}
	slices.Free(mem.System, st.patterns)
}
