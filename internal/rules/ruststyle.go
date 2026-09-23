package rules

import (
	"slices"
	"strings"
)

// PB940–PB942, PB944 lint the Arch Rust package guidelines
// (https://wiki.archlinux.org/title/Rust_package_guidelines): release builds,
// debug-assertion-preserving tests, --no-track installs, and the toolchain
// declared in makedepends. Lockfile enforcement (--locked/--frozen, including
// on `cargo fetch` in prepare()) is hermeticity and already PB203's.

// --- shared cargo argument reading -------------------------------------------
//
// PB203 (cargo without a lockfile flag, in hermetic.go) reads its arguments
// through these too: every rule that reports a cargo flag missing has the same
// problem, which is that PKGBUILDs collect cargo's flags in a variable and
// expand it at each call site. The answer is in two halves — read the variable
// (cargoWords), and where it cannot be read, report nothing rather than report
// a flag missing from a command nobody can see the flags of (cargoFlagsHidden).

// cargoWords returns the words cargo is handed, which is not quite the
// command's argument list: an argument written as a variable arrives carrying
// whatever the variable holds, and bash splits an unquoted `$_cargo_flags` of
// "--release --locked" into two words before cargo ever sees it. Splitting
// every argument reads a quoted "$_cargo_flags" the same way, which is the
// harmless direction to be wrong in — the PKGBUILD asked for the release
// profile either way, and cargo choking on the glued-together word it really
// gets is a different complaint than this one.
//
// Words that are still expansions afterwards are kept as they rendered, so
// cargoFlagsHidden can tell what is left unread.
func cargoWords(c Command) []string {
	var out []string
	for i := range c.Args {
		out = append(out, argWords(c, i)...)
	}
	return out
}

// cargoHasFlag reports whether the command passes any of flags to cargo,
// variables included: a PKGBUILD that keeps `--locked --no-track` in one place
// and expands it at every call site has passed them.
//
// Unlike cargoReleaseFlag this does not stop at the `--` separator. The rules
// asking are the ones that report a flag missing, and a command that spells the
// flag out — even in the one place cargo will not read it — is not one to hand
// a second copy of.
func cargoHasFlag(c Command, flags ...string) bool {
	for _, a := range cargoWords(c) {
		if slices.Contains(flags, a) {
			return true
		}
	}
	return false
}

// cargoValueFlags are the cargo options that take their value as the next
// word. An unreadable word right after one of them is that value — a feature
// list, a target triple, the $pkgdir install root the guidelines ask for — and
// not somewhere a flag could be hiding.
var cargoValueFlags = map[string]bool{
	"--artifact-dir": true, "--bench": true, "--bin": true, "--branch": true,
	"--color": true, "--config": true, "--crate-type": true, "--example": true,
	"--exclude": true, "--features": true, "--git": true, "--index": true,
	"--jobs": true, "--manifest-path": true, "--message-format": true,
	"--out-dir": true, "--package": true, "--path": true, "--profile": true,
	"--registry": true, "--rev": true, "--root": true, "--tag": true,
	"--target": true, "--target-dir": true, "--test": true, "--version": true,
	"-F": true, "-j": true, "-p": true, "-Z": true,
}

// cargoFlagsHidden reports whether the command still carries a word that could
// be a cargo flag pkglint cannot read after cargoWords has read what it can:
// `cargo build $CARGO_ARGS`, where the name belongs to makepkg.conf or the
// environment and nothing in the file says what is in it. The flag a rule is
// about to report missing may well be in there.
//
// An unreadable word is not automatically a hidden flag. One that begins with
// a dash is a flag whose value is unreadable rather than an unreadable flag
// (`--root="$pkgdir/usr"`), one that follows a value-taking option is that
// option's value, and one past the `--` separator is the called program's
// argument, where cargo does not read flags at all. All three are common
// enough that treating every expansion as a possible flag would cost these
// rules most of what they catch.
func cargoFlagsHidden(c Command) bool {
	prev := ""
	for _, w := range cargoWords(c) {
		if w == "--" {
			return false
		}
		if hasVarRef(w) && !strings.HasPrefix(w, "-") && !cargoValueFlags[prev] {
			return true
		}
		prev = w
	}
	return false
}

// cargoHasSeparator reports whether the command carries a `--`, the word that
// ends cargo's own arguments and begins those of the program it runs. A fix
// that writes a cargo flag at the end of the command has to ask: past the
// separator the flag reaches rustc or the test binary, which rejects it.
//
// A separator inside a variable counts, since it separates all the same. The
// caller then finds no argument of the command spelling it, which is the cue
// that there is nowhere in the text to write in front of.
func cargoHasSeparator(c Command) bool {
	return slices.Contains(cargoWords(c), "--")
}

// --- PB940: cargo test/check --release ---------------------------------------

// cargoReleaseFlag returns the spelling the command uses to ask cargo for the
// release profile — `--release` or its short form `-r`, which cargo documents
// as the same flag and which real PKGBUILDs prefer — or "" when it asks for
// neither. Everything after the `--` separator goes to the program cargo runs,
// where `--release` is that program's own argument and none of cargo's
// business.
func cargoReleaseFlag(c Command) string {
	for _, a := range cargoWords(c) {
		switch a {
		case "--":
			return ""
		case "--release", "-r":
			return a
		}
	}
	return ""
}

// cargoRelease reports whether the command passes --release to cargo itself.
func cargoRelease(c Command) bool { return cargoReleaseFlag(c) != "" }

func checkCargoCheckRelease(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.CommandsNamed("cargo") {
		sub := c.Subcommand()
		if (sub != "test" && sub != "check") || !cargoRelease(c) {
			continue
		}
		out = append(out, c.finding("PB940", Warn,
			"cargo %s --release disables debug assertions and overflow checks, weakening exactly what the tests are there to catch; the Rust package guidelines run tests without it", sub))
	}
	return out
}

// --- PB941: cargo install leaves tracking metadata ---------------------------

// cargoUntrackedInstalls returns the cargo installs that still write cargo's
// tracking files into the install root: the commands PB941 reports, and the
// ones its fix appends --no-track to, so rule and fix cannot drift apart.
func cargoUntrackedInstalls(ctx *Context) []Command {
	var out []Command
	for _, c := range ctx.CommandsNamed("cargo") {
		// A flags variable pkglint cannot read may hold the --no-track this
		// would report missing; the install root and crate path it can't read
		// are values, and say nothing either way.
		if c.Subcommand() != "install" || cargoHasFlag(c, "--no-track") || cargoFlagsHidden(c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func checkCargoInstallTracked(ctx *Context) []Finding {
	var out []Finding
	for _, c := range cargoUntrackedInstalls(ctx) {
		out = append(out, c.finding("PB941", Warn,
			"cargo install without --no-track writes .crates.toml/.crates2.json into the install root, which ends up in the package"))
	}
	return out
}

// --- PB942: cargo build without --release ------------------------------------

// cargoDevProfileBuilds returns the build() cargo builds left on cargo's
// unoptimized default profile: the commands PB942 reports, and the ones its
// fix inserts --release into, so rule and fix cannot drift apart.
//
// Two builds are somebody's decision rather than an oversight. One that names
// a profile explicitly (--profile) has chosen it, like an explicit -buildmode
// in PB914. And one whose build() also runs a release build is half of a
// deliberate pair — packages compile the dev profile alongside it for a test
// that needs its stricter limits — where the binary that ships still comes out
// of the release half.
//
// A third build is not a decision but an unreadable command: one whose flags
// come out of a variable pkglint cannot follow (cargoFlagsHidden), where
// --release may be sitting in the part it cannot see. Such a build is skipped
// rather than counted — it is no evidence of a dev-profile artifact, and none
// of a release one for a sibling build either.
func cargoDevProfileBuilds(ctx *Context) []Command {
	var dev []Command
	release := map[string]bool{}
	for _, c := range ctx.CommandsNamed("cargo") {
		if c.Subcommand() != "build" || c.Fn != "build" {
			continue
		}
		if cargoRelease(c) || cargoHasProfile(c) {
			release[c.Unit.Path] = true
			continue
		}
		if cargoFlagsHidden(c) {
			continue
		}
		dev = append(dev, c)
	}
	out := dev[:0]
	for _, c := range dev {
		if !release[c.Unit.Path] {
			out = append(out, c)
		}
	}
	return out
}

func checkCargoBuildRelease(ctx *Context) []Finding {
	var out []Finding
	for _, c := range cargoDevProfileBuilds(ctx) {
		out = append(out, c.finding("PB942", Info,
			"cargo build without --release ships an unoptimized debug binary; the Rust package guidelines build with --release"))
	}
	return out
}

func cargoHasProfile(c Command) bool {
	for _, a := range cargoWords(c) {
		if a == "--profile" || strings.HasPrefix(a, "--profile=") {
			return true
		}
	}
	return false
}

// --- PB944: rust toolchain missing from makedepends --------------------------

// rustToolchainPackages are the packages that put cargo on $PATH.
var rustToolchainPackages = []string{"rust", "rustup"}

func rustMakedependsGap(ctx *Context) (Command, bool) {
	return toolMakedependsGap(ctx, buildToolCommands(ctx, "cargo"), rustToolchainPackages...)
}

func checkRustMakedepends(ctx *Context) []Finding {
	c, ok := rustMakedependsGap(ctx)
	if !ok {
		return nil
	}
	// One finding per PKGBUILD: the remedy is one makedepends entry.
	return []Finding{c.finding("PB944", Info,
		"cargo is used but neither rust nor rustup is in makedepends; a clean build environment cannot run this build")}
}
