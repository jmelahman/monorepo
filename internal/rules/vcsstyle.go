package rules

import (
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB960–PB963 lint the Arch VCS package guidelines
// (https://wiki.archlinux.org/title/VCS_package_guidelines): a pkgver()
// function so the version tracks the checkout, provides/conflicts on the
// release counterpart, a stable checkout folder name, and a -git suffix that
// actually means what it promises. The security half of VCS packaging —
// unpinned sources on non-VCS packages, missing VCS clients in makedepends —
// is PB103/PB711's.

// vcsNameSuffixes maps the conventional VCS pkgname suffixes to the VCS whose
// sources they promise.
var vcsNameSuffixes = map[string]string{
	"-git": "git", "-svn": "svn", "-hg": "hg", "-bzr": "bzr",
}

// vcsPinnedFragment returns the fragment key that pins the source to a fixed
// ref ("commit", "tag", "revision"), or "" for a source that follows a tip.
func vcsPinnedFragment(e pkgbuild.SourceEntry) string {
	for _, k := range []string{"commit", "tag", "revision"} {
		if _, ok := e.Fragment[k]; ok {
			return k
		}
	}
	return ""
}

// vcsPinned additionally treats a source containing an opaque expansion as
// possibly pinned: octopi's `${_commit:+#commit=$_commit}` renders to the
// unresolvable-marker NUL, hiding a pin only bash can see. Claiming such a
// source follows tip would be a guess.
func vcsPinned(e pkgbuild.SourceEntry) bool {
	if vcsPinnedFragment(e) != "" {
		return true
	}
	return strings.ContainsRune(e.Raw, 0) || strings.ContainsRune(e.Expanded, 0)
}

// --- PB960: VCS source with no pkgver() --------------------------------------

func checkVCSPkgverFn(ctx *Context) []Finding {
	// localFuncs rather than Unit.Functions: the release-or-git idiom declares
	// pkgver() inside a top-level `if`, and it still runs when makepkg calls
	// the phase.
	if ctx.localFuncs[ctx.Pkg.PKGBUILD.Path+"\x00"+"pkgver"] {
		return nil
	}
	// A pinned VCS source anywhere means versions advance by bumping the pin
	// (megasync's #tag=v$pkgver); an unpinned *helper* checkout beside it is
	// unreviewed drift, which is PB103's finding, not a versioning one.
	for _, e := range ctx.Pkg.Sources() {
		if e.VCS != "" && vcsPinned(e) {
			return nil
		}
	}
	for _, e := range ctx.Pkg.Sources() {
		if e.VCS == "" {
			continue
		}
		// One finding per PKGBUILD: one pkgver() covers every source.
		return []Finding{findingAt("PB960", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"%s source follows upstream tip but there is no pkgver() function, so every build reuses the hardcoded version and pacman never sees an upgrade", e.VCS)}
	}
	return nil
}

// --- PB961: -git package without provides/conflicts on its counterpart -------

// vcsCounterpartGap is one -git style package that declares neither provides
// nor conflicts on the release package it shadows.
type vcsCounterpartGap struct {
	Name string
	Base string
	Pos  syntax.Pos
}

// vcsCounterpartGaps returns those packages: the entries PB961 reports, and
// the ones its fix declares provides and conflicts for.
func vcsCounterpartGaps(ctx *Context) []vcsCounterpartGap {
	// Split -git packages declare provides/conflicts per package_*()
	// function, beyond static reading; claiming they are absent would be a
	// guess against half the data.
	if fieldSetInPackageFns(ctx, "provides") || fieldSetInPackageFns(ctx, "conflicts") {
		return nil
	}
	provides := depsFor(ctx, "provides")
	conflicts := depsFor(ctx, "conflicts")
	var out []vcsCounterpartGap
	reported := map[string]bool{}
	for _, e := range varElems(ctx.Pkg.Vars["pkgname"]) {
		name, ok := staticVal(ctx.Pkg, e.Value)
		if !ok || name == "" || reported[name] {
			continue
		}
		for suffix := range vcsNameSuffixes {
			base, ok := strings.CutSuffix(name, suffix)
			if !ok || base == "" {
				continue
			}
			if _, p := provides[base]; p {
				continue
			}
			if _, c := conflicts[base]; c {
				continue
			}
			reported[name] = true
			out = append(out, vcsCounterpartGap{Name: name, Base: base, Pos: e.Pos})
		}
	}
	return out
}

func checkVCSProvidesConflicts(ctx *Context) []Finding {
	var out []Finding
	for _, g := range vcsCounterpartGaps(ctx) {
		out = append(out, findingAt("PB961", Info, ctx.Pkg.PKGBUILD.Path, g.Pos,
			"%q builds the same software as %q but declares neither provides=(%s) nor conflicts=(%s), so both can be installed at once and nothing can depend on either",
			g.Name, g.Base, g.Base, g.Base))
	}
	return out
}

// --- PB962: $pkgver in the checkout folder name ------------------------------

func checkVCSPkgverInFolder(ctx *Context) []Finding {
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		if e.VCS == "" {
			continue
		}
		folder, _, ok := strings.Cut(e.Raw, "::")
		if !ok {
			continue
		}
		if !strings.Contains(folder, "$pkgver") && !strings.Contains(folder, "${pkgver") {
			continue
		}
		out = append(out, findingAt("PB962", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"$pkgver in the checkout folder name changes every time pkgver() runs, so makepkg abandons the incremental clone and re-fetches the whole repository each build"))
	}
	return out
}

// --- PB963: -git suffix and the sources disagree -----------------------------

func checkVCSSuffixMismatch(ctx *Context) []Finding {
	haveVCS := map[string]bool{}
	for _, e := range ctx.Pkg.Sources() {
		if e.VCS != "" {
			haveVCS[e.VCS] = true
		}
	}
	var out []Finding
	// A suffixed name with no source from that VCS: the package builds a
	// tarball while claiming to follow a repository.
	reported := map[string]bool{}
	for _, e := range varElems(ctx.Pkg.Vars["pkgname"]) {
		name, ok := staticVal(ctx.Pkg, e.Value)
		if !ok || reported[name] {
			continue
		}
		for suffix, vcs := range vcsNameSuffixes {
			if !strings.HasSuffix(name, suffix) || haveVCS[vcs] {
				continue
			}
			reported[name] = true
			out = append(out, findingAt("PB963", Info, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"%q has the %s suffix but no %s source; the suffix promises a build from the repository tip", name, suffix, vcs))
		}
	}
	// The converse: the suffix promises upstream tip, the source pins a ref.
	for _, e := range ctx.Pkg.Sources() {
		if e.VCS == "" {
			continue
		}
		frag := vcsPinnedFragment(e)
		suffix := "-" + e.VCS
		if frag == "" || vcsNameSuffixes[suffix] != e.VCS || !nameSuffixed(ctx, suffix) {
			continue
		}
		out = append(out, findingAt("PB963", Info, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"the %s suffix promises to follow upstream tip, but this source is pinned to a fixed %s; drop the suffix (and version normally) or drop the pin", suffix, frag))
	}
	return out
}
