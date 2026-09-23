package rules

import (
	"strings"
)

// PB970–PB984 lint the prefix-triggered ecosystem pages of the Arch package
// guidelines: fonts (ttf-/otf-), DKMS modules (-dkms), 32-bit multilib
// (lib32-), MinGW cross packages (mingw-w64-), Node.js (npm), Java, CLR/.NET
// (mono, msbuild), Haskell (haskell-) and PHP (php-). Each family's rules
// only fire on packages of that kind, so a PKGBUILD outside the family never
// sees them.

// --- PB970/PB971/PB972: font packages ----------------------------------------

func isFontPackage(ctx *Context) bool {
	return namePrefixed(ctx, "ttf-", "otf-")
}

func checkFontArch(ctx *Context) []Finding {
	if !isFontPackage(ctx) || archIsAny(ctx) || len(concreteArches(ctx)) == 0 {
		return nil
	}
	return []Finding{varFinding(ctx, "PB970", Warn, []string{"arch"},
		"font files are architecture-independent, so the font package guidelines require arch=('any')")}
}

// fontDepends returns a font package's depends entries: what PB971 reports,
// and what its fix deletes.
func fontDepends(ctx *Context) []depEntry {
	if !isFontPackage(ctx) {
		return nil
	}
	return depEntries(ctx, "depends")
}

func checkFontDepends(ctx *Context) []Finding {
	var out []Finding
	for _, d := range fontDepends(ctx) {
		out = append(out, findingAt("PB971", Info, ctx.Pkg.PKGBUILD.Path, d.Pos,
			"font packages need no dependencies — fontconfig discovers installed fonts by itself; drop %q", d.Full))
	}
	return out
}

// unstableFontHosts serve download URLs whose content changes as the font is
// revised, so a pinned checksum starts failing without any PKGBUILD change.
var unstableFontHosts = []string{"fonts.google.com", "fontspace.com"}

func checkFontUnstableSource(ctx *Context) []Finding {
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		host := strings.ToLower(e.Host())
		if host == "" {
			continue
		}
		for _, bad := range unstableFontHosts {
			if host != bad && !strings.HasSuffix(host, "."+bad) {
				continue
			}
			out = append(out, findingAt("PB972", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"%s serves a generated archive that changes as the font is revised, so the pinned checksum will break; the font package guidelines say to fetch from the font's own release channel", host))
			break
		}
	}
	return out
}

// --- PB973/PB974: DKMS packages ----------------------------------------------

// dkmsDependsMissing reports the gap PB973 names, and the one its fix closes
// by adding dkms to depends.
func dkmsDependsMissing(ctx *Context) bool {
	if !nameSuffixed(ctx, "-dkms") || hasDep(ctx, "depends", "dkms") {
		return false
	}
	// Split PKGBUILDs (zfs-dkms) declare the -dkms split's depends inside its
	// package_*() function, beyond static reading.
	return !fieldSetInPackageFns(ctx, "depends")
}

func checkDkmsDepends(ctx *Context) []Finding {
	if !dkmsDependsMissing(ctx) {
		return nil
	}
	return []Finding{varFinding(ctx, "PB973", Warn, []string{"depends", "pkgname"},
		"a -dkms package ships module sources that only dkms can build and install; it must depend on dkms")}
}

// dkmsPinnedHeaders returns a -dkms package's pinned kernel-headers depends:
// what PB974 reports, and what its fix deletes.
func dkmsPinnedHeaders(ctx *Context) []depEntry {
	if !nameSuffixed(ctx, "-dkms") {
		return nil
	}
	var out []depEntry
	for _, d := range depEntries(ctx, "depends") {
		// linux-api-headers is glibc's userspace headers, not a kernel headers
		// package.
		if !strings.HasPrefix(d.Name, "linux") || !strings.HasSuffix(d.Name, "-headers") ||
			d.Name == "linux-api-headers" {
			continue
		}
		out = append(out, d)
	}
	return out
}

func checkDkmsKernelHeaders(ctx *Context) []Finding {
	var out []Finding
	for _, d := range dkmsPinnedHeaders(ctx) {
		out = append(out, findingAt("PB974", Warn, ctx.Pkg.PKGBUILD.Path, d.Pos,
			"%q pins one kernel's headers, but dkms already pulls the right headers for every installed kernel; the DKMS package guidelines say to drop it", d.Full))
	}
	return out
}

// --- PB975/PB976: lib32 packages ---------------------------------------------

func checkLib32Pkgdesc(ctx *Context) []Finding {
	if !namePrefixed(ctx, "lib32-") {
		return nil
	}
	desc, ok := pkgdescValue(ctx)
	// "(32-bit, beta version)" carries the marker too.
	if !ok || desc == "" || strings.Contains(desc, "(32-bit") {
		return nil
	}
	return []Finding{varFinding(ctx, "PB975", Info, []string{"pkgdesc"},
		"lib32 package descriptions end with \"(32-bit)\" so the variant is visible everywhere pacman shows the one-line description")}
}

func checkLib32M32(ctx *Context) []Finding {
	if !namePrefixed(ctx, "lib32-") {
		return nil
	}
	// A lib32 package that repackages upstream 32-bit binaries compiles
	// nothing, so only PKGBUILDs with a build() are held to the flag.
	if !ctx.localFuncs[ctx.Pkg.PKGBUILD.Path+"\x00"+"build"] {
		return nil
	}
	// Any -m32 spelling counts — a CFLAGS/CXXFLAGS/LDFLAGS assignment, a
	// `gcc -m32` argument, CC="gcc -m32" — the point is that *something*
	// selects the 32-bit ABI.
	if strings.Contains(string(ctx.Pkg.PKGBUILD.Raw), "-m32") {
		return nil
	}
	return []Finding{varFinding(ctx, "PB976", Warn, []string{"pkgname"},
		"nothing selects the 32-bit ABI: without -m32 in CFLAGS/CXXFLAGS/LDFLAGS the build produces 64-bit objects and the lib32- name is a lie")}
}

// --- PB977/PB978: MinGW cross packages ---------------------------------------

// mingwRequiredOptions come from the MinGW package guidelines: strip and Arch's
// buildflags target the build host's ABI and corrupt or miscompile Windows
// binaries, and Windows links against the static/import libraries.
var mingwRequiredOptions = []string{"!strip", "staticlibs", "!buildflags"}

func checkMingwOptions(ctx *Context) []Finding {
	if !namePrefixed(ctx, "mingw-w64-") {
		return nil
	}
	var missing []string
	for _, opt := range mingwRequiredOptions {
		if !optionSet(ctx, opt) {
			missing = append(missing, opt)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []Finding{varFinding(ctx, "PB977", Warn, []string{"options", "pkgname"},
		"MinGW packages need options=(!strip staticlibs !buildflags) — the host's strip and build flags do not fit Windows binaries; missing: %s", strings.Join(missing, " "))}
}

func checkMingwPkgdesc(ctx *Context) []Finding {
	if !namePrefixed(ctx, "mingw-w64-") {
		return nil
	}
	desc, ok := pkgdescValue(ctx)
	if !ok || desc == "" || strings.HasSuffix(desc, "(mingw-w64)") {
		return nil
	}
	return []Finding{varFinding(ctx, "PB978", Info, []string{"pkgdesc"},
		"MinGW package descriptions end with \"(mingw-w64)\" so the cross target is visible everywhere pacman shows the one-line description")}
}

// --- PB979/PB980: Node.js packages -------------------------------------------

func npmMakedependsGap(ctx *Context) (Command, bool) {
	return toolMakedependsGap(ctx, buildToolCommands(ctx, "npm"), "npm")
}

func checkNpmMakedepends(ctx *Context) []Finding {
	c, ok := npmMakedependsGap(ctx)
	if !ok {
		return nil
	}
	return toolMakedependsFinding("PB979", c, "npm", "npm")
}

// npmUncachedInstalls returns the npm installs still writing the invoking
// user's ~/.npm: the commands PB980 reports, and the ones its fix redirects
// into $srcdir.
func npmUncachedInstalls(ctx *Context) []Command {
	var out []Command
	for _, c := range buildToolCommands(ctx, "npm") {
		switch c.Subcommand() {
		case "install", "i", "ci":
		default:
			continue
		}
		cached := false
		for _, a := range c.Args {
			if a == "--cache" || strings.HasPrefix(a, "--cache=") {
				cached = true
				break
			}
		}
		// npm reads its config from the environment case-insensitively, so an
		// exported (or prefixed) npm_config_cache redirects the cache as well.
		if cached || len(assignmentsTo(ctx, "npm_config_cache", c)) > 0 || len(assignmentsTo(ctx, "NPM_CONFIG_CACHE", c)) > 0 {
			continue
		}
		out = append(out, c)
	}
	return out
}

func checkNpmUserCache(ctx *Context) []Finding {
	var out []Finding
	for _, c := range npmUncachedInstalls(ctx) {
		out = append(out, c.finding("PB980", Info,
			"npm %s writes the invoking user's ~/.npm cache; the Node.js package guidelines pass --cache \"$srcdir/npm-cache\" so the build leaves no droppings outside $srcdir", c.Subcommand()))
	}
	return out
}

// --- PB981: Java without a runtime dependency --------------------------------

// javaRuntimeDepPrefixes are the depends spellings that put a JVM on the
// system: the java-runtime/java-environment virtuals the guidelines ask for,
// or a concrete jre/jdk package that provides them.
var javaRuntimeDepPrefixes = []string{"java-runtime", "java-environment", "jre", "jdk"}

func javaUsed(ctx *Context) bool {
	if len(buildToolCommands(ctx, "java", "javac")) > 0 {
		return true
	}
	for _, w := range packagePhaseWrites(ctx) {
		for _, a := range w.Cmd.Args {
			if strings.HasSuffix(a, ".jar") {
				return true
			}
		}
	}
	return false
}

// javaRuntimeDepMissing reports the gap PB981 names, and the one its fix
// closes by adding java-runtime to depends.
func javaRuntimeDepMissing(ctx *Context) bool {
	if !javaUsed(ctx) {
		return false
	}
	for _, d := range depEntries(ctx, "depends") {
		if hasPrefixAny(d.Name, javaRuntimeDepPrefixes...) {
			return false
		}
	}
	// Split packages (ant, jdk builds) declare depends inside their
	// package_*() functions, where the values are beyond static reading;
	// claiming "no JVM dependency" against half the data would be a guess.
	return !fieldSetInPackageFns(ctx, "depends")
}

func checkJavaRuntimeDependency(ctx *Context) []Finding {
	if !javaRuntimeDepMissing(ctx) {
		return nil
	}
	return []Finding{varFinding(ctx, "PB981", Info, []string{"depends", "pkgname"},
		"the package ships Java artifacts but nothing in depends provides a JVM; the Java package guidelines depend on java-runtime (or java-environment when a JDK is needed)")}
}

// --- PB982: CLR/.NET metadata ------------------------------------------------

func checkCLRMetadata(ctx *Context) []Finding {
	if !hasDep(ctx, "depends", "mono") && len(buildToolCommands(ctx, "msbuild", "xbuild")) == 0 {
		return nil
	}
	var missing []string
	if !archIsAny(ctx) {
		missing = append(missing, "arch=('any')")
	}
	if !optionSet(ctx, "!strip") {
		missing = append(missing, "options=('!strip')")
	}
	if len(missing) == 0 {
		return nil
	}
	return []Finding{varFinding(ctx, "PB982", Info, []string{"arch", "pkgname"},
		"CLR assemblies are architecture-independent CIL that strip corrupts; the CLR package guidelines set %s", strings.Join(missing, " and "))}
}

// --- PB983/PB984: Haskell and PHP architecture -------------------------------

func checkHaskellArch(ctx *Context) []Finding {
	if !namePrefixed(ctx, "haskell-") || !archIsAny(ctx) {
		return nil
	}
	return []Finding{varFinding(ctx, "PB983", Warn, []string{"arch"},
		"GHC compiles every Haskell library to native code, so no haskell- package is arch=('any'); the Haskell package guidelines list the real architectures")}
}

// phpCompileCommands are the commands that mean a php- package builds a
// native extension rather than shipping pure PHP.
var phpCompileCommands = []string{
	"phpize", "configure", "make", "gcc", "cc", "clang", "cmake", "meson",
}

func checkPhpArch(ctx *Context) []Finding {
	if !namePrefixed(ctx, "php-") || archIsAny(ctx) || len(concreteArches(ctx)) == 0 {
		return nil
	}
	if len(buildToolCommands(ctx, phpCompileCommands...)) > 0 {
		return nil // a compiled extension really is architecture-specific
	}
	return []Finding{varFinding(ctx, "PB984", Info, []string{"arch"},
		"nothing here compiles: a pure-PHP package is architecture-independent and should set arch=('any')")}
}
