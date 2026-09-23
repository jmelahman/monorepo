package rules

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Fixes for the build-system and language-toolchain rules: the ones whose
// remedy is a flag the invocation is missing, a flag value it got wrong, or
// the block of exports a toolchain needs to see Arch's own build flags. The
// edit machinery they build on lives in fix.go; the checks they pair with are
// in buildsys.go, ruststyle.go, gostyle.go and naming.go, and every fixer here
// walks the same command selector its check does so the two cannot drift.
//
// All of these change what the build produces — where it installs, which
// optimization level it compiles at, which cache it writes — so they are
// FixUnsafe to a rule. A flag that only moves droppings out of the user's home
// is no exception: the build still runs differently afterwards, and a
// maintainer who reads the diff is the point.

// --- PB950: cmake configure without an install prefix ------------------------

// fixCMakePrefix appends -DCMAKE_INSTALL_PREFIX=/usr to the cmake configure
// steps PB950 reports. cmake reads cache defines anywhere in the argument
// list, including after the source-directory positional, so the end of the
// command is a placement that works whatever the invocation looks like in
// between.
func fixCMakePrefix(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range cmakeUnprefixedConfigures(ctx) {
		if hiddenFlagWords(c) {
			continue
		}
		edit, ok := appendFlagEdit(c, "-DCMAKE_INSTALL_PREFIX=/usr",
			"add -DCMAKE_INSTALL_PREFIX=/usr to the cmake configure (installs under /usr, not /usr/local)")
		if !ok {
			continue
		}
		edits = append(edits, edit)
	}
	return edits
}

// --- PB951: CMAKE_BUILD_TYPE=Release clobbers Arch's flags -------------------

// fixCMakeBuildType rewrites the Release build type to None, which is what
// stops cmake appending its own -O3 -DNDEBUG after the flags makepkg exported.
//
// Only the value is rewritten, not the whole argument: the define may be
// written -DCMAKE_BUILD_TYPE=Release, quoted, spelled with an explicit
// :STRING type, or split across "-D" and its value, and all four keep whatever
// they said around the word Release. A define whose value did not come through
// literally is one the fix leaves alone — replaceWordTailEdit declines when
// the word does not end on the value the check read, which is the fixer saying
// it is not looking at the bytes it was told about.
func fixCMakeBuildType(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range cmakeReleaseConfigures(ctx) {
		_, i, ok := cmakeDefineAt(c, "CMAKE_BUILD_TYPE")
		if !ok || i >= len(c.ArgWord) {
			continue
		}
		edit, ok := replaceWordTailEdit(c.Unit, c.ArgWord[i], "Release", "None",
			"set -DCMAKE_BUILD_TYPE=None so cmake keeps Arch's CFLAGS")
		if !ok {
			continue
		}
		// The define may sit on a continuation line of a wrapped invocation;
		// the finding — and any directive waiving it — is at the command.
		edit.Line = int(c.Stmt.Pos().Line())
		edits = append(edits, edit)
	}
	return edits
}

// --- PB953: meson setup without --prefix -------------------------------------

// fixMesonPrefix appends --prefix=/usr to the meson configure steps PB953
// reports. meson's argument parser takes options after the positional build
// and source directories, so the end of the command is a safe placement for
// both the `meson setup build` and the legacy `meson build` spellings.
func fixMesonPrefix(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range mesonUnprefixedConfigures(ctx) {
		if hiddenFlagWords(c) {
			continue
		}
		edit, ok := appendFlagEdit(c, "--prefix=/usr",
			"add --prefix=/usr to the meson configure (installs under /usr, not /usr/local)")
		if !ok {
			continue
		}
		edits = append(edits, edit)
	}
	return edits
}

// --- PB941: cargo install without --no-track ---------------------------------

// fixCargoInstallTracked adds --no-track so cargo stops writing .crates.toml
// and .crates2.json into the install root, from where they are staged into the
// package. The flag goes in front of a `--` separator when there is one, for
// the reason fixCargoLocked spells out: past it the words are the installed
// program's, and cargo never reads them.
//
// cargoUntrackedInstalls has already left out the installs whose flags come
// out of a variable pkglint cannot follow, where the --no-track may be sitting
// in the part it cannot see.
func fixCargoInstallTracked(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range cargoUntrackedInstalls(ctx) {
		edit, ok := appendFlagEdit(c, "--no-track",
			"add --no-track to `cargo install` (keeps cargo's tracking files out of the package)")
		if !ok {
			continue
		}
		edits = append(edits, edit)
	}
	return edits
}

// --- PB980: npm writing the user's ~/.npm cache ------------------------------

// fixNpmUserCache points npm at a cache inside $srcdir, which is where the
// Node.js package guidelines put it: the build then leaves nothing in the
// invoking user's home, and makepkg cleans the cache up with the rest of the
// source tree.
//
// The value is written as "$srcdir/npm-cache" — quoted, because $srcdir
// contains the package name and a maintainer whose checkout path has a space
// in it should not get a broken build out of a lint fix.
func fixNpmUserCache(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range npmUncachedInstalls(ctx) {
		if hiddenFlagWords(c) {
			continue
		}
		edit, ok := appendFlagEdit(c, `--cache "$srcdir/npm-cache"`,
			`add --cache "$srcdir/npm-cache" to npm (keeps the cache out of the user's home)`)
		if !ok {
			continue
		}
		edits = append(edits, edit)
	}
	return edits
}

// --- PB917: hardening flags never reach cgo ----------------------------------

// fixGoCgoFlags writes the Go package guidelines' block of CGO_* exports at
// the top of the function that runs the build, so the C parts of a cgo build
// compile with the same hardening flags as everything else in the package.
//
// All four variables are exported rather than the two the finding names: they
// are what the guidelines prescribe, and a build whose C++ or preprocessor
// flags were left behind is the same bug one step over. One block fixes the
// whole PKGBUILD, matching the single finding checkGoCgoFlags emits.
//
// The block goes at the top of the function rather than beside the go command
// because that is where it provably covers every build in it — including ones
// in a loop or a conditional, whose own line is no place to put an export. A
// function whose body opens on the same line as its first statement has no
// line of its own to take, and the finding stands.
func fixGoCgoFlags(ctx *Context, _ *FixEnv) []Edit {
	for _, c := range goBuildCommands(ctx) {
		if cgoFlagsForwarded(ctx, c) {
			continue
		}
		if edit, ok := cgoExportEdit(c); ok {
			return []Edit{edit}
		}
		return nil
	}
	return nil
}

func cgoExportEdit(c Command) (Edit, bool) {
	u := c.Unit
	fd := u.Functions[c.Fn]
	if fd == nil {
		return Edit{}, false
	}
	block, ok := fd.Body.Cmd.(*syntax.Block)
	if !ok || len(block.Stmts) == 0 {
		return Edit{}, false
	}
	first := block.Stmts[0]
	if first.Pos().Line() == block.Lbrace.Line() {
		return Edit{}, false // a one-line body; nowhere to put whole lines
	}
	at := lineStart(u.Raw, off(first.Pos()))
	indent := lineIndent(u.Raw, off(first.Pos()))
	var b strings.Builder
	for _, name := range cgoFlagVars {
		// CGO_CFLAGS takes CFLAGS, CGO_LDFLAGS takes LDFLAGS, and so on: the
		// variable each one forwards is its own name without the prefix.
		b.WriteString(indent + "export " + name + `="$` + strings.TrimPrefix(name, "CGO_") + "\"\n")
	}
	return Edit{
		Path:  u.Path,
		Start: at,
		End:   at,
		New:   b.String(),
		// The finding's line, not the insertion point's: a directive waiving
		// PB917 sits at the go command its finding points at, and the
		// suppression check reads the edit's Line to honor it.
		Line: int(c.Stmt.Pos().Line()),
		Desc: "export the CGO_* flags in " + c.Fn + "() so Arch's hardening reaches cgo",
	}, true
}
