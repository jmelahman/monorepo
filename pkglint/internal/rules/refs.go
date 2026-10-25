package rules

// Upstream documents the rules cite. Each rule's Refs field lists the ones it
// enforces; a Ref says the document backs the rule, not merely that it is about
// the same subject, and a rule without any is pkglint's own policy or one no
// Arch document states. The Arch package guidelines moved from
// the wiki to the developer manual; the wiki's pages redirect there.
const (
	manualGuidelines = "https://manual.archlinux.page/package-guidelines/"
	manPKGBUILD      = "https://man.archlinux.org/man/PKGBUILD.5"
	manDirectives    = "OPTIONS_AND_DIRECTIVES"
)

// guideline cites a section of an Arch package guidelines page: family is the
// page's path under manualGuidelines ("" for the general page) and page its
// title as the manual shows it.
func guideline(family, page, section, anchor string) Ref {
	url := manualGuidelines
	if family != "" {
		url += family + "/"
	}
	return Ref{Title: page + ": " + section, URL: url + "#" + anchor}
}

// pkgbuildMan cites a section of PKGBUILD(5). The man page anchors only its
// top-level sections, so the title names the directive the rule is about.
func pkgbuildMan(directive, anchor string) Ref {
	return Ref{Title: "PKGBUILD(5): " + directive, URL: manPKGBUILD + "#" + anchor}
}

var (
	refGeneralEtiquette    = guideline("", "Arch package guidelines", "Package etiquette", "package-etiquette")
	refGeneralNaming       = guideline("", "Arch package guidelines", "Package naming", "package-naming")
	refGeneralVersioning   = guideline("", "Arch package guidelines", "Package versioning", "package-versioning")
	refGeneralDependencies = guideline("", "Arch package guidelines", "Package dependencies", "package-dependencies")
	refGeneralRelations    = guideline("", "Arch package guidelines", "Package relations", "package-relations")
	refGeneralSources      = guideline("", "Arch package guidelines", "Package sources", "package-sources")
	refGeneralDirectories  = guideline("", "Arch package guidelines", "Directories", "directories")
	refGeneralArch         = guideline("", "Arch package guidelines", "Architectures", "architectures")
	refGeneralLicense      = guideline("", "Arch package guidelines", "PKGBUILD's license field", "pkgbuilds-license-field")

	refGoFlags   = guideline("go", "Go package guidelines", "Flags and build options", "flags-and-build-options")
	refRustBuild = guideline("rust", "Rust package guidelines", "Build", "build")
	refPyCheck   = guideline("python", "Python package guidelines", "Check", "check")
	refPyPEP517  = guideline("python", "Python package guidelines", "Standards based (PEP 517)", "standards-based-pep-517")
	refVcsDeps   = guideline("vcs", "VCS package guidelines", "Conflicts and dependencies", "conflicts-and-dependencies")
	refMingwPkg  = guideline("mingw", "MinGW package guidelines", "Packaging", "packaging")
	refDkmsDeps  = guideline("dkms", "DKMS package guidelines", "Dependencies", "dependencies")
)
