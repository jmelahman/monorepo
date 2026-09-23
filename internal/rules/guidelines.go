package rules

// PB918–PB922 lint the main page of the Arch package guidelines
// (https://wiki.archlinux.org/title/Arch_package_guidelines): redundant
// self-provides/self-conflicts, pre-SPDX license identifiers, and installs
// into the directories the guidelines reserve (/usr/local for the
// administrator, /usr/libexec unused on Arch). PB921/PB922 are the static
// PKGBUILD-side complements of PB820, which sees the same layout problems in
// the built archive.

// --- PB918/PB919: a package providing or conflicting with itself -------------

// selfReferences returns the entries of field that name one of the packages
// this PKGBUILD itself builds: what PB918/PB919 report, and what their fixes
// delete.
func selfReferences(ctx *Context, field string) []depEntry {
	names := map[string]bool{}
	for _, n := range pkgnames(ctx) {
		names[n] = true
	}
	if len(names) == 0 {
		return nil
	}
	var out []depEntry
	for _, d := range depEntries(ctx, field) {
		if names[d.Name] {
			out = append(out, d)
		}
	}
	return out
}

// checkSelfReference flags entries of field that name one of the packages this
// PKGBUILD itself builds.
func checkSelfReference(ctx *Context, id, field, why string) []Finding {
	var out []Finding
	for _, d := range selfReferences(ctx, field) {
		out = append(out, findingAt(id, Warn, ctx.Pkg.PKGBUILD.Path, d.Pos,
			"%s lists the package's own name %q; %s", field, d.Full, why))
	}
	return out
}

func checkSelfProvides(ctx *Context) []Finding {
	return checkSelfReference(ctx, "PB918", "provides",
		"a package always provides itself, so the entry is dead metadata the guidelines say to drop")
}

func checkSelfConflicts(ctx *Context) []Finding {
	return checkSelfReference(ctx, "PB919", "conflicts",
		"a package can never conflict with itself, so the entry is dead metadata the guidelines say to drop")
}

// --- PB920: pre-SPDX license identifiers -------------------------------------

// legacyLicenseIDs are the pre-RFC 16 spellings from Arch's old licenses
// array that are not valid SPDX identifiers. A deny-list rather than a full
// SPDX table: an unlisted-but-valid identifier can never be flagged by
// mistake. Spellings that were already valid SPDX (MIT, Unlicense, W3C) are
// deliberately absent.
var legacyLicenseIDs = map[string]bool{
	"GPL": true, "GPL2": true, "GPL3": true,
	"LGPL": true, "LGPL2": true, "LGPL2.1": true, "LGPL3": true,
	"AGPL": true, "AGPL3": true,
	"Apache": true, "BSD": true, "Boost": true,
	"CCPL": true, "CDDL": true, "CPL": true, "EPL": true,
	"FDL": true, "FDL1.2": true, "FDL1.3": true,
	"LPPL": true, "MPL": true, "MPL2": true,
	"PerlArtistic": true, "PHP": true, "PSF": true,
	"RUBY": true, "ZLIB": true,
	"custom": true,
}

func checkNonSPDXLicense(ctx *Context) []Finding {
	var out []Finding
	for _, e := range varElems(ctx.Pkg.Vars["license"]) {
		val, ok := staticVal(ctx.Pkg, e.Value)
		if !ok || val == "" {
			continue
		}
		for _, tok := range licenseTokens(val) {
			if !legacyLicenseIDs[tok] {
				continue
			}
			out = append(out, findingAt("PB920", Info, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"license %q is a pre-SPDX identifier; Arch packages declare SPDX license expressions (e.g. GPL-2.0-or-later, BSD-3-Clause)", tok))
		}
	}
	return out
}

// --- PB921/PB922: installs into reserved directories -------------------------

// checkPkgdirSubdirWrites flags package()-phase writes landing under
// $pkgdir<sub>.
func checkPkgdirSubdirWrites(ctx *Context, id string, sev Severity, sub, why string) []Finding {
	var out []Finding
	for _, w := range packagePhaseWrites(ctx) {
		for _, dest := range w.Dests {
			if !pkgdirSubdir(dest, sub) {
				continue
			}
			out = append(out, w.Cmd.finding(id, sev, "%s installs under %s: %s", w.Cmd.Name, sub, why))
			break // one finding per command, however many destinations match
		}
	}
	return out
}

func checkUsrLocalInstall(ctx *Context) []Finding {
	return checkPkgdirSubdirWrites(ctx, "PB921", Warn, "/usr/local",
		"/usr/local is reserved for the administrator's unpackaged software; packaged files belong under /usr")
}

func checkUsrLibexecInstall(ctx *Context) []Finding {
	return checkPkgdirSubdirWrites(ctx, "PB922", Info, "/usr/libexec",
		"Arch does not use /usr/libexec; internal helpers belong in /usr/lib/"+pkgnamePlaceholder(ctx))
}

// pkgnamePlaceholder names the package in a message, falling back to the
// generic "$pkgname" when the name is not statically known.
func pkgnamePlaceholder(ctx *Context) string {
	if names := pkgnames(ctx); len(names) > 0 {
		return names[0]
	}
	return "$pkgname"
}
