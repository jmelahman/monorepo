package rules

import (
	"fmt"
	"strings"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgfile"
)

// PB8xx: built package analysis. These rules run over .pkg.tar.* archives —
// the half of namcap that inspects what a build actually produced rather than
// what the PKGBUILD says. ELF hardening and placement, dependency inference
// from linked libraries and script interpreters, and filesystem hygiene all
// live here. The dependency rules resolve "which package owns this?" through
// pacman's local database and stay silent when it is unavailable.
func packageRules() []Rule {
	return []Rule{
		{
			ID: "PB801", Name: "package-arch-mismatch", Scope: ScopePackage,
			Severity: Info, MaxSeverity: Error, // error for ELF in an 'any' package
			Doc: "arch=('any') promises the package works on every architecture, so an ELF binary or " +
				"static archive inside one is machine code that will crash everywhere else — usually a " +
				"vendored blob the build slipped in. The mirror image, an architecture-specific package " +
				"containing no binary at all, is only a hint that arch=('any') would save mirror space.",
			Check: checkPackageArch,
			Bad: `arch=('any')
package() {
  install -Dm755 prebuilt/tool "$pkgdir/usr/bin/tool"   # a compiled ELF binary
}`,
			Good: `arch=('x86_64')
package() {
  install -Dm755 prebuilt/tool "$pkgdir/usr/bin/tool"
}`,
			Refs: []Ref{refGeneralArch},
		},
		{
			ID: "PB802", Name: "elf-nonstandard-location", Scope: ScopePackage,
			Severity: Info, MaxSeverity: Error, // opt/ is merely noted
			Doc: "Executables and libraries belong under /usr/bin and /usr/lib. An ELF file anywhere " +
				"else (usr/share, etc, var) escapes ldconfig, striping and the linker's default search " +
				"path, and usually means a Makefile installed something to the wrong place. /opt is " +
				"noted informationally: self-contained vendor trees legitimately keep binaries there.",
			Check: checkELFPlacement,
			Bad: `package() {
  install -Dm755 helper "$pkgdir/usr/share/demo/helper"   # ELF outside usr/bin, usr/lib
}`,
			Good: `package() {
  install -Dm755 helper "$pkgdir/usr/lib/demo/helper"
}`,
			Refs: []Ref{refGeneralDirectories},
		},
		{
			ID: "PB803", Name: "elf-executable-stack", Scope: ScopePackage,
			Severity: Warn,
			Doc: "A PT_GNU_STACK header requesting an executable stack disables a decades-old exploit " +
				"mitigation for the whole process: any buffer overflow on the stack becomes directly " +
				"runnable shellcode. Almost always caused by hand-written assembly missing a " +
				".note.GNU-stack section, and fixed by -Wl,-z,noexecstack.",
			Check: checkELFExecStack,
			Bad: `# hand-written assembly without a .note.GNU-stack section makes the
# linker request an executable stack for the whole binary
build() {
  make
}`,
			Good: `build() {
  export LDFLAGS="$LDFLAGS -Wl,-z,noexecstack"
  make
}`,
			Refs: []Ref{guideline("security", "Security package guidelines", "NX", "nx")},
		},
		{
			ID: "PB804", Name: "elf-text-relocation", Scope: ScopePackage,
			Severity: Warn,
			Doc: "DT_TEXTREL means the loader must patch the code segment at runtime, so the text " +
				"pages can't stay read-only and shared — a security and memory cost. Usually " +
				"non-PIC assembly in a shared library; rebuild with -fPIC.",
			Check: checkELFTextRel,
			Bad: `build() {
  gcc -shared -o libdemo.so demo.c   # non-PIC objects force text relocations
}`,
			Good: `build() {
  gcc -shared -fPIC -o libdemo.so demo.c
}`,
		},
		{
			ID: "PB805", Name: "elf-missing-relro", Scope: ScopePackage,
			Severity: Warn,
			Doc: "Full RELRO (PT_GNU_RELRO plus BIND_NOW) makes the GOT read-only after startup, " +
				"closing the classic GOT-overwrite exploit path. Arch's default LDFLAGS include " +
				"-Wl,-z,relro,-z,now, so a binary without both usually overrode the distribution's " +
				"hardening flags.",
			Check: checkELFRelro,
			Bad:   `options=(!buildflags)   # drops Arch's default -Wl,-z,relro,-z,now hardening`,
			Good: `# keep the default buildflags; Arch's LDFLAGS already enable full RELRO
build() {
  make
}`,
			Refs: []Ref{guideline("security", "Security package guidelines", "RELRO", "relro")},
		},
		{
			ID: "PB806", Name: "elf-not-pie", Scope: ScopePackage,
			Severity: Warn,
			Doc: "A position-dependent executable loads at a fixed address, so ASLR cannot randomize " +
				"its layout and every gadget sits at a known offset. Arch builds executables as PIE by " +
				"default; a non-PIE binary here means upstream's build system overrode it (common in " +
				"prebuilt -bin packages).",
			Check: checkELFNoPIE,
			Bad: `build() {
  export LDFLAGS="-no-pie"
  make
}`,
			Good: `# keep the default toolchain flags; Arch builds executables as PIE
build() {
  make
}`,
			Refs: []Ref{guideline("security", "Security package guidelines", "PIE", "pie")},
		},
		{
			ID: "PB807", Name: "elf-unstripped", Scope: ScopePackage,
			Severity: Warn,
			Doc: "An ELF file still carrying its .symtab was not stripped: makepkg does this " +
				"automatically unless options=(!strip) or a manual install bypassed it. Debug symbols " +
				"belong in the -debug split package, not in the shipping binary.",
			Check: checkELFUnstripped,
			Bad:   `options=(!strip)   # ships every binary with its full symbol table`,
			Good:  `options=(strip debug)   # symbols go to the -debug split package`,
		},
		{
			ID: "PB808", Name: "insecure-rpath", Scope: ScopePackage,
			Severity: Warn, MaxSeverity: Error, // /usr/local/lib warns, anything else errors
			Doc: "An RPATH/RUNPATH pointing at a writable or nonexistent directory (/tmp, a build " +
				"directory, a relative path) lets anyone who can write there hijack the library load " +
				"of every user running the binary. Only /usr/lib, /usr/lib32, /lib and $ORIGIN-relative " +
				"entries are safe to ship.",
			Check: checkRpath,
			Bad: `build() {
  export LDFLAGS="-Wl,-rpath,/home/builder/demo/lib"
  make
}`,
			Good: `build() {
  export LDFLAGS="-Wl,-rpath,\$ORIGIN/../lib/demo"
  make
}`,
			Refs: []Ref{guideline("security", "Security package guidelines", "RPATH/RUNPATH", "rpathrunpath")},
		},
		{
			ID: "PB809", Name: "missing-library-dependency", Scope: ScopePackage,
			Severity: Info, MaxSeverity: Error, // error when nothing in depends pulls the library
			Doc: "Every DT_NEEDED library must be reachable from depends, or the binary fails to " +
				"start on a system with exactly the declared dependencies installed. Reported as an " +
				"error when no depends entry pulls the owning package in even transitively, a warning " +
				"when only an optdepends does, and informationally when coverage is transitive (it " +
				"breaks silently when the middleman drops its own dependency) or the library cannot " +
				"be resolved on this system. Needs the pacman local database.",
			Check:    checkMissingLibDeps,
			FixLevel: FixSafe,
			Fix:      fixMissingLibDeps,
			Bad:      `depends=()   # but usr/bin/demo links libpng16.so.16`,
			Good:     `depends=('libpng')`,
			Refs:     []Ref{refGeneralDependencies},
		},
		{
			ID: "PB810", Name: "unused-linked-library", Scope: ScopePackage,
			Severity: Info,
			Doc: "The binary lists a library in DT_NEEDED but imports none of its symbols, so the " +
				"dependency is pure startup cost and an extra package pulled onto every user's system. " +
				"Usually a build system linking everything it found; -Wl,--as-needed (in Arch's default " +
				"LDFLAGS) removes these. Judged statically from the dynamic symbol tables — pkglint " +
				"never executes the inspected binary the way `ldd -u` would — and informational " +
				"because some toolchains (GHC notably) link whole dependency sets by design.",
			Check: checkUnusedLibs,
			Bad: `# the build links every library pkg-config mentions, used or not:
# usr/bin/demo lists libxml2.so.2 in DT_NEEDED but imports nothing from it
build() {
  make
}`,
			Good: `build() {
  export LDFLAGS="$LDFLAGS -Wl,--as-needed"
  make
}`,
		},
		{
			ID: "PB811", Name: "stale-soname-declaration", Scope: ScopePackage,
			Severity: Warn, MaxSeverity: Error, // unversioned soname entries error
			Doc: "Soname-style entries in depends and provides (libfoo.so=2-64) must match what the " +
				"binaries actually link and ship: a leftover entry pins pacman's dependency resolution " +
				"to a library nothing uses, and an unversioned `libfoo.so` matches every future ABI, " +
				"defeating the versioned-provides mechanism entirely.",
			Check: checkSonameDeclarations,
			Bad:   `depends=('libfoo.so')   # unversioned: matches every future ABI`,
			Good:  `depends=('libfoo.so=2-64')`,
			Refs:  []Ref{refGeneralRelations},
		},
		{
			ID: "PB812", Name: "script-interpreter-dependency", Scope: ScopePackage,
			Severity: Info, MaxSeverity: Error, // error when the interpreter's package is missing from depends
			Doc: "Every shebang line is a runtime dependency: a script starting #!/usr/bin/python " +
				"needs python installed, whether or not any library is linked. Reported as an error " +
				"when the interpreter's owning package is not reachable from depends, and " +
				"informationally when the interpreter can't be resolved on this system. Needs the " +
				"pacman local database.",
			Check: checkShebangDeps,
			Bad:   `depends=()   # but the package installs scripts beginning #!/usr/bin/python`,
			Good:  `depends=('python')`,
			Refs:  []Ref{refGeneralEtiquette},
		},
		{
			ID: "PB813", Name: "unneeded-dependency", Scope: ScopePackage,
			Severity: Info,
			Doc: "No binary links against this dependency and no script names it as interpreter, so " +
				"as far as static analysis can see nothing requires it at runtime. Data files, dlopen " +
				"and IPC dependencies are invisible to this check — hence informational — but a stale " +
				"entry from a dropped feature is the common case. Needs the pacman local database and " +
				"a package with at least one binary or script.",
			Check: checkUnneededDeps,
			Bad: `depends=('qt5-base' 'libpng')   # the qt frontend was dropped a year ago;
                                # nothing links or runs qt5-base anymore`,
			Good: `depends=('libpng')`,
		},
		{
			ID: "PB814", Name: "implicit-path-dependency", Scope: ScopePackage,
			Severity: Warn,
			Doc: "Some files only work when another package is present: GSettings schemas need dconf, " +
				"GIO modules need glib2, hicolor icons need hicolor-icon-theme (or the icon directory " +
				"is orphaned), and Java bytecode needs a java-runtime provider. Shipping the file " +
				"without the dependency breaks it quietly at runtime.",
			Check: checkPathDeps,
			Bad: `package() {
  install -Dm644 demo.gschema.xml "$pkgdir/usr/share/glib-2.0/schemas/demo.gschema.xml"
}   # ...but depends does not include dconf`,
			Good: `depends=('dconf')
package() {
  install -Dm644 demo.gschema.xml "$pkgdir/usr/share/glib-2.0/schemas/demo.gschema.xml"
}`,
		},
		{
			ID: "PB815", Name: "hook-covered-dependency", Scope: ScopePackage,
			Severity: Info,
			Doc: "desktop-file-utils and shared-mime-info are only needed for their update commands, " +
				"which pacman's own hooks run since 5.0 regardless of what this package depends on. " +
				"A package that just ships .desktop or MIME files can drop the dependency.",
			Check: checkHookCoveredDeps,
			Bad: `depends=('desktop-file-utils')   # only there for update-desktop-database,
                                 # which pacman's hook runs anyway`,
			Good: `# ship the .desktop file; pacman's hook updates the database
package() {
  install -Dm644 demo.desktop "$pkgdir/usr/share/applications/demo.desktop"
}`,
		},
		{
			ID: "PB816", Name: "pkgconfig-dependency", Scope: ScopePackage,
			Severity: Warn,
			Doc: "A shipped .pc file whose Requires: names modules this package neither ships nor " +
				"reaches through depends breaks `pkg-config --cflags` for everything built against it. " +
				"The requirement belongs in depends of the -devel consumer chain. Needs the pacman " +
				"local database.",
			Check: checkPkgconfigDeps,
			Bad: `# usr/lib/pkgconfig/demo.pc says "Requires: libcrypto"
depends=()   # ...but nothing in depends provides openssl's .pc files`,
			Good: `depends=('openssl')`,
		},
		{
			ID: "PB817", Name: "package-metadata", Scope: ScopePackage,
			Severity: Warn,
			Doc: "url and pkgdesc are required by Arch packaging standards — without them pacman -Qi " +
				"shows blanks — and package names are lowercase by convention. These are the " +
				".PKGINFO-side counterparts of the PKGBUILD checks, for auditing archives whose " +
				"PKGBUILD is not at hand.",
			Check: checkPackageMetadata,
			Bad: `# built with a PKGBUILD that never set url= or pkgdesc=,
# so .PKGINFO ships blanks and pacman -Qi shows empty fields`,
			Good: `pkgdesc='Fast demonstration tool'
url='https://github.com/example/demo'`,
		},
		{
			ID: "PB820", Name: "filesystem-layout", Scope: ScopePackage,
			Severity: Info, MaxSeverity: Error, // temp dirs error, odd-but-plausible paths inform
			Doc: "Packages may only install into the FHS directories pacman manages (usr/bin, " +
				"usr/lib, usr/share, etc, opt, var/lib, …). Files under tmp, run or var/run sit on " +
				"tmpfs and vanish at boot — an error. usr/local is reserved for the administrator, " +
				"usr/man and usr/info were retired decades ago, and a stray 'man' or 'info' path " +
				"component is usually a mis-installed page.",
			Check: checkFilesystemLayout,
			Bad: `package() {
  install -d "$pkgdir/run/demo"   # tmpfs: gone at first boot
}`,
			Good: `package() {
  install -Dm644 demo.tmpfiles "$pkgdir/usr/lib/tmpfiles.d/demo.conf"
}`,
			Refs: []Ref{refGeneralDirectories},
		},
		{
			ID: "PB821", Name: "file-permissions", Scope: ScopePackage,
			Severity: Warn, MaxSeverity: Error, // world-writable files error
			Doc: "A world-writable file in a package is a system-wide backdoor invitation — any local " +
				"user can replace its contents. Setuid/setgid bits deserve review every time, files " +
				"the world cannot read break non-root users, and static libraries should be plain 644.",
			Check: checkFilePermissions,
			Bad: `package() {
  chmod 777 "$pkgdir/var/lib/demo"   # world-writable
}`,
			Good: `package() {
  chmod 755 "$pkgdir/var/lib/demo"
}`,
		},
		{
			ID: "PB822", Name: "file-ownership", Scope: ScopePackage,
			Severity: Error,
			Doc: "makepkg packages every file as root:root (remapped at install time). Any other " +
				"owner in the archive means the build wrote user-owned files into the package — on " +
				"install they'd hand a login user ownership of system files.",
			Check: checkFileOwnership,
			Bad: `package() {
  cp -a ~/demo-data "$pkgdir/usr/share/demo"   # preserves the build user's ownership
}`,
			Good: `package() {
  cp -a --no-preserve=ownership demo-data "$pkgdir/usr/share/demo"
}`,
		},
		{
			ID: "PB823", Name: "empty-directory", Scope: ScopePackage,
			Severity: Info,
			Doc: "An empty directory is usually build-system residue; makepkg's purge step removes " +
				"them unless options=(emptydirs) asked to keep them. Intentional mount points and " +
				"spool directories are the exception — hence informational.",
			Check: checkEmptyDirectories,
			Bad: `package() {
  install -d "$pkgdir/var/log/demo"   # ships as an empty directory
}`,
			Good: `# create runtime directories at boot instead of shipping them empty
package() {
  install -Dm644 demo.tmpfiles "$pkgdir/usr/lib/tmpfiles.d/demo.conf"
}`,
		},
		{
			ID: "PB824", Name: "invalid-filename", Scope: ScopePackage,
			Severity: Warn,
			Doc: "Filenames containing control characters or non-ASCII bytes break scripts, terminals " +
				"and some filesystems, and are a favorite spot to hide lookalike files. Almost always " +
				"an encoding accident in upstream's tarball.",
			Check: checkFilenames,
			Bad: `package() {
  # upstream tarball contains "README (kopie).txt" with a non-ASCII name
  cp -r docs "$pkgdir/usr/share/doc/$pkgname"
}`,
			Good: `package() {
  install -Dm644 docs/README.txt "$pkgdir/usr/share/doc/$pkgname/README.txt"
}`,
		},
		{
			ID: "PB825", Name: "cross-directory-hardlink", Scope: ScopePackage,
			Severity: Error,
			Doc: "A hard link whose two names live in different directories breaks when the " +
				"filesystem boundary between them differs on the user's machine, and pacman's " +
				"extraction order makes no guarantees across directories. Use a symlink.",
			Check: checkHardlinks,
			Bad: `package() {
  ln "$pkgdir/usr/bin/demo" "$pkgdir/usr/lib/demo/demo"   # hard link across directories
}`,
			Good: `package() {
  ln -s /usr/bin/demo "$pkgdir/usr/lib/demo/demo"
}`,
		},
		{
			ID: "PB826", Name: "dangling-symlink", Scope: ScopePackage,
			Severity: Error,
			Doc: "A symlink pointing at a path that neither this package nor anything reachable from " +
				"its depends ships is broken the moment it is installed. Usually a target renamed " +
				"during packaging, or a dependency that should have been declared. Needs the pacman " +
				"local database.",
			Check: checkSymlinks,
			Bad: `package() {
  ln -s /usr/share/demo/old-name "$pkgdir/usr/bin/demo"   # target was renamed upstream
}`,
			Good: `package() {
  ln -s /usr/share/demo/demo.sh "$pkgdir/usr/bin/demo"   # target ships in this package
}`,
		},
		{
			ID: "PB827", Name: "libtool-archive", Scope: ScopePackage,
			Severity: Warn,
			Doc: ".la files are libtool bookkeeping that modern linkers never read; shipping them " +
				"makes other packages' libtool builds embed stale paths. Arch policy is to delete " +
				"them (makepkg's !libtool option once did this automatically).",
			Check: checkLibtoolFiles,
			Bad: `package() {
  make DESTDIR="$pkgdir" install   # leaves libtool .la files in usr/lib
}`,
			Good: `package() {
  make DESTDIR="$pkgdir" install
  find "$pkgdir" -name '*.la' -delete
}`,
		},
		{
			ID: "PB828", Name: "perllocal-pod", Scope: ScopePackage,
			Severity: Error,
			Doc: "perllocal.pod is perl's global install log: every perl module build appends to the " +
				"same file, so two packages shipping it conflict on install. Delete it in package().",
			Check: checkPerllocal,
			Bad: `package() {
  make DESTDIR="$pkgdir" install   # perl's install step wrote perllocal.pod
}`,
			Good: `package() {
  make DESTDIR="$pkgdir" install
  find "$pkgdir" -name perllocal.pod -delete
}`,
		},
		{
			ID: "PB829", Name: "info-directory-file", Scope: ScopePackage,
			Severity: Error,
			Doc: "usr/share/info/dir is the info reader's generated index — shared by every package " +
				"and rebuilt by pacman's texinfo hook. A package shipping its own copy conflicts with " +
				"every other package that does. Delete it in package().",
			Check: checkInfoDirFile,
			Bad: `package() {
  make DESTDIR="$pkgdir" install-info   # writes usr/share/info/dir
}`,
			Good: `package() {
  make DESTDIR="$pkgdir" install-info
  rm -f "$pkgdir/usr/share/info/dir"
}`,
		},
		{
			ID: "PB830", Name: "stale-python-bytecode", Scope: ScopePackage,
			Severity: Warn, MaxSeverity: Error, // tar timestamps stale = error, mtree-only = warn
			Doc: "A .pyc older than its .py source is invalid: python either recompiles it into " +
				"__pycache__ litter it can't write as non-root, or (pre-compileall hashes) silently " +
				"serves stale bytecode. Happens when package() touches .py files after compileall ran.",
			Check: checkPycMtimes,
			Bad: `package() {
  python -m compileall "$pkgdir"
  sed -i "s/@VERSION@/$pkgver/" "$pkgdir"/usr/lib/python*/site-packages/demo/version.py
}   # the .py is now newer than its .pyc`,
			Good: `package() {
  sed -i "s/@VERSION@/$pkgver/" "$pkgdir"/usr/lib/python*/site-packages/demo/version.py
  python -m compileall "$pkgdir"
}`,
		},
		{
			ID: "PB831", Name: "python-tests-directory", Scope: ScopePackage,
			Severity: Error,
			Doc: "A bare 'tests' directory directly under site-packages squats a generic top-level " +
				"module name: every package that makes this packaging mistake collides with every " +
				"other, and `import tests` resolves to whichever won. Exclude it in package().",
			Check: checkPyTestsDir,
			Bad: `package() {
  python -m installer --destdir="$pkgdir" dist/*.whl   # the wheel ships a top-level tests/
}`,
			Good: `package() {
  python -m installer --destdir="$pkgdir" dist/*.whl
  rm -r "$pkgdir"/usr/lib/python*/site-packages/tests
}`,
			Refs: []Ref{guideline("python", "Python package guidelines", "Test directory in site-package", "test-directory-in-site-package")},
		},
		{
			ID: "PB832", Name: "systemd-unit-in-etc", Scope: ScopePackage,
			Severity: Warn,
			Doc: "/etc/systemd/system is the administrator's override directory; packaged units " +
				"belong in /usr/lib/systemd/system. A package writing to etc shadows the admin's " +
				"namespace and fights local overrides.",
			Check: checkSystemdLocation,
			Bad: `package() {
  install -Dm644 demo.service "$pkgdir/etc/systemd/system/demo.service"
}`,
			Good: `package() {
  install -Dm644 demo.service "$pkgdir/usr/lib/systemd/system/demo.service"
}`,
		},
		{
			ID: "PB833", Name: "dbus-policy-in-etc", Scope: ScopePackage,
			Severity: Warn,
			Doc: "/etc/dbus-1/system.d is for local administrator overrides; packaged D-Bus system " +
				"bus policies belong in /usr/share/dbus-1/system.d (supported since dbus 1.10).",
			Check: checkDbusLocation,
			Bad: `package() {
  install -Dm644 demo.conf "$pkgdir/etc/dbus-1/system.d/demo.conf"
}`,
			Good: `package() {
  install -Dm644 demo.conf "$pkgdir/usr/share/dbus-1/system.d/demo.conf"
}`,
		},
		{
			ID: "PB834", Name: "missing-license-file", Scope: ScopePackage,
			Severity: Error,
			Doc: "license=() must be declared, and licenses that aren't distributed system-wide " +
				"(custom, LicenseRef-*, and any SPDX identifier not shipped in " +
				"/usr/share/licenses/spdx) must install their text under " +
				"usr/share/licenses/$pkgname/. Without it the package distributes software whose " +
				"license terms the user can't read.",
			Check: checkLicenseFiles,
			Bad: `license=('LicenseRef-demo-eula')
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"   # the EULA text is never installed
}`,
			Good: `license=('LicenseRef-demo-eula')
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
  install -Dm644 EULA "$pkgdir/usr/share/licenses/$pkgname/EULA"
}`,
		},
		{
			ID: "PB835", Name: "missing-backup-file", Scope: ScopePackage,
			Severity: Error,
			Doc: "Every backup=() entry must name a file the package actually ships; an entry for a " +
				"nonexistent path does nothing except mask the typo that broke the config-preservation " +
				"it was meant to provide.",
			Check: checkBackupFiles,
			Bad: `backup=('etc/demo.conf')
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"   # etc/demo.conf is never installed
}`,
			Good: `backup=('etc/demo.conf')
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
  install -Dm644 demo.conf "$pkgdir/etc/demo.conf"
}`,
		},
		{
			ID: "PB836", Name: "docs-heavy-package", Scope: ScopePackage,
			Severity: Info,
			Doc: "More than half of this package's bytes are documentation under usr/share/doc. " +
				"Consider a -docs split package so users who just want the software don't carry the " +
				"manuals. Packages named *-doc/-docs are exempt — being the manuals is their job.",
			Check: checkDocsHeavy,
			Bad: `package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
  cp -r manual/ "$pkgdir/usr/share/doc/$pkgname"   # 300 MB of HTML for a 2 MB tool
}`,
			Good: `# move the manual to a demo-docs split package
package_demo-docs() {
  cp -r manual/ "$pkgdir/usr/share/doc/demo"
}`,
		},
		{
			ID: "PB837", Name: "sphinx-build-cache", Scope: ScopePackage,
			Severity: Warn,
			Doc: "environment.pickle is sphinx-build's incremental-build cache, not documentation: " +
				"it embeds absolute build paths and pickled Python objects. Its presence means the " +
				"whole .doctrees cache directory was installed by accident.",
			Check: checkSphinxCache,
			Bad: `package() {
  cp -r docs/_build "$pkgdir/usr/share/doc/$pkgname"   # includes .doctrees/environment.pickle
}`,
			Good: `package() {
  cp -r docs/_build/html "$pkgdir/usr/share/doc/$pkgname"
}`,
		},
		{
			ID: "PB838", Name: "generated-mime-cache", Scope: ScopePackage,
			Severity: Error,
			Doc: "mimeinfo.cache and the usr/share/mime databases (globs, magic, aliases, …) are " +
				"generated on the user's machine by pacman's hooks from every installed package's " +
				"MIME declarations. Shipping a build-time copy conflicts with other packages and " +
				"freezes the database at build state.",
			Check: checkMimeCache,
			Bad: `package() {
  update-mime-database "$pkgdir/usr/share/mime"   # bakes generated caches into the package
}`,
			Good: `package() {
  install -Dm644 demo-mime.xml "$pkgdir/usr/share/mime/packages/demo.xml"
}   # pacman's hook regenerates the database on the user's system`,
		},
		{
			ID: "PB839", Name: "scrollkeeper-directory", Scope: ScopePackage,
			Severity: Warn,
			Doc: "scrollkeeper's catalog directories are relics of a documentation system GNOME " +
				"retired in 2008; a package creating var/lib/scrollkeeper ran an ancient " +
				"documentation hook that should be disabled (--disable-scrollkeeper).",
			Check: checkScrollkeeper,
			Bad: `build() {
  ./configure --prefix=/usr   # an ancient scrollkeeper hook creates var/lib/scrollkeeper
  make
}`,
			Good: `build() {
  ./configure --prefix=/usr --disable-scrollkeeper
  make
}`,
		},
		{
			ID: "PB842", Name: "java-jar-location", Scope: ScopePackage,
			Severity: Info,
			Doc: "The Java package guidelines put every .jar under usr/share/java (usually in a " +
				"per-package subdirectory) so other packages can find libraries by convention and " +
				"wrapper scripts share one classpath root; an application that insists on its own " +
				"layout gets symlinks into its tree, not private copies of the jars.",
			Check: checkJarLocation,
			Bad: `package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/demo/demo.jar"   # private jar copy
}`,
			Good: `package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
  ln -s /usr/share/java/demo/demo.jar "$pkgdir/usr/share/demo/demo.jar"
}`,
			Refs: []Ref{guideline("java", "Java package guidelines", "Java packaging on Arch Linux", "java-packaging-on-arch-linux")},
		},
	}
}

// entryRule is a package-scope check that reports, with one fixed message,
// every archive entry match accepts.
func entryRule(id string, sev Severity, match func(*pkgfile.Entry) bool, msg string) func(*Context) []Finding {
	return func(ctx *Context) []Finding {
		var out []Finding
		for i := range ctx.File.Entries {
			if e := &ctx.File.Entries[i]; match(e) {
				out = append(out, pkgFinding(id, sev, e.Name, "%s", msg))
			}
		}
		return out
	}
}

// pkgFinding builds a finding anchored at a member path of the package.
// Package findings have no line/column: the location is the member itself.
func pkgFinding(id string, sev Severity, path, format string, args ...any) Finding {
	return Finding{RuleID: id, Severity: sev, Message: fmt.Sprintf(format, args...), Path: path}
}

// packageFacts is everything several package rules need, computed once.
type packageFacts struct {
	depNames    map[string]bool // direct depends, constraint-stripped
	optNames    map[string]bool // direct optdepends, description-stripped
	depClosure  map[string]bool // transitive closure of depends via the DB
	optClosure  map[string]bool // transitive closure of optdepends
	shipsSoname map[string]bool // sonames and lib basenames this package ships
	neededSet   map[string]bool // every DT_NEEDED across the package's ELFs

	// closureComplete is true when every declared dependency resolves to an
	// installed provider, i.e. the closures above are trustworthy. When a
	// declared dependency is NOT installed on the lint host, its subtree is
	// invisible and "not reachable from depends" verdicts soften to
	// informational.
	closureComplete bool

	hostExports map[string]map[string]bool // soname -> exported dynamic symbols (host lookups)
}

// facts returns the package facts, computing them on first use.
func (ctx *Context) facts() *packageFacts {
	if ctx.pkgFacts != nil {
		return ctx.pkgFacts
	}
	f := &packageFacts{
		depNames:    map[string]bool{},
		optNames:    map[string]bool{},
		shipsSoname: map[string]bool{},
		neededSet:   map[string]bool{},
		hostExports: map[string]map[string]bool{},
	}
	info := ctx.File.Info
	for _, d := range info.Depends {
		f.depNames[pkgDepName(d)] = true
	}
	for _, d := range info.OptDepends {
		f.optNames[pkgDepName(d)] = true
	}
	f.depClosure = ctx.DB.Closure(stripOptDescs(info.Depends))
	f.optClosure = ctx.DB.Closure(stripOptDescs(info.OptDepends))
	f.closureComplete = ctx.DB != nil
	for _, d := range append(stripOptDescs(info.Depends), stripOptDescs(info.OptDepends)...) {
		if name := alpmdb.DepName(d); name != "" && !strings.Contains(name, ".so") &&
			len(ctx.DB.ProvidersOf(name)) == 0 {
			f.closureComplete = false
		}
	}
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !e.IsFile() {
			continue
		}
		if e.ELF != nil {
			for _, n := range e.ELF.Needed {
				f.neededSet[n] = true
			}
			if e.ELF.Soname != "" {
				f.shipsSoname[e.ELF.Soname] = true
			}
		}
		if e.IsELF && strings.Contains(basename(e.Name), ".so") {
			f.shipsSoname[basename(e.Name)] = true
		}
	}
	// Symlinks make a library reachable under more names (libfoo.so.1 ->
	// libfoo.so.1.2.3); count those as shipped too.
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsSymlink() && strings.Contains(basename(e.Name), ".so") {
			f.shipsSoname[basename(e.Name)] = true
		}
	}
	ctx.pkgFacts = f
	return f
}

// pkgDepName normalizes a .PKGINFO dependency entry to a bare name:
// optdepends carry ": description" suffixes and any entry may carry a version
// constraint.
func pkgDepName(entry string) string {
	if name, _, ok := strings.Cut(entry, ":"); ok {
		entry = name
	}
	return alpmdb.DepName(entry)
}

// stripOptDescs drops ": description" suffixes so entries can seed a closure.
func stripOptDescs(entries []string) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if name, _, ok := strings.Cut(e, ":"); ok {
			e = name
		}
		out = append(out, strings.TrimSpace(e))
	}
	return out
}

// depSatisfaction says how (or whether) the depends graph reaches a package.
type depSatisfaction int

const (
	depMissing depSatisfaction = iota
	depDirect
	depTransitive
	depOptional
	depSelf
)

// satisfaction classifies how the package's depends reach owner, mirroring
// namcap's analyze_depends: direct beats transitive beats optional.
func (ctx *Context) satisfaction(owner string) depSatisfaction {
	if owner == ctx.File.Info.Name || owner == ctx.File.Info.Base {
		return depSelf
	}
	f := ctx.facts()
	if f.depNames[owner] {
		return depDirect
	}
	// A depends entry may name a capability the owner provides ("python3"
	// rather than "python"); that is a direct dependency on the owner too.
	if p := ctx.DB.Get(owner); p != nil {
		for _, prov := range p.Provides {
			if f.depNames[alpmdb.DepName(prov)] {
				return depDirect
			}
			if f.optNames[alpmdb.DepName(prov)] {
				return depOptional
			}
		}
	}
	if f.depClosure[owner] {
		return depTransitive
	}
	if f.optNames[owner] || f.optClosure[owner] {
		return depOptional
	}
	return depMissing
}

// isPackageELF reports whether the entry is a parsed ELF regular file.
func isPackageELF(e *pkgfile.Entry) bool {
	return e.IsFile() && e.IsELF && e.ELF != nil
}
