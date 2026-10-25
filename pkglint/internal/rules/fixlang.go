package rules

import (
	"fmt"
	"regexp"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// --- PB203: cargo without --locked -----------------------------------------

// fixCargoLocked adds --locked to the cargo commands PB203 reports, at the end
// of the command. Appending, rather than inserting after the subcommand, is
// what lets it fix a command with an unreadable value in it — `cargo fetch
// --target "$CARCH-…"`, the guidelines' own prepare() — since the edit never
// has to know what the words in between say. cargoUnlockedCommands has already
// left out the commands whose flags pkglint cannot read.
//
// The end is the wrong place once the command has a `--`, since past it the
// words are the called program's: appending to `cargo rustc -- -C
// target-cpu=native` hands rustc a --locked it rejects, trading the finding for
// a build that no longer runs. The flag goes in front of the separator
// instead — and where the separator arrives through a variable there is no
// place in the text to put it, so the finding stands.
func fixCargoLocked(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range cargoUnlockedCommands(ctx) {
		// cargoHasSeparator sees a `--` that arrives through a variable;
		// appendFlagAt only sees one written out. A separator with no word
		// spelling it has nowhere in the text to write in front of, so the
		// finding stands.
		if cargoHasSeparator(c) && wordByValue(c, "--") == nil {
			continue
		}
		edit, ok := appendFlagEdit(c, "--locked",
			fmt.Sprintf("add --locked to `cargo %s`", c.Subcommand()))
		if !ok {
			continue
		}
		edits = append(edits, edit)
	}
	return edits
}

// --- PB940: cargo test/check --release -------------------------------------

// fixCargoCheckRelease deletes the release flag from cargo test/check so the
// run keeps the debug assertions and overflow checks it exists to exercise. It
// removes whichever spelling the command used, `--release` or `-r`, and only
// one written literally before the `--` separator: one that arrives through a
// variable is not ours to rewrite, and one after `--` is the test binary's
// argument, which cargoReleaseFlag already declines to report.
func fixCargoCheckRelease(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range ctx.CommandsNamed("cargo") {
		sub := c.Subcommand()
		flag := cargoReleaseFlag(c)
		if (sub != "test" && sub != "check") || flag == "" {
			continue
		}
		w := wordByValue(c, flag)
		if w == nil {
			continue // written as an expansion: the finding stands
		}
		start, end := flagCut(c.Unit.Raw, off(w.Pos()), off(w.End()))
		edits = append(edits, Edit{
			Path:  c.Unit.Path,
			Start: start,
			End:   end,
			New:   "",
			Line:  int(w.Pos().Line()),
			Desc:  fmt.Sprintf("drop %s from `cargo %s` (keeps debug assertions and overflow checks on)", flag, sub),
		})
	}
	return edits
}

// flagCut widens the byte range of a flag word to what deleting it should
// actually take: the whitespace in front of it, and — when the flag sits alone
// on a continuation line — that whole line, including the backslash that would
// otherwise be left continuing into nothing or dangling off the line above.
func flagCut(raw []byte, start, end int) (int, int) {
	for start > 0 && (raw[start-1] == ' ' || raw[start-1] == '\t') {
		start--
	}
	if start == 0 || raw[start-1] != '\n' {
		return start, end
	}
	rest := end
	for rest < len(raw) && (raw[rest] == ' ' || raw[rest] == '\t') {
		rest++
	}
	switch {
	case rest+1 < len(raw) && raw[rest] == '\\' && raw[rest+1] == '\n':
		// More words follow: take this line and its newline.
		return start, rest + 2
	case rest >= len(raw) || raw[rest] == '\n':
		// Last line of the call: the previous line's `\` continued into it and
		// must go too, or the command runs on into whatever comes next.
		cut := start - 1 // the newline
		if cut > 0 && raw[cut-1] == '\\' {
			cut--
			for cut > 0 && (raw[cut-1] == ' ' || raw[cut-1] == '\t') {
				cut--
			}
			return cut, rest
		}
	}
	return start, end
}

// --- PB942: cargo build without --release ----------------------------------

// fixCargoBuildRelease inserts --release into the build() cargo build, right
// after the subcommand word: cargo reads its own flags anywhere ahead of the
// `--` separator, and putting it there keeps it out of whatever the PKGBUILD
// hands the compiler on the far side.
//
// Two shapes are declined rather than guessed at. A command whose words do not
// all render — `cargo build --features "${myfeatures[@]}"` — is one the rule
// reports (cargoDevProfileBuilds has already stood down where the unreadable
// word could be a profile flag) but the fix will not rewrite: an argument list
// pkglint cannot read in full is not one to edit blind. And a PKGBUILD that
// reaches into a debug/ directory is reading the artifact this flag moves:
// cargo writes the dev profile to target/debug and the release one to
// target/release, so the flag alone would leave package() copying a path that
// no longer exists. Repointing it is a rewrite of package(), not a flag
// insertion, so the finding stands.
func fixCargoBuildRelease(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range cargoDevProfileBuilds(ctx) {
		if hasOpaqueArg(c) || namesDevArtifact(c.Unit) {
			continue
		}
		w := wordByValue(c, "build")
		if w == nil {
			continue // the subcommand itself came through an expansion
		}
		at := off(w.End())
		edits = append(edits, Edit{
			Path:  c.Unit.Path,
			Start: at,
			End:   at,
			New:   " --release",
			Line:  int(c.Stmt.Pos().Line()),
			Desc:  "insert --release into `cargo build` (ships the optimized profile)",
		})
	}
	return edits
}

// devArtifactRe matches a reference to a debug/ directory, which in a Rust
// build tree is cargo's dev-profile output: target/debug, the
// target/<triple>/debug of a --target build, or the same under whatever
// CARGO_TARGET_DIR the PKGBUILD picked. Matching the directory rather than any
// particular target path costs the odd unrelated debug/ (Arch's own
// /usr/lib/debug among them) a fix it could have had, which is the cheap side
// of the trade: the alternative is emitting one that stops the build.
var devArtifactRe = regexp.MustCompile(`/debug\b`)

// namesDevArtifact reports whether the unit names a debug/ path anywhere,
// source text included — a fix that has to be sure cannot limit itself to the
// arguments it managed to resolve.
func namesDevArtifact(u *pkgbuild.Unit) bool {
	return u != nil && devArtifactRe.Match(u.Raw)
}

// --- PB206: npm/yarn lockfile-faithful install -----------------------------

func fixNpmCI(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range ctx.CommandsNamed("npm") {
		sub := c.Subcommand()
		if (sub != "install" && sub != "i") || namesPackages(c) {
			continue // `npm install <pkg>` is not equivalent to `npm ci`
		}
		w := wordByValue(c, sub)
		if w == nil {
			continue
		}
		edits = append(edits, Edit{
			Path:  c.Unit.Path,
			Start: off(w.Pos()),
			End:   off(w.End()),
			New:   "ci",
			Line:  int(c.Stmt.Pos().Line()),
			Desc:  fmt.Sprintf("replace `npm %s` with `npm ci` (installs exactly the committed lockfile)", sub),
		})
	}
	return append(edits, fixLockfileFlags(ctx, "PB206")...)
}
