package pkgbuild

import (
	"net/url"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// SourceEntry is one element of a source=() array, parsed per makepkg's
// [filename::]url[#fragment][?query] convention.
type SourceEntry struct {
	Raw      string // as written
	Expanded string // with known variables substituted
	Filename string // explicit filename before ::, if any
	URL      string
	Proto    string            // full protocol, e.g. "git+https", "https", "" for local files
	VCS      string            // "git", "hg", "svn", "bzr", "fossil" or ""
	Fragment map[string]string // commit/tag/branch/revision -> value
	Query    string            // e.g. "signed"
	Local    bool
	Index    int
	Arch     string // "" for the plain source array, else e.g. "x86_64"
	Pos      syntax.Pos

	// ElemIndex is the index of the written array element this entry came
	// from, counting across every assignment merged into the array. It
	// differs from Index whenever an element expands to more than one entry:
	// `foo{,.sig}` is two entries (Index 0 and 1) written as one element
	// (ElemIndex 0 for both), and an array reference like "${files[@]}" is
	// one element shared by every entry it expands to. -1 means the entry has
	// no written element of its own (padded in by an indexed write). Use
	// Index to pair with checksums, ElemIndex to address the source text.
	ElemIndex int
}

var vcsProtos = map[string]bool{"git": true, "hg": true, "svn": true, "bzr": true, "fossil": true}

// Sources parses every source array (source, source_x86_64, ...) in the
// PKGBUILD. Brace groups expand to one entry each — `foo.tar.gz{,.sig}` is
// two sources to makepkg — and Index counts expanded entries, matching how
// makepkg pairs sources with checksums.
func (p *Package) Sources() []SourceEntry {
	p.sourcesOnce.Do(func() { p.sources = p.computeSources() })
	return p.sources
}

func (p *Package) computeSources() []SourceEntry {
	arches := p.declaredArches()
	var out []SourceEntry
	for name, v := range p.Vars {
		var arch string
		switch {
		case name == "source":
		case strings.HasPrefix(name, "source_"):
			// makepkg fetches source_$CARCH only for an architecture the
			// package declares. A suffix that names none of them is an
			// ordinary variable that happens to collide with the namespace —
			// `source_prefix` holding a URL stem, or a `source_linux_386`
			// written beside arch=('i686') — and nothing in it is ever
			// downloaded. PB902 is what flags the collision; treating it as a
			// source array here would report on a fetch that never happens.
			arch = strings.TrimPrefix(name, "source_")
			if arches != nil && !arches[arch] {
				continue
			}
		default:
			continue
		}
		idx := 0
		for rawIdx, raw := range v.Values {
			// Each entry reports its own written element's position, via the
			// Var's value→element mapping (identity until an array reference
			// expanded). Values with no element of their own — scalar-derived,
			// appended by a `+=` assignment the merge did not keep an Assign
			// for — fall back to the array's own position.
			elemIdx := v.elemAt(rawIdx)
			pos := v.Pos
			if v.Assign != nil && v.Assign.Array != nil && elemIdx >= 0 && elemIdx < len(v.Assign.Array.Elems) {
				if el := v.Assign.Array.Elems[elemIdx]; el.Value != nil {
					pos = el.Value.Pos()
				}
			}
			for _, expanded := range ExpandBraces(p.Expand(raw)) {
				e := parseSourceEntry(raw, expanded)
				e.Index = idx
				e.ElemIndex = elemIdx
				e.Arch = arch
				e.Pos = pos
				out = append(out, e)
				idx++
			}
		}
	}
	return out
}

// declaredArches returns the architectures arch=() names, or nil when the
// parser could not pin them down — no arch at all, a value built from a
// variable, or a length a top-level conditional makes unknowable. nil means
// "do not filter": losing a whole source array from the analysis is worse than
// looking at one makepkg would skip.
func (p *Package) declaredArches() map[string]bool {
	p.archesOnce.Do(func() { p.arches = p.computeDeclaredArches() })
	return p.arches
}

func (p *Package) computeDeclaredArches() map[string]bool {
	v := p.Vars["arch"]
	if v == nil || v.CountUnknown || len(v.Values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(v.Values))
	for _, s := range v.Values {
		for _, exp := range ExpandBraces(s) {
			if exp == "" || strings.ContainsAny(exp, "$\x00") {
				return nil
			}
			out[exp] = true
		}
	}
	return out
}

// braceExpandMax caps how many values one element may expand to. Brace groups
// multiply — `{a,b}{a,b}{a,b}…` doubles per group — so a hostile PKGBUILD could
// otherwise turn a short line into an unbounded allocation. Past the cap the
// element is left unexpanded, which no real source array comes close to needing.
const braceExpandMax = 4096

// ExpandBraces performs bash-style brace expansion for the {a,b,c} groups that
// appear in array literals, including nested ones: `{a{b,c},d}` is three values
// to bash and three here. ${var} parameter braces and groups without a
// top-level comma are left alone, which is what bash does with them too.
//
// Bash expands braces in array assignments but not in scalar ones — `a=({x,y}z)`
// is two elements, `a={x,y}z` is the literal string — so callers must only
// apply this to array elements.
func ExpandBraces(s string) []string {
	if out, ok := expandBraces(s, braceExpandMax); ok {
		return out
	}
	return []string{s}
}

// expandBraces expands s, abandoning the attempt as soon as the result would
// exceed budget; the bool reports whether it stayed within it. The budget has
// to travel down the recursion rather than being checked on the way back up:
// the blowup happens inside the nested call, so a check after it returns is a
// check that never runs.
func expandBraces(s string, budget int) ([]string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		if i > 0 && s[i-1] == '$' { // ${var}: not a brace group
			if j := strings.IndexByte(s[i:], '}'); j >= 0 {
				i += j
			}
			continue
		}
		end, alts := braceGroup(s[i:])
		// An unbalanced group, or one with no top-level comma, is literal text.
		if end < 0 || len(alts) < 2 {
			continue
		}
		var out []string
		for _, alt := range alts {
			vals, ok := expandBraces(s[:i]+alt+s[i+end+1:], budget-len(out))
			if !ok {
				return nil, false
			}
			if out = append(out, vals...); len(out) > budget {
				return nil, false
			}
		}
		return out, true
	}
	return []string{s}, true
}

// braceGroup splits the brace group starting at s[0] into its top-level
// alternatives and returns the offset of the matching '}'. Nested groups count
// toward the depth but are not split here: `{a{b,c},d}` has the two
// alternatives "a{b,c}" and "d", and the inner group expands on the next pass
// over the substituted string. That ordering is what makes the result match
// bash — splitting on every comma instead yields `.tar.gz` twice from
// `.tar.gz{,.sha256sum{,.asc}}` and drops a value.
//
// A group with no top-level comma comes back as a single alternative, which the
// caller reads as "not an expansion".
func braceGroup(s string) (end int, alts []string) {
	depth, start := 0, 1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i, append(alts, s[start:i])
			}
		case ',':
			if depth == 1 {
				alts = append(alts, s[start:i])
				start = i + 1
			}
		}
	}
	return -1, nil
}

func parseSourceEntry(raw, expanded string) SourceEntry {
	e := SourceEntry{Raw: raw, Expanded: expanded, Fragment: map[string]string{}}
	rest := expanded

	if name, url, ok := strings.Cut(rest, "::"); ok {
		e.Filename = name
		rest = url
	}
	// URL ends at the first '#' (fragment) or '?' (query); makepkg allows
	// either order (…#frag?query or …?query#frag), so parse them positionally.
	tail := ""
	if i := strings.IndexAny(rest, "#?"); i >= 0 {
		tail = rest[i:]
		rest = rest[:i]
	}
	for tail != "" {
		delim := tail[0]
		seg := tail[1:]
		if j := strings.IndexAny(seg, "#?"); j >= 0 {
			tail = seg[j:]
			seg = seg[:j]
		} else {
			tail = ""
		}
		if delim == '#' {
			if k, v, ok := strings.Cut(seg, "="); ok {
				e.Fragment[k] = v
			}
		} else {
			e.Query = seg
		}
	}
	e.URL = rest

	scheme, _, ok := strings.Cut(rest, "://")
	if !ok {
		e.Local = true
		return e
	}
	e.Proto = strings.ToLower(scheme)
	vcs, _, plus := strings.Cut(e.Proto, "+")
	if plus && vcsProtos[vcs] {
		e.VCS = vcs
	} else if vcsProtos[e.Proto] {
		e.VCS = e.Proto
	}
	return e
}

// Host returns the URL's hostname, when determinable.
func (e SourceEntry) Host() string {
	raw := e.URL
	if e.VCS != "" {
		if _, u, ok := strings.Cut(raw, "+"); ok {
			raw = u
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// checksum algorithms recognized by makepkg, weakest first.
var sumAlgos = []string{"ck", "md5", "sha1", "sha224", "sha256", "sha384", "sha512", "b2"}

// Checksums returns algo -> values for the sums arrays matching arch
// ("" for the base arrays). Brace groups expand to one value each, matching
// Sources: `sha256sums=(SKIP{,,,})` is four checksums to makepkg, so both
// sides have to count expanded entries or the index pairing slips.
func (p *Package) Checksums(arch string) map[string][]string {
	out := map[string][]string{}
	for _, algo := range sumAlgos {
		name := algo + "sums"
		if arch != "" {
			name += "_" + arch
		}
		v, ok := p.Vars[name]
		if !ok {
			continue
		}
		vals := make([]string, 0, len(v.Values))
		for _, raw := range v.Values {
			vals = append(vals, ExpandBraces(raw)...)
		}
		out[algo] = vals
	}
	return out
}

// SumsFor returns every checksum value across all algorithms for the source
// entry at the given index/arch.
func (p *Package) SumsFor(e SourceEntry) []string {
	var out []string
	for _, vals := range p.Checksums(e.Arch) {
		if e.Index < len(vals) {
			out = append(out, vals[e.Index])
		}
	}
	return out
}
