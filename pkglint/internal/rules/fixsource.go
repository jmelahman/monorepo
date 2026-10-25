package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// --- PB103: pin a mutable VCS ref to a commit ------------------------------

var commitHashRe = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

func fixVCSPins(ctx *Context, env *FixEnv) []Edit {
	if env == nil || env.ResolveRef == nil {
		return nil // offline: the finding stands with its suggestion
	}
	elems := sourceElems(&ctx.Pkg.PKGBUILD)
	// How many sources each written element expands to. A brace group that
	// yields several URLs shares one #fragment; resolving one URL's ref and
	// rewriting the shared text would pin every URL in the group to it.
	perElem := map[elemID]int{}
	for _, e := range ctx.Pkg.Sources() {
		perElem[elemKey(e.Arch, e.ElemIndex)]++
	}
	var edits []Edit
	for _, e := range ctx.Pkg.Sources() {
		if e.VCS != "git" {
			continue // only git refs are resolvable with `git ls-remote`
		}
		if perElem[elemKey(e.Arch, e.ElemIndex)] > 1 {
			continue
		}
		if _, ok := e.Fragment["commit"]; ok {
			continue
		}
		if _, ok := e.Fragment["revision"]; ok {
			continue
		}
		var fragKey, refVal string
		if tag, ok := e.Fragment["tag"]; ok {
			if strings.Contains(e.Query, "signed") {
				continue
			}
			fragKey, refVal = "tag", tag
		} else if br, ok := e.Fragment["branch"]; ok {
			if ctx.tracksTip(e.VCS) {
				continue // checkVCSPins does not flag it; pinning would freeze a -git package
			}
			fragKey, refVal = "branch", br
		} else {
			continue // no named ref to resolve into a commit deterministically
		}
		w := elems[elemKey(e.Arch, e.ElemIndex)]
		if w == nil {
			continue
		}
		if refVal == "" || strings.ContainsAny(refVal, "\x00$") {
			continue // the expanded ref is not statically known; nothing to resolve
		}
		raw := string(ctx.Pkg.PKGBUILD.Raw[off(w.Pos()):off(w.End())])
		// The written value may be the literal ref or spelled through a
		// variable (#tag=v$pkgver); either way the bytes from the key through
		// the end of the fragment value are what pin the ref, and the resolved
		// commit replaces them wholesale. Only a fragment whose *key* is hidden
		// inside a variable leaves nothing addressable to rewrite.
		key := "#" + fragKey + "="
		i := strings.Index(raw, key)
		if i < 0 {
			continue
		}
		j := i + len(key)
		for j < len(raw) && !strings.ContainsRune(`#?"'`, rune(raw[j])) {
			j++
		}
		sha, err := env.ResolveRef(e.URL, refVal)
		if err != nil || !commitHashRe.MatchString(sha) {
			continue
		}
		edits = append(edits, Edit{
			Path:  ctx.Pkg.PKGBUILD.Path,
			Start: off(w.Pos()),
			End:   off(w.End()),
			New:   raw[:i] + "#commit=" + sha + raw[j:],
			Line:  int(w.Pos().Line()),
			Desc:  fmt.Sprintf("pin %s %q to commit %s", fragKey, refVal, shortSHA(sha)),
		})
	}
	return edits
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

type elemID struct {
	arch string
	idx  int
}

func elemKey(arch string, idx int) elemID { return elemID{arch, idx} }

// sourceElems maps each written source-array element to its AST word, keyed by
// arch and SourceEntry.ElemIndex so a finding can be matched back to the text
// it is about.
//
// Numbering must mirror how Package merges assignments, since that is what
// ElemIndex counts: `source+=(...)` continues the preceding array's numbering,
// while a plain `source=(...)` replaces the array and restarts at zero, exactly
// as in bash. Keying each assignment's elements from zero instead made every
// `+=` shadow the base array — usually losing the fix, and, when both entries
// carried the same `#ref` text, computing an edit for one element and writing
// it over another's byte range.
func sourceElems(u *pkgbuild.Unit) map[elemID]*syntax.Word {
	out := map[elemID]*syntax.Word{}
	next := map[string]int{}
	record := func(as *syntax.Assign) {
		if as == nil || as.Name == nil {
			return
		}
		name := as.Name.Value
		if name != "source" && !strings.HasPrefix(name, "source_") {
			return
		}
		arch := strings.TrimPrefix(strings.TrimPrefix(name, "source"), "_")
		if as.Index != nil {
			// `source[i]=url` updates one element in place — no reset, no new
			// element — mirroring Package.recordIndexed. Remap the element only
			// for a literal index; extend the numbering when the write lands
			// past the end, as the merged Values then do.
			if idx, ok := pkgbuild.AssignIndex(as); ok {
				if !as.Append {
					out[elemKey(arch, idx)] = as.Value
				}
				if idx >= next[arch] {
					next[arch] = idx + 1
				}
			}
			return
		}
		if !as.Append {
			for i := range next[arch] {
				delete(out, elemKey(arch, i))
			}
			next[arch] = 0
		}
		switch {
		case as.Array != nil:
			for _, el := range as.Array.Elems {
				out[elemKey(arch, next[arch])] = el.Value
				next[arch]++
			}
		case as.Value != nil:
			// A scalar assignment merges in as one value with no element of
			// its own. Consume the index so later elements stay aligned; the
			// missing key just means no edit, rather than a misdirected one.
			next[arch]++
		}
	}
	for _, stmt := range u.TopLevel {
		switch cmd := stmt.Cmd.(type) {
		case *syntax.CallExpr:
			if len(cmd.Args) == 0 {
				for _, as := range cmd.Assigns {
					record(as)
				}
			}
		case *syntax.DeclClause:
			for _, as := range cmd.Args {
				record(as)
			}
		}
	}
	return out
}

// --- PB104/PB112: upgrade an insecure source transport to https ------------

// httpsProto returns the encrypted spelling of an unencrypted source protocol,
// and reports whether one exists that a probe can actually vouch for.
//
// http gains a "s"; ftp becomes https, since a host still publishing over ftp
// almost always serves the same tree over https (ftp.gnu.org and the kernel
// mirrors are the common cases) and makepkg has no encrypted ftp agent to
// switch to. A bare git:// URL — the unauthenticated git wire protocol — is
// rewritten to git+https://, which every forge that speaks git:// also serves
// at the same path, and git+http:// likewise moves to git+https://.
//
// svn:// and rsync:// are deliberately absent: svn+https:// depends on the
// server exposing a DAV endpoint that svn:// says nothing about, and rsync has
// no encrypted spelling of its own at all. There is no rewrite that is even
// usually right, so those findings stand. hg+http and svn+http are absent for
// a different reason: the spelling is obvious, but neither capability in
// FixEnv can check it — ProbeHTTPS speaks plain https and ResolveRef speaks
// git — and an offer the probe cannot vet would be an unverified rewrite.
func httpsProto(proto string) (string, bool) {
	switch proto {
	case "http", "ftp":
		return "https", true
	case "git", "git+http":
		return "git+https", true
	}
	return "", false
}

func fixInsecureTransport(ctx *Context, env *FixEnv) []Edit {
	return insecureTransportEdits(ctx, env, false)
}

func fixInsecureSignatureTransport(ctx *Context, env *FixEnv) []Edit {
	return insecureTransportEdits(ctx, env, true)
}

// probeTarget is the URL the rewritten source would be fetched from, and
// whether it could be determined at all.
//
// It is the expanded URL with its scheme replaced, minus the `filename::`
// prefix makepkg strips before fetching. A VCS source loses the makepkg
// fragment and query too — `#commit=…` and `?signed` address makepkg, not the
// remote — while a plain download keeps its query string, which belongs to the
// server and often decides what it sends back.
//
// A URL still holding an unexpanded variable has no determinable target: the
// bytes on the line are not the address anything would be fetched from, so
// there is nothing a probe could confirm and the fix declines.
func probeTarget(e pkgbuild.SourceEntry, proto string) (string, bool) {
	raw := e.Expanded
	if e.Filename != "" {
		if _, rest, ok := strings.Cut(raw, "::"); ok {
			raw = rest
		}
	}
	if e.VCS != "" {
		raw = e.URL // fragment and query already stripped
	}
	old := e.Proto + "://"
	if len(raw) < len(old) || !strings.EqualFold(raw[:len(old)], old) {
		return "", false
	}
	rest := raw[len(old):]
	if rest == "" || strings.ContainsAny(rest, "$\x00") {
		return "", false
	}
	return proto + "://" + rest, true
}

// httpsServed reports whether the rewritten URL is really served, asking the
// protocol's own client: a git remote answers `git ls-remote` or it is not a
// git remote, and everything else is an HTTP endpoint a headers-only request
// can reach. Without the capability to ask — offline — it answers false, so
// the fix never fires on an assumption.
func httpsServed(env *FixEnv, e pkgbuild.SourceEntry, url string) bool {
	if e.VCS == "git" {
		if env.ResolveRef == nil {
			return false
		}
		// Any ref would do; HEAD is the one every remote has, so this asks
		// "does a git repository answer here" and nothing more.
		_, err := env.ResolveRef(url, "HEAD")
		return err == nil
	}
	if env.ProbeHTTPS == nil {
		return false
	}
	return env.ProbeHTTPS(url) == nil
}

// insecureTransportEdits rewrites the scheme of every insecurely fetched
// source. The tier split mirrors the checks': the PB104 fixer claims a written
// element holding an ordinary source, the PB112 fixer one holding a signature,
// so each rule fixes what it reports. A brace element carrying both
// (`foo{,.sig}`) is claimed by each; their identical edits collapse when
// applied.
//
// Every rewrite is checked against the server first: the edit is a claim that
// the host serves this path over https, which is not a claim a PKGBUILD can
// settle, and an unverified rewrite trades a compromisable build for a broken
// one. One written element can expand to several sources sharing the one
// written scheme, and the single edit re-addresses all of them — so all of
// them must verify, the other rule's included. A probe that fails, or a probe
// that cannot be made at all, for any URL the element expands to, leaves the
// finding standing for a maintainer to resolve.
//
// It stays an unsafe fix even so. A reachable URL is not the same URL: the
// probe establishes that something answers over https, never that it answers
// with the bytes the http fetch would have returned. For a source pinned to a
// strong digest, makepkg catches the difference on the next build; for a SKIP
// or a VCS clone, only a human does. That gap is exactly the review
// --unsafe-fix asks for.
//
// The digests are left alone: https fetches the same artifact, so a sums array
// that verified the http download verifies the https one. If it does not, the
// bytes differed, and that is precisely what the checksum is there to catch.
func insecureTransportEdits(ctx *Context, env *FixEnv, signatures bool) []Edit {
	if env == nil {
		return nil
	}
	// Group the entries by the written element they expand from: the element
	// is the unit of rewriting, however many sources it becomes.
	var order []elemID
	groups := map[elemID][]pkgbuild.SourceEntry{}
	for _, e := range ctx.Pkg.Sources() {
		if e.Local {
			continue
		}
		k := elemKey(e.Arch, e.ElemIndex)
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], e)
	}
	elems := sourceElems(&ctx.Pkg.PKGBUILD)
	var edits []Edit
	seen := map[int]bool{}
	for _, k := range order {
		entries := groups[k]
		claimed := false
		for _, e := range entries {
			claimed = claimed || isSignatureSource(e) == signatures
		}
		e0 := entries[0]
		if !claimed {
			continue
		}
		if _, insecure := insecureProto(e0); !insecure {
			continue
		}
		proto, ok := httpsProto(e0.Proto)
		if !ok {
			continue
		}
		w := elems[k]
		if w == nil {
			continue
		}
		raw := string(ctx.Pkg.PKGBUILD.Raw[off(w.Pos()):off(w.End())])
		old := e0.Proto + "://"
		i := strings.Index(raw, old)
		// The scheme has to be written out literally and exactly once. Spelled
		// through a variable ("$_proto://…") there is nothing to rewrite, and
		// appearing twice — a proxy URL carrying another URL in its query, say
		// — leaves no way to tell which occurrence is the transport.
		if i < 0 || strings.Contains(raw[i+1:], old) {
			continue
		}
		at := off(w.Pos()) + i
		if seen[at] {
			continue
		}
		// Each distinct URL the element expands to is probed once, and one
		// refusal vetoes the whole element: the rewrite would re-address that
		// URL too, on nothing but hope.
		var targets []string
		probed := map[string]bool{}
		served := true
		for _, e := range entries {
			target, ok := probeTarget(e, proto)
			if !ok || e.Proto != e0.Proto {
				served = false
				break
			}
			if probed[target] {
				continue
			}
			probed[target] = true
			if !httpsServed(env, e, target) {
				served = false
				break
			}
			targets = append(targets, target)
		}
		if !served {
			continue
		}
		seen[at] = true
		edits = append(edits, Edit{
			Path:  ctx.Pkg.PKGBUILD.Path,
			Start: at,
			End:   at + len(old),
			New:   proto + "://",
			Line:  int(e0.Pos.Line()),
			Desc:  fmt.Sprintf("fetch over %s:// instead of %s:// (%s answers)", proto, e0.Proto, strings.Join(targets, ", ")),
		})
	}
	return edits
}

// --- PB102: add a strong digest beside a weak one --------------------------

// fixWeakChecksums writes a sha256sums array next to a weak md5sums/sha1sums,
// but only when it can show the bytes it hashed are the bytes the weak digest
// already vouched for.
//
// This fix is unlike every other one here: the value to write is data, not
// syntax, and no amount of reading the PKGBUILD recovers it. env.LocalDigest
// supplies it from sources already fetched, so a package whose sources have
// never been downloaded simply keeps the finding — see LocalDigest's comment
// for why pkglint will not fetch them to close it.
//
// What makes the result trustworthy rather than merely plausible is that the
// weak digest is re-computed from the very same read as the sha256 and must
// match. The sha256 written therefore describes bytes the existing chain
// already covered: substituting them needs an md5 *preimage*, not the
// collision that makes md5 unfit for new use. Without that check the fix would
// just be laundering whatever happens to sit in the cache into an
// authoritative-looking digest. A mismatch — upstream silently re-rolled the
// tarball, or something worse — abandons the array rather than certifying it.
//
// Arrays are all-or-nothing per arch. makepkg pairs sums to sources by index,
// so an array with a gap in it is not a partial fix but a misaligned one.
func fixWeakChecksums(ctx *Context, env *FixEnv) []Edit {
	if env == nil || env.LocalDigest == nil {
		return nil
	}
	var edits []Edit
	for _, arch := range ctx.archesWithSums() {
		if e, ok := strongSumsEdit(ctx, env, arch); ok {
			edits = append(edits, e)
		}
	}
	return edits
}

// strongSumsEdit builds the one sha256sums array for arch, or reports that it
// could not. Both weak arrays being present (md5sums *and* sha1sums) still
// yields a single edit, since one strong array is what clears the finding.
func strongSumsEdit(ctx *Context, env *FixEnv, arch string) (Edit, bool) {
	sums := ctx.Pkg.Checksums(arch)
	if hasStrongSum(sums) {
		return Edit{}, false // PB102 is silent here; nothing to fix
	}
	witness, v := weakSumsVar(ctx, sums, arch)
	if v == nil {
		return Edit{}, false
	}
	vals := sums[witness.algo]
	// One plain array assignment holding exactly these values: the array the
	// edit writes has to pair index-for-index with what makepkg pairs, and a
	// count arrived at through `+=` merges or brace groups is a count this
	// fixer cannot reproduce by writing elements out one per line. Those are
	// left to updpkgsums.
	if v.CountUnknown || v.Assign == nil || v.Assign.Array == nil ||
		len(v.Assign.Array.Elems) != len(v.Values) || len(vals) != len(v.Values) {
		return Edit{}, false
	}
	srcs := sourcesForArch(ctx, arch)
	if len(srcs) != len(vals) {
		return Edit{}, false // PB110's territory; pairing here would misalign
	}
	strong := make([]string, len(vals))
	hashed := 0
	for i, e := range srcs {
		s, ok := strongSumValue(ctx, env, e, witness, vals[i])
		if !ok {
			return Edit{}, false
		}
		if !isSkip(s) {
			hashed++
		}
		strong[i] = s
	}
	// An all-SKIP array would satisfy the rule's "a strong digest is present"
	// test while verifying nothing — silencing PB102 instead of fixing it.
	if hashed == 0 {
		return Edit{}, false
	}
	raw := ctx.Pkg.PKGBUILD.Raw
	// Insert on the line after the weak array ends, past any trailing comment
	// on its closing line.
	at := off(v.Assign.End())
	for at < len(raw) && raw[at] != '\n' {
		at++
	}
	return Edit{
		Path:  ctx.Pkg.PKGBUILD.Path,
		Start: at,
		End:   at,
		New:   "\n" + sumsArrayText(sumsName("sha256", arch), strong),
		Line:  int(v.Pos.Line()),
		Desc: fmt.Sprintf("add %s (%d digest(s) hashed locally and checked against %s)",
			sumsName("sha256", arch), hashed, sumsName(witness.algo, arch)),
	}, true
}

// strongSumValue returns one source's sha256sums entry: SKIP where makepkg wants
// one, else the local file's hash — gated on the weak digest matching the same
// bytes.
func strongSumValue(ctx *Context, env *FixEnv, e pkgbuild.SourceEntry, witness weakWitness, weak string) (string, bool) {
	// A VCS source has no single file to hash and takes SKIP in every sums
	// array. So does an entry already written SKIP: that is a detached
	// signature doing the verifying, or an unverified source, and either way
	// PB101 and PB111 own it. Carrying the SKIP across keeps this fix from
	// quietly changing what is verified.
	if e.VCS != "" || isSkip(weak) {
		return "SKIP", true
	}
	d, err := env.LocalDigest(ctx.Pkg.Dir, effectiveFilename(e))
	if err != nil || d.SHA256 == "" {
		return "", false
	}
	if got := witness.of(d); got == "" || !strings.EqualFold(got, weak) {
		return "", false
	}
	return d.SHA256, true
}

// weakWitness is a weak digest the fixer can re-compute, and so can use to
// check that a local file is the one a PKGBUILD already describes.
type weakWitness struct {
	algo string
	of   func(Digests) string
}

// weakWitnesses are those digests, best first: sha1 is the stronger witness of
// the two. ck is deliberately absent — it is makepkg's CRC, which pkglint does
// not compute, so a ck-only package offers nothing to check the bytes against
// and gets no fix.
var weakWitnesses = []weakWitness{
	{"sha1", func(d Digests) string { return d.SHA1 }},
	{"md5", func(d Digests) string { return d.MD5 }},
}

// weakSumsVar picks the weak array to verify against, returning the witness
// and the Var holding it. A nil Var means there is nothing usable.
func weakSumsVar(ctx *Context, sums map[string][]string, arch string) (weakWitness, *pkgbuild.Var) {
	for _, w := range weakWitnesses {
		if !hasRealSum(sums[w.algo]) {
			continue
		}
		if v := ctx.Pkg.Vars[sumsName(w.algo, arch)]; v != nil {
			return w, v
		}
	}
	return weakWitness{}, nil
}

// sourcesForArch returns arch's sources in the index order makepkg pairs
// checksums by.
func sourcesForArch(ctx *Context, arch string) []pkgbuild.SourceEntry {
	var out []pkgbuild.SourceEntry
	for _, e := range ctx.Pkg.Sources() {
		if e.Arch == arch {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

// sumsArrayText renders a sums array the way updpkgsums does: one value per
// line, aligned under the first, so a long array stays readable.
func sumsArrayText(name string, vals []string) string {
	var b strings.Builder
	b.WriteString(name + "=('" + vals[0] + "'")
	indent := strings.Repeat(" ", len(name)+2)
	for _, v := range vals[1:] {
		b.WriteString("\n" + indent + "'" + v + "'")
	}
	b.WriteString(")")
	return b.String()
}
