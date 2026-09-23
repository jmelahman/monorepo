package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// Fixes for the rules whose remedy is one entry in a dependency array: a build
// tool the build already runs but nothing declares, or metadata the rule's own
// message calls dead. The array primitives they build on live in fix.go; every
// fixer here walks the same selector its check does, so a rule that stands
// down takes its fix with it.
//
// The adds are FixSafe and the removals are mostly FixUnsafe, which looks
// backwards until you ask what each one changes. Declaring a tool the build
// already invokes changes nothing about what gets built — it restores a
// requirement that a clean chroot was going to enforce anyway, and a package
// that could not build before can build after. Deleting a dependency is the
// other direction: something that was installed no longer is, and only the
// maintainer knows whether anything was leaning on it. The removals that stay
// safe are the ones where the entry provably did nothing — a name already in
// depends, a package naming itself.

// suppressedAt reports whether a directive waives the finding a fixer is
// answering for at pos. CollectEdits checks suppression at each edit's own
// Line, so most fixers never ask; the ones here must, because their edit
// lands somewhere other than where the finding points — the fix for a cargo
// invocation writes a makedepends line — or because one edit speaks for
// several findings that can be waived one at a time.
func suppressedAt(ctx *Context, id string, pos syntax.Pos) bool {
	return ctx.Pkg.Suppressed(id, ctx.Pkg.PKGBUILD.Path, int(pos.Line()))
}

// addArrayEntries is the one-array add every rule here except PB961 makes.
// line is where the rule reported its finding: the edit answers to a
// directive there, not at the array it happens to rewrite.
func addArrayEntries(ctx *Context, field string, entries []string, line int, why string) []Edit {
	edit, ok := addArrayElemsEdit(ctx, field, entries, addDesc(field, entries, why))
	if !ok {
		return nil
	}
	edit.Line = line
	return []Edit{edit}
}

func addDesc(field string, entries []string, why string) string {
	quoted := make([]string, len(entries))
	for i, e := range entries {
		quoted[i] = strconv.Quote(e)
	}
	return fmt.Sprintf("add %s to %s (%s)", strings.Join(quoted, ", "), field, why)
}

// removeDepEntries deletes the array elements entries were written as.
//
// The entries are grouped by the array each came out of, because whether the
// whole assignment goes — every element deleted, leaving nothing worth an
// empty array — can only be decided once all of one array's doomed entries are
// in hand. A rule reading `depends` also reads its declared `depends_x86_64`,
// so more than one array in a pass is ordinary.
func removeDepEntries(ctx *Context, id string, entries []depEntry, why string) []Edit {
	u := &ctx.Pkg.PKGBUILD
	var order []*pkgbuild.Var
	byVar := map[*pkgbuild.Var][]depEntry{}
	for _, d := range entries {
		// A value with no source text of its own — merged in by a later `+=`,
		// padded in by an indexed write — has no bytes to delete. So does one
		// name of a brace group: `depends=(python-{foo,bar})` is one word for
		// two entries, and deleting the word would drop both.
		if d.Var == nil || d.Word == nil || wordValueCount(d.Var, d.Word) != 1 {
			continue
		}
		// A waived entry stays, checked per entry rather than left to the
		// central per-edit check: deleting an emptied array is one edit whose
		// line cannot speak for every entry it swallows.
		if suppressedAt(ctx, id, d.Pos) {
			continue
		}
		if _, seen := byVar[d.Var]; !seen {
			order = append(order, d.Var)
		}
		byVar[d.Var] = append(byVar[d.Var], d)
	}
	var edits []Edit
	for _, v := range order {
		group := byVar[v]
		words := make([]*syntax.Word, 0, len(group))
		names := make([]string, 0, len(group))
		for _, d := range group {
			words = append(words, d.Word)
			names = append(names, strconv.Quote(d.Full))
		}
		desc := fmt.Sprintf("drop %s from %s (%s)", strings.Join(names, ", "), v.Name, why)
		edits = append(edits, removeArrayElemEdits(u, v, words, desc)...)
	}
	return edits
}

// --- PB711: VCS sources need their client in makedepends ---------------------

// fixVCSMakedepends declares the clients the sources need. One edit covers
// every missing client: a PKGBUILD fetching from git and from hg needs both,
// and they go into the same array. A client whose finding is waived by a
// directive at its source entry is left out.
func fixVCSMakedepends(ctx *Context, _ *FixEnv) []Edit {
	var gaps []vcsClientGap
	for _, g := range vcsClientGaps(ctx) {
		if suppressedAt(ctx, "PB711", g.Pos) {
			continue
		}
		gaps = append(gaps, g)
	}
	if len(gaps) == 0 {
		return nil
	}
	tools := make([]string, 0, len(gaps))
	for _, g := range gaps {
		tools = append(tools, g.Tool)
	}
	return addArrayEntries(ctx, "makedepends", tools, int(gaps[0].Pos.Line()),
		"a clean build environment has no VCS client")
}

// --- PB944/PB952/PB954/PB979: a build tool missing from makedepends ----------

// fixToolMakedepends is the fixer the four tool-makedepends rules share: when
// the gap its rule reports is there, declare the package that closes it. The
// gap function is the rule's own, so an exemption the check makes — cmake
// reached through a dependency whose name contains "cmake", say — exempts the
// fix too.
func fixToolMakedepends(gap func(*Context) (Command, bool), tool, pkg string) Fixer {
	return func(ctx *Context, _ *FixEnv) []Edit {
		c, ok := gap(ctx)
		if !ok {
			return nil
		}
		return addArrayEntries(ctx, "makedepends", []string{pkg}, int(c.Stmt.Pos().Line()),
			"the build runs "+tool)
	}
}

// --- PB933: python build backend missing from makedepends --------------------

func fixPythonBuildBackend(ctx *Context, _ *FixEnv) []Edit {
	var gaps []pythonBackendGap
	for _, g := range pythonBackendGaps(ctx) {
		if suppressedAt(ctx, "PB933", g.Cmd.Stmt.Pos()) {
			continue
		}
		gaps = append(gaps, g)
	}
	if len(gaps) == 0 {
		return nil
	}
	pkgs := make([]string, 0, len(gaps))
	modules := make([]string, 0, len(gaps))
	for _, g := range gaps {
		pkgs = append(pkgs, g.Pkg)
		modules = append(modules, "python -m "+g.Module)
	}
	return addArrayEntries(ctx, "makedepends", pkgs, int(gaps[0].Cmd.Stmt.Pos().Line()),
		"the build runs "+strings.Join(modules, " and "))
}

// --- PB973: a -dkms package must depend on dkms ------------------------------

func fixDkmsDepends(ctx *Context, _ *FixEnv) []Edit {
	if !dkmsDependsMissing(ctx) {
		return nil
	}
	return addArrayEntries(ctx, "depends", []string{"dkms"}, varFindingLine(ctx, "depends", "pkgname"),
		"only dkms can build and install the module sources this package ships")
}

// --- PB981: Java artifacts without a runtime dependency ----------------------

// fixJavaRuntimeDependency declares java-runtime, the virtual the guidelines
// name for a package that only needs a JVM to run. A package that needs a
// compiler at runtime wants java-environment instead, but nothing in a
// PKGBUILD says which, and java-runtime is the one every JDK also provides.
func fixJavaRuntimeDependency(ctx *Context, _ *FixEnv) []Edit {
	if !javaRuntimeDepMissing(ctx) {
		return nil
	}
	return addArrayEntries(ctx, "depends", []string{"java-runtime"}, varFindingLine(ctx, "depends", "pkgname"),
		"the package ships Java artifacts and nothing else puts a JVM on the system")
}

// --- PB961: a -git package shadowing its release counterpart -----------------

// fixVCSProvidesConflicts declares both halves of the pair the guidelines ask
// for, tied by one group so a package never gets half of it. provides without
// conflicts lets both packages install at once; conflicts without provides
// breaks everything that depended on the counterpart. Either alone is worse
// than the finding.
func fixVCSProvidesConflicts(ctx *Context, _ *FixEnv) []Edit {
	var gaps []vcsCounterpartGap
	for _, g := range vcsCounterpartGaps(ctx) {
		if suppressedAt(ctx, "PB961", g.Pos) {
			continue
		}
		gaps = append(gaps, g)
	}
	if len(gaps) == 0 {
		return nil
	}
	line := int(gaps[0].Pos.Line())
	var bases []string
	seen := map[string]bool{}
	for _, g := range gaps {
		if !seen[g.Base] {
			seen[g.Base] = true
			bases = append(bases, g.Base)
		}
	}
	fields := []string{"provides", "conflicts"}
	var missing []string
	var edits []Edit
	for _, field := range fields {
		if ctx.Pkg.Vars[field] == nil {
			missing = append(missing, field)
			continue
		}
		edit, ok := addArrayElemsEdit(ctx, field, bases, addDesc(field, bases, whyCounterpart))
		if !ok {
			return nil
		}
		edit.Line = line
		edits = append(edits, edit)
	}
	// Both arrays absent is the ordinary case, and they are created by a
	// single edit: two edits creating arrays would claim the same anchor line
	// and overlap resolution would drop one, which for a grouped pair means
	// dropping both.
	if len(missing) > 0 {
		edit, ok := newArraysEdit(ctx, missing, bases, addDesc(strings.Join(missing, " and "), bases, whyCounterpart))
		if !ok {
			return nil
		}
		edit.Line = line
		edits = append(edits, edit)
	}
	if len(edits) > 1 {
		for i := range edits {
			edits[i].Group = "PB961"
		}
	}
	return edits
}

const whyCounterpart = "the package builds the same software as its release counterpart"

// --- PB904: makedepends already in depends -----------------------------------

func fixRedundantMakedepends(ctx *Context, _ *FixEnv) []Edit {
	return removeDepEntries(ctx, "PB904", redundantMakedepends(ctx),
		"already in depends, which is installed at build time too")
}

// --- PB912: depends repeated in optdepends -----------------------------------

// fixDuplicatedOptdepends drops the optdepends half of the pair. That is the
// half the rule can settle: the entry in depends is what the package actually
// installs, and calling the same package optional beside it is the statement
// with nothing behind it. Which of the two a maintainer meant to keep is a
// question about the package, so the other direction is not a fix.
func fixDuplicatedOptdepends(ctx *Context, _ *FixEnv) []Edit {
	return removeDepEntries(ctx, "PB912", duplicatedOptdepends(ctx),
		"already a hard dependency in depends")
}

// --- PB918/PB919: a package providing or conflicting with itself -------------

func fixSelfProvides(ctx *Context, _ *FixEnv) []Edit {
	return removeDepEntries(ctx, "PB918", selfReferences(ctx, "provides"),
		"a package always provides itself")
}

func fixSelfConflicts(ctx *Context, _ *FixEnv) []Edit {
	return removeDepEntries(ctx, "PB919", selfReferences(ctx, "conflicts"),
		"a package can never conflict with itself")
}

// --- PB971: font packages need no dependencies -------------------------------

func fixFontDepends(ctx *Context, _ *FixEnv) []Edit {
	return removeDepEntries(ctx, "PB971", fontDepends(ctx),
		"fontconfig discovers installed fonts by itself")
}

// --- PB974: a -dkms package pinning one kernel's headers ---------------------

func fixDkmsKernelHeaders(ctx *Context, _ *FixEnv) []Edit {
	return removeDepEntries(ctx, "PB974", dkmsPinnedHeaders(ctx),
		"dkms already pulls the right headers for every installed kernel")
}

// --- PB931: lint plugins in checkdepends -------------------------------------

// pytestPluginFlags maps a lint plugin to the pytest flag that switches it on.
// python-pytest-runner is deliberately absent: it is a setup.py plugin with no
// pytest flag, so nothing on a command line can be depending on it.
var pytestPluginFlags = map[string]string{
	"python-pytest-cov":    "--cov",
	"python-pytest-black":  "--black",
	"python-pytest-flake8": "--flake8",
	"python-pytest-isort":  "--isort",
	"python-pytest-mypy":   "--mypy",
	"python-pytest-pylint": "--pylint",
	"python-pytest-ruff":   "--ruff",
}

// fixPythonLintCheckdepends drops the plugin from checkdepends, but only when
// nothing in the file is asking pytest for it.
//
// This is PB930's trap one rule over: deleting the dependency while a command
// still passes its flag leaves a check phase that cannot run, and the flag —
// which may equally live in a pyproject.toml addopts pkglint never sees — is
// not something this fix can rewrite. When a command does pass it, the finding
// stands and the maintainer removes both.
func fixPythonLintCheckdepends(ctx *Context, _ *FixEnv) []Edit {
	var drop []depEntry
	for _, d := range lintCheckdepends(ctx) {
		if flag, ok := pytestPluginFlags[d.Name]; ok && flagPassedInFunctions(ctx, flag) {
			continue
		}
		drop = append(drop, d)
	}
	return removeDepEntries(ctx, "PB931", drop,
		"check() verifies the package works, not that upstream's style rules pass")
}

// flagPassedInFunctions reports whether any command in a build function passes
// flag, in either the `--x value` or `--x=value` spelling.
func flagPassedInFunctions(ctx *Context, flag string) bool {
	for _, c := range ctx.Commands() {
		if c.Unit.Scriptlet || c.Fn == "" {
			continue
		}
		for _, a := range c.Args {
			if a == flag || strings.HasPrefix(a, flag+"=") {
				return true
			}
		}
	}
	return false
}
