package rules

import (
	"fmt"
	"strconv"
	"strings"
)

// PB6xx: metadata consistency and trust. AUR helpers show users .SRCINFO
// metadata while makepkg executes the PKGBUILD — when the two disagree, the
// user reviewed something other than what runs — and package metadata can
// also claim identities (provides/replaces) it has no business claiming.
var consistencyRules = []Rule{
	{
		ID:       "PB601",
		Name:     "srcinfo-mismatch",
		Severity: Warn,
		Doc: ".SRCINFO is what the AUR web interface and helpers display; the PKGBUILD is what " +
			"executes. A mismatch means reviewers saw different metadata than the build uses — " +
			"sometimes a stale regeneration, sometimes deliberate misdirection.",
		Check: checkSrcInfoMatch,
	},
	{
		ID:       "PB602",
		Name:     "network-in-pkgver",
		Severity: Warn,
		Doc: "pkgver() should derive a version from the already-fetched sources (git describe, " +
			"cat VERSION). Downloading inside pkgver() executes network-dependent code on every " +
			"makepkg invocation, including ones the user thought were offline.",
		Check: checkPkgverNetwork,
	},
	{
		ID:       "PB603",
		Name:     "core-package-claim",
		Severity: Warn,
		Doc: "A provides, replaces or conflicts entry naming a core system package hijacks the " +
			"dependency graph: installing the package silently satisfies dependencies meant for the " +
			"real pacman/glibc/systemd, or removes the real one outright. Variant packages (foo-git, " +
			"foo-bin) legitimately provide their parent, so those are exempt — anything else claiming " +
			"a core identity deserves a hard look.",
		Check: checkCoreClaims,
	},
}

// coreSystemPackages are packages whose identity being claimed via
// provides/replaces/conflicts would compromise the system's package manager,
// trust anchors, privilege boundaries, or boot chain. Deliberately small:
// every entry is a package no AUR upload has a plausible reason to supplant.
var coreSystemPackages = map[string]bool{
	"pacman": true, "archlinux-keyring": true,
	"glibc": true, "filesystem": true, "bash": true, "coreutils": true,
	"util-linux": true, "shadow": true,
	"systemd": true, "systemd-libs": true, "dbus": true,
	"sudo": true, "polkit": true, "openssh": true,
	"openssl": true, "gnupg": true, "ca-certificates": true, "curl": true,
	"linux": true, "linux-lts": true, "linux-firmware": true,
	"grub": true, "mkinitcpio": true,
}

// aurVariantSuffixes are the conventional AUR variant markers stripped from a
// pkgname before comparing: pacman-git providing pacman is the variant
// convention working as intended, not a hijack.
var aurVariantSuffixes = []string{"-git", "-svn", "-hg", "-bzr", "-bin", "-nightly"}

func stripVariantSuffix(name string) string {
	for changed := true; changed; {
		changed = false
		for _, s := range aurVariantSuffixes {
			if rest, ok := strings.CutSuffix(name, s); ok && rest != "" {
				name, changed = rest, true
			}
		}
	}
	return name
}

func checkCoreClaims(ctx *Context) []Finding {
	// Identities this package legitimately owns: its pkgname/pkgbase values,
	// plus their variant-suffix-stripped forms.
	own := map[string]bool{}
	for _, field := range []string{"pkgname", "pkgbase"} {
		for _, e := range varElems(ctx.Pkg.Vars[field]) {
			if val, ok := staticVal(ctx.Pkg, e.Value); ok && val != "" {
				own[val] = true
				own[stripVariantSuffix(val)] = true
			}
		}
	}
	consequences := map[string]string{
		"provides":  "installing it silently satisfies dependencies meant for the real package",
		"replaces":  "pacman replaces the real package with this one on the next sysupgrade",
		"conflicts": "installing it forces the real package to be removed",
	}
	var out []Finding
	for name, v := range ctx.Pkg.Vars {
		field, _, _ := strings.Cut(name, "_") // provides_x86_64 -> provides
		why, ok := consequences[field]
		if !ok {
			continue
		}
		for _, e := range varElems(v) {
			val, ok := staticVal(ctx.Pkg, e.Value)
			if !ok || val == "" {
				continue
			}
			target := val
			if i := strings.IndexAny(target, "<>="); i >= 0 {
				target = target[:i]
			}
			if !coreSystemPackages[target] || own[target] {
				continue
			}
			out = append(out, findingAt("PB603", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"%s entry %q claims core system package %q: %s", field, val, target, why))
		}
	}
	return out
}

func checkSrcInfoMatch(ctx *Context) []Finding {
	si := ctx.Pkg.SrcInfo
	if si == nil {
		return nil
	}
	var out []Finding
	pos := ctx.Pkg.PKGBUILD.File.Pos()
	mismatch := func(field, pkgVal, siVal string) {
		out = append(out, findingAt("PB601", Warn, ctx.Pkg.PKGBUILD.Path, pos,
			"%s differs between PKGBUILD (%q) and .SRCINFO (%q); regenerate .SRCINFO", field, pkgVal, siVal))
	}

	_, hasPkgverFn := ctx.Pkg.PKGBUILD.Functions["pkgver"]
	if !hasPkgverFn { // VCS packages update pkgver dynamically; drift is expected
		if v, ok := ctx.Pkg.Scalar("pkgver"); ok {
			if sv, ok := si.Get("pkgver"); ok && sv != v {
				mismatch("pkgver", v, sv)
			}
		}
		if v, ok := ctx.Pkg.Scalar("pkgrel"); ok {
			if sv, ok := si.Get("pkgrel"); ok && sv != v {
				mismatch("pkgrel", v, sv)
			}
		}
	}
	if v, ok := ctx.Pkg.Scalar("url"); ok && !strings.Contains(v, "$") {
		if sv, ok := si.Get("url"); ok && sv != v {
			mismatch("url", v, sv)
		}
	}

	var pkgbuildSources int
	for _, e := range ctx.Pkg.Sources() {
		_ = e
		pkgbuildSources++
	}
	var srcinfoSources int
	for key, vals := range si.Fields {
		if key == "source" || strings.HasPrefix(key, "source_") {
			srcinfoSources += len(vals)
		}
	}
	if srcinfoSources != pkgbuildSources && pkgbuildSources > 0 {
		mismatch("source count", strconv.Itoa(pkgbuildSources), fmt.Sprint(srcinfoSources))
	}
	return out
}

func checkPkgverNetwork(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		if c.Fn != "pkgver" || c.Unit.Scriptlet {
			continue
		}
		fetches, ok := networkCommands[c.Name]
		if !ok || !fetches(c) {
			continue
		}
		out = append(out, c.finding("PB602", Warn,
			"%s accesses the network inside pkgver(); derive the version from fetched sources instead", c.Name))
	}
	return out
}
