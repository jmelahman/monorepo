package rules

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// PB950–PB955 lint the Arch CMake and Meson package guidelines
// (https://manual.archlinux.page/package-guidelines/cmake/,
// https://manual.archlinux.page/package-guidelines/meson/): the /usr
// prefix, CMake's flag-clobbering Release build type, the build tools
// declared in makedepends, and meson's compile wrapper instead of bare
// ninja. Redirecting the install step into $pkgdir is PB404's.

// --- shared cmake/meson argument helpers -------------------------------------

// cmakeConfigures reports whether a cmake invocation is a configure step —
// the one that fixes the install prefix and build type — rather than one of
// cmake's other modes (--build, --install, script/tool execution).
func cmakeConfigures(c Command) bool {
	for _, a := range c.Args {
		switch a {
		case "--build", "--install", "--open", "--find-package", "--workflow",
			"-E", "-P", "--version", "--help":
			return false
		}
	}
	return true
}

// cmakeDefine returns the value of a -Dname=value cache define, handling the
// split "-D name=value" spelling and cmake's typed "-Dname:TYPE=value" form.
func cmakeDefine(c Command, name string) (string, bool) {
	v, _, ok := cmakeDefineAt(c, name)
	return v, ok
}

// cmakeDefineAt additionally returns the index of the argument the value was
// read from, which for the split spelling is the word after the bare "-D".
// A fixer that rewrites a define has to edit that word and no other.
func cmakeDefineAt(c Command, name string) (string, int, bool) {
	tail := func(a, prefix string) (string, bool) {
		rest, ok := strings.CutPrefix(a, prefix)
		if !ok {
			return "", false
		}
		if i := strings.IndexByte(rest, '='); i >= 0 && (i == 0 || rest[0] == ':') {
			return rest[i+1:], true
		}
		return "", false
	}
	for i, a := range c.Args {
		if v, ok := tail(a, "-D"+name); ok {
			return v, i, true
		}
		if a == "-D" && i+1 < len(c.Args) {
			if v, ok := tail(c.Args[i+1], name); ok {
				return v, i + 1, true
			}
		}
	}
	return "", 0, false
}

// hasArraySplat reports whether any argument word expands a whole array
// ("${_cmake_args[@]}") — the long-invocation idiom whose flags static
// reading cannot see, so absence claims about them would be guesses.
func hasArraySplat(c Command) bool {
	raw := c.Unit.Raw
	for _, w := range c.Call.Args {
		start, end := int(w.Pos().Offset()), int(w.End().Offset())
		if start < 0 || end > len(raw) || start >= end {
			continue
		}
		if strings.Contains(string(raw[start:end]), "[@]") || strings.Contains(string(raw[start:end]), "[*]") {
			return true
		}
	}
	return false
}

// mesonNonSetupSubcommands are the meson verbs that are not a project
// configure; anything else — `meson setup`, or the legacy `meson builddir`
// spelling where the first word is a directory — configures the build.
var mesonNonSetupSubcommands = map[string]bool{
	"compile": true, "test": true, "install": true, "dist": true,
	"configure": true, "introspect": true, "init": true, "wrap": true,
	"subprojects": true, "rewrite": true, "devenv": true, "format": true,
}

func mesonConfigures(c Command) bool {
	return !mesonNonSetupSubcommands[c.Subcommand()]
}

func mesonSetsPrefix(c Command) bool {
	for _, a := range c.Args {
		if a == "--prefix" || strings.HasPrefix(a, "--prefix=") || strings.HasPrefix(a, "-Dprefix=") {
			return true
		}
	}
	return false
}

// buildToolCommands returns the non-scriptlet, in-function invocations of the
// named tools.
func buildToolCommands(ctx *Context, names ...string) []Command {
	var out []Command
	for _, c := range ctx.CommandsNamed(names...) {
		if c.Unit.Scriptlet || c.Fn == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

// toolMakedependsGap is the shared shape of PB944/PB952/PB954/PB979: the build
// invokes a tool but no listed package puts it on $PATH. It returns the first
// invocation — the one the finding points at, since the remedy is a single
// makedepends entry however many times the tool is run — and whether the gap
// is there at all. The fixes read the same answer, so a rule that stands down
// takes its fix with it.
func toolMakedependsGap(ctx *Context, cmds []Command, packages ...string) (Command, bool) {
	if len(cmds) == 0 {
		return Command{}, false
	}
	for _, name := range packages {
		if hasDep(ctx, "makedepends", name) || hasDep(ctx, "depends", name) {
			return Command{}, false
		}
	}
	return cmds[0], true
}

// toolMakedependsFinding is the message all four of those rules share.
func toolMakedependsFinding(id string, c Command, tool, pkg string) []Finding {
	return []Finding{c.finding(id, Warn,
		"%s is used but %q is not in makedepends; a clean build environment cannot run this build", tool, pkg)}
}

// --- PB950: cmake configure without an install prefix ------------------------

// cmakeUnprefixedConfigures returns the cmake configure steps left on the
// default /usr/local prefix: the commands PB950 reports, and the ones its fix
// inserts -DCMAKE_INSTALL_PREFIX=/usr into, so rule and fix cannot drift apart.
func cmakeUnprefixedConfigures(ctx *Context) []Command {
	var configures []Command
	for _, c := range buildToolCommands(ctx, "cmake") {
		if !cmakeConfigures(c) {
			continue
		}
		// Any explicit prefix is a decision (the guidelines allow /opt for
		// self-contained trees) — and one that covers the whole PKGBUILD: a
		// second configure without it builds bundled deps or a test tree that
		// installs nothing (neovim's cmake.deps), not the shipped artifacts.
		// A splatted flag array may carry the prefix invisibly, so it stands
		// the whole rule down too. CMAKE_STAGING_PREFIX is where
		// `cmake --install` writes when it is set (wasi-libc++ installs into
		// /usr/share/wasi-sysroot that way), so it is the same decision.
		if hasArraySplat(c) {
			return nil
		}
		for _, name := range []string{"CMAKE_INSTALL_PREFIX", "CMAKE_STAGING_PREFIX"} {
			if _, ok := cmakeDefine(c, name); ok {
				return nil
			}
		}
		for _, a := range c.Args {
			if a == "--install-prefix" || strings.HasPrefix(a, "--install-prefix=") {
				return nil // cmake 3.21's spelling of CMAKE_INSTALL_PREFIX
			}
		}
		configures = append(configures, c)
	}
	return configures
}

func checkCMakePrefix(ctx *Context) []Finding {
	var out []Finding
	for _, c := range cmakeUnprefixedConfigures(ctx) {
		out = append(out, c.finding("PB950", Warn,
			"cmake configure without -DCMAKE_INSTALL_PREFIX=/usr defaults to /usr/local, which Arch packages must not touch"))
	}
	return out
}

// --- PB951: CMAKE_BUILD_TYPE=Release clobbers Arch's flags -------------------

// cmakeReleaseConfigures returns the cmake configure steps that ask for the
// Release build type: the commands PB951 reports. The ones its fix rewrites to
// None are a subset, see cmakeOwnFlags.
//
// cmake seeds CMAKE_C_FLAGS/CMAKE_CXX_FLAGS from $CFLAGS/$CXXFLAGS only when
// the command line does not define them, so a configure that defines every
// one it sets to its own flags — llvm's device runtimes pass an empty
// `-DCMAKE_C_FLAGS=`, wasi-compiler-rt a hand-written list — never sees Arch's
// -O2, and Release's -O3 is not overriding anything. Such a configure is left
// out. One that hands at least one language Arch's flags is still reported.
func cmakeReleaseConfigures(ctx *Context) []Command {
	var out []Command
	for _, c := range buildToolCommands(ctx, "cmake") {
		if !cmakeConfigures(c) {
			continue
		}
		v, ok := cmakeDefine(c, "CMAKE_BUILD_TYPE")
		if !ok || v != "Release" {
			continue
		}
		own, arch := cmakeFlagSources(c)
		if own && !arch {
			continue
		}
		if cmakeDefinesReleaseFlags(c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// cmakeDefinesReleaseFlags reports whether c replaces the Release
// configuration's own flags (CMAKE_C_FLAGS_RELEASE and friends) with
// makepkg's. That swaps the -O3 -DNDEBUG Release would add for $CFLAGS, so the
// build type is how the PKGBUILD chooses the flags — c3c sets them to
// "$CFLAGS" — and None would leave them unused. A Release flags define that
// is something else (cmake's own "-O3 -DNDEBUG", a hand-written list) keeps
// the finding: Arch's -O2 is still overridden. As with cmakeFlagSources, a
// language the configure does not mention is left out of the question.
func cmakeDefinesReleaseFlags(c Command) bool {
	found := false
	for _, d := range []struct{ define, env string }{
		{"CMAKE_C_FLAGS_RELEASE", "CFLAGS"},
		{"CMAKE_CXX_FLAGS_RELEASE", "CXXFLAGS"},
	} {
		_, i, ok := cmakeDefineAt(c, d.define)
		if !ok || i >= len(c.ArgWord) {
			continue
		}
		if !carriesEnvFlags(c, c.ArgWord[i], d.env, map[string]bool{}) {
			return false
		}
		found = true
	}
	return found
}

// cmakeOwnFlags reports whether any language's flags in c are the
// PKGBUILD's own rather than Arch's. Release is that language's only -O, so
// None would build it unoptimized: the configure may still be reported for
// the other language, but its build type is not one to rewrite.
func cmakeOwnFlags(c Command) bool {
	own, _ := cmakeFlagSources(c)
	return own
}

// cmakeFlagSources sorts the configure's CMAKE_C_FLAGS/CMAKE_CXX_FLAGS
// defines: own when one is defined to flags that do not carry makepkg's,
// arch when one does. A language left undefined is neither — it gets Arch's
// flags from the environment, but whether the project compiles that language
// at all is not something the command line says.
func cmakeFlagSources(c Command) (own, arch bool) {
	for _, d := range []struct{ define, env string }{
		{"CMAKE_C_FLAGS", "CFLAGS"},
		{"CMAKE_CXX_FLAGS", "CXXFLAGS"},
	} {
		_, i, ok := cmakeDefineAt(c, d.define)
		if !ok || i >= len(c.ArgWord) {
			continue
		}
		if carriesEnvFlags(c, c.ArgWord[i], d.env, map[string]bool{}) {
			arch = true
		} else {
			own = true
		}
	}
	return own, arch
}

// makepkgPathVars are the variables makepkg defines that name a package or a
// directory: they turn up in hand-written flags (-I$srcdir/include,
// --sysroot=…) but never hold CFLAGS.
var makepkgPathVars = map[string]bool{
	"srcdir": true, "pkgdir": true, "startdir": true, "pkgname": true,
	"pkgbase": true, "pkgver": true, "pkgrel": true, "epoch": true,
	"CARCH": true, "CHOST": true,
}

// carriesEnvFlags reports whether w may expand to the value of $env: it
// references env itself (in any form — `${CFLAGS/-O2/-O3}` is still Arch's
// flags), or a variable the PKGBUILD builds from it (lib32-llvm's
// `LIB32_CFLAGS="$CFLAGS -m32"`), or something pkglint cannot read — a
// command substitution, or a variable the file never assigns.
//
// It reads the source word, not the rendered argument: rendering substitutes
// top-level assignments, so a file's own `CFLAGS+=" -fPIC"` would otherwise
// erase the very reference this looks for. seen holds the variables already
// followed; each is read once, which bounds the work by the file's size and
// makes `_flags="$_flags -b"` mean what it says — the earlier value, which is
// already being read.
func carriesEnvFlags(c Command, w *syntax.Word, env string, seen map[string]bool) bool {
	carries := false
	syntax.Walk(w, func(n syntax.Node) bool {
		if carries {
			return false
		}
		switch x := n.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst:
			carries = true
		case *syntax.ParamExp:
			if x.Param == nil {
				carries = true
				break
			}
			name := x.Param.Value
			switch {
			case name == env || x.Excl:
				carries = true
			case makepkgPathVars[name] || seen[name]:
			default:
				seen[name] = true
				carries = !assignedOwnFlags(c, name, env, seen)
			}
		}
		return !carries
	})
	return carries
}

// assignedOwnFlags reports whether the file assigns name, in c's scope, and
// none of those assignments carry $env.
func assignedOwnFlags(c Command, name, env string, seen map[string]bool) bool {
	found, carries := false, false
	scanAssignments(c.Unit, name, c.Fn, off(c.Stmt.Pos()), c.Call, true, func(as *syntax.Assign) {
		found = true
		if carries {
			return
		}
		if as.Value != nil && carriesEnvFlags(c, as.Value, env, seen) {
			carries = true
		}
		if as.Array != nil {
			for _, el := range as.Array.Elems {
				if el.Value != nil && carriesEnvFlags(c, el.Value, env, seen) {
					carries = true
				}
			}
		}
	})
	return found && !carries
}

func checkCMakeBuildType(ctx *Context) []Finding {
	var out []Finding
	for _, c := range cmakeReleaseConfigures(ctx) {
		out = append(out, c.finding("PB951", Warn,
			"-DCMAKE_BUILD_TYPE=Release appends -O3 -DNDEBUG after Arch's CFLAGS, overriding the distribution's -O2; the CMake package guidelines build with -DCMAKE_BUILD_TYPE=None"))
	}
	return out
}

// --- PB952/PB954: build system missing from makedepends ----------------------

// depNameContains reports whether any depends/makedepends entry's name
// contains sub — the leniency the cross-toolchain wrappers need, where
// mingw-w64-cmake (which pulls cmake) is the declared dependency.
func depNameContains(ctx *Context, sub string) bool {
	for _, field := range []string{"depends", "makedepends"} {
		for name := range depsFor(ctx, field) {
			if strings.Contains(name, sub) {
				return true
			}
		}
	}
	return false
}

func cmakeMakedependsGap(ctx *Context) (Command, bool) {
	if depNameContains(ctx, "cmake") {
		return Command{}, false
	}
	return toolMakedependsGap(ctx, buildToolCommands(ctx, "cmake"), "cmake")
}

func checkCMakeMakedepends(ctx *Context) []Finding {
	c, ok := cmakeMakedependsGap(ctx)
	if !ok {
		return nil
	}
	return toolMakedependsFinding("PB952", c, "cmake", "cmake")
}

func mesonMakedependsGap(ctx *Context) (Command, bool) {
	if depNameContains(ctx, "meson") {
		return Command{}, false
	}
	return toolMakedependsGap(ctx, buildToolCommands(ctx, "meson", "arch-meson"), "meson")
}

func checkMesonMakedepends(ctx *Context) []Finding {
	c, ok := mesonMakedependsGap(ctx)
	if !ok {
		return nil
	}
	return toolMakedependsFinding("PB954", c, "meson", "meson")
}

// --- PB953: meson setup without --prefix -------------------------------------

// mesonUnprefixedConfigures returns the meson configure steps left on the
// default /usr/local prefix: the commands PB953 reports, and the ones its fix
// appends --prefix=/usr to.
func mesonUnprefixedConfigures(ctx *Context) []Command {
	var out []Command
	for _, c := range buildToolCommands(ctx, "meson") {
		if !mesonConfigures(c) || mesonSetsPrefix(c) || hasArraySplat(c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func checkMesonPrefix(ctx *Context) []Finding {
	var out []Finding
	for _, c := range mesonUnprefixedConfigures(ctx) {
		out = append(out, c.finding("PB953", Warn,
			"meson setup without --prefix=/usr defaults to /usr/local, which Arch packages must not touch (arch-meson passes the guideline flags for you)"))
	}
	return out
}

// --- PB955: bare ninja in a meson build --------------------------------------

func checkMesonNinjaDirect(ctx *Context) []Finding {
	if len(buildToolCommands(ctx, "meson", "arch-meson")) == 0 {
		return nil
	}
	var out []Finding
	for _, c := range buildToolCommands(ctx, "ninja") {
		out = append(out, c.finding("PB955", Info,
			"this project is configured with meson; `meson compile` (and `meson install`, `meson test`) wraps ninja with the configured environment — prefer it over calling ninja directly"))
	}
	return out
}
