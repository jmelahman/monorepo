package rules

import (
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// Ecosystem predicates shared by the Arch package-guideline rules
// (guidelines.go, pystyle.go, ruststyle.go, buildsys.go, vcsstyle.go,
// naming.go). Nearly every guideline rule is conditional on the package being
// of a kind — named with a prefix, depending on a toolchain, invoking a build
// system — so the "is this that kind of package" questions live here, built on
// the same primitives the other rules already use (varElems, staticVal,
// depsFor), rather than being re-derived per rule.

// pkgnames returns the statically-known package names this PKGBUILD builds:
// every pkgname element whose value is resolvable without running the file.
func pkgnames(ctx *Context) []string {
	var out []string
	for _, e := range varElems(ctx.Pkg.Vars["pkgname"]) {
		if val, ok := staticVal(ctx.Pkg, e.Value); ok && val != "" {
			out = append(out, val)
		}
	}
	return out
}

// allPkgNames returns pkgnames plus the statically-known pkgbase, for the
// prefix/suffix conventions that read through either (as PB103's -git
// exemption does).
func allPkgNames(ctx *Context) []string {
	out := pkgnames(ctx)
	for _, e := range varElems(ctx.Pkg.Vars["pkgbase"]) {
		if val, ok := staticVal(ctx.Pkg, e.Value); ok && val != "" {
			out = append(out, val)
		}
	}
	return out
}

// nameSuffixed reports whether any built package name (pkgname element or
// pkgbase) ends with suffix.
func nameSuffixed(ctx *Context, suffix string) bool {
	for _, n := range allPkgNames(ctx) {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}
	return false
}

// namePrefixed reports whether any built package name (pkgname element or
// pkgbase) starts with one of the prefixes.
func namePrefixed(ctx *Context, prefixes ...string) bool {
	for _, n := range allPkgNames(ctx) {
		if hasPrefixAny(n, prefixes...) {
			return true
		}
	}
	return false
}

// archIsAny reports whether the arch array declares "any".
func archIsAny(ctx *Context) bool {
	for _, e := range varElems(ctx.Pkg.Vars["arch"]) {
		if val, ok := staticVal(ctx.Pkg, e.Value); ok && val == "any" {
			return true
		}
	}
	return false
}

// optionSet reports whether options=() contains opt (e.g. "!strip"),
// tolerating the escaped \! spelling as PB706 does.
func optionSet(ctx *Context, opt string) bool {
	for _, e := range varElems(ctx.Pkg.Vars["options"]) {
		if val, ok := staticVal(ctx.Pkg, e.Value); ok && strings.TrimPrefix(val, `\`) == opt {
			return true
		}
	}
	return false
}

// pkgdescValue returns the statically-known pkgdesc, mirroring
// checkPkgnameInDesc's reading of the field.
func pkgdescValue(ctx *Context) (string, bool) {
	v := ctx.Pkg.Vars["pkgdesc"]
	if v == nil || v.Array {
		return "", false
	}
	return staticVal(ctx.Pkg, firstValue(v.Values))
}

// varFinding reports a finding anchored at the first of fields that is
// declared, falling back to the top of the PKGBUILD — for rules about a
// package-wide property rather than one command or array element.
func varFinding(ctx *Context, id string, sev Severity, fields []string, format string, args ...any) Finding {
	for _, f := range fields {
		if v := ctx.Pkg.Vars[f]; v != nil {
			return findingAt(id, sev, ctx.Pkg.PKGBUILD.Path, v.Pos, format, args...)
		}
	}
	return Finding{RuleID: id, Severity: sev, Path: ctx.Pkg.PKGBUILD.Path, Line: 1, Col: 1,
		Message: message(format, args...)}
}

// varFindingLine is the line varFinding anchors a finding at for fields — for
// a fixer whose edit lands elsewhere but must answer to the same suppression
// directive as the finding it resolves.
func varFindingLine(ctx *Context, fields ...string) int {
	for _, f := range fields {
		if v := ctx.Pkg.Vars[f]; v != nil {
			return int(v.Pos.Line())
		}
	}
	return 1
}

// hasDep reports whether field (or a declared _$arch variant of it) names the
// package, ignoring version constraints.
func hasDep(ctx *Context, field, name string) bool {
	_, ok := depsFor(ctx, field)[name]
	return ok
}

// depEntry is one statically-known dependency-array element with its position,
// for rules that report on the entry itself.
//
// Var and Word address the bytes the entry was written as, so the fixes that
// delete a dependency read the same entries their rule does rather than
// re-walking Vars for them. Word is nil whenever the value has no source text
// of its own — one a later `+=` merged in, or a scalar's — and a fixer must
// leave those alone; every rule that only reports can ignore both fields.
type depEntry struct {
	Name string // version constraint and description stripped
	Full string // as written
	Pos  syntax.Pos
	Var  *pkgbuild.Var
	Word *syntax.Word
}

// depEntries yields the static entries of field and its declared _$arch
// variants. Unlike depsFor it keeps positions and duplicates, so findings can
// point at the entry rather than the array.
func depEntries(ctx *Context, field string) []depEntry {
	var out []depEntry
	names := []string{field}
	for _, a := range concreteArches(ctx) {
		names = append(names, field+"_"+a)
	}
	for _, n := range names {
		v := ctx.Pkg.Vars[n]
		for _, e := range varElems(v) {
			if val, ok := staticVal(ctx.Pkg, e.Value); ok && val != "" {
				out = append(out, depEntry{
					Name: depName(val), Full: val, Pos: e.Pos, Var: v, Word: e.Word,
				})
			}
		}
	}
	return out
}

// pkgdirSubdir reports whether a rendered write target lands under
// $pkgdir<sub> (sub like "/usr/local"), tolerating the ${pkgdir} spelling.
func pkgdirSubdir(target, sub string) bool {
	t := strings.ReplaceAll(target, "${pkgdir}", "$pkgdir")
	return strings.HasPrefix(t, "$pkgdir"+sub)
}

// packagePhaseWrites yields every file-writing command in package() or a
// split package_*() function together with its destination arguments — the
// commands that decide where in $pkgdir the package's files land.
func packagePhaseWrites(ctx *Context) []struct {
	Cmd   Command
	Dests []string
} {
	var out []struct {
		Cmd   Command
		Dests []string
	}
	for _, c := range ctx.Commands() {
		if c.Fn != "package" && !strings.HasPrefix(c.Fn, "package_") {
			continue
		}
		if !fileWriters[c.Name] {
			continue
		}
		if dests := writeDests(c); len(dests) > 0 {
			out = append(out, struct {
				Cmd   Command
				Dests []string
			}{c, dests})
		}
	}
	return out
}
