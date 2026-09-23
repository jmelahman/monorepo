package rules

import (
	"debug/elf"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgfile"
)

// --- PB809: DT_NEEDED libraries vs depends -----------------------------------

// runpathDirs expands an ELF's RPATH/RUNPATH entries to package-relative
// directories, resolving $ORIGIN against the member's own location.
func runpathDirs(e *pkgfile.Entry) []string {
	var out []string
	for _, p := range append(append([]string{}, e.ELF.Rpath...), e.ELF.Runpath...) {
		p = strings.ReplaceAll(p, "${ORIGIN}", "$ORIGIN")
		p = strings.ReplaceAll(p, "$ORIGIN", "/"+path.Dir(e.Name))
		p = strings.TrimPrefix(path.Clean(p), "/")
		if p != "" && p != "." {
			out = append(out, p)
		}
	}
	return out
}

// resolveNeeded finds who satisfies a DT_NEEDED entry: the package itself
// (standard dirs, soname, or its RPATH dirs), or an installed package (by
// standard library dirs, or by the binary's RPATH dirs).
func (ctx *Context) resolveNeeded(e *pkgfile.Entry, soname string) (owner string, self bool) {
	f := ctx.facts()
	if f.shipsSoname[soname] {
		return "", true
	}
	for _, dir := range runpathDirs(e) {
		if ctx.File.Has(dir + "/" + soname) {
			return "", true
		}
		if o := ctx.DB.PathOwner(dir + "/" + soname); o != "" {
			return o, false
		}
	}
	return ctx.DB.LibraryOwner(soname, e.ELF.Class == elf.ELFCLASS32), false
}

// libDepGap is one installed package this package's binaries link against and
// whose depends coverage falls short, as found by the walk checkMissingLibDeps
// and fixMissingLibDeps share. Sonames and Files are sorted; Files holds up to
// three examples.
type libDepGap struct {
	Owner   string
	Sonames []string
	Files   []string
	State   depSatisfaction
}

// missingLibDeps resolves every DT_NEEDED entry in the package to the installed
// package that owns it and reports the ones depends does not already cover
// directly. Gaps come back ordered by owner; orphans maps a soname no installed
// package provides to up to three files needing it.
//
// The fixer walks this too, so a soname that stops being a gap — because the
// rule's resolution changed, not because the fix ran — takes the fix with it.
func missingLibDeps(ctx *Context) (gaps []libDepGap, orphans map[string][]string) {
	f := ctx.facts()
	type need struct {
		sonames map[string]bool
		files   map[string]bool
		state   depSatisfaction
	}
	byOwner := map[string]*need{}
	orphans = map[string][]string{}
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !isPackageELF(e) {
			continue
		}
		for _, soname := range e.ELF.Needed {
			// A depends entry may pin the soname itself (libz.so=1-64).
			if f.depNames[sonameBase(soname)] {
				continue
			}
			owner, self := ctx.resolveNeeded(e, soname)
			if self {
				continue // satisfied by this very package
			}
			if owner == "" {
				if len(orphans[soname]) < 3 {
					orphans[soname] = append(orphans[soname], e.Name)
				}
				continue
			}
			sat := ctx.satisfaction(owner)
			if sat == depSelf || sat == depDirect {
				continue
			}
			n := byOwner[owner]
			if n == nil {
				n = &need{sonames: map[string]bool{}, files: map[string]bool{}, state: sat}
				byOwner[owner] = n
			}
			n.sonames[soname] = true
			if len(n.files) < 3 {
				n.files[e.Name] = true
			}
			if sat == depMissing {
				n.state = depMissing
			}
		}
	}
	for _, owner := range sortedKeys(mapKeySet(byOwner)) {
		n := byOwner[owner]
		gaps = append(gaps, libDepGap{
			Owner:   owner,
			Sonames: sortedKeys(n.sonames),
			Files:   sortedKeys(n.files),
			State:   n.state,
		})
	}
	return gaps, orphans
}

func checkMissingLibDeps(ctx *Context) []Finding {
	if ctx.DB == nil {
		return nil
	}
	gaps, orphans := missingLibDeps(ctx)
	f := ctx.facts()
	var out []Finding
	for _, g := range gaps {
		libs := strings.Join(g.Sonames, ", ")
		switch g.State {
		case depMissing:
			if !f.closureComplete {
				out = append(out, pkgFinding("PB809", Info, g.Files[0],
					"binaries link %s from package %q, not reachable from depends as installed here — but some declared dependencies are not installed, so this may be incomplete",
					libs, g.Owner))
				continue
			}
			out = append(out, pkgFinding("PB809", Error, g.Files[0],
				"binaries link %s from package %q, which is not reachable from depends; add it",
				libs, g.Owner))
		case depTransitive:
			out = append(out, pkgFinding("PB809", Info, g.Files[0],
				"binaries link %s from package %q, reached only transitively; a direct depends entry protects against the middleman dropping it",
				libs, g.Owner))
		case depOptional:
			out = append(out, pkgFinding("PB809", Warn, g.Files[0],
				"binaries link %s from package %q, which is only an optdepends; a hard link needs a hard dependency",
				libs, g.Owner))
		}
	}
	for soname, files := range orphans {
		at := ".PKGINFO"
		if len(files) > 0 {
			at = files[0]
		}
		out = append(out, pkgFinding("PB809", Info, at,
			"no installed package provides %s, needed by this package's binaries; the dependency cannot be verified on this system", soname))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

// sonameBase strips trailing version digits: "libz.so.1" -> "libz.so".
func sonameBase(soname string) string {
	if i := strings.Index(soname, ".so"); i >= 0 {
		return soname[:i+3]
	}
	return soname
}

// mapKeySet turns any map's keys into the set sortedKeys takes, so a walk that
// accumulates per-owner state can still be emitted in a stable order.
func mapKeySet[V any](m map[string]V) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- PB810: NEEDED libraries no symbol comes from -----------------------------

// hostLibDirs is where exported-symbol lookups for libraries outside the
// package are attempted. Read-only host access; nothing is executed.
var hostLibDirs = []string{"/usr/lib/", "/usr/lib32/", "/lib/"}

// libExports resolves a needed soname to its exported dynamic symbols: first
// inside the package itself, then on the host filesystem.
func (ctx *Context) libExports(soname string) map[string]bool {
	f := ctx.facts()
	if syms, ok := f.hostExports[soname]; ok {
		return syms
	}
	var syms map[string]bool
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if isPackageELF(e) && (e.ELF.Soname == soname || basename(e.Name) == soname) {
			syms = e.ELF.Exported
			break
		}
	}
	if syms == nil {
		for _, dir := range hostLibDirs {
			ef, err := elf.Open(dir + soname)
			if err != nil {
				continue
			}
			dynsyms, err := ef.DynamicSymbols()
			ef.Close()
			if err != nil {
				break
			}
			syms = map[string]bool{}
			for _, s := range dynsyms {
				if s.Section != elf.SHN_UNDEF && s.Name != "" {
					syms[s.Name] = true
				}
			}
			break
		}
	}
	f.hostExports[soname] = syms // nil is cached too: "could not resolve"
	return syms
}

func checkUnusedLibs(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !isPackageELF(e) || len(e.ELF.Needed) == 0 || len(e.ELF.UndefinedSyms) == 0 {
			continue
		}
		for _, soname := range e.ELF.Needed {
			exports := ctx.libExports(soname)
			if len(exports) == 0 {
				continue // can't resolve the library here; no verdict
			}
			used := false
			for _, sym := range e.ELF.UndefinedSyms {
				if exports[sym] {
					used = true
					break
				}
			}
			if !used {
				out = append(out, pkgFinding("PB810", Warn, e.Name,
					"links %s but imports none of its symbols; -Wl,--as-needed would drop it", soname))
			}
		}
	}
	return out
}

// --- PB811: soname-style depends/provides vs reality ---------------------------

func checkSonameDeclarations(ctx *Context) []Finding {
	f := ctx.facts()
	var out []Finding
	neededBases := map[string]bool{}
	for n := range f.neededSet {
		neededBases[sonameBase(n)] = true
	}
	for _, d := range ctx.File.Info.Depends {
		name := pkgDepName(d)
		if !strings.Contains(name, ".so") {
			continue
		}
		if strings.HasSuffix(d, ".so") {
			out = append(out, pkgFinding("PB811", Error, ".PKGINFO",
				"depends entry %q is an unversioned soname; pin it (e.g. %s=1-64) or the entry matches every ABI", d, d))
		}
		if !neededBases[sonameBase(name)] {
			out = append(out, pkgFinding("PB811", Warn, ".PKGINFO",
				"depends entry %q names a library no binary in this package links", d))
		}
	}
	providedBases := map[string]bool{}
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if isPackageELF(e) && e.ELF.Soname != "" && (strings.HasPrefix(e.Name, "usr/lib/") || strings.HasPrefix(e.Name, "usr/lib32/")) {
			providedBases[sonameBase(e.ELF.Soname)] = true
		}
	}
	for _, p := range ctx.File.Info.Provides {
		name := alpmdb.DepName(p)
		if !strings.Contains(name, ".so") {
			continue
		}
		if strings.HasSuffix(p, ".so") {
			out = append(out, pkgFinding("PB811", Error, ".PKGINFO",
				"provides entry %q is an unversioned soname; declare the version (e.g. %s=1-64)", p, p))
		}
		if !providedBases[sonameBase(name)] {
			out = append(out, pkgFinding("PB811", Warn, ".PKGINFO",
				"provides entry %q names a library this package does not actually ship", p))
		}
	}
	return out
}

// --- PB812: shebang interpreters vs depends ------------------------------------

func checkShebangDeps(ctx *Context) []Finding {
	if ctx.DB == nil {
		return nil
	}
	type need struct {
		interps map[string]bool
		files   map[string]bool
		state   depSatisfaction
	}
	byOwner := map[string]*need{}
	unresolved := map[string]string{} // interpreter -> example file
	shipped := map[string]bool{}
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsFile() || e.IsSymlink() {
			shipped[basename(e.Name)] = true
		}
	}
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !e.IsFile() || !e.IsScript {
			continue
		}
		// Example scripts under the documentation tree are not run in place.
		if strings.HasPrefix(e.Name, "usr/share/doc/") {
			continue
		}
		interp := e.Interpreter()
		if interp == "" || interp == "sh" || interp == "bash" {
			continue // guaranteed by the base system, and bash noise dominates
		}
		if shipped[interp] {
			continue // the package ships its own interpreter
		}
		owner := ctx.DB.CommandOwner(interp)
		if owner == "" {
			if _, ok := unresolved[interp]; !ok {
				unresolved[interp] = e.Name
			}
			continue
		}
		sat := ctx.satisfaction(owner)
		if sat == depSelf || sat == depDirect {
			continue
		}
		n := byOwner[owner]
		if n == nil {
			n = &need{interps: map[string]bool{}, files: map[string]bool{}, state: sat}
			byOwner[owner] = n
		}
		n.interps[interp] = true
		if len(n.files) < 3 {
			n.files[e.Name] = true
		}
		if sat == depMissing {
			n.state = depMissing
		}
	}
	var out []Finding
	for owner, n := range byOwner {
		files := sortedKeys(n.files)
		interps := sortedKeys(n.interps)
		switch n.state {
		case depMissing:
			if !ctx.facts().closureComplete {
				out = append(out, pkgFinding("PB812", Info, files[0],
					"scripts run %s from package %q, not reachable from depends as installed here — but some declared dependencies are not installed, so this may be incomplete",
					strings.Join(interps, ", "), owner))
				continue
			}
			out = append(out, pkgFinding("PB812", Error, files[0],
				"scripts run %s from package %q, which is not reachable from depends; add it",
				strings.Join(interps, ", "), owner))
		case depTransitive:
			out = append(out, pkgFinding("PB812", Warn, files[0],
				"scripts run %s from package %q, reached only transitively; declare it directly",
				strings.Join(interps, ", "), owner))
		case depOptional:
			out = append(out, pkgFinding("PB812", Warn, files[0],
				"scripts run %s from package %q, which is only an optdepends", strings.Join(interps, ", "), owner))
		}
	}
	for interp, file := range unresolved {
		out = append(out, pkgFinding("PB812", Info, file,
			"script interpreter %q is not provided by any installed package; the dependency cannot be verified here", interp))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

// --- PB813: depends nothing appears to need ------------------------------------

func checkUnneededDeps(ctx *Context) []Finding {
	if ctx.DB == nil {
		return nil
	}
	info := ctx.File.Info

	// The detectors below see linked libraries and interpreters; without at
	// least one of either, every dependency would be "unneeded" and the check
	// is meaningless (fonts, icon themes, pure data).
	detected := map[string]bool{}
	sawEvidence := false
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if isPackageELF(e) {
			sawEvidence = true
			for _, soname := range e.ELF.Needed {
				if owner := ctx.DB.LibraryOwner(soname, e.ELF.Class == elf.ELFCLASS32); owner != "" {
					detected[owner] = true
				}
			}
		}
		if e.IsFile() && e.IsScript {
			sawEvidence = true
			if owner := ctx.DB.CommandOwner(e.Interpreter()); owner != "" {
				detected[owner] = true
			}
		}
	}
	if !sawEvidence {
		return nil
	}
	for owner := range pathImpliedDeps(ctx) {
		detected[owner] = true
	}

	// Anything a detected package provides also counts as detected: depending
	// on the capability instead of the concrete package is fine.
	detectedProvides := map[string]bool{}
	for d := range detected {
		if p := ctx.DB.Get(d); p != nil {
			for _, prov := range p.Provides {
				detectedProvides[alpmdb.DepName(prov)] = true
			}
		}
	}

	var out []Finding
	for _, d := range info.Depends {
		name := pkgDepName(d)
		if strings.Contains(name, ".so") {
			continue // PB811's concern
		}
		if name == info.Name || name == info.Base {
			continue
		}
		// lib32-foo conventionally depends on foo without a code-level link.
		if strings.HasPrefix(info.Name, "lib32-") && name == strings.TrimPrefix(info.Name, "lib32-") {
			continue
		}
		if detected[name] || detectedProvides[name] {
			continue
		}
		// A dependency whose own providers include a detected package
		// (dependency on a virtual name) is in use too.
		satisfied := false
		for _, p := range ctx.DB.ProvidersOf(name) {
			if detected[p] {
				satisfied = true
				break
			}
		}
		if satisfied {
			continue
		}
		out = append(out, pkgFinding("PB813", Info, ".PKGINFO",
			"no binary links and no script runs anything from %q; if it is a data, dlopen or IPC dependency this is expected — otherwise it can be dropped", name))
	}
	return out
}

// --- PB814: paths that imply a dependency ---------------------------------------

// pathDepRules maps a path pattern to the package (or virtual capability)
// whose absence breaks the shipped file. Mirrors namcap's pathdepends plus its
// javafiles rule.
var pathDepRules = []struct {
	pattern *regexp.Regexp
	dep     string
	reason  string
}{
	{regexp.MustCompile(`^usr/share/glib-2\.0/schemas$`), "dconf", "GSettings schemas are read through dconf"},
	{regexp.MustCompile(`^usr/lib/gio/modules/.*\.so$`), "glib2", "GIO modules are loaded by glib2"},
	{regexp.MustCompile(`^usr/share/icons/hicolor$`), "hicolor-icon-theme", "icons under hicolor need the base theme's directory index"},
}

// pathImpliedDeps returns dep -> reason for every path rule the package trips,
// plus java-runtime when it ships Java bytecode.
func pathImpliedDeps(ctx *Context) map[string]string {
	out := map[string]string{}
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		for _, r := range pathDepRules {
			if r.pattern.MatchString(e.Name) {
				out[r.dep] = r.reason
			}
		}
		if e.IsFile() && (strings.HasSuffix(e.Name, ".jar") || isJavaClass(e)) {
			out["java-runtime"] = "Java bytecode needs a runtime"
		}
	}
	return out
}

// isJavaClass matches compiled Java classes. File contents are not retained
// for most members, so the extension is the signal.
func isJavaClass(e *pkgfile.Entry) bool {
	return strings.HasSuffix(e.Name, ".class")
}

func checkPathDeps(ctx *Context) []Finding {
	if ctx.DB == nil {
		return nil
	}
	var out []Finding
	deps := pathImpliedDeps(ctx)
	keys := make([]string, 0, len(deps))
	for d := range deps {
		keys = append(keys, d)
	}
	sort.Strings(keys)
	for _, dep := range keys {
		satisfied := false
		for _, p := range ctx.DB.ProvidersOf(dep) {
			if s := ctx.satisfaction(p); s == depSelf || s == depDirect || s == depTransitive {
				satisfied = true
				break
			}
		}
		if satisfied {
			continue
		}
		out = append(out, pkgFinding("PB814", Warn, ".PKGINFO",
			"package ships files that need %q (%s) but depends does not reach it", dep, deps[dep]))
	}
	return out
}

// --- PB815: dependencies made redundant by pacman hooks -------------------------

var hookCoveredDepRules = []struct {
	dep     string
	pattern *regexp.Regexp
}{
	{"desktop-file-utils", regexp.MustCompile(`^usr/share/applications/.*\.desktop$`)},
	{"shared-mime-info", regexp.MustCompile(`^usr/share/mime$`)},
}

func checkHookCoveredDeps(ctx *Context) []Finding {
	f := ctx.facts()
	var out []Finding
	for _, r := range hookCoveredDepRules {
		if !f.depNames[r.dep] {
			continue
		}
		for i := range ctx.File.Entries {
			if r.pattern.MatchString(ctx.File.Entries[i].Name) {
				out = append(out, pkgFinding("PB815", Info, ".PKGINFO",
					"%q is only needed for its update hook, which pacman runs regardless; the dependency can be dropped", r.dep))
				break
			}
		}
	}
	return out
}

// --- PB816: pkg-config Requires ---------------------------------------------------

var pcRequiresRe = regexp.MustCompile(`(?m)^Requires(?:\.private)?:\s*(.+)$`)

// pcModules extracts module names from a Requires value: names separated by
// commas/whitespace, each optionally followed by a version constraint.
func pcModules(val string) []string {
	var out []string
	expectVersion := false
	for _, tok := range strings.FieldsFunc(val, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if tok == "" {
			continue
		}
		if strings.ContainsAny(tok[:1], "<>=!") {
			expectVersion = true
			if len(tok) > 2 { // operator glued to version: ">=1.2"
				expectVersion = false
			}
			continue
		}
		if expectVersion {
			expectVersion = false
			continue // the version operand
		}
		out = append(out, tok)
	}
	return out
}

func checkPkgconfigDeps(ctx *Context) []Finding {
	if ctx.DB == nil {
		return nil
	}
	shippedPC := map[string]bool{}
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if strings.HasSuffix(e.Name, ".pc") {
			shippedPC[strings.TrimSuffix(basename(e.Name), ".pc")] = true
		}
	}
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if len(e.Data) == 0 || !strings.HasSuffix(e.Name, ".pc") {
			continue
		}
		for _, m := range pcRequiresRe.FindAllStringSubmatch(string(e.Data), -1) {
			for _, mod := range pcModules(m[1]) {
				if shippedPC[mod] {
					continue
				}
				owner := ""
				for _, dir := range []string{"usr/lib/pkgconfig/", "usr/share/pkgconfig/", "usr/lib32/pkgconfig/"} {
					if owner = ctx.DB.PathOwner(dir + mod + ".pc"); owner != "" {
						break
					}
				}
				if owner == "" || !ctx.facts().closureComplete {
					continue // unknown module or unverifiable closure; no verdict
				}
				if s := ctx.satisfaction(owner); s == depSelf || s == depDirect || s == depTransitive {
					continue
				}
				out = append(out, pkgFinding("PB816", Warn, e.Name,
					"pkg-config file requires module %q from package %q, which depends does not reach", mod, owner))
			}
		}
	}
	return out
}

// --- PB817: .PKGINFO metadata -----------------------------------------------------

func checkPackageMetadata(ctx *Context) []Finding {
	info := ctx.File.Info
	var out []Finding
	if info.URL == "" {
		out = append(out, pkgFinding("PB817", Warn, ".PKGINFO", "package has no url"))
	}
	if info.Desc == "" {
		out = append(out, pkgFinding("PB817", Warn, ".PKGINFO", "package has no description"))
	}
	if info.Name != strings.ToLower(info.Name) {
		out = append(out, pkgFinding("PB817", Warn, ".PKGINFO",
			"package name %q contains uppercase letters; Arch package names are lowercase by convention", info.Name))
	}
	return out
}
