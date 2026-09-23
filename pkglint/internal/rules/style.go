package rules

import (
	"regexp"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB9xx: PKGBUILD style and distro hygiene. These mirror namcap's stylistic
// PKGBUILD checks: nothing here breaks a build or smuggles code, but each is a
// deviation from Arch packaging conventions that makes a package harder to
// maintain, port, or review. Severities are deliberately modest — Warn for
// things a reviewer would ask to change, Info for pure convention.
var styleRules = []Rule{
	{
		ID:       "PB901",
		Name:     "specific-host-arch",
		Severity: Warn,
		Doc: "Hardcoding an architecture name (x86_64, aarch64, …) in sources, flags or paths " +
			"ties the PKGBUILD to one machine type; $CARCH expands to the architecture being built " +
			"for. Assignments to arch=() and to already arch-suffixed fields (source_x86_64=…) are " +
			"exempt — those are the places an explicit architecture belongs — as are constructs " +
			"that dispatch on $CARCH and names of foreign cross-compilation targets " +
			"(x86_64-w64-mingw32, arm-none-eabi-gcc), which are not the host at all.",
		Check: checkSpecificHostArch,
	},
	{
		ID:       "PB902",
		Name:     "unprefixed-custom-variable",
		Severity: Warn,
		Doc: "Custom variables must start with an underscore (_commit, _pyname) so they can never " +
			"collide with a current or future makepkg field. A bare custom name is silently shipped " +
			"to any tool that parses PKGBUILD metadata and may change meaning under a newer pacman.",
		Check:    checkUnprefixedCustomVars,
		FixLevel: FixUnsafe,
		Fix:      fixUnprefixedCustomVars,
	},
	{
		ID:       "PB903",
		Name:     "startdir-reference",
		Severity: Warn,
		Doc: "$startdir is a makepkg implementation detail from before $srcdir and $pkgdir " +
			"existed. Paths built from it ($startdir/pkg, $startdir/src) break under makepkg's " +
			"BUILDDIR/PKGDEST redirection; use $srcdir and $pkgdir, which are always correct.",
		Check: checkStartdirReference,
	},
	{
		ID:       "PB904",
		Name:     "redundant-makedepends",
		Severity: Warn,
		Doc: "Everything in depends is installed during the build too, so repeating a package in " +
			"makedepends adds nothing and misleads readers about the build's real extra " +
			"requirements. An entry is only kept when its version constraint differs. The " +
			"auto-fix deletes the makedepends entry.",
		Check: checkRedundantMakedepends,
		// The entry provably does nothing: depends already installs the package
		// at build time, and the rule has already exempted any entry whose
		// version constraint asks for something different.
		FixLevel: FixSafe,
		Fix:      fixRedundantMakedepends,
	},
	{
		ID:       "PB905",
		Name:     "sourceforge-mirror-url",
		Severity: Info,
		Doc: "SourceForge URLs pinned to a specific mirror (heanet.dl.sourceforge.net, " +
			"dl.sf.net, …) break when that mirror retires. downloads.sourceforge.net picks a " +
			"working mirror automatically and is the form Arch packaging guidelines ask for.",
		Check: checkSourceforgeMirror,
	},
	{
		ID:       "PB906",
		Name:     "pkgname-in-pkgdesc",
		Severity: Info,
		Doc: "pkgdesc is displayed next to the package name everywhere pacman shows it, so " +
			"repeating the name inside the description wastes the one line that should say what " +
			"the software does.",
		Check: checkPkgnameInDesc,
	},
	{
		ID:       "PB907",
		Name:     "makepkg-internal-function",
		Severity: Warn,
		Doc: "msg, msg2, warning, error and plain are makepkg's private output helpers, not an " +
			"API: they vanish when the PKGBUILD is parsed by anything other than makepkg itself " +
			"and their behavior changes between pacman releases. Use echo or printf.",
		Check: checkMakepkgInternalFunctions,
	},
	{
		ID:       "PB908",
		Name:     "missing-maintainer-comment",
		Severity: Info,
		Doc: "AUR convention puts a '# Maintainer: Name <email>' comment at the top of every " +
			"PKGBUILD so users know who to contact and whether the package is orphaned. Namcap " +
			"checks for the same tag.",
		Check: checkMaintainerComment,
	},
	{
		ID:       "PB909",
		Name:     "uppercase-pkgname",
		Severity: Warn,
		Doc: "Arch package names are lowercase by convention, even when upstream's name is not " +
			"(python, not Python). makepkg accepts uppercase, so this only surfaces in repo " +
			"tooling and user confusion later — rename now, keep the case in pkgdesc.",
		Check: checkUppercasePkgname,
	},
	{
		ID:       "PB910",
		Name:     "missing-metadata",
		Severity: Warn,
		Doc: "pkgdesc, url and license are not required by makepkg but are required by Arch " +
			"packaging standards: without them pacman -Qi shows blanks and the package's legal " +
			"status is undeclared. Split packages may set them per package function instead.",
		Check: checkMissingMetadata,
	},
	{
		ID:       "PB911",
		Name:     "non-unique-source-name",
		Severity: Warn,
		Doc: "A source that downloads to a bare version-numbered file (v1.2.3.tar.gz) collides " +
			"with every other package doing the same in a shared SRCDEST cache, silently reusing " +
			"the wrong tarball. Rename it: \"$pkgname-$pkgver.tar.gz::https://…/v1.2.3.tar.gz\".",
		Check: checkNonUniqueSourceName,
	},
	{
		ID:       "PB912",
		Name:     "duplicated-optdepends",
		Severity: Warn,
		Doc: "A package listed in both depends and optdepends is contradictory metadata: the " +
			"hard dependency always wins, and the optdepends line falsely suggests the feature " +
			"is optional. Keep exactly one of the two. The auto-fix deletes the optdepends entry, " +
			"which is the half with nothing behind it.",
		Check: checkDuplicatedOptdepends,
		// Nothing installed changes: depends already made the package
		// mandatory, and the optdepends line was describing a choice pacman
		// never offered.
		FixLevel: FixSafe,
		Fix:      fixDuplicatedOptdepends,
	},
	{
		ID:       "PB913",
		Name:     "stale-ignore-directive",
		Severity: Warn,
		Doc: "A '# pkglint: ignore=' directive that matches no finding on its own or the next " +
			"line — the issue was fixed, or the ID was never a pkglint rule — no longer documents " +
			"a reviewed exception. It misleads reviewers into thinking a finding exists there, and " +
			"if the flagged construct ever comes back, the leftover directive silences it without " +
			"anyone re-reviewing. The auto-fix removes the stale IDs, or the whole comment once " +
			"none remain.",
		Check:    checkStaleIgnores,
		FixLevel: FixSafe,
		Fix:      fixStaleIgnores,
	},
	{
		ID:       "PB914",
		Name:     "go-no-pie",
		Severity: Warn,
		Doc: "The Arch Go packaging guidelines build with -buildmode=pie: without it the Go " +
			"toolchain links a position-dependent executable that ASLR cannot relocate, which " +
			"PB806 flags on the built package. Set it in GOFLAGS or on the go build command; a " +
			"PKGBUILD that chooses another -buildmode explicitly is left alone.",
		Check: checkGoPIE,
		// Changing the link layout of the shipped binary is behavior a human
		// should sign off on, like the other build-command rewrites.
		FixLevel: FixUnsafe,
		Fix:      fixGoPIE,
	},
	{
		ID:       "PB915",
		Name:     "go-no-trimpath",
		Severity: Warn,
		Doc: "Without -trimpath, go embeds absolute $srcdir paths in the binary, so two builders " +
			"never produce the same bytes and the package leaks its build layout. The Arch Go " +
			"packaging guidelines put -trimpath in GOFLAGS on every build.",
		Check:    checkGoTrimpath,
		FixLevel: FixUnsafe,
		Fix:      fixGoTrimpath,
	},
	{
		ID:       "PB916",
		Name:     "go-readonly-modcache",
		Severity: Info,
		Doc: "go writes its module cache read-only, so after a build that fetched modules, " +
			"cleaning $srcdir (or $GOPATH under it) fails until someone chmods the tree. The Arch " +
			"Go packaging guidelines pass -modcacherw on every module-aware command to keep the " +
			"cache removable.",
		Check: checkGoModcacheRW,
		// -modcacherw only loosens permissions on cache files; it cannot
		// change what gets built, so the rewrite is safe.
		FixLevel: FixSafe,
		Fix:      fixGoModcacheRW,
	},
	{
		ID:       "PB917",
		Name:     "go-cgo-flags-dropped",
		Severity: Info,
		Doc: "cgo compiles C with CGO_CFLAGS/CGO_LDFLAGS, not the CFLAGS/LDFLAGS makepkg exports, " +
			"so unless the PKGBUILD forwards them (export CGO_CFLAGS=\"$CFLAGS\", …) Arch's " +
			"fortify/RELRO hardening never reaches the C parts of a Go build. Pure-Go packages " +
			"can set CGO_ENABLED=0 instead, which also silences this rule. The fix writes the " +
			"guidelines' block of CGO_* exports at the top of the function that runs the build.",
		Check: checkGoCgoFlags,
		// Forwarding the flags recompiles the C parts with Arch's fortify and
		// RELRO settings, which is a different compile: code that only built
		// without them now fails, and that failure is a human's to read.
		FixLevel: FixUnsafe,
		Fix:      fixGoCgoFlags,
	},
	{
		ID:       "PB918",
		Name:     "self-provides",
		Severity: Warn,
		Doc: "Every package implicitly provides its own name, so listing $pkgname in provides is " +
			"dead metadata — at best ignored, at worst masking a typo of the capability the entry " +
			"was meant to declare. The Arch package guidelines say provides is for other " +
			"capabilities the package supplies (a library soname, a renamed predecessor). The " +
			"auto-fix deletes the entry, and the whole array with it when nothing else is in it.",
		Check: checkSelfProvides,
		// pacman derives the implicit self-provide from pkgname either way, so
		// deleting the entry cannot change what the package satisfies.
		FixLevel: FixSafe,
		Fix:      fixSelfProvides,
	},
	{
		ID:       "PB919",
		Name:     "self-conflicts",
		Severity: Warn,
		Doc: "A package can never conflict with itself — pacman resolves same-name conflicts by " +
			"upgrading — so listing $pkgname in conflicts is dead metadata that misleads readers " +
			"about what the package actually displaces. The Arch package guidelines say to drop " +
			"it, and the auto-fix does — along with the whole array when nothing else is in it.",
		Check: checkSelfConflicts,
		// pacman resolves a same-name conflict by upgrading whatever the array
		// says, so the entry has no effect to preserve.
		FixLevel: FixSafe,
		Fix:      fixSelfConflicts,
	},
	{
		ID:       "PB920",
		Name:     "non-spdx-license",
		Severity: Info,
		Doc: "Arch migrated the license field to SPDX expressions (RFC 16): 'GPL-2.0-or-later' " +
			"says which GPL and whether later versions qualify, where the legacy 'GPL' said " +
			"neither, and license-audit tooling parses only the SPDX form. Only known-legacy " +
			"spellings are flagged, so a valid-but-uncommon SPDX identifier never is.",
		Check: checkNonSPDXLicense,
	},
	{
		ID:       "PB921",
		Name:     "usr-local-install",
		Severity: Warn,
		Doc: "/usr/local is reserved for software the administrator builds outside the package " +
			"manager; the Arch package guidelines forbid packages from touching it. This is the " +
			"PKGBUILD-side complement of PB820, which flags the same files in the built archive. " +
			"Install under /usr (usually /usr/bin, /usr/lib, /usr/share) instead.",
		Check: checkUsrLocalInstall,
	},
	{
		ID:       "PB922",
		Name:     "usr-libexec-install",
		Severity: Info,
		Doc: "Arch does not use /usr/libexec: the package guidelines put internal executables " +
			"that users should not invoke directly in /usr/lib/$pkgname instead. Upstream build " +
			"systems often default libexecdir to /usr/libexec, so the fix is usually " +
			"--libexecdir=/usr/lib on configure rather than moving files by hand.",
		Check: checkUsrLibexecInstall,
	},
	{
		ID:       "PB930",
		Name:     "python-tox",
		Severity: Warn,
		Doc: "tox exists to test against multiple Python versions by building fresh virtualenvs " +
			"and re-resolving dependencies from PyPI — the opposite of checking that *this* " +
			"package works against *this* system's packages, and a network dependency inside " +
			"check(). The Arch Python package guidelines forbid it; invoke pytest (or upstream's " +
			"runner) directly.",
		Check: checkPythonTox,
	},
	{
		ID:       "PB931",
		Name:     "python-lint-checkdepends",
		Severity: Info,
		Doc: "pytest plugins like pytest-cov, pytest-black or pytest-mypy lint upstream's code " +
			"style or measure coverage; neither says anything about whether the built package " +
			"works, and a new linter release starts failing builds that did not change. The Arch " +
			"Python package guidelines say check() runs tests, not upstream's CI. The auto-fix " +
			"deletes the entry, unless a command in the file still passes the plugin's flag.",
		Check: checkPythonLintCheckdepends,
		// Dropping a declared dependency is a real change to the build
		// environment: check() runs with one less package installed.
		FixLevel: FixUnsafe,
		Fix:      fixPythonLintCheckdepends,
	},
	{
		ID:       "PB932",
		Name:     "python-prebuilt-wheel",
		Severity: Warn,
		Doc: "A .whl source is a pre-built artifact: what lands in the package was compiled by " +
			"whoever uploaded the wheel, not by this PKGBUILD, so nothing reviewed here is what " +
			"ships. The Arch Python package guidelines require building from the source " +
			"distribution except where no sdist exists at all.",
		Check: checkPythonWheelSource,
	},
	{
		ID:       "PB933",
		Name:     "python-missing-build-backend",
		Severity: Warn,
		Doc: "`python -m build` and `python -m installer` come from the python-build and " +
			"python-installer packages, which makepkg does not install for you: without them in " +
			"makedepends the build fails in any clean chroot, however reliably it works on the " +
			"maintainer's machine. The auto-fix adds the missing package to makedepends.",
		Check:    checkPythonBuildBackend,
		FixLevel: FixSafe,
		Fix:      fixPythonBuildBackend,
	},
	{
		ID:       "PB934",
		Name:     "python-setup-py",
		Severity: Warn,
		Doc: "`python setup.py install` runs the distutils flow that setuptools has removed: it " +
			"bypasses PEP 517 build isolation, writes egg-info metadata pip cannot manage, and " +
			"breaks outright on modern setuptools. The Arch Python package guidelines build with " +
			"`python -m build` and stage with `python -m installer`.",
		Check: checkPythonSetupPy,
	},
	{
		ID:       "PB940",
		Name:     "cargo-check-release",
		Severity: Warn,
		Doc: "--release on cargo test/check (or its short spelling, -r) disables debug assertions " +
			"and integer-overflow checks — precisely the invariants a test run exists to " +
			"exercise. The Arch Rust package guidelines run check() without --release; the " +
			"shipped binary is still built --release in build(). A --release kept in a variable " +
			"counts as passed wherever the PKGBUILD assigns it, though only one written out in " +
			"the command itself can be removed automatically.",
		Check: checkCargoCheckRelease,
		// Dropping the flag recompiles the crate in the dev profile: the check
		// gets slower, and a suite that only passed with assertions compiled
		// out now fails. That failure is the point, but it is a human's call.
		FixLevel: FixUnsafe,
		Fix:      fixCargoCheckRelease,
	},
	{
		ID:       "PB941",
		Name:     "cargo-install-tracked",
		Severity: Warn,
		Doc: "cargo install records what it installed in .crates.toml and .crates2.json under the " +
			"install root; staged into $pkgdir those files ship in the package, conflict with " +
			"every other Rust package doing the same, and describe a cargo state directory that " +
			"does not exist on the user's system. The Rust package guidelines pass --no-track. " +
			"Flags kept in a variable count as passed wherever the PKGBUILD assigns it; an " +
			"install whose flags come from outside the file is left alone, since --no-track may " +
			"be in there. The auto-fix appends --no-track, before any -- separator the command has.",
		Check: checkCargoInstallTracked,
		// --no-track changes what cargo install leaves in the install root, so
		// a package() that went on to delete or rewrite the tracking files
		// itself is now operating on paths that are not there.
		FixLevel: FixUnsafe,
		Fix:      fixCargoInstallTracked,
	},
	{
		ID:       "PB942",
		Name:     "cargo-build-not-release",
		Severity: Info,
		Doc: "cargo builds the unoptimized, debug-assertion-laden dev profile unless told " +
			"otherwise, so a plain `cargo build` in build() ships a slow binary. The Arch Rust " +
			"package guidelines build with --release (-r is the same flag); a PKGBUILD that " +
			"selects another profile explicitly (--profile) has made the call, as has one whose " +
			"build() runs a release build beside the dev one, and both are left alone. Flags kept " +
			"in a variable count as passed wherever the PKGBUILD assigns it; a build whose flags " +
			"come from outside the file is left alone too, since --release may be in there.",
		Check: checkCargoBuildRelease,
		// The release profile is a different compile — slower, and a crate that
		// only builds with debug assertions on now fails — and it moves the
		// binary from target/debug to target/release, so anything that copies
		// the artifact has to be checked against the new path.
		FixLevel: FixUnsafe,
		Fix:      fixCargoBuildRelease,
	},
	{
		ID:       "PB944",
		Name:     "rust-missing-makedepends",
		Severity: Info,
		Doc: "cargo is not part of the base build environment: without rust (or rustup) in " +
			"makedepends the build fails in any clean chroot. rustup satisfies the dependency " +
			"for maintainers who manage toolchains themselves, so either spelling counts. The " +
			"auto-fix declares rust.",
		Check: checkRustMakedepends,
		// Declaring a tool the build already invokes restores a requirement the
		// clean chroot was going to enforce anyway: a package that could not
		// build before can build after, and nothing else moves.
		FixLevel: FixSafe,
		Fix:      fixToolMakedepends(rustMakedependsGap, "cargo", "rust"),
	},
	{
		ID:       "PB950",
		Name:     "cmake-missing-prefix",
		Severity: Warn,
		Doc: "CMake's install prefix defaults to /usr/local, which Arch packages must not touch " +
			"(PB921/PB820 flag the resulting files); the CMake package guidelines configure with " +
			"-DCMAKE_INSTALL_PREFIX=/usr. A PKGBUILD that sets another prefix explicitly (/opt " +
			"for a self-contained tree) has made a decision and is left alone.",
		Check: checkCMakePrefix,
		// The prefix moves every installed file, so a package() that copies
		// out of the build tree by hardcoded path has to be checked against
		// the new layout.
		FixLevel: FixUnsafe,
		Fix:      fixCMakePrefix,
	},
	{
		ID:       "PB951",
		Name:     "cmake-build-type-release",
		Severity: Warn,
		Doc: "-DCMAKE_BUILD_TYPE=Release appends -O3 -DNDEBUG after the flags makepkg exports, " +
			"silently overriding Arch's chosen -O2 and fortify settings. The CMake package " +
			"guidelines build with -DCMAKE_BUILD_TYPE=None so the distribution's CFLAGS are what " +
			"actually compile the code. The auto-fix rewrites the value to None, leaving the rest " +
			"of the argument — a -D prefix, a :STRING type, the quoting — as written.",
		Check: checkCMakeBuildType,
		// None is a different compile: -O2 without -DNDEBUG keeps assertions
		// in, so a project that only passes its tests with them compiled out
		// now fails. That failure is the point, but it is a human's call.
		FixLevel: FixUnsafe,
		Fix:      fixCMakeBuildType,
	},
	{
		ID:       "PB952",
		Name:     "cmake-missing-makedepends",
		Severity: Warn,
		Doc: "cmake is not part of the base build environment: without it in makedepends the " +
			"build fails in any clean chroot, however reliably it configures on the maintainer's " +
			"machine. The auto-fix declares cmake.",
		Check: checkCMakeMakedepends,
		// As with PB944: restoring a build requirement, not changing the build.
		FixLevel: FixSafe,
		Fix:      fixToolMakedepends(cmakeMakedependsGap, "cmake", "cmake"),
	},
	{
		ID:       "PB953",
		Name:     "meson-missing-prefix",
		Severity: Warn,
		Doc: "Meson's install prefix defaults to /usr/local, which Arch packages must not touch; " +
			"the Meson package guidelines configure with --prefix=/usr, or use arch-meson, which " +
			"passes the distribution defaults for you.",
		Check: checkMesonPrefix,
		// As with PB950, the prefix moves every installed file.
		FixLevel: FixUnsafe,
		Fix:      fixMesonPrefix,
	},
	{
		ID:       "PB954",
		Name:     "meson-missing-makedepends",
		Severity: Warn,
		Doc: "meson (and the arch-meson wrapper it ships) is not part of the base build " +
			"environment: without meson in makedepends the build fails in any clean chroot. The " +
			"auto-fix declares meson, which is also what ships arch-meson.",
		Check: checkMesonMakedepends,
		// As with PB944: restoring a build requirement, not changing the build.
		FixLevel: FixSafe,
		Fix:      fixToolMakedepends(mesonMakedependsGap, "meson", "meson"),
	},
	{
		ID:       "PB955",
		Name:     "meson-ninja-direct",
		Severity: Info,
		Doc: "In a meson project, `meson compile -C build` wraps ninja with the environment and " +
			"argument handling meson configured; calling ninja directly bypasses that and breaks " +
			"quietly when the backend or its options change. The Meson package guidelines use the " +
			"meson wrappers for compile, test and install.",
		Check: checkMesonNinjaDirect,
	},
	{
		ID:       "PB960",
		Name:     "vcs-missing-pkgver-fn",
		Severity: Warn,
		Doc: "A VCS source that follows upstream tip fetches new code on every build, but without " +
			"a pkgver() function the package version is whatever literal pkgver= holds — so two " +
			"different checkouts build 'the same' version and pacman never offers the upgrade. " +
			"The VCS package guidelines derive pkgver from the checkout (git describe, revision " +
			"counts).",
		Check: checkVCSPkgverFn,
	},
	{
		ID:       "PB961",
		Name:     "vcs-missing-provides-conflicts",
		Severity: Info,
		Doc: "A -git (or -svn, -hg, -bzr) package builds the same software as its release " +
			"counterpart: without provides and conflicts on the base name, pacman happily " +
			"installs both at once and dependencies on the release name are unsatisfiable by the " +
			"VCS build. The VCS package guidelines declare both. The auto-fix declares the pair " +
			"together — either one alone is worse than the finding.",
		Check: checkVCSProvidesConflicts,
		// Both fields are installed metadata pacman acts on: after the fix the
		// release package can no longer be installed alongside, and dependencies
		// on it resolve to this build instead.
		FixLevel: FixUnsafe,
		Fix:      fixVCSProvidesConflicts,
	},
	{
		ID:       "PB962",
		Name:     "vcs-pkgver-in-folder",
		Severity: Warn,
		Doc: "makepkg keeps one persistent clone per checkout folder name and updates it " +
			"incrementally. With $pkgver in the name:: prefix of a VCS source, pkgver() renames " +
			"the folder on every version bump, so each build abandons the cache and re-clones the " +
			"entire repository.",
		Check: checkVCSPkgverInFolder,
	},
	{
		ID:       "PB963",
		Name:     "vcs-suffix-mismatch",
		Severity: Info,
		Doc: "The VCS suffixes are a promise about what gets built: a -git package follows " +
			"upstream's tip. A -git name with no git source builds something else under that " +
			"promise, and a git source pinned to a fixed commit or tag is a release snapshot " +
			"wearing a -git name — either way users and AUR helpers are misled about what " +
			"updates mean.",
		Check: checkVCSSuffixMismatch,
	},
	{
		ID:       "PB970",
		Name:     "font-package-arch",
		Severity: Warn,
		Doc: "Font files render identically on every machine type, so the Arch font package " +
			"guidelines require arch=('any') on ttf-/otf- packages. A concrete architecture " +
			"makes pacman rebuild and re-download the same bytes per architecture for nothing.",
		Check: checkFontArch,
	},
	{
		ID:       "PB971",
		Name:     "font-package-depends",
		Severity: Info,
		Doc: "Installed fonts are discovered by fontconfig on its own; a font package needs no " +
			"dependencies, and the historical fontconfig/xorg-font-utils entries only force " +
			"unrelated software onto minimal systems. The font package guidelines say to declare " +
			"none. The auto-fix drops them.",
		Check: checkFontDepends,
		// Dropping a declared dependency is a real change to what gets
		// installed: a font package that was quietly pulling in a renderer for
		// its .install scriptlet stops doing so.
		FixLevel: FixUnsafe,
		Fix:      fixFontDepends,
	},
	{
		ID:       "PB972",
		Name:     "font-unstable-source",
		Severity: Warn,
		Doc: "fonts.google.com and fontspace.com serve archives generated on demand from the " +
			"font's current revision: the bytes change without the URL changing, so the pinned " +
			"checksum eventually fails on every user's machine at once. Fetch the versioned " +
			"release artifact from the font's own repository instead.",
		Check: checkFontUnstableSource,
	},
	{
		ID:       "PB973",
		Name:     "dkms-missing-depends",
		Severity: Warn,
		Doc: "A -dkms package installs module sources under /usr/src that only dkms can build, " +
			"register and rebuild across kernel upgrades; without dkms in depends the package " +
			"installs sources nothing will ever compile. The auto-fix declares dkms.",
		Check: checkDkmsDepends,
		// depends is installed-runtime metadata, not the build environment:
		// after the fix pacman pulls dkms onto every user's machine.
		FixLevel: FixUnsafe,
		Fix:      fixDkmsDepends,
	},
	{
		ID:       "PB974",
		Name:     "dkms-kernel-headers",
		Severity: Warn,
		Doc: "dkms discovers every installed kernel and pulls the matching headers itself, so a " +
			"-dkms package depending on linux-headers (or any kernel's -headers) pins one kernel " +
			"flavor: it drags the stock headers onto linux-lts systems and adds nothing on any " +
			"other. The DKMS package guidelines say to leave headers to dkms. The auto-fix drops " +
			"the pinned entry.",
		Check: checkDkmsKernelHeaders,
		// A package that really did need one kernel's headers at install time —
		// an .install scriptlet building against them, say — loses them.
		FixLevel: FixUnsafe,
		Fix:      fixDkmsKernelHeaders,
	},
	{
		ID:       "PB975",
		Name:     "lib32-pkgdesc-suffix",
		Severity: Info,
		Doc: "A lib32- package's one-line description is shown in lists where only it " +
			"distinguishes the multilib build from its 64-bit sibling; the 32-bit package " +
			"guidelines end pkgdesc with \"(32-bit)\".",
		Check: checkLib32Pkgdesc,
	},
	{
		ID:       "PB976",
		Name:     "lib32-missing-m32",
		Severity: Warn,
		Doc: "On x86_64 nothing compiles 32-bit unless the PKGBUILD asks: without -m32 in " +
			"CFLAGS/CXXFLAGS/LDFLAGS (or an equivalent CC spelling) the build emits 64-bit " +
			"objects and the lib32- package ships the wrong ABI. Repackaging PKGBUILDs with no " +
			"build() are exempt — they compile nothing.",
		Check: checkLib32M32,
	},
	{
		ID:       "PB977",
		Name:     "mingw-required-options",
		Severity: Warn,
		Doc: "Cross-built Windows binaries must escape the host's toolchain defaults: the build " +
			"host's strip corrupts PE binaries (mingw-strip differs), Windows linking needs the " +
			"static/import libraries makepkg would delete, and Arch's CFLAGS target the host ABI. " +
			"The MinGW package guidelines therefore require options=(!strip staticlibs !buildflags).",
		Check: checkMingwOptions,
	},
	{
		ID:       "PB978",
		Name:     "mingw-pkgdesc-suffix",
		Severity: Info,
		Doc: "A mingw-w64- package's description is shown in lists where only it distinguishes " +
			"the cross build from the native one; the MinGW package guidelines end pkgdesc with " +
			"\"(mingw-w64)\".",
		Check: checkMingwPkgdesc,
	},
	{
		ID:       "PB979",
		Name:     "npm-missing-makedepends",
		Severity: Warn,
		Doc: "npm is not part of the base build environment (and not bundled with the nodejs " +
			"package): without npm in makedepends the build fails in any clean chroot. The " +
			"auto-fix declares npm.",
		Check: checkNpmMakedepends,
		// As with PB944: restoring a build requirement, not changing the build.
		FixLevel: FixSafe,
		Fix:      fixToolMakedepends(npmMakedependsGap, "npm", "npm"),
	},
	{
		ID:       "PB980",
		Name:     "npm-user-cache",
		Severity: Info,
		Doc: "npm writes every downloaded tarball into the invoking user's ~/.npm, so a build " +
			"leaves root-owned droppings in the builder's home directory that no clean step " +
			"removes. The Node.js package guidelines pass --cache \"$srcdir/npm-cache\" so the " +
			"cache dies with the build directory. The auto-fix appends that flag to the npm " +
			"command.",
		Check: checkNpmUserCache,
		// A fresh cache directory means the install refetches everything, so
		// a build that quietly relied on a warm ~/.npm — or on running with no
		// network at all — behaves differently afterwards.
		FixLevel: FixUnsafe,
		Fix:      fixNpmUserCache,
	},
	{
		ID:       "PB981",
		Name:     "java-runtime-dependency",
		Severity: Info,
		Doc: "Shipped .jar files and compiled classes run on nothing without a JVM, and pacman " +
			"cannot know that from the file list. The Java package guidelines depend on the " +
			"java-runtime virtual (or java-environment when a full JDK is needed) so any " +
			"installed JVM satisfies it. The auto-fix declares java-runtime.",
		Check: checkJavaRuntimeDependency,
		// depends is installed-runtime metadata: after the fix pacman pulls a
		// JVM onto every user's machine, and a package that in fact needs a full
		// JDK wants java-environment instead — a distinction the file does not
		// state.
		FixLevel: FixUnsafe,
		Fix:      fixJavaRuntimeDependency,
	},
	{
		ID:       "PB982",
		Name:     "clr-required-metadata",
		Severity: Info,
		Doc: "CLR assemblies are CIL bytecode, identical on every architecture and structured in " +
			"a way binutils strip corrupts. The CLR package guidelines therefore set arch=('any') " +
			"and options=('!strip') on mono/.NET packages; a package missing either ships " +
			"per-arch duplicates or broken assemblies.",
		Check: checkCLRMetadata,
	},
	{
		ID:       "PB983",
		Name:     "haskell-arch-any",
		Severity: Warn,
		Doc: "GHC compiles Haskell libraries to native code with an ABI hash baked into every " +
			"interface file, so no haskell- package is architecture-independent: arch=('any') " +
			"ships x86_64 objects to every architecture. The Haskell package guidelines list the " +
			"real architectures.",
		Check: checkHaskellArch,
	},
	{
		ID:       "PB984",
		Name:     "php-arch-any",
		Severity: Info,
		Doc: "A pure-PHP package — one whose PKGBUILD compiles nothing — is architecture-" +
			"independent and should declare arch=('any') so one build serves every machine. " +
			"php- packages that build a native extension (phpize/configure/make) are " +
			"architecture-specific and exempt.",
		Check: checkPhpArch,
	},
}

// --- PB901: hardcoded host architecture ------------------------------------

// foreignTokenRe matches an architecture name that is part of a longer
// cross-compilation target triple or a foreign platform identifier —
// `x86_64-w64-mingw32`, `arm-none-eabi-gcc`, `/usr/lib/x86_64-linux-gnu`,
// `android-aarch64-qt6-base`. Those name a target the build produces or
// consumes, not the machine it runs on; substituting $CARCH would break the
// build outright.
//
// The suffixes are anchored directly to the architecture name so that a full
// native triple is still reported: `x86_64-pc-linux-gnu` and
// `x86_64-unknown-linux-gnu` name the host and do want $CARCH, while Debian's
// two-component multiarch directory `x86_64-linux-gnu` does not.
var foreignTokenRe = regexp.MustCompile(
	`(?:^|[^a-zA-Z0-9_])(?:android|macos|mac|gcc)-` + hostArchAlt +
		`|` + hostArchAlt + `-(?:w64-mingw32|none-eabi|esp-elf|elf-|linux-gnu\b` +
		`|linux-gnueabi|linux-android|apple-darwin|pc-windows|softmmu|unknown-none)`)

// hostArchAlt is the bare alternation of architecture names, shared by
// foreignTokenRe and the specification regexp in hostarch_test.go.
// TestHostArchRegexpCoversNames keeps it in agreement with hostArchNames.
const hostArchAlt = `(?:i686|x86_64|aarch64|armv6h|armv7h|arm|riscv64|loong64)`

// hostArchSet is hostArchNames as a set, for the whole-word test below.
var hostArchSet = func() map[string]bool {
	m := make(map[string]bool, len(hostArchNames))
	for _, a := range hostArchNames {
		m[a] = true
	}
	return m
}()

// hostArchIn returns the leftmost architecture name appearing in s as a whole
// word, or "" if there is none.
//
// This is the regexp `\b` + hostArchAlt + `\b` the rule used to run, scanned
// by hand. Every alternative is a run of word characters delimited by \b,
// which is exactly "some maximal word equals one of these", so walking the
// word runs decides the same thing without the backtracking. It is worth the
// few lines because the rule tests every literal in the file and almost none
// of them mention an architecture: the regexp made this the most expensive
// check in the registry. TestHostArchInMatchesRegexp holds the two in
// agreement.
func hostArchIn(s string) string {
	for i := 0; i < len(s); {
		if !isWordByte(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && isWordByte(s[j]) {
			j++
		}
		if word := s[i:j]; hostArchSet[word] {
			return word
		}
		i = j
	}
	return ""
}

// isWordByte reports whether b is a word character in the ASCII sense Go's
// regexp \b uses. A multi-byte rune is never one, and each of its bytes has the
// high bit set, so treating it as a non-word byte splits words the same way.
func isWordByte(b byte) bool {
	return b == '_' || ('0' <= b && b <= '9') || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

// checkSpecificHostArch mirrors namcap's carch rule on the AST: any literal
// mentioning a concrete architecture is flagged unless it is (part of) an
// arch=() assignment, an assignment to an arch-suffixed field, part of a
// foreign target triple, or inside a construct that already dispatches on
// $CARCH (the idiom the rule is nudging toward).
func checkSpecificHostArch(ctx *Context) []Finding {
	u := ctx.Pkg.PKGBUILD

	// Lines exempt from the check.
	skip := map[uint]bool{}
	markSpan := func(from, to syntax.Pos) {
		for l := from.Line(); l <= to.Line(); l++ {
			skip[l] = true
		}
	}
	// One walk does every job: it marks the exempt spans and collects the
	// literals that name an architecture. The candidates are filtered after
	// the walk rather than as they are found, because an exemption can come
	// later in the file than the literal it covers (a $CARCH later on the same
	// line), so nothing can be decided until the walk is done.
	//
	// Exempt assignments are found by walking rather than reading ctx.Pkg.Vars:
	// Vars holds only the last top-level assignment of a name, so an arch=()
	// inside a package_*() split function — or an earlier one shadowed by a
	// later assignment — would otherwise be flagged for declaring exactly what
	// it is meant to declare.
	//
	// $CARCH anywhere on a line exempts that line, and a `case $CARCH`/`if
	// [[ $CARCH == … ]]` exempts its whole body: the per-architecture literals
	// inside such a construct *are* the portable idiom, and they necessarily
	// sit on lines below the one naming $CARCH.
	type candidate struct {
		arch string
		pos  syntax.Pos
	}
	var found []candidate
	note := func(val string, pos syntax.Pos) {
		if m := hostArchIn(val); m != "" && !foreignTokenRe.MatchString(val) {
			found = append(found, candidate{m, pos})
		}
	}
	syntax.Walk(u.File, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.Assign:
			if x.Name != nil && (strings.HasSuffix(x.Name.Value, "arch") || archSuffixed(x.Name.Value)) {
				markSpan(x.Pos(), x.End())
			}
		case *syntax.ParamExp:
			if x.Param != nil && x.Param.Value == "CARCH" {
				skip[x.Pos().Line()] = true
			}
		case *syntax.CaseClause:
			// x.Word is nil-checked here: an interface holding a typed nil
			// would slip past a nil check inside dispatchesOnArch.
			if x.Word != nil && dispatchesOnArch(x.Word) {
				markSpan(x.Pos(), x.End())
			}
		case *syntax.IfClause:
			for _, st := range x.Cond {
				if dispatchesOnArch(st) {
					markSpan(x.Pos(), x.End())
					break
				}
			}
		case *syntax.Lit:
			note(x.Value, x.Pos())
		case *syntax.SglQuoted:
			note(x.Value, x.Pos())
		}
		return true
	})

	var out []Finding
	type key struct {
		line uint
		arch string
	}
	seen := map[key]bool{}
	for _, c := range found {
		if skip[c.pos.Line()] || seen[key{c.pos.Line(), c.arch}] {
			continue
		}
		seen[key{c.pos.Line(), c.arch}] = true
		out = append(out, findingAt("PB901", Warn, u.Path, c.pos,
			"architecture %q is hardcoded; use $CARCH so the PKGBUILD ports to other architectures", c.arch))
	}
	return out
}

// dispatchesOnArch reports whether n selects on the architecture being built
// for — `case "$CARCH"`, `if [[ $CARCH == … ]]`, `case $(uname -m)`. The
// per-architecture literals in such a construct are the portable idiom, not a
// hardcoded host.
func dispatchesOnArch(n syntax.Node) bool {
	found := false
	syntax.Walk(n, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.ParamExp:
			if x.Param != nil && x.Param.Value == "CARCH" {
				found = true
			}
		case *syntax.Lit:
			if x.Value == "uname" {
				found = true
			}
		}
		return !found
	})
	return found
}

// hostArchNames are the architectures PB901 recognizes — the names makepkg can
// actually set $CARCH to. i386/i486/i586/ppc are deliberately absent: Arch and
// Arch Linux ARM never use them, so a literal `i386` is a foreign platform
// name and "use $CARCH" is never the fix for it.
var hostArchNames = []string{
	"i686", "x86_64", "aarch64", "armv6h", "armv7h", "arm", "riscv64", "loong64",
}

// archSuffixed reports whether name is a schema field suffixed with an
// architecture (source_x86_64, depends_aarch64, …). The arch names themselves
// contain underscores, so this is a suffix test, not a split on '_'.
func archSuffixed(name string) bool {
	for _, a := range hostArchNames {
		if strings.HasSuffix(name, "_"+a) {
			return true
		}
	}
	return false
}

// --- PB902: custom variables without an underscore prefix -------------------

// knownPKGBUILDVars is every variable name makepkg itself reads, i.e. the ones
// a PKGBUILD may set without an underscore prefix.
func knownPKGBUILDVars(ctx *Context) map[string]bool {
	known := map[string]bool{"pkgname": true, "pkgbase": true}
	for _, n := range schemaArrayVars {
		known[n] = true
	}
	for _, n := range schemaStringVars {
		known[n] = true
	}
	for _, a := range concreteArches(ctx) {
		for _, n := range schemaArchArrayVars {
			known[n+"_"+a] = true
		}
	}
	return known
}

// unprefixedCustomVars returns the variables PB902 flags, keyed by name. The
// fixer shares it so what gets renamed is exactly what got reported.
func unprefixedCustomVars(ctx *Context) map[string]*pkgbuild.Var {
	known := knownPKGBUILDVars(ctx)
	out := map[string]*pkgbuild.Var{}
	for name, v := range ctx.Pkg.Vars {
		if known[name] || strings.HasPrefix(name, "_") {
			continue
		}
		// All-uppercase and mixed-case names are either makepkg.conf overrides
		// (PB108's concern) or at least visually distinct; namcap only flags
		// all-lowercase names, and so do we.
		if name != strings.ToLower(name) {
			continue
		}
		out[name] = v
	}
	return out
}

func checkUnprefixedCustomVars(ctx *Context) []Finding {
	var out []Finding
	for name, v := range unprefixedCustomVars(ctx) {
		out = append(out, findingAt("PB902", Warn, ctx.Pkg.PKGBUILD.Path, v.Pos,
			"custom variable %q should be prefixed with an underscore (_%s) to avoid clashing with makepkg fields",
			name, name))
	}
	return out
}

// --- PB903: $startdir --------------------------------------------------------

func checkStartdirReference(ctx *Context) []Finding {
	u := ctx.Pkg.PKGBUILD
	var out []Finding
	syntax.Walk(u.File, func(node syntax.Node) bool {
		pe, ok := node.(*syntax.ParamExp)
		if !ok || pe.Param == nil || pe.Param.Value != "startdir" {
			return true
		}
		// Peek at what follows in the raw text: "$startdir/pkg" and
		// "$startdir/src" have exact replacements, anything else is a generic
		// deprecation. An intervening closing quote ("$startdir"/pkg) is
		// skipped, as namcap does.
		msg := "$startdir is deprecated; use $srcdir and $pkgdir instead"
		tail := u.Raw[min(int(pe.End().Offset()), len(u.Raw)):]
		if len(tail) > 0 && tail[0] == '"' {
			tail = tail[1:]
		}
		switch {
		case hasPrefixAny(string(tail), "/pkg"):
			msg = "use $pkgdir instead of $startdir/pkg"
		case hasPrefixAny(string(tail), "/src"):
			msg = "use $srcdir instead of $startdir/src"
		}
		out = append(out, findingAt("PB903", Warn, u.Path, pe.Pos(), "%s", msg))
		return true
	})
	return out
}

// --- PB904: makedepends already in depends ----------------------------------

// depName strips the version constraint from a dependency entry:
// "foo>=1.2" -> "foo". optdepends-style ": description" suffixes are cut too.
func depName(entry string) string {
	if i := strings.IndexByte(entry, ':'); i >= 0 {
		entry = entry[:i]
	}
	if i := strings.IndexAny(entry, "<>="); i >= 0 {
		entry = entry[:i]
	}
	return strings.TrimSpace(entry)
}

// depsFor collects the statically-known entries of field and its declared
// _$arch variants, mapping name -> full entry as written.
func depsFor(ctx *Context, field string) map[string]string {
	out := map[string]string{}
	names := []string{field}
	for _, a := range concreteArches(ctx) {
		names = append(names, field+"_"+a)
	}
	for _, n := range names {
		for _, e := range varElems(ctx.Pkg.Vars[n]) {
			if val, ok := staticVal(ctx.Pkg, e.Value); ok && val != "" {
				out[depName(val)] = val
			}
		}
	}
	return out
}

// redundantMakedepends returns the makedepends entries depends already
// covers: the entries PB904 reports, and the ones its fix deletes.
func redundantMakedepends(ctx *Context) []depEntry {
	depends := depsFor(ctx, "depends")
	if len(depends) == 0 {
		return nil
	}
	var out []depEntry
	for _, d := range depEntries(ctx, "makedepends") {
		full, dup := depends[d.Name]
		if !dup {
			continue
		}
		// A makedepends entry with its own, different version constraint
		// is a deliberate build-time requirement; leave it alone.
		if d.Name != d.Full && full != d.Full {
			continue
		}
		out = append(out, d)
	}
	return out
}

func checkRedundantMakedepends(ctx *Context) []Finding {
	var out []Finding
	for _, d := range redundantMakedepends(ctx) {
		out = append(out, findingAt("PB904", Warn, ctx.Pkg.PKGBUILD.Path, d.Pos,
			"%q is already in depends; every runtime dependency is installed at build time too", d.Full))
	}
	return out
}

// --- PB905: pinned SourceForge mirrors ---------------------------------------

func checkSourceforgeMirror(ctx *Context) []Finding {
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		host := strings.ToLower(e.Host())
		if host == "" {
			continue
		}
		pinned := host == "dl.sourceforge.net" || host == "dl.sf.net" ||
			strings.HasSuffix(host, ".dl.sourceforge.net") || strings.HasSuffix(host, ".dl.sf.net")
		if !pinned {
			continue
		}
		out = append(out, findingAt("PB905", Info, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"source pins the SourceForge mirror %q; use https://downloads.sourceforge.net so a working mirror is chosen automatically", host))
	}
	return out
}

// --- PB906: package name repeated in pkgdesc ---------------------------------

func checkPkgnameInDesc(ctx *Context) []Finding {
	v := ctx.Pkg.Vars["pkgdesc"]
	if v == nil || v.Array {
		return nil
	}
	desc, ok := staticVal(ctx.Pkg, firstValue(v.Values))
	if !ok || desc == "" {
		return nil
	}
	words := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(desc)) {
		words[w] = true
	}
	for _, e := range varElems(ctx.Pkg.Vars["pkgname"]) {
		name, ok := staticVal(ctx.Pkg, e.Value)
		if !ok || name == "" {
			continue
		}
		if words[strings.ToLower(name)] {
			return []Finding{findingAt("PB906", Info, ctx.Pkg.PKGBUILD.Path, v.Pos,
				"pkgdesc repeats the package name %q; describe what the software does instead", name)}
		}
	}
	return nil
}

// --- PB907: makepkg's private output helpers ---------------------------------

var makepkgInternalFns = map[string]bool{
	"msg": true, "msg2": true, "warning": true, "error": true, "plain": true,
}

func checkMakepkgInternalFunctions(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		if c.Unit.Scriptlet || c.Fn == "" || !makepkgInternalFns[c.Name] || len(c.Args) == 0 {
			continue
		}
		// A function of the same name defined in the PKGBUILD is the
		// packager's own; only bare calls reach makepkg's helpers.
		if _, defined := c.Unit.Functions[c.Name]; defined {
			continue
		}
		out = append(out, c.finding("PB907", Warn,
			"%s is a makepkg-internal helper, not a stable API; use echo or printf", c.Name))
	}
	return out
}

// --- PB908: maintainer tag ---------------------------------------------------

// A qualifier may sit between the tag and its colon: "# Maintainer (since
// 5.1.8): …". Without the colon it is not the tag tools parse.
var maintainerTagRe = regexp.MustCompile(`(?im)^\s*#\s*Maintainers?\b[^:\n]*:`)

func checkMaintainerComment(ctx *Context) []Finding {
	if maintainerTagRe.Match(ctx.Pkg.PKGBUILD.Raw) {
		return nil
	}
	return []Finding{{RuleID: "PB908", Severity: Info, Path: ctx.Pkg.PKGBUILD.Path, Line: 1, Col: 1,
		Message: "no '# Maintainer:' comment; AUR convention names the responsible maintainer at the top of the PKGBUILD"}}
}

// --- PB909: uppercase package names ------------------------------------------

func checkUppercasePkgname(ctx *Context) []Finding {
	var out []Finding
	for _, field := range []string{"pkgname", "pkgbase"} {
		for _, e := range varElems(ctx.Pkg.Vars[field]) {
			val, ok := staticVal(ctx.Pkg, e.Value)
			if !ok || val == strings.ToLower(val) {
				continue
			}
			out = append(out, findingAt("PB909", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"%s %q contains uppercase letters; Arch package names are lowercase by convention", field, val))
		}
	}
	return out
}

// --- PB910: pkgdesc / url / license presence ---------------------------------

func checkMissingMetadata(ctx *Context) []Finding {
	var out []Finding
	for _, field := range []string{"pkgdesc", "url", "license"} {
		if v := ctx.Pkg.Vars[field]; v != nil && len(v.Values) > 0 && firstValue(v.Values) != "" {
			continue
		}
		if fieldSetInPackageFns(ctx, field) {
			continue // split packages may declare it per package function
		}
		if ctx.Pkg.ConditionalVars[field] {
			continue // set by a top-level branch the parser cannot resolve
		}
		out = append(out, Finding{RuleID: "PB910", Severity: Warn, Path: ctx.Pkg.PKGBUILD.Path,
			Line: 1, Col: 1,
			Message: field + " is not set; Arch packaging standards require it on every package"})
	}
	return out
}

// fieldSetInPackageFns reports whether any package_*() function assigns name.
func fieldSetInPackageFns(ctx *Context, name string) bool {
	found := false
	for fn, fd := range ctx.Pkg.PKGBUILD.Functions {
		if fn != "package" && !strings.HasPrefix(fn, "package_") {
			continue
		}
		syntax.Walk(fd, func(node syntax.Node) bool {
			call, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}
			for _, as := range call.Assigns {
				if as.Name != nil && as.Name.Value == name {
					found = true
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// --- PB911: version-only download names --------------------------------------

// versionNameRe matches basenames that are nothing but a (possibly v-prefixed)
// version or date followed by an extension: v1.2.3.tar.gz, 20240101.zip.
var versionNameRe = regexp.MustCompile(`^[vV]?(([0-9]){8}|([0-9]+\.?)+)\.`)

func checkNonUniqueSourceName(ctx *Context) []Finding {
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		if e.Local || e.VCS != "" || e.Filename != "" || e.URL == "" {
			continue
		}
		base := basename(e.URL)
		if i := strings.IndexAny(base, "#?"); i >= 0 {
			base = base[:i]
		}
		if !versionNameRe.MatchString(base) {
			continue
		}
		out = append(out, findingAt("PB911", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"source downloads to %q, which does not identify the package; rename it with \"$pkgname-$pkgver.tar.gz::url\"", base))
	}
	return out
}

// --- PB912: depends repeated in optdepends -----------------------------------

// duplicatedOptdepends returns the optdepends entries depends already makes
// mandatory: the entries PB912 reports, and the ones its fix deletes.
func duplicatedOptdepends(ctx *Context) []depEntry {
	depends := depsFor(ctx, "depends")
	if len(depends) == 0 {
		return nil
	}
	var out []depEntry
	for _, d := range depEntries(ctx, "optdepends") {
		if _, dup := depends[d.Name]; dup {
			out = append(out, d)
		}
	}
	return out
}

func checkDuplicatedOptdepends(ctx *Context) []Finding {
	var out []Finding
	for _, d := range duplicatedOptdepends(ctx) {
		out = append(out, findingAt("PB912", Warn, ctx.Pkg.PKGBUILD.Path, d.Pos,
			"%q is optional here but already a hard dependency in depends; keep one of the two", d.Name))
	}
	return out
}
