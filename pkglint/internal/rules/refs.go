package rules

// Upstream documents the rules cite. The Arch package guidelines moved from
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

// refs maps a rule ID to the upstream documents it enforces. Every entry has
// been read against the rule: a Ref says the document backs the rule, not
// merely that it is about the same subject. A rule absent here is pkglint's
// own policy, or one no Arch document states.
var refs = map[string][]Ref{
	// Sources and integrity.
	"PB101": {refGeneralSources},
	"PB103": {refGeneralSources},
	"PB104": {refGeneralSources},
	"PB111": {refGeneralSources},
	"PB112": {refGeneralSources},
	"PB905": {refGeneralSources},
	"PB911": {refGeneralSources},

	// Build hermeticity.
	"PB203": {
		guideline("rust", "Rust package guidelines", "Prepare", "prepare"),
		refRustBuild,
	},
	"PB204": {guideline("go", "Go package guidelines", "Upstream project with Go modules", "upstream-project-with-go-modules")},

	// makepkg's own syntax.
	"PB701": {refGeneralNaming, pkgbuildMan("pkgname", manDirectives)},
	"PB702": {refGeneralVersioning, pkgbuildMan("pkgver", manDirectives)},
	"PB703": {refGeneralVersioning, pkgbuildMan("pkgrel", manDirectives)},
	"PB704": {pkgbuildMan("epoch", manDirectives)},
	"PB705": {pkgbuildMan("backup", manDirectives)},
	"PB706": {pkgbuildMan("options", manDirectives)},
	"PB707": {pkgbuildMan("provides", manDirectives)},
	"PB709": {pkgbuildMan("package splitting", "PACKAGE_SPLITTING")},
	"PB711": {refVcsDeps},

	// The built package.
	"PB801": {refGeneralArch},
	"PB802": {refGeneralDirectories},
	"PB803": {guideline("security", "Security package guidelines", "NX", "nx")},
	"PB805": {guideline("security", "Security package guidelines", "RELRO", "relro")},
	"PB806": {guideline("security", "Security package guidelines", "PIE", "pie")},
	"PB808": {guideline("security", "Security package guidelines", "RPATH/RUNPATH", "rpathrunpath")},
	"PB809": {refGeneralDependencies},
	"PB811": {refGeneralRelations},
	"PB812": {refGeneralEtiquette},
	"PB820": {refGeneralDirectories},
	"PB831": {guideline("python", "Python package guidelines", "Test directory in site-package", "test-directory-in-site-package")},
	"PB842": {guideline("java", "Java package guidelines", "Java packaging on Arch Linux", "java-packaging-on-arch-linux")},

	// General style.
	"PB902": {refGeneralEtiquette},
	"PB906": {refGeneralEtiquette},
	"PB907": {refGeneralEtiquette},
	"PB909": {refGeneralNaming},
	"PB912": {refGeneralEtiquette},
	"PB918": {refGeneralRelations},
	"PB919": {refGeneralRelations},
	"PB920": {refGeneralLicense},
	"PB921": {refGeneralEtiquette},
	"PB922": {refGeneralEtiquette},

	// Language and build-system guidelines.
	"PB914": {refGoFlags, guideline("security", "Security package guidelines", "Golang", "golang")},
	"PB915": {refGoFlags},
	"PB916": {refGoFlags},
	"PB917": {refGoFlags},
	"PB930": {refPyCheck},
	"PB931": {refPyCheck},
	"PB932": {refPyPEP517},
	"PB933": {refPyPEP517},
	"PB934": {guideline("python", "Python package guidelines", "setuptools or distutils", "setuptools-or-distutils")},
	"PB940": {guideline("rust", "Rust package guidelines", "Check", "check")},
	"PB941": {guideline("rust", "Rust package guidelines", "Notes about using cargo install", "notes-about-using-cargo-install")},
	"PB942": {refRustBuild},
	"PB944": {guideline("rust", "Rust package guidelines", "Depends", "depends")},
	"PB950": {guideline("cmake", "CMake package guidelines", "Prefix and library install directories", "prefix-and-library-install-directories")},
	"PB951": {guideline("cmake", "CMake package guidelines", "CMake can automatically override the default compiler optimization flag", "cmake-can-automatically-override-the-default-compiler-optimization-flag")},
	"PB953": {guideline("meson", "Meson package guidelines", "Using meson binary directly", "using-meson-binary-directly")},
	"PB954": {guideline("meson", "Meson package guidelines", "Requirements", "requirements")},
	"PB955": {guideline("meson", "Meson package guidelines", "build()", "build")},
	"PB960": {guideline("vcs", "VCS package guidelines", "The pkgver() function", "the-pkgver-function")},
	"PB961": {refVcsDeps},
	"PB962": {guideline("vcs", "VCS package guidelines", "VCS sources", "vcs-sources")},
	"PB963": {guideline("vcs", "VCS package guidelines", "Package naming", "package-naming")},
	"PB970": {guideline("font", "Font package guidelines", "Architecture", "architecture")},
	"PB971": {guideline("font", "Font package guidelines", "Dependencies", "dependencies")},
	"PB972": {guideline("font", "Font package guidelines", "Source", "source")},
	"PB973": {refDkmsDeps},
	"PB974": {refDkmsDeps},
	"PB975": {guideline("32-bit", "32-bit package guidelines", "Package naming", "package-naming")},
	"PB976": {guideline("32-bit", "32-bit package guidelines", "lib32", "lib32")},
	"PB977": {refMingwPkg},
	"PB978": {refMingwPkg},
	"PB979": {guideline("nodejs", "Node.js package guidelines", "Using npm", "using-npm")},
	"PB980": {guideline("nodejs", "Node.js package guidelines", "Setting temporary cache", "setting-temporary-cache")},
	"PB981": {guideline("java", "Java package guidelines", "Dependencies", "dependencies")},
	"PB982": {guideline("clr", "CLR package guidelines", "Packaging gotchas", "packaging-gotchas")},
	"PB983": {guideline("haskell", "Haskell package guidelines", "Architecture", "architecture")},
	"PB984": {guideline("php", "PHP package guidelines", "Architecture", "architecture"), refGeneralArch},
}
