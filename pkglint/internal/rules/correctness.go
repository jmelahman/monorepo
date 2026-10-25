package rules

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB7xx: metadata correctness. These mirror the build-breaking checks makepkg's
// own lint_pkgbuild performs (util/schema.sh, pkgname.sh, pkgver.sh, …), so a
// PKGBUILD that makepkg would refuse to build is caught — and where the fix is
// mechanical, rewritten — before the build is even attempted. They are not
// security rules, but a package that will not build is not reviewable either,
// and several of them (setuid-style bad backups, unknown options) are hygiene
// issues in their own right.

// makepkg's PKGBUILD schema (util/schema.sh). pkgname is deliberately ABSENT
// from schemaArrayVars: makepkg accepts both scalar `pkgname=foo` and the array
// form, so its type is never checked — only its characters are (PB701). Every
// other array field errors the build when assigned a scalar.
var hashAlgoSums = []string{
	"cksums", "md5sums", "sha1sums", "sha224sums",
	"sha256sums", "sha384sums", "sha512sums", "b2sums",
}

var schemaArrayVars = append([]string{
	"arch", "backup", "checkdepends", "conflicts", "depends", "groups",
	"license", "makedepends", "noextract", "optdepends", "options",
	"provides", "replaces", "source", "validpgpkeys", "xdata",
}, hashAlgoSums...)

var schemaStringVars = []string{
	"changelog", "epoch", "install", "pkgbase", "pkgdesc", "pkgrel", "pkgver", "url",
}

// schemaArchArrayVars are the schema arrays makepkg also recognizes with an
// _$arch suffix for each declared architecture (util/schema.sh
// pkgbuild_schema_arch_arrays).
var schemaArchArrayVars = append([]string{
	"checkdepends", "conflicts", "depends", "makedepends", "optdepends",
	"options", "provides", "replaces", "source",
}, hashAlgoSums...)

// packageOverrideVars are the schema variables makepkg lets a package_*()
// function override (util/schema.sh pkgbuild_schema_package_overrides). Setting
// any other schema variable inside a package function is an error.
var packageOverrideVars = map[string]bool{
	"pkgdesc": true, "arch": true, "url": true, "license": true, "groups": true,
	"depends": true, "optdepends": true, "provides": true, "conflicts": true,
	"replaces": true, "backup": true, "options": true, "install": true, "changelog": true,
}

// validOptions are the accepted options=() entries: makepkg's packaging_options
// plus build_options. Each is also valid with a leading "!" (disable).
var validOptions = map[string]bool{
	"docs": true, "emptydirs": true, "libtool": true, "pestrip": true, "purge": true,
	"staticlibs": true, "strip": true, "debug": true, "zipkmod": true, "zipman": true,
	"buildflags": true, "makeflags": true, "lto": true, "ccache": true, "distcc": true,
}

func correctnessRules() []Rule {
	return []Rule{
		{
			ID:       "PB701",
			Name:     "invalid-pkgname",
			Severity: Error,
			Doc: "pkgname and pkgbase may only contain the characters makepkg allows " +
				"([A-Za-z0-9@._+-]), must be ASCII, and must not start with a hyphen or dot. makepkg " +
				"refuses to build a package whose name breaks these rules.",
			Check: checkPkgName,
			Bad:   `pkgname=foo:bar   # ':' is not allowed by makepkg`,
			Good:  `pkgname=foo-bar`,
			Refs:  []Ref{refGeneralNaming, pkgbuildMan("pkgname", manDirectives)},
		},
		{
			ID:       "PB702",
			Name:     "invalid-pkgver",
			Severity: Error,
			Doc: "pkgver must not be empty and may not contain colons, forward slashes, hyphens or " +
				"whitespace — those characters have meaning in pacman's version syntax (epoch:, -pkgrel), " +
				"so makepkg rejects them. makepkg lints the literal value before any pkgver() function " +
				"runs, so even VCS packages need a valid placeholder.",
			Check: checkPkgVer,
			Bad:   `pkgver=1.2.3-beta   # '-' separates pkgver from pkgrel and is not allowed here`,
			Good:  `pkgver=1.2.3.beta`,
			Refs:  []Ref{refGeneralVersioning, pkgbuildMan("pkgver", manDirectives)},
		},
		{
			ID:       "PB703",
			Name:     "invalid-pkgrel",
			Severity: Error,
			Doc:      "pkgrel must be of the form 'integer[.integer]' (e.g. 1 or 1.5). makepkg errors otherwise.",
			Check:    checkPkgRel,
			Bad:      `pkgrel=1a   # pkgrel must be an integer, optionally with a .minor suffix`,
			Good:     `pkgrel=1`,
			Refs:     []Ref{refGeneralVersioning, pkgbuildMan("pkgrel", manDirectives)},
		},
		{
			ID:       "PB704",
			Name:     "invalid-epoch",
			Severity: Error,
			Doc:      "epoch, when set, must be a non-negative integer; makepkg refuses any other value.",
			Check:    checkEpoch,
			Bad:      `epoch=1.0   # epoch must be a non-negative integer`,
			Good:     `epoch=1`,
			Refs:     []Ref{pkgbuildMan("epoch", manDirectives)},
		},
		{
			ID:       "PB705",
			Name:     "backup-leading-slash",
			Severity: Error,
			Doc: "backup entries are paths relative to the filesystem root, so a leading slash is an " +
				"error makepkg rejects. Drop it: `etc/foo.conf`, not `/etc/foo.conf`.",
			Check:    checkBackupSlash,
			FixLevel: FixSafe,
			Fix:      fixBackupSlash,
			Bad:      `backup=('/etc/foo.conf')   # backup paths are relative to /`,
			Good:     `backup=('etc/foo.conf')`,
			Refs:     []Ref{pkgbuildMan("backup", manDirectives)},
		},
		{
			ID:       "PB706",
			Name:     "unknown-option",
			Severity: Error,
			Doc: "The options array only accepts makepkg's known toggles (strip, debug, lto, " +
				"staticlibs, emptydirs, …), each optionally prefixed with '!'. An unrecognized entry is " +
				"usually a typo that silently does nothing; makepkg flags it as an error.",
			Check: checkUnknownOptions,
			Bad:   `options=('!striped')   # typo: not a known makepkg option`,
			Good:  `options=('!strip')`,
			Refs:  []Ref{pkgbuildMan("options", manDirectives)},
		},
		{
			ID:       "PB707",
			Name:     "provides-comparison",
			Severity: Error,
			Doc: "provides entries may pin an exact version with '=' but must not use '<' or '>' " +
				"comparisons — a provide is a concrete capability, not a range. makepkg errors on comparison operators.",
			Check: checkProvidesComparison,
			Bad:   `provides=('libfoo<2')   # a provide is a concrete capability, not a range`,
			Good:  `provides=('libfoo=1.9')`,
			Refs:  []Ref{pkgbuildMan("provides", manDirectives)},
		},
		{
			ID:       "PB708",
			Name:     "variable-type",
			Severity: Error,
			Doc: "makepkg requires list fields (depends, source, sums, license, options, …) to be " +
				"arrays and scalar fields (pkgver, url, pkgdesc, …) to be plain strings. A bare " +
				"`depends=foo` builds by luck at best and errors under makepkg; wrap it: `depends=(foo)`.",
			Check:    checkVariableTypes,
			FixLevel: FixSafe,
			Fix:      fixVariableTypes,
			Bad:      `depends=gtk3   # list fields must be arrays`,
			Good:     `depends=('gtk3')`,
		},
		{
			ID:       "PB709",
			Name:     "package-function-variable",
			Severity: Error,
			Doc: "A package_*() function may only override packaging metadata (pkgdesc, depends, " +
				"optdepends, provides, backup, install, …). Setting pkgver, source, makedepends, or the " +
				"checksums there is an error: those are resolved before packaging runs.",
			Check: checkPackageFunctionVars,
			Bad: `package() {
  makedepends=('go')   # makedepends can't be overridden per-package
  make DESTDIR="$pkgdir" install
}`,
			Good: `makedepends=('go')
package() {
  depends=('glibc')   # only packaging metadata may be set here
  make DESTDIR="$pkgdir" install
}`,
			Refs: []Ref{pkgbuildMan("package splitting", "PACKAGE_SPLITTING")},
		},
		{
			ID:       "PB710",
			Name:     "invalid-arch",
			Severity: Error,
			Doc: "arch must be set and non-empty, must not combine 'any' with concrete architectures, " +
				"must not repeat a value, and each entry may only contain [A-Za-z0-9_]. makepkg rejects " +
				"a PKGBUILD that violates any of these.",
			Check: checkArch,
			Bad:   `arch=('any' 'x86_64')   # 'any' cannot be combined with concrete architectures`,
			Good:  `arch=('x86_64')`,
		},
		{
			ID:       "PB711",
			Name:     "missing-vcs-makedepends",
			Severity: Warn,
			Doc: "A VCS source (git+…, hg+…) is fetched by the corresponding client, which makepkg " +
				"does not install for you: without the tool in makedepends the build fails on any " +
				"machine that doesn't happen to have it — which is every clean chroot. The auto-fix " +
				"adds the client to makedepends.",
			Check: checkVCSMakedepends,
			// Declaring a tool the sources already need changes nothing about what
			// is built: it restores a requirement the clean chroot was going to
			// enforce anyway.
			FixLevel: FixSafe,
			Fix:      fixVCSMakedepends,
			Bad: `source=("git+https://github.com/example/demo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")
# nothing installs git, so a clean chroot build fails at the fetch step`,
			Good: `makedepends=('git')
source=("git+https://github.com/example/demo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")`,
			Refs: []Ref{refVcsDeps},
		},
	}
}

// --- shared helpers --------------------------------------------------------

func firstValue(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return xs[0]
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// --- PB701: pkgname / pkgbase characters -----------------------------------

func checkPkgName(ctx *Context) []Finding {
	var out []Finding
	path := ctx.Pkg.PKGBUILD.Path
	for _, field := range []string{"pkgname", "pkgbase"} {
		v := ctx.Pkg.Vars[field]
		if v == nil {
			continue
		}
		for _, e := range v.Elems() {
			val, ok := ctx.Pkg.StaticValue(e.Value)
			if !ok {
				continue
			}
			if prob := pkgbuild.NameProblem(val); prob != "" {
				out = append(out, findingAt("PB701", Error, path, e.Pos, "%s %q %s", field, val, prob))
			}
		}
	}
	return out
}

// --- PB702/703/704: pkgver / pkgrel / epoch --------------------------------

var (
	pkgverBadChars = regexp.MustCompile(`[ \t/:-]`)
	pkgrelRe       = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
	digitsRe       = regexp.MustCompile(`^[0-9]+$`)
)

func checkPkgVer(ctx *Context) []Finding {
	// Even with a pkgver() function, makepkg lints the literal value first,
	// so an invalid placeholder still breaks the build.
	v := ctx.Pkg.Vars["pkgver"]
	if v == nil || v.Array { // array type is PB708's concern
		return nil
	}
	val, ok := ctx.Pkg.StaticValue(firstValue(v.Values))
	if !ok {
		return nil
	}
	var msg string
	switch {
	case val == "":
		msg = "must not be empty"
	case pkgverBadChars.MatchString(val):
		msg = "must not contain colons, forward slashes, hyphens, or whitespace"
	case !isASCII(val):
		msg = "may only contain ASCII characters"
	}
	if msg == "" {
		return nil
	}
	return []Finding{findingAt("PB702", Error, ctx.Pkg.PKGBUILD.Path, v.Pos, "pkgver %q %s", val, msg)}
}

func checkPkgRel(ctx *Context) []Finding {
	v := ctx.Pkg.Vars["pkgrel"]
	if v == nil || v.Array {
		return nil
	}
	val, ok := ctx.Pkg.StaticValue(firstValue(v.Values))
	if !ok {
		return nil
	}
	if val == "" {
		return []Finding{findingAt("PB703", Error, ctx.Pkg.PKGBUILD.Path, v.Pos, "pkgrel must not be empty")}
	}
	if !pkgrelRe.MatchString(val) {
		return []Finding{findingAt("PB703", Error, ctx.Pkg.PKGBUILD.Path, v.Pos,
			"pkgrel %q must be of the form 'integer[.integer]'", val)}
	}
	return nil
}

func checkEpoch(ctx *Context) []Finding {
	v := ctx.Pkg.Vars["epoch"]
	if v == nil || v.Array {
		return nil
	}
	val, ok := ctx.Pkg.StaticValue(firstValue(v.Values))
	if !ok || val == "" { // unset/empty epoch is valid (the default)
		return nil
	}
	if !digitsRe.MatchString(val) {
		return []Finding{findingAt("PB704", Error, ctx.Pkg.PKGBUILD.Path, v.Pos,
			"epoch %q must be a non-negative integer", val)}
	}
	return nil
}

// --- PB705: backup leading slash -------------------------------------------

func checkBackupSlash(ctx *Context) []Finding {
	v := ctx.Pkg.Vars["backup"]
	if v == nil {
		return nil
	}
	var out []Finding
	for _, e := range v.Elems() {
		val, ok := ctx.Pkg.StaticValue(e.Value)
		if !ok || !strings.HasPrefix(val, "/") {
			continue
		}
		out = append(out, findingAt("PB705", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"backup entry %q must not have a leading slash (paths are relative to /)", val))
	}
	return out
}

func fixBackupSlash(ctx *Context, _ *FixEnv) []Edit {
	v := ctx.Pkg.Vars["backup"]
	if v == nil {
		return nil
	}
	raw := ctx.Pkg.PKGBUILD.Raw
	path := ctx.Pkg.PKGBUILD.Path
	var edits []Edit
	// A brace group yields one Elem per expansion sharing a single Word, and
	// the edit below is computed from that Word's text — so emit at most one
	// edit per written element or the same byte range would be patched twice.
	patched := map[*syntax.Word]bool{}
	for _, e := range v.Elems() {
		// Only fix var-free literals, so the leading '/' is literally present in Raw.
		if strings.ContainsRune(e.Value, 0) || strings.Contains(e.Value, "$") || !strings.HasPrefix(e.Value, "/") {
			continue
		}
		// A value merged in from a later `+=` or an indexed write has no word
		// of its own here, so there is no byte range to rewrite. The finding
		// still stands; only the fix stands down.
		if e.Word == nil || patched[e.Word] {
			continue
		}
		start, end := off(e.Word.Pos()), off(e.Word.End())
		if start < 0 || end > len(raw) || start >= end {
			continue
		}
		sub := raw[start:end]
		i := bytes.IndexByte(sub, '/')
		if i < 0 {
			continue
		}
		j := i
		for j < len(sub) && sub[j] == '/' {
			j++
		}
		patched[e.Word] = true
		edits = append(edits, Edit{
			Path: path, Start: start + i, End: start + j, New: "",
			Line: int(e.Pos.Line()),
			Desc: "remove leading slash from backup entry",
		})
	}
	return edits
}

// --- PB706: unknown options ------------------------------------------------

func checkUnknownOptions(ctx *Context) []Finding {
	v := ctx.Pkg.Vars["options"]
	if v == nil {
		return nil
	}
	var out []Finding
	for _, e := range v.Elems() {
		val, ok := ctx.Pkg.StaticValue(e.Value)
		if !ok || val == "" {
			continue
		}
		// `\!strip` is a common way to write the negation, and outside an
		// interactive shell the backslash is dropped: bash has no history
		// expansion to escape there, so `\!` is just `!` by the time makepkg
		// reads the array.
		name := strings.TrimPrefix(strings.TrimPrefix(val, `\`), "!")
		if !validOptions[name] {
			out = append(out, findingAt("PB706", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"options array contains unknown option %q", val))
		}
	}
	return out
}

// --- PB707: provides comparison operators ----------------------------------

func checkProvidesComparison(ctx *Context) []Finding {
	v := ctx.Pkg.Vars["provides"]
	if v == nil {
		return nil
	}
	var out []Finding
	for _, e := range v.Elems() {
		val, ok := ctx.Pkg.StaticValue(e.Value)
		if !ok {
			continue
		}
		if strings.ContainsAny(val, "<>") {
			out = append(out, findingAt("PB707", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"provides entry %q cannot use a '<' or '>' comparison operator", val))
		}
	}
	return out
}

// --- PB708: variable array/scalar type -------------------------------------

// concreteArches returns the statically-known arch values other than "any".
func concreteArches(ctx *Context) []string {
	var out []string
	for _, e := range ctx.Pkg.Vars["arch"].Elems() {
		if val, ok := ctx.Pkg.StaticValue(e.Value); ok && val != "" && val != "any" {
			out = append(out, val)
		}
	}
	return out
}

// schemaArrayNames yields every array field name makepkg type-checks: the base
// schema arrays plus the _$arch variants for each declared architecture.
func schemaArrayNames(ctx *Context) []string {
	names := append([]string(nil), schemaArrayVars...)
	for _, a := range concreteArches(ctx) {
		for _, n := range schemaArchArrayVars {
			names = append(names, n+"_"+a)
		}
	}
	return names
}

func checkVariableTypes(ctx *Context) []Finding {
	path := ctx.Pkg.PKGBUILD.Path
	var out []Finding
	for _, name := range schemaArrayNames(ctx) {
		if v := ctx.Pkg.Vars[name]; v != nil && !v.Array {
			out = append(out, findingAt("PB708", Error, path, v.Pos,
				"%s should be an array (makepkg refuses to build otherwise)", name))
		}
	}
	for _, name := range schemaStringVars {
		if v := ctx.Pkg.Vars[name]; v != nil && v.Array {
			out = append(out, findingAt("PB708", Error, path, v.Pos, "%s should not be an array", name))
		}
	}
	return out
}

func fixVariableTypes(ctx *Context, _ *FixEnv) []Edit {
	path := ctx.Pkg.PKGBUILD.Path
	raw := ctx.Pkg.PKGBUILD.Raw
	var edits []Edit
	for _, name := range schemaArrayNames(ctx) {
		v := ctx.Pkg.Vars[name]
		if v == nil || v.Array || v.Assign == nil || v.Assign.Value == nil {
			continue
		}
		// `name[i]=val` is already an array-element write; wrapping the value
		// would produce `name[i]=(val)`, which bash rejects ("cannot assign
		// list to array member"). Such a Var is Array=true and unreachable
		// here, but a fixer must never be one refactor away from emitting
		// syntax errors.
		if v.Assign.Index != nil {
			continue
		}
		// A scalar assignment never word-splits, but an unquoted expansion
		// inside an array does; only wrap values that are fully static.
		if hasVarRef(firstValue(v.Values)) {
			continue
		}
		vs, ve := off(v.Assign.Value.Pos()), off(v.Assign.Value.End())
		if vs < 0 || ve > len(raw) || vs > ve {
			continue
		}
		edits = append(edits, Edit{
			Path: path, Start: vs, End: ve, New: "(" + string(raw[vs:ve]) + ")",
			Line: int(v.Assign.Value.Pos().Line()),
			Desc: "wrap " + name + " value in an array",
		})
	}
	return edits
}

// --- PB709: schema variables set inside package functions -------------------

func checkPackageFunctionVars(ctx *Context) []Finding {
	u := ctx.Pkg.PKGBUILD
	// Non-override schema variables plus their _$arch variants; makepkg
	// rejects both forms inside a package function.
	disallowed := map[string]bool{}
	for _, name := range append(append([]string(nil), schemaArrayVars...), schemaStringVars...) {
		if packageOverrideVars[name] {
			continue
		}
		disallowed[name] = true
		for _, a := range concreteArches(ctx) {
			disallowed[name+"_"+a] = true
		}
	}
	var out []Finding
	for name, fd := range u.Functions {
		if name != "package" && !strings.HasPrefix(name, "package_") {
			continue
		}
		// makepkg greps the function body for bare `name=` / `name+=`
		// assignments (util/pkgbuild.sh:extract_function_variable). Prefixed
		// forms — local/declare/export/readonly — are DeclClause nodes here,
		// not CallExpr.Assigns, so matching only the latter mirrors makepkg and
		// avoids flagging a local shadow. Walk descends into nested blocks, as
		// makepkg's grep-the-whole-body does.
		syntax.Walk(fd, func(node syntax.Node) bool {
			call, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}
			for _, as := range call.Assigns {
				if as.Name == nil {
					continue
				}
				if disallowed[as.Name.Value] {
					out = append(out, findingAt("PB709", Error, u.Path, as.Pos(),
						"%s cannot be set inside a package function", as.Name.Value))
				}
			}
			return true
		})
	}
	return out
}

// --- PB710: arch ------------------------------------------------------------

var archBadChars = regexp.MustCompile(`[^A-Za-z0-9_]`)

func checkArch(ctx *Context) []Finding {
	path := ctx.Pkg.PKGBUILD.Path
	v := ctx.Pkg.Vars["arch"]
	if v == nil {
		// `if [[ $CARCH == x86_64 ]]; then arch=('x86_64'); else arch=('any'); fi`
		// sets arch as far as makepkg is concerned; the parser just cannot say
		// to what. Reporting it unset would be wrong in every branch.
		if ctx.Pkg.ConditionalVars["arch"] {
			return nil
		}
		return []Finding{{RuleID: "PB710", Severity: Error, Path: path, Line: 1, Col: 1,
			Message: "arch is not set; makepkg requires it (e.g. arch=('x86_64') or arch=('any'))"}}
	}
	var out []Finding
	seen := map[string]bool{}
	elems := v.Elems()
	anyCount := 0
	for _, e := range elems {
		val, ok := ctx.Pkg.StaticValue(e.Value)
		if !ok {
			continue // present but not statically validatable
		}
		if val == "any" {
			anyCount++
		}
		if seen[val] {
			out = append(out, findingAt("PB710", Error, path, e.Pos, "arch contains duplicate value %q", val))
		}
		seen[val] = true
		if val != "" && archBadChars.MatchString(val) {
			out = append(out, findingAt("PB710", Error, path, e.Pos, "arch value %q contains invalid characters", val))
		}
	}
	// An `arch=()` a top-level conditional later appends to is only empty in
	// the text; CountUnknown is how the parser says it could not follow.
	if len(elems) == 0 && !v.CountUnknown {
		out = append(out, findingAt("PB710", Error, path, v.Pos, "arch is not allowed to be empty"))
	}
	if anyCount > 0 && len(elems) > 1 {
		out = append(out, findingAt("PB710", Error, path, v.Pos,
			"the 'any' architecture cannot be combined with other architectures"))
	}
	return out
}

// --- PB711: VCS sources need their client in makedepends --------------------

// vcsClientPackages maps a VCS proto to the Arch package shipping its client.
var vcsClientPackages = map[string]string{
	"git": "git", "hg": "mercurial", "svn": "subversion", "bzr": "breezy", "fossil": "fossil",
}

// vcsClientGap is one VCS client package the sources need and no dependency
// array declares, anchored at the first source that needs it.
type vcsClientGap struct {
	VCS  string
	Tool string
	Pos  syntax.Pos
}

// vcsClientGaps returns those packages in source order, once each: the entries
// PB711 reports, and the ones its fix writes into makedepends.
func vcsClientGaps(ctx *Context) []vcsClientGap {
	have := map[string]bool{}
	for _, field := range []string{"depends", "makedepends"} {
		for name := range depsFor(ctx, field) {
			have[name] = true
		}
	}
	var out []vcsClientGap
	reported := map[string]bool{}
	for _, e := range ctx.Pkg.Sources() {
		tool, ok := vcsClientPackages[e.VCS]
		if !ok || have[tool] || reported[tool] {
			continue
		}
		reported[tool] = true
		out = append(out, vcsClientGap{VCS: e.VCS, Tool: tool, Pos: e.Pos})
	}
	return out
}

func checkVCSMakedepends(ctx *Context) []Finding {
	var out []Finding
	for _, g := range vcsClientGaps(ctx) {
		out = append(out, findingAt("PB711", Warn, ctx.Pkg.PKGBUILD.Path, g.Pos,
			"%s source needs %q in makedepends; a clean build environment does not have it", g.VCS, g.Tool))
	}
	return out
}
