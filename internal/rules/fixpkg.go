package rules

// Fixes for the package-scope rules: the ones whose finding comes out of a
// built archive but whose remedy is a line of the PKGBUILD that produced it.
//
// Every other fixer in this package reads the same file it writes. These read
// two, and the pairing is the whole risk: an edit derived from *some* archive
// would let any archive dictate a depends entry. So the fix context is only
// ever assembled where the pairing is known — `pkglint build`, which gated the
// PKGBUILD, ran makepkg on it, and holds what came out — and the fixers below
// re-check it anyway (fixablePackageArchive), because a cheap identity test at
// the point of use outlives whoever remembers why the caller was trusted.
//
// Nothing here executes the archive or the PKGBUILD. The archive is parsed
// exactly as `pkglint <archive>` parses it; the edit is computed from the
// PKGBUILD's own AST. The only new thing is that the two meet.

// fixablePackageArchive reports whether ctx.File is the one package ctx.Pkg
// builds, so an edit to the PKGBUILD's top-level metadata speaks for it.
//
// A split PKGBUILD fails this deliberately: its per-package depends live in
// each package_<name>() function, and a top-level array would answer for all
// of them at once. A -debug archive fails it for free — its pkgname is not the
// PKGBUILD's — as does an archive that simply belongs to another package.
func fixablePackageArchive(ctx *Context) bool {
	if ctx.File == nil || ctx.Pkg == nil || ctx.File.Info.Name == "" {
		return false
	}
	// Both counts, not just the resolvable one: pkgname=(demo $_extra) has one
	// statically-known name and still builds two packages.
	if len(varElems(ctx.Pkg.Vars["pkgname"])) != 1 || ctx.Pkg.ConditionalVars["pkgname"] {
		return false
	}
	names := pkgnames(ctx)
	return len(names) == 1 && names[0] == ctx.File.Info.Name
}

// packageFnAssigns reports whether the PKGBUILD's package() function assigns
// field. makepkg reads the function's assignment last, so a top-level array
// written beside one would be overwritten rather than extended.
func packageFnAssigns(ctx *Context, field string) bool {
	return assignedIn(&ctx.Pkg.PKGBUILD, "package", false)[field]
}

// --- PB809: DT_NEEDED libraries vs depends -----------------------------------

// fixMissingLibDeps declares the packages that own the libraries the built
// binaries link and depends does not reach. Only the gaps the rule reports as
// errors are written: a package that is merely transitive or merely an
// optdepends is already installed for the user who hits it, and which way the
// maintainer wants that resolved is a judgement the fix cannot make.
//
// Adding a dependency the binaries provably load is behavior-preserving in the
// direction that matters — the package could fail to start before and cannot
// after — which is what makes it FixSafe, on the same reasoning as the build
// tools fixdeps.go declares.
//
// Note what the written text is: the owner names come from the pacman local
// database's own package list, reached by resolving a soname to the installed
// file that provides it. No bytes out of the archive are ever written into the
// PKGBUILD — the archive only selects which installed package gets named — so
// a build that produced a hostile ELF can at most cause a real local package
// to be added to depends.
func fixMissingLibDeps(ctx *Context, _ *FixEnv) []Edit {
	if ctx.DB == nil || !fixablePackageArchive(ctx) || packageFnAssigns(ctx, "depends") {
		return nil
	}
	// The error wording is the one this fix answers; the informational variant
	// of the same gap exists precisely because the closure could not be
	// checked, and guessing from an incomplete one is how a wrong name lands.
	if !ctx.facts().closureComplete {
		return nil
	}
	gaps, _ := missingLibDeps(ctx)
	var owners []string
	for _, g := range gaps {
		if g.State == depMissing {
			owners = append(owners, g.Owner)
		}
	}
	if len(owners) == 0 {
		return nil
	}
	return addArrayEntries(ctx, "depends", owners, varFindingLine(ctx, "depends", "pkgname"),
		"the built binaries link libraries these packages own")
}
