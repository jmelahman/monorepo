package rules

import (
	"sort"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB914–PB917 lint the build-flag conventions of the Arch Linux Go package
// guidelines (https://wiki.archlinux.org/title/Go_package_guidelines): PIE
// hardening, reproducible paths, a writable module cache, and toolchain flags
// reaching cgo. The guidelines' module-download and verification advice is
// hermeticity and already covered by PB204/PB205; `-mod=readonly` is not
// linted because it has been Go's default since 1.16.

// assignmentsTo returns the rendered value of every assignment to name that
// has taken effect by the time c runs. GOFLAGS and the CGO_* variables reach
// the toolchain through the environment, so their scope is makepkg's rather
// than the file's: a top-level assignment is sourced before every phase, an
// assignment inside a phase function reaches that function's later commands
// and the phases makepkg runs after it — never one it has already finished —
// and a command's own environment prefix reaches nothing but that command.
// An `export GOFLAGS=-modcacherw` in build() therefore says nothing about the
// `go mod download` prepare() already ran.
//
// Dynamic parts render as their literal fragments, so `GOFLAGS="$GOFLAGS
// -trimpath"` still reveals the appended flag; values that are entirely
// dynamic render empty but still count as an assignment.
func assignmentsTo(ctx *Context, name string, c Command) []string {
	at := -1
	if c.Stmt != nil {
		at = off(c.Stmt.Pos())
	}
	return assignmentsInScope(c.Unit, name, c.Fn, at, c.Call)
}

// assignmentsInScope is assignmentsTo addressed by position instead of by
// command, so a fix can ask what a line it is about to write would inherit.
// Assignments in fn count only when they start before at (negative: all of
// them); own names the CallExpr whose environment prefix belongs to the
// caller, if any.
func assignmentsInScope(u *pkgbuild.Unit, name, fn string, at int, own *syntax.CallExpr) []string {
	var out []string
	scanAssignments(u, name, fn, at, own, exportedInScope(u, name, fn, at, own), func(as *syntax.Assign) {
		if as.Value == nil {
			return
		}
		s, _ := renderPlain(as.Value)
		out = append(out, s)
	})
	return out
}

// wordsInScope is assignmentsInScope for a name that holds words rather than
// one environment value: `_cargo_flags="--locked --release"` and the
// `_cargo_flags=(--locked --release)` array spelling both hand a command two
// words, and an array assignment is an assignment like any other here.
//
// found separates a name the file never assigns — where the words really are
// unknown — from one it assigns nothing, and the words keep their expansions
// ("$CARGO_ARGS", "\x00") so a caller can see which of them it still cannot
// read. A value is split the way bash splits an unquoted expansion; an array
// element is one word, quoted or not, which is the point of the spelling.
//
// A plain `_cargo_flags=…` statement counts, unlike in assignmentsInScope: it
// sets a shell variable the next line expands, which is the whole point here,
// even though it exports nothing to a child process's environment.
func wordsInScope(u *pkgbuild.Unit, name, fn string, at int, own *syntax.CallExpr) (words []string, found bool) {
	scanAssignments(u, name, fn, at, own, true, func(as *syntax.Assign) {
		found = true
		if as.Value != nil {
			s, _ := pkgbuild.RenderWord(as.Value, nil)
			words = append(words, strings.Fields(s)...)
		}
		if as.Array == nil {
			return
		}
		for _, el := range as.Array.Elems {
			s, _ := pkgbuild.RenderWord(el.Value, nil)
			words = append(words, s)
		}
	})
	return words, found
}

// scanAssignments calls visit for every assignment to name in scope, in the
// order described on assignmentsInScope. standalone additionally counts a
// `name=value` statement of its own, which the parser reports as a command
// with no arguments and which sets a shell variable rather than a child
// process's environment.
func scanAssignments(u *pkgbuild.Unit, name, fn string, at int, own *syntax.CallExpr, standalone bool, visit func(*syntax.Assign)) {
	if u == nil || u.Scriptlet {
		return
	}
	visible := map[string]bool{}
	for _, p := range precedingPhases(fn) {
		visible[p] = true
	}
	// The caller's own environment prefix sits at the caller's position, so
	// it is exempt from the "must start before at" cutoff below.
	ownAssigns := map[*syntax.Assign]bool{}
	if own != nil {
		for _, as := range own.Assigns {
			ownAssigns[as] = true
		}
	}
	// A subtree contributes every assignment to name except another command's
	// environment prefix. syntax.Walk is pre-order, so a CallExpr is seen
	// before its own assignments and can disown them first.
	scan := func(n syntax.Node, before int) {
		if n == nil {
			return
		}
		foreign := map[*syntax.Assign]bool{}
		syntax.Walk(n, func(node syntax.Node) bool {
			if ce, ok := node.(*syntax.CallExpr); ok && ce != own && !(standalone && len(ce.Args) == 0) {
				for _, as := range ce.Assigns {
					foreign[as] = true
				}
				return true
			}
			as, ok := node.(*syntax.Assign)
			if !ok || as.Name == nil || as.Name.Value != name || foreign[as] {
				return true
			}
			if before >= 0 && off(as.Pos()) >= before && !ownAssigns[as] {
				return true
			}
			visit(as)
			return true
		})
	}
	// Top-level code runs in full before any function does — unless the
	// caller is itself top-level, where only the lines above it have run.
	topLimit := -1
	if fn == "" {
		topLimit = at
	}
	for _, stmt := range u.TopLevel {
		scan(stmt, topLimit)
	}
	names := make([]string, 0, len(u.Functions))
	for n := range u.Functions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		switch {
		case n == fn:
			scan(u.Functions[n].Body, at)
		case visible[n], !makepkgPhase(n):
			// A helper the PKGBUILD calls itself has no fixed place in the
			// order, so assume it can run before c and count its assignments.
			scan(u.Functions[n].Body, -1)
		}
	}
}

// exportedInScope reports whether an `export name` (or `declare -x`) in scope
// has put name in the environment. Once it has, a plain `name=…` statement
// reaches child processes too, before the export or after it — seafile-server
// builds GOFLAGS up over five `GOFLAGS+=` lines and exports it once — so
// scanAssignments has to count those statements.
func exportedInScope(u *pkgbuild.Unit, name, fn string, at int, own *syntax.CallExpr) bool {
	if u == nil || u.File == nil {
		return false
	}
	exports := map[*syntax.Assign]bool{}
	syntax.Walk(u.File, func(n syntax.Node) bool {
		if dc, ok := n.(*syntax.DeclClause); ok && declExports(dc) {
			for _, as := range dc.Args {
				exports[as] = true
			}
		}
		return true
	})
	found := false
	scanAssignments(u, name, fn, at, own, false, func(as *syntax.Assign) {
		found = found || exports[as]
	})
	return found
}

// makepkgPhase reports whether fn is a function makepkg calls itself, and so
// has a known position in the run order.
func makepkgPhase(fn string) bool {
	if fn == "package" || strings.HasPrefix(fn, "package_") {
		return true
	}
	for _, p := range buildPhases {
		if fn == p {
			return true
		}
	}
	return false
}

// goFlags returns the GOFLAGS values in effect for c, word by word — the
// rendered assignments assignmentsTo sees, plus what an assignment that is
// nothing but a variable reference holds: `GOFLAGS="${goflags[*]}"` over an
// array of flags declared a few lines up is how a PKGBUILD keeps a long flag
// list readable, and the flags are in the array, not in the empty string the
// reference renders to.
func goFlags(ctx *Context, c Command) []string {
	at := -1
	if c.Stmt != nil {
		at = off(c.Stmt.Pos())
	}
	var out []string
	exported := exportedInScope(c.Unit, "GOFLAGS", c.Fn, at, c.Call)
	scanAssignments(c.Unit, "GOFLAGS", c.Fn, at, c.Call, exported, func(as *syntax.Assign) {
		if as.Value == nil {
			return
		}
		if name := varRefName(as.Value); name != "" && name != "GOFLAGS" {
			if words, ok := wordsInScope(c.Unit, name, c.Fn, at, c.Call); ok {
				out = append(out, words...)
				return
			}
		}
		s, _ := renderPlain(as.Value)
		out = append(out, strings.Fields(s)...)
	})
	return out
}

// goWords returns c's arguments with flag variables read through: `go build
// "${flags[@]}"` passes whatever the array holds.
func goWords(c Command) []string {
	var out []string
	for i := range c.Args {
		out = append(out, argWords(c, i)...)
	}
	return out
}

// goFlagAddressed reports whether the PKGBUILD says anything about flag for
// this command: as an argument (any "-flag..." spelling, so an explicit
// opt-out counts as a decision) or inside a GOFLAGS assignment.
func goFlagAddressed(goflags []string, c Command, flag string) bool {
	for _, a := range goWords(c) {
		if strings.HasPrefix(a, flag) {
			return true
		}
	}
	for _, v := range goflags {
		if strings.Contains(v, flag) {
			return true
		}
	}
	return false
}

// goBuildCommands returns the `go build` / `go install` invocations that
// produce the artifacts the package ships (build/check/package functions).
func goBuildCommands(ctx *Context) []Command {
	var out []Command
	for _, c := range ctx.CommandsNamed("go") {
		if !c.InBuildPhase() {
			continue
		}
		switch c.Subcommand() {
		case "build", "install":
			out = append(out, c)
		}
	}
	return out
}

// secondSubcommand returns the verb after a two-word go subcommand ("mod
// download" → "download"), or "".
func secondSubcommand(c Command) string {
	seen := false
	for _, a := range c.Args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if !seen {
			seen = true
			continue
		}
		return a
	}
	return ""
}

// goModuleCommands returns every go invocation that writes the module cache,
// in any function of the PKGBUILD — prepare() is exactly where the guidelines
// put `go mod download`.
func goModuleCommands(ctx *Context) []Command {
	var out []Command
	for _, c := range ctx.CommandsNamed("go") {
		if c.Unit.Scriptlet || c.Fn == "" {
			continue
		}
		switch c.Subcommand() {
		case "build", "install", "test", "run", "get":
			out = append(out, c)
		case "mod":
			switch secondSubcommand(c) {
			case "download", "tidy", "vendor":
				out = append(out, c)
			}
		}
	}
	return out
}

// --- PB914: go build without -buildmode=pie ----------------------------------

func checkGoPIE(ctx *Context) []Finding {
	var out []Finding
	for _, c := range goBuildCommands(ctx) {
		if goFlagAddressed(goFlags(ctx, c), c, "-buildmode") {
			continue
		}
		out = append(out, c.finding("PB914", Warn,
			"go %s without -buildmode=pie produces a non-PIE executable, so ASLR cannot relocate it; set it in GOFLAGS or on the command", c.Subcommand()))
	}
	return out
}

// --- PB915: go build without -trimpath ----------------------------------------

func checkGoTrimpath(ctx *Context) []Finding {
	var out []Finding
	for _, c := range goBuildCommands(ctx) {
		if goFlagAddressed(goFlags(ctx, c), c, "-trimpath") {
			continue
		}
		out = append(out, c.finding("PB915", Warn,
			"go %s without -trimpath embeds $srcdir paths in the binary, so builds are not reproducible and leak the build layout", c.Subcommand()))
	}
	return out
}

// --- PB916: module cache written read-only ------------------------------------

func checkGoModcacheRW(ctx *Context) []Finding {
	var out []Finding
	for _, c := range goModuleCommands(ctx) {
		if goFlagAddressed(goFlags(ctx, c), c, "-modcacherw") {
			continue
		}
		out = append(out, c.finding("PB916", Info,
			"go %s writes a read-only module cache; without -modcacherw, cleaning the build directory needs a chmod first", c.Subcommand()))
	}
	return out
}

// --- PB917: hardening flags never reach cgo ------------------------------------

// cgoFlagVars are the variables that forward the exported toolchain flags to
// cgo-compiled code; any one of them being set counts as the PKGBUILD having
// made the call.
var cgoFlagVars = []string{"CGO_CPPFLAGS", "CGO_CFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS"}

func checkGoCgoFlags(ctx *Context) []Finding {
	for _, c := range goBuildCommands(ctx) {
		if cgoFlagsForwarded(ctx, c) {
			continue
		}
		// One finding per PKGBUILD: the remedy is a block of exports, not a
		// per-command flag. The first uncovered command carries it.
		return []Finding{c.finding("PB917", Info,
			"CFLAGS/LDFLAGS are not forwarded to cgo: without CGO_CFLAGS/CGO_LDFLAGS exports, Arch's hardening flags never reach C code in this build; export them or set CGO_ENABLED=0")}
	}
	return nil
}

// cgoFlagsForwarded reports whether c inherits the toolchain flags, either
// because a CGO_*FLAGS export reached it or because cgo is off for it.
func cgoFlagsForwarded(ctx *Context, c Command) bool {
	for _, name := range cgoFlagVars {
		if len(assignmentsTo(ctx, name, c)) > 0 {
			return true
		}
	}
	for _, v := range assignmentsTo(ctx, "CGO_ENABLED", c) {
		if strings.TrimSpace(v) == "0" {
			return true // cgo is off; there is nothing to forward
		}
	}
	return false
}
