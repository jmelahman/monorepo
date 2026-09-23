package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// PB1xx: source integrity.
var integrityRules = []Rule{
	{
		ID:       "PB101",
		Name:     "skipped-checksum",
		Severity: Error,
		Doc: "Every remote, non-VCS source should carry a real checksum. `SKIP` means makepkg " +
			"performs no integrity verification at all: if the upstream file or the connection is " +
			"tampered with, nothing notices. Pin the artifact with a sha256 or stronger digest. " +
			"Detached signature files and the artifacts they verify are exempt — there the PGP " +
			"machinery does the verifying, and PB111 flags it when that machinery is unanchored.",
		Check: checkSkippedChecksums,
	},
	{
		ID:       "PB102",
		Name:     "weak-checksum",
		Severity: Warn,
		Doc: "md5 and sha1 are broken for collision resistance. When they are the only digests " +
			"present, an attacker able to produce collisions can substitute the source artifact. " +
			"Add sha256sums, sha512sums or b2sums. --fix adds them for sources already downloaded " +
			"into the package directory or makepkg's source cache, and only after re-checking the " +
			"weak digest against those same bytes; it never downloads a source to do so, so a " +
			"package whose sources are not present keeps the finding and `updpkgsums` remains the " +
			"way to close it.",
		Check:    checkWeakChecksums,
		FixLevel: FixSafe,
		Fix:      fixWeakChecksums,
	},
	{
		ID:       "PB103",
		Name:     "unpinned-vcs-source",
		Severity: Warn,
		Doc: "A VCS source without `#commit=` pins to a mutable reference: branches move and tags " +
			"can be re-pointed by anyone with push access (or a compromised forge account). Pinning " +
			"the exact commit hash makes the fetched tree tamper-evident. Packages named for the VCS " +
			"they track (`foo-git`, `foo-hg`, …) are exempt while they follow a branch: building " +
			"whatever upstream's tip points at today is what such a package is for, and pinning it " +
			"would make it a different package. A mutable `#tag=` is still reported there — a tag is " +
			"not the tip, and it can be re-pointed.",
		Check:    checkVCSPins,
		FixLevel: FixSafe,
		Fix:      fixVCSPins,
	},
	{
		ID:          "PB104",
		Name:        "insecure-transport",
		Severity:    Warn,
		MaxSeverity: Error,
		Doc: "Sources fetched over http://, git:// or ftp:// can be modified in transit. Pinned to a " +
			"strong digest the download is still verified, so an attacker in the path can break the " +
			"build but not change what it produces — a warning. With SKIP, a weak sum, or no sum at " +
			"all — which includes every git:// clone — nothing checks what arrives, and it is a " +
			"working man-in-the-middle vector: an error. Use https:// (or git+https://) either way. " +
			"Signature files fetched insecurely are PB112's concern. --unsafe-fix rewrites the " +
			"scheme (http and ftp to https, bare git:// to git+https://), but only after checking that " +
			"the https URL answers — a headers-only request, or `git ls-remote` for a git remote — so " +
			"a host that does not serve it keeps the finding instead of getting a broken build. It stays " +
			"unsafe because reachable is not identical: only the checksum can say the https URL serves " +
			"the same bytes, so rebuild after applying it. --offline skips it.",
		Check:    checkInsecureTransport,
		FixLevel: FixUnsafe,
		Fix:      fixInsecureTransport,
	},
	{
		ID:       "PB105",
		Name:     "source-url-mismatch",
		Severity: Info,
		Doc: "A source hosted on a different domain than the project's `url` is worth a second " +
			"look: repackaged or 'mirrored' artifacts are a common way to slip in modified binaries. " +
			"Often legitimate (CDNs, release mirrors) — hence only informational.",
		Check: checkSourceDomains,
	},
	{
		ID:       "PB106",
		Name:     "dlagents-override",
		Severity: Warn,
		Doc: "Overriding DLAGENTS replaces makepkg's download logic with arbitrary commands, which " +
			"run before any checksum is verified. Legitimate uses exist but are rare enough that " +
			"every override deserves review.",
		Check: checkDLAgents,
	},
	{
		ID:       "PB107",
		Name:     "missing-install-script",
		Severity: Warn,
		Doc: "The PKGBUILD references an install scriptlet that is not present next to it, so its " +
			"contents cannot be reviewed or linted.",
		Check: checkMissingInstall,
	},
	{
		ID:       "PB108",
		Name:     "makepkg-config-override",
		Severity: Warn, MaxSeverity: Critical, // critical for the variables makepkg executes
		Doc: "Assigning a makepkg.conf variable at the top level reconfigures makepkg itself. " +
			"VCSCLIENTS and the COMPRESS* arrays are executed as commands, so overriding them injects " +
			"code into the fetch and packaging steps; PACKAGER/GPGKEY spoof package identity and " +
			"BUILDENV/INTEGRITY_CHECK can silently disable verification. Like DLAGENTS (PB106), these " +
			"belong in makepkg.conf, never in a package.",
		Check: checkMakepkgConfOverride,
	},
	{
		ID:       "PB109",
		Name:     "forge-owner-mismatch",
		Severity: Warn,
		Doc: "A source hosted on the same forge as the project url but under a different owner is a " +
			"common repackaging vector: recent AUR compromises kept github.com and changed only the " +
			"account in the path, which a host-only comparison (PB105) misses. Often a legitimate fork " +
			"or mirror — hence a warning, not an error.",
		Check: checkForgeOwner,
	},
	{
		ID:       "PB110",
		Name:     "checksum-count-mismatch",
		Severity: Error,
		Doc: "makepkg pairs each source with the checksum at the same index, so a sums array of the " +
			"wrong length means some sources are unverified (or verified against the wrong digest). " +
			"makepkg errors on this; regenerate with updpkgsums.",
		Check: checkChecksumCounts,
	},
	{
		ID:       "PB111",
		Name:     "signature-without-key",
		Severity: Error,
		Doc: "A detached signature (.sig/.asc) or a ?signed VCS source is only as strong as the key " +
			"it is checked against. With validpgpkeys empty, makepkg accepts any key already present " +
			"in the builder's keyring — whatever the user was last told to import — instead of a " +
			"maintainer-pinned fingerprint, so the strongest verification the PKGBUILD declares is " +
			"never actually anchored. Pin the upstream signing key's full fingerprint in validpgpkeys.",
		Check: checkSignatureKeys,
	},
	{
		ID:       "PB112",
		Name:     "insecure-signature-transport",
		Severity: Warn,
		Doc: "This signature file is fetched over an unencrypted transport. With validpgpkeys pinned " +
			"the signature still verifies cryptographically — hence a warning where PB104 errors for " +
			"ordinary sources — but a man-in-the-middle can strip or swap the file to break builds, " +
			"and combined with an unpinned key (PB111) it is a working bypass. Use https://. " +
			"--unsafe-fix rewrites the scheme once it has confirmed the https URL answers, with " +
			"PB104's caveat: a reachable signature is not necessarily the same signature.",
		Check:    checkSignatureTransport,
		FixLevel: FixUnsafe,
		Fix:      fixInsecureSignatureTransport,
	},
	{
		ID:       "PB113",
		Name:     "unused-validpgpkeys",
		Severity: Info,
		Doc: "validpgpkeys pins signing keys, but no source carries a detached signature (.sig/.asc) " +
			"and no VCS source requests ?signed verification, so nothing is ever checked against the " +
			"keys. Either dead configuration or a signature source was dropped — confusing at review " +
			"time either way.",
		Check: checkUnusedPGPKeys,
	},
	{
		ID:       "PB114",
		Name:     "malformed-checksum",
		Severity: Error,
		Doc: "A checksum with the wrong length for its algorithm or with non-hex characters can " +
			"never match any download, so verification is guaranteed to fail — usually a truncated " +
			"paste or a digest computed with a different algorithm than the array names.",
		Check: checkMalformedChecksums,
	},
}

// signatureExts are the filename extensions makepkg's check_pgpsigs treats as
// detached PGP signatures.
var signatureExts = []string{".sig", ".asc", ".sign"}

// effectiveFilename is the local name makepkg gives a fetched source entry.
func effectiveFilename(e pkgbuild.SourceEntry) string {
	if e.Filename != "" {
		return e.Filename
	}
	return basename(e.URL)
}

// isSignatureSource reports whether the entry is a detached PGP signature.
func isSignatureSource(e pkgbuild.SourceEntry) bool {
	name := strings.ToLower(effectiveFilename(e))
	for _, ext := range signatureExts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// signatureTarget returns the filename a signature source verifies (its own
// name minus the signature extension).
func signatureTarget(e pkgbuild.SourceEntry) string {
	name := effectiveFilename(e)
	lower := strings.ToLower(name)
	for _, ext := range signatureExts {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return ""
}

// isSignedVCS reports whether the entry is a VCS source with ?signed
// verification of its tag/commit.
func isSignedVCS(e pkgbuild.SourceEntry) bool {
	return e.VCS != "" && strings.Contains(e.Query, "signed")
}

// pgpKeysPinned reports whether validpgpkeys declares at least one key.
func (ctx *Context) pgpKeysPinned() bool {
	v, ok := ctx.Pkg.Vars["validpgpkeys"]
	if !ok {
		return false
	}
	for _, val := range v.Values {
		if strings.TrimSpace(val) != "" {
			return true
		}
	}
	return false
}

func checkSignatureKeys(ctx *Context) []Finding {
	if ctx.pgpKeysPinned() {
		return nil
	}
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		switch {
		case isSignatureSource(e):
			out = append(out, findingAt("PB111", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"signature %q is declared but validpgpkeys is empty: makepkg checks it against whatever "+
					"key the builder's keyring happens to hold, not a maintainer-pinned one", e.Raw))
		case isSignedVCS(e):
			out = append(out, findingAt("PB111", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"VCS source %q requests ?signed verification but validpgpkeys is empty: any key in the "+
					"builder's keyring satisfies it", e.Raw))
		}
	}
	return out
}

func checkSignatureTransport(ctx *Context) []Finding {
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		if e.Local || !isSignatureSource(e) {
			continue
		}
		if proto, insecure := insecureProto(e); insecure {
			out = append(out, findingAt("PB112", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"signature %q is fetched over unencrypted %s://; the signature itself can be stripped or swapped in transit", e.Raw, proto))
		}
	}
	return out
}

func checkUnusedPGPKeys(ctx *Context) []Finding {
	if !ctx.pgpKeysPinned() {
		return nil
	}
	for _, e := range ctx.Pkg.Sources() {
		if isSignatureSource(e) || isSignedVCS(e) {
			return nil
		}
	}
	v := ctx.Pkg.Vars["validpgpkeys"]
	return []Finding{findingAt("PB113", Info, ctx.Pkg.PKGBUILD.Path, v.Pos,
		"validpgpkeys is set but no source has a detached signature (.sig/.asc) and no VCS source uses ?signed; nothing is verified against these keys")}
}

// makepkgConfExec are makepkg.conf variables whose values makepkg executes as
// commands; overriding them at the package level is code injection.
var makepkgConfExec = map[string]string{
	"VCSCLIENTS":  "makepkg runs VCSCLIENTS entries to fetch VCS sources",
	"COMPRESSZST": "makepkg runs the COMPRESS* command to package the build",
	"COMPRESSGZ":  "makepkg runs the COMPRESS* command to package the build",
	"COMPRESSXZ":  "makepkg runs the COMPRESS* command to package the build",
	"COMPRESSBZ2": "makepkg runs the COMPRESS* command to package the build",
	"COMPRESSLRZ": "makepkg runs the COMPRESS* command to package the build",
	"COMPRESSLZO": "makepkg runs the COMPRESS* command to package the build",
	"COMPRESSZ":   "makepkg runs the COMPRESS* command to package the build",
	"COMPRESSLZ4": "makepkg runs the COMPRESS* command to package the build",
	"COMPRESSLZ":  "makepkg runs the COMPRESS* command to package the build",
	"PACMAN_AUTH": "PACMAN_AUTH is run to escalate privileges during packaging",
}

// makepkgConfTrust are makepkg.conf variables that affect package identity,
// output location, or verification when overridden at the package level.
var makepkgConfTrust = map[string]bool{
	"PACKAGER": true, "GPGKEY": true, "BUILDENV": true, "INTEGRITY_CHECK": true,
	"PKGEXT": true, "SRCEXT": true, "PKGDEST": true, "SRCDEST": true, "SRCPKGDEST": true,
	"BUILDDIR": true, "OPTIONS": true, "PURGE_TARGETS": true, "PACMAN": true,
	"STRIP_BINARIES": true, "STRIP_SHARED": true, "STRIP_STATIC": true,
}

func checkMakepkgConfOverride(ctx *Context) []Finding {
	var out []Finding
	for name, v := range ctx.Pkg.Vars {
		if why, ok := makepkgConfExec[name]; ok {
			out = append(out, findingAt("PB108", Critical, ctx.Pkg.PKGBUILD.Path, v.Pos,
				"%s is set at the package level: %s", name, why))
		} else if makepkgConfTrust[name] {
			out = append(out, findingAt("PB108", Warn, ctx.Pkg.PKGBUILD.Path, v.Pos,
				"%s overrides a makepkg.conf setting from within the package", name))
		}
	}
	return out
}

var knownForges = map[string]bool{
	"github.com": true, "gitlab.com": true, "codeberg.org": true,
	"bitbucket.org": true, "git.sr.ht": true,
}

// forgeOwner returns the lowercased owner segment of a forge URL path.
func forgeOwner(rawurl string) (string, bool) {
	_, rest, ok := strings.Cut(rawurl, "://")
	if !ok {
		return "", false
	}
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return "", false
	}
	parts := strings.Split(strings.Trim(rest[slash:], "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", false
	}
	return strings.ToLower(strings.TrimSuffix(parts[0], ".git")), true
}

func checkForgeOwner(ctx *Context) []Finding {
	urlVal, ok := ctx.Pkg.Scalar("url")
	if !ok {
		return nil
	}
	urlHost := hostOf(urlVal)
	if !knownForges[urlHost] {
		return nil
	}
	urlOwner, ok := forgeOwner(urlVal)
	if !ok {
		return nil
	}
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		if e.Host() != urlHost {
			continue // different host is PB105's concern
		}
		owner, ok := forgeOwner(e.URL)
		if !ok || owner == urlOwner {
			continue
		}
		out = append(out, findingAt("PB109", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"source is under %q on %s but the project url is under %q; verify this is the same project",
			owner, urlHost, urlOwner))
	}
	return out
}

func checkChecksumCounts(ctx *Context) []Finding {
	counts := map[string]int{}
	for _, e := range ctx.Pkg.Sources() {
		counts[e.Arch]++
	}
	var out []Finding
	for arch, n := range counts {
		srcName := "source"
		if arch != "" {
			srcName += "_" + arch
		}
		// An array reference the parser could not size ("${_files[@]}" with
		// _files built dynamically) makes the source count meaningless; a
		// mismatch claim would be a guess.
		if v := ctx.Pkg.Vars[srcName]; v != nil && v.CountUnknown {
			continue
		}
		sums := ctx.Pkg.Checksums(arch)
		// Whether any algorithm actually covers this source array. makepkg
		// only needs one: check_checksums verifies each algo in turn and is
		// satisfied as soon as one has the same length as the sources.
		covered := false
		for _, vals := range sums {
			if len(vals) == n {
				covered = true
				break
			}
		}
		for _, algo := range sumAlgoNames {
			vals, ok := sums[algo]
			if !ok || len(vals) == n {
				continue
			}
			// An empty array declares the algorithm unused rather than short —
			// makepkg's verify_integrity_sums only raises "differ in size" for
			// a non-empty one — so `md5sums=()` beside a filled sha256sums is
			// a legitimate way to retire an algorithm. With nothing else
			// covering the sources it is still a failure, just the "integrity
			// checks are missing" one.
			if len(vals) == 0 && covered {
				continue
			}
			name := algo + "sums"
			if arch != "" {
				name += "_" + arch
			}
			v := ctx.Pkg.Vars[name]
			if v.CountUnknown {
				continue // the sums array's own length is not statically known
			}
			out = append(out, findingAt("PB110", Error, ctx.Pkg.PKGBUILD.Path, v.Pos,
				"%s has %d checksum(s) but there are %d source(s) for this arch; makepkg pairs them by index",
				name, len(vals), n))
		}
	}
	return out
}

func isSkip(s string) bool { return strings.EqualFold(s, "SKIP") }

func checkSkippedChecksums(ctx *Context) []Finding {
	// Filenames covered by a detached signature: those artifacts are verified
	// by the signature (SKIP is the convention), and the signature file itself
	// never gets a meaningful digest. PB111 owns the case where the signature
	// is not anchored to a pinned key.
	sigVerified := map[string]bool{}
	for _, e := range ctx.Pkg.Sources() {
		if t := signatureTarget(e); t != "" {
			sigVerified[t] = true
		}
	}
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		// file:// reads a file already on the machine — ttf-ms-win10-auto
		// names fonts it extracts from a Windows image — so nothing is
		// downloaded for a checksum to verify.
		if e.VCS != "" || e.Local || e.Proto == "file" {
			continue
		}
		if isSignatureSource(e) || sigVerified[effectiveFilename(e)] {
			continue
		}
		sums := ctx.Pkg.SumsFor(e)
		allSkip := true
		for _, s := range sums {
			if !isSkip(s) {
				allSkip = false
			}
		}
		if len(sums) == 0 || allSkip {
			out = append(out, findingAt("PB101", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"remote source %q has no checksum (SKIP): the download is never verified", e.Raw))
		}
	}
	return out
}

func checkWeakChecksums(ctx *Context) []Finding {
	var out []Finding
	for _, arch := range ctx.archesWithSums() {
		sums := ctx.Pkg.Checksums(arch)
		if hasStrongSum(sums) {
			continue
		}
		for _, algo := range []string{"ck", "md5", "sha1"} {
			if hasRealSum(sums[algo]) {
				name := sumsName(algo, arch)
				v := ctx.Pkg.Vars[name]
				out = append(out, findingAt("PB102", Warn, ctx.Pkg.PKGBUILD.Path, v.Pos,
					"%s is the strongest digest present; add sha256sums or b2sums", name))
			}
		}
	}
	return out
}

// hasStrongSum reports whether a Checksums map carries any digest strong
// enough to satisfy PB102. The fixer shares it so that what clears the rule
// and what the fix aims to produce cannot drift apart.
func hasStrongSum(sums map[string][]string) bool {
	for _, algo := range []string{"sha224", "sha256", "sha384", "sha512", "b2"} {
		if len(sums[algo]) > 0 {
			return true
		}
	}
	return false
}

// hasRealSum reports whether the array carries at least one actual digest, as
// opposed to being empty or entirely SKIP.
func hasRealSum(vals []string) bool {
	for _, s := range vals {
		if !isSkip(s) {
			return true
		}
	}
	return false
}

// sumsName is the variable a checksum array lives in: "md5sums" for the base
// array, "md5sums_x86_64" for an arch-specific one.
func sumsName(algo, arch string) string {
	if arch == "" {
		return algo + "sums"
	}
	return algo + "sums_" + arch
}

func (ctx *Context) archesWithSums() []string {
	seen := map[string]bool{"": true}
	out := []string{""}
	for name := range ctx.Pkg.Vars {
		for _, algo := range sumAlgoNames {
			if rest, ok := strings.CutPrefix(name, algo+"sums_"); ok && !seen[rest] {
				seen[rest] = true
				out = append(out, rest)
			}
		}
	}
	return out
}

var sumAlgoNames = []string{"ck", "md5", "sha1", "sha224", "sha256", "sha384", "sha512", "b2"}

// vcsPackageSuffix maps a source VCS to the AUR name suffix that marks a
// package built from that VCS's moving ref: pacman-git tracks a git repository,
// pacman-svn an svn one. The suffix has to match the source's VCS — a -git
// package's svn source is outside the convention, so it stays pinnable.
var vcsPackageSuffix = map[string]string{
	"git": "-git", "hg": "-hg", "svn": "-svn", "bzr": "-bzr", "fossil": "-fossil",
}

// tracksTip reports whether the package's own name declares it a VCS package
// for vcs, i.e. one whose job is to build whatever upstream's moving ref points
// at today. Only the literal suffix matters, so a name assembled from variables
// ("${_pkgname}-git") counts too, and any pkgname of a split package marks the
// whole PKGBUILD — its sources are shared.
func (ctx *Context) tracksTip(vcs string) bool {
	suffix, ok := vcsPackageSuffix[vcs]
	if !ok {
		return false
	}
	for _, field := range []string{"pkgbase", "pkgname"} {
		for _, e := range varElems(ctx.Pkg.Vars[field]) {
			if strings.HasSuffix(ctx.Pkg.Expand(e.Value), suffix) {
				return true
			}
		}
	}
	return false
}

func checkVCSPins(ctx *Context) []Finding {
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		if e.VCS == "" {
			continue
		}
		if _, ok := e.Fragment["commit"]; ok {
			continue
		}
		if _, ok := e.Fragment["revision"]; ok { // svn/fossil
			continue
		}
		signed := strings.Contains(e.Query, "signed")
		if tag, ok := e.Fragment["tag"]; ok {
			if signed {
				continue // signed tag: verified against validpgpkeys
			}
			out = append(out, findingAt("PB103", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"VCS source pinned to mutable tag %q; tags can be re-pointed — pin with #commit=", tag))
			continue
		}
		if ctx.tracksTip(e.VCS) {
			continue // a -git package following a branch is its purpose, not a lapse
		}
		ref := "default branch"
		if b, ok := e.Fragment["branch"]; ok {
			ref = "branch " + b
		}
		out = append(out, findingAt("PB103", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"VCS source follows %s with no #commit= pin; the fetched tree is not tamper-evident", ref))
	}
	return out
}

// insecureProto returns the entry's transport protocol and whether it is
// unencrypted.
func insecureProto(e pkgbuild.SourceEntry) (string, bool) {
	proto := e.Proto
	if _, rest, ok := strings.Cut(proto, "+"); ok {
		proto = rest
	}
	switch proto {
	case "http", "ftp", "git", "svn", "rsync":
		return proto, true
	}
	return proto, false
}

// strongSumFor reports whether this source entry is pinned to a collision-
// resistant digest. md5, sha1 and ck do not count: someone able to rewrite the
// download is generally able to make it collide, so a weak sum is no backstop
// against the very attacker insecure transport lets in.
func strongSumFor(p *pkgbuild.Package, e pkgbuild.SourceEntry) bool {
	sums := p.Checksums(e.Arch)
	for _, algo := range []string{"sha224", "sha256", "sha384", "sha512", "b2"} {
		vals := sums[algo]
		if e.Index >= len(vals) {
			continue
		}
		if v := vals[e.Index]; v != "" && !isSkip(v) {
			return true
		}
	}
	return false
}

func checkInsecureTransport(ctx *Context) []Finding {
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		if e.Local || isSignatureSource(e) { // signature transport is PB112's concern
			continue
		}
		if proto, insecure := insecureProto(e); insecure {
			if strongSumFor(ctx.Pkg, e) {
				out = append(out, findingAt("PB104", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
					"source %q is fetched over unencrypted %s://; the pinned digest still verifies it, "+
						"so this costs availability rather than integrity", e.Raw, proto))
				continue
			}
			out = append(out, findingAt("PB104", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
				"source %q is fetched over unencrypted %s:// with nothing verifying what arrives", e.Raw, proto))
		}
	}
	return out
}

// domainPairs holds host suffix pairs that commonly belong to the same
// project despite different registrable domains.
var domainPairs = [][2]string{
	{"github.com", "githubusercontent.com"},
	{"github.com", "github.io"},
	{"gitlab.com", "gitlab.io"},
	{"pypi.org", "pythonhosted.org"},
	{"sourceforge.net", "sourceforge.io"},
}

var wellKnownHosts = []string{
	"files.pythonhosted.org", "registry.npmjs.org", "crates.io", "static.crates.io",
	"downloads.sourceforge.net", "ftp.gnu.org", "gitlab.freedesktop.org", "proxy.golang.org",
}

func checkSourceDomains(ctx *Context) []Finding {
	urlVal, ok := ctx.Pkg.Scalar("url")
	if !ok {
		return nil
	}
	urlHost := hostOf(urlVal)
	if urlHost == "" {
		return nil
	}
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		h := e.Host()
		// A file:// source has no host: `file://courbi.ttf` is a local file.
		if h == "" || e.Proto == "file" || sameSite(h, urlHost) {
			continue
		}
		known := false
		for _, k := range wellKnownHosts {
			if h == k {
				known = true
			}
		}
		if known {
			continue
		}
		out = append(out, findingAt("PB105", Info, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"source host %q differs from project url host %q", h, urlHost))
	}
	return out
}

func hostOf(rawurl string) string {
	_, rest, ok := strings.Cut(rawurl, "://")
	if !ok {
		return ""
	}
	host := rest
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if i := strings.LastIndexByte(host, '@'); i >= 0 {
		host = host[i+1:]
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return strings.ToLower(host)
}

// sameSite is a naive registrable-domain comparison plus an allowlist of
// related host pairs.
func sameSite(a, b string) bool {
	site := func(h string) string {
		parts := strings.Split(h, ".")
		if len(parts) <= 2 {
			return h
		}
		return strings.Join(parts[len(parts)-2:], ".")
	}
	sa, sb := site(a), site(b)
	if sa == sb {
		return true
	}
	for _, pair := range domainPairs {
		if (sa == pair[0] && sb == pair[1]) || (sa == pair[1] && sb == pair[0]) {
			return true
		}
	}
	return false
}

func checkDLAgents(ctx *Context) []Finding {
	if v, ok := ctx.Pkg.Vars["DLAGENTS"]; ok {
		return []Finding{findingAt("PB106", Warn, ctx.Pkg.PKGBUILD.Path, v.Pos,
			"DLAGENTS override replaces makepkg's download logic with custom commands")}
	}
	return nil
}

func checkMissingInstall(ctx *Context) []Finding {
	v, ok := ctx.Pkg.Vars["install"]
	if !ok {
		return nil
	}
	var out []Finding
	for _, val := range v.Values {
		name := ctx.Pkg.Expand(val)
		if name == "" || strings.ContainsAny(name, "$\x00") {
			continue
		}
		// Only ever stat a name the loader itself would read, so the joined
		// path cannot escape ctx.Pkg.Dir and the report cannot become an
		// existence oracle for arbitrary host paths.
		if !pkgbuild.ValidInstallName(name) {
			out = append(out, findingAt("PB107", Warn, ctx.Pkg.PKGBUILD.Path, v.Pos,
				"install scriptlet %q must be a plain file name next to the PKGBUILD (no path separators)", name))
			continue
		}
		if _, err := os.Stat(filepath.Join(ctx.Pkg.Dir, name)); err != nil {
			out = append(out, findingAt("PB107", Warn, ctx.Pkg.PKGBUILD.Path, v.Pos,
				"install scriptlet %q referenced but not found next to the PKGBUILD", name))
		}
	}
	return out
}

// --- PB114: malformed checksum values ---------------------------------------

// sumHexLengths is the exact digest length, in hex characters, per algorithm.
// cksums (a decimal CRC) is absent: its values aren't fixed-width hex.
var sumHexLengths = map[string]int{
	"md5": 32, "sha1": 40, "sha224": 56, "sha256": 64, "sha384": 96, "sha512": 128, "b2": 128,
}

var hexRe = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func checkMalformedChecksums(ctx *Context) []Finding {
	var out []Finding
	for algo, want := range sumHexLengths {
		names := []string{algo + "sums"}
		for _, a := range ctx.archesWithSums() {
			if a != "" {
				names = append(names, algo+"sums_"+a)
			}
		}
		for _, name := range names {
			for _, e := range varElems(ctx.Pkg.Vars[name]) {
				val, ok := staticVal(ctx.Pkg, e.Value)
				if !ok || val == "" || isSkip(val) {
					continue
				}
				switch {
				case len(val) != want:
					out = append(out, findingAt("PB114", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
						"%s entry %q is %d characters, but a %s digest is %d hex characters; this can never verify",
						name, val, len(val), algo, want))
				case !hexRe.MatchString(val):
					out = append(out, findingAt("PB114", Error, ctx.Pkg.PKGBUILD.Path, e.Pos,
						"%s entry %q contains non-hex characters; this can never verify", name, val))
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}
