// Package pkgbuild statically parses PKGBUILD and .install files.
//
// PKGBUILDs are untrusted input: they must never be sourced or executed
// (even `makepkg --printsrcinfo` runs top-level code). Everything here is
// pure static analysis on the bash AST via mvdan.cc/sh.
package pkgbuild

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/syntax"
)

// Var is a top-level variable assignment in a PKGBUILD.
type Var struct {
	Name   string
	Values []string // rendered values; one element for scalar assignments
	// ElemFor maps each Values index to the written-element counter the value
	// came from — continuing across merged `+=` assignments — or -1 when it
	// has no written element of its own (padded in by an indexed write). nil
	// means the identity mapping — value i came from element i — which holds
	// until a whole-array reference ("${files[@]}") expands one written
	// element into a different number of values.
	ElemFor []int
	// ElemCount is how many written elements (or scalar assignments) merged
	// into this Var: the counter space ElemFor indexes into. It matches the
	// element numbering fix code derives from the AST, which is why merges
	// track it instead of len(Values).
	ElemCount int
	Assign    *syntax.Assign // underlying AST node, for byte-offset edits
	Pos       syntax.Pos
	Array     bool
	// CountUnknown marks an array holding a whole-array reference that could
	// not be statically expanded ("${files[@]}" with files unknown), so
	// len(Values) is not the array's real length. Rules that compare lengths
	// must not trust the count.
	CountUnknown bool
}

// elemAt returns v's written-element counter for Values[i], -1 when the value
// has no written element of its own.
func (v *Var) elemAt(i int) int {
	if v.ElemFor == nil {
		return i
	}
	if i < len(v.ElemFor) {
		return v.ElemFor[i]
	}
	return -1
}

// ElemWord returns the array element Values[i] was written as, and the position
// to report it at. The word is nil — and the position falls back to the
// variable's own — whenever the value has no source text of its own: a value
// appended by a later `+=` (the merge keeps only the first assignment's AST),
// one padded in by an indexed write, or a scalar's. Callers that rewrite source
// text need the word; callers that only report a location can use the position
// unconditionally.
func (v *Var) ElemWord(i int) (*syntax.Word, syntax.Pos) {
	if v.Assign == nil {
		return nil, v.Pos
	}
	if v.Assign.Array == nil {
		// A scalar's sole value is its whole right-hand side. Anything past
		// index 0 was merged in from elsewhere and has no word here.
		if i == 0 && v.Assign.Value != nil {
			return v.Assign.Value, v.Assign.Value.Pos()
		}
		return nil, v.Pos
	}
	elems := v.Assign.Array.Elems
	idx := v.elemAt(i)
	if idx < 0 || idx >= len(elems) || elems[idx].Value == nil {
		return nil, v.Pos
	}
	return elems[idx].Value, elems[idx].Value.Pos()
}

// Unit is a single bash file under analysis: the PKGBUILD itself or an
// install scriptlet.
type Unit struct {
	Path      string
	Raw       []byte
	File      *syntax.File
	Scriptlet bool
	Functions map[string]*syntax.FuncDecl
	TopLevel  []*syntax.Stmt // top-level statements that are not function declarations
}

// ScriptletError records an install scriptlet that was present but could not
// be parsed. Such a file is analyzed by no rule yet still runs as root at
// install time, so it must be surfaced rather than silently skipped.
type ScriptletError struct {
	Path string
	Err  string
}

// Package is a fully loaded package directory.
type Package struct {
	Dir        string
	PKGBUILD   Unit
	Scriptlets []Unit
	// ScriptletErrors holds scriptlets that were read but failed to parse.
	ScriptletErrors []ScriptletError
	Vars            map[string]*Var
	SrcInfo         *SrcInfo // nil when no .SRCINFO is present

	// ConditionalVars names variables assigned by top-level control flow — an
	// `if`, a `for`, a `case "$CARCH"` — rather than by a plain assignment.
	// makepkg runs all of that before it reads the metadata, so these names
	// are set as far as it is concerned even though Vars has no entry for
	// them: which branch fires is not knowable without executing the file.
	// Rules that report a field missing must check here before doing so.
	ConditionalVars map[string]bool

	// Suppressions maps a file path to that file's inline
	// "# pkglint: ignore=PB123[,PB456]" directives (line number -> rule IDs
	// disabled on that line; a directive applies to the same and next line).
	// Keying by path keeps a directive in one file from suppressing findings in
	// another file that happens to share a line number.
	Suppressions map[string]map[int]map[string]bool

	// Parsing is finished by the time Load returns, so the derived views below
	// are computed once and shared. Dozens of rules ask for the source list,
	// and recomputing it — expanding every element's variables and brace groups
	// again each time — was the linter's second-largest allocation source.
	// Callers must treat both as read-only. The sync.Once fields sit together
	// at the end because they align to 4 rather than 8: interleaved with the
	// pointer-width fields above, each would cost a word of padding.
	sources []SourceEntry
	arches  map[string]bool

	sourcesOnce sync.Once
	archesOnce  sync.Once
}

// newParser returns a fresh parser per parse: syntax.Parser is not safe for
// concurrent use, and Load is called from concurrent scanners.
func newParser() *syntax.Parser {
	return syntax.NewParser(syntax.KeepComments(true), syntax.Variant(syntax.LangBash))
}

// maxSourceFile bounds a PKGBUILD, .SRCINFO, or install scriptlet. The
// largest real PKGBUILDs in the AUR are well under 1 MiB; the ceiling exists
// so a checkout that points one of these names at a device or a huge file
// fails to load instead of hanging the linter. A var so tests can lower it.
var maxSourceFile int64 = 8 << 20

// readSourceFile reads one of the package's own files, refusing anything that
// is not a regular file (a device, a FIFO, a directory) and anything past
// maxSourceFile. Symlinks to regular files are followed; the check is on what
// the name resolves to.
func readSourceFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSourceFile+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSourceFile {
		return nil, fmt.Errorf("%s: larger than %d bytes", path, maxSourceFile)
	}
	return data, nil
}

// Load reads the PKGBUILD at path (a file or a directory containing one),
// plus any install scriptlets referenced by install= or living next to it.
func Load(path string) (*Package, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	dir, file := filepath.Split(path)
	if info.IsDir() {
		dir = path
		file = "PKGBUILD"
	}
	pkgPath := filepath.Join(dir, file)

	raw, err := readSourceFile(pkgPath)
	if err != nil {
		return nil, err
	}
	unit, err := parseUnit(pkgPath, raw, false)
	if err != nil {
		return nil, err
	}

	pkg := &Package{
		Dir:      dir,
		PKGBUILD: unit,
		Vars:     map[string]*Var{},
		// Keyed by unit.Path (== pkgPath), the same string findings on the
		// PKGBUILD carry, so Suppressed lookups match exactly.
		Suppressions: map[string]map[int]map[string]bool{unit.Path: parseSuppressions(raw)},
	}
	pkg.extractTopLevel()

	if data, err := readSourceFile(filepath.Join(dir, ".SRCINFO")); err == nil {
		pkg.SrcInfo = ParseSrcInfo(data)
	}

	for _, name := range pkg.installFiles() {
		p := filepath.Join(dir, name)
		data, err := readSourceFile(p)
		if err != nil {
			continue // missing scriptlet is reported by a rule
		}
		// Record the scriptlet's directives before parsing it. parseSuppressions
		// is a plain byte scan that needs no bash AST, and a scriptlet that fails
		// to parse still has to be able to waive its own PB503 finding (which
		// reports at this path).
		pkg.Suppressions[p] = parseSuppressions(data)
		su, err := parseUnit(p, data, true)
		if err != nil {
			// The file runs as root at install time but no rule can walk it;
			// record it so PB503 can report the blind spot. Keep only the
			// scriptlet's basename in the message: Path already carries the
			// location, and the message is republished verbatim (e.g. by the
			// site generator, which strips prefixes from paths but not
			// messages).
			msg := strings.TrimPrefix(err.Error(), "parse "+p+": ")
			msg = strings.ReplaceAll(msg, p, name)
			pkg.ScriptletErrors = append(pkg.ScriptletErrors, ScriptletError{Path: p, Err: msg})
			continue
		}
		pkg.Scriptlets = append(pkg.Scriptlets, su)
	}
	return pkg, nil
}

// ParseScriptlet parses raw as an install scriptlet unit. It exists for
// callers that get scriptlet bytes from somewhere other than the package
// directory — notably the .INSTALL member of a built package archive.
func ParseScriptlet(path string, raw []byte) (Unit, error) {
	return parseUnit(path, raw, true)
}

func parseUnit(path string, raw []byte, scriptlet bool) (Unit, error) {
	f, err := newParser().Parse(bytes.NewReader(raw), path)
	if err != nil {
		// Bash accepts several constructs that mvdan.cc/sh does not yet
		// parse; rescueParse recovers a position-accurate AST for the known
		// ones rather than leaving the whole file unscanned (see rescue.go).
		if rf := rescueParse(path, raw); rf != nil {
			f = rf
		} else {
			return Unit{}, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	u := Unit{
		Path:      path,
		Raw:       raw,
		File:      f,
		Scriptlet: scriptlet,
		Functions: map[string]*syntax.FuncDecl{},
	}
	for _, stmt := range f.Stmts {
		// upstream parses `()cmd` into a FuncDecl with a nil Name; keep the
		// statement visible to rules rather than crash or drop it.
		if fd, ok := stmt.Cmd.(*syntax.FuncDecl); ok && fd.Name != nil {
			u.Functions[fd.Name.Value] = fd
			continue
		}
		u.TopLevel = append(u.TopLevel, stmt)
	}
	return u, nil
}

// wordSplits reports whether this array element is one written element that
// bash will turn into an unknown number of values.
//
// Two things do that, and quoting stops both. An unquoted command substitution
// word-splits: `source=($(_source))` is however many lines _source prints, and
// `sha256sums=($(awk 'BEGIN{…printf "SKIP\n"…}'))` however many that awk emits.
// An unquoted glob is pathname-expanded against the build directory:
// `source=(… *.md)` is however many .md files sit next to the PKGBUILD, which
// is exactly why the array below it carries more checksums than there are
// written sources. The parser records one value in either case, so the array's
// length is only trustworthy without them.
//
// A glob only changes the count when it matches, though — bash leaves an
// unmatched pattern literal — and a URL never can: matching would need a
// directory literally named "https:" and then an empty path component. So the
// `?` in an unquoted `…/download.php?file=x` is a query string, the element
// really is one source, and length checks stay in business.
func wordSplits(w *syntax.Word) bool {
	if w == nil {
		return false
	}
	// Only top-level parts: anything inside DblQuoted or SglQuoted is protected
	// from both expansions, which is the whole distinction being drawn here.
	glob := false
	for _, part := range w.Parts {
		switch x := part.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst:
			return true
		case *syntax.Lit:
			if strings.ContainsAny(x.Value, "*?[") {
				glob = true
			}
		}
	}
	if !glob {
		return false
	}
	s, _ := RenderWord(w, nil)
	return !strings.Contains(s, "://")
}

// extractTopLevel records top-level variable assignments from the PKGBUILD.
func (p *Package) extractTopLevel() {
	record := func(as *syntax.Assign) {
		if as.Name == nil {
			return
		}
		if as.Index != nil {
			p.recordIndexed(as)
			return
		}
		v := &Var{Name: as.Name.Value, Pos: as.Pos(), Assign: as}
		if as.Array != nil {
			v.Array = true
			v.ElemCount = len(as.Array.Elems)
			expanded := false
			for elemIdx, el := range as.Array.Elems {
				vals, ok, isRef := p.expandArrayRef(el.Value)
				if ok && len(v.Values)+len(vals) > assignIndexMax {
					ok = false // hostile input must not stack expansions past the cap
				}
				if ok {
					expanded = true
					v.Values = append(v.Values, vals...)
					for range vals {
						v.ElemFor = append(v.ElemFor, elemIdx)
					}
					continue
				}
				if isRef || wordSplits(el.Value) {
					v.CountUnknown = true
				}
				s, _ := RenderWord(el.Value, nil)
				v.Values = append(v.Values, s)
				v.ElemFor = append(v.ElemFor, elemIdx)
			}
			if !expanded {
				v.ElemFor = nil // identity mapping; keep the compact form
			}
		} else if as.Value != nil {
			s, _ := RenderWord(as.Value, nil)
			v.Values = []string{s}
			v.ElemCount = 1
		}
		// `source+=(...)` appends in bash; overwriting here would hide the
		// original assignment from every rule. Appending to a name that has no
		// prior assignment is a plain assignment, as in bash.
		if as.Append {
			if prev, ok := p.Vars[v.Name]; ok {
				p.Vars[v.Name] = mergeAppend(prev, v)
				return
			}
		}
		p.Vars[v.Name] = v
	}
	for _, stmt := range p.PKGBUILD.TopLevel {
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
	p.markConditionalAssigns()
}

// markConditionalAssigns records the variables set by top-level control flow.
// The loop above records only assignments that run unconditionally, but plenty
// of PKGBUILDs set arch, source and sums inside a top-level `if`, `for`, or
// `case "$CARCH"` — makepkg runs all of that before it reads the metadata, so
// what it ends up with is not what was recorded here. Which branch fires, and
// how many times a loop turns, is not knowable without executing the file, so
// rather than guess: name the variable so rules that report a field missing
// stand down, and for arrays additionally mark the length untrustworthy so
// rules that compare lengths do too. Either claim would otherwise be made
// against half the data.
func (p *Package) markConditionalAssigns() {
	for _, stmt := range p.PKGBUILD.TopLevel {
		switch stmt.Cmd.(type) {
		case *syntax.CallExpr, *syntax.DeclClause:
			continue // unconditional, and already recorded above
		}
		syntax.Walk(stmt, func(n syntax.Node) bool {
			switch x := n.(type) {
			case *syntax.FuncDecl:
				// Function bodies run long after makepkg has read the
				// metadata, so what they assign is not part of it.
				return false
			case *syntax.Subshell, *syntax.CmdSubst, *syntax.ProcSubst:
				// These run in a child shell; their assignments die with it
				// and never reach the shell makepkg reads metadata from.
				return false
			case *syntax.CallExpr:
				if len(x.Args) > 0 {
					// `FOO=1 make` scopes FOO to that one command.
					return false
				}
				return true
			case *syntax.Assign:
				if x.Name == nil {
					return true
				}
				if p.ConditionalVars == nil {
					p.ConditionalVars = map[string]bool{}
				}
				p.ConditionalVars[x.Name.Value] = true
				// A scalar counts too. `source="x"` followed by
				// `source+=(a b c)` is four elements to bash, not three: the
				// append converts the scalar into element zero. Any assignment
				// the parser did not record therefore leaves the length of
				// whatever it merges into unknown.
				if v, ok := p.Vars[x.Name.Value]; ok {
					v.CountUnknown = true
				}
			}
			return true
		})
	}
}

// mergeAppend combines a prior top-level assignment with a later `+=` append
// to the same name. Values are concatenated in source order, matching bash,
// where `arr+=(x)` appends an element and `str+=x` concatenates text. If
// either side is an array the merged value is an array; that degenerate
// mismatch (`x=1; x+=(a b)`) keeps both sets of values rather than dropping
// any, since a dropped source is a source no rule can see.
//
// The result keeps the *first* assignment's Pos and Assign, so it stays a
// stable identity for byte-offset edits. Consequently auto-fixes and
// per-element positions anchor to the first assignment: appended elements have
// no entry in Assign.Array.Elems and fall back to the array's start position.
func mergeAppend(prev, add *Var) *Var {
	out := &Var{
		Name: prev.Name, Pos: prev.Pos, Assign: prev.Assign, Array: prev.Array || add.Array,
		ElemCount:    prev.ElemCount + add.ElemCount,
		CountUnknown: prev.CountUnknown || add.CountUnknown,
	}
	if out.Array {
		out.Values = append(append([]string{}, prev.Values...), add.Values...)
		// Element counters continue across the merge, offset by how many
		// elements prev consumed — the same numbering fix code counts over the
		// AST. Only worth materializing once either side left identity.
		if prev.ElemFor != nil || add.ElemFor != nil {
			out.ElemFor = make([]int, 0, len(out.Values))
			for i := range prev.Values {
				out.ElemFor = append(out.ElemFor, prev.elemAt(i))
			}
			for i := range add.Values {
				e := add.elemAt(i)
				if e >= 0 {
					e += prev.ElemCount
				}
				out.ElemFor = append(out.ElemFor, e)
			}
		}
	} else {
		out.Values = []string{strings.Join(prev.Values, "") + strings.Join(add.Values, "")}
	}
	return out
}

// recordIndexed merges an element write (`name[i]=val`) into the recorded
// variable. Bash marks the variable as an array and updates that one element,
// keeping the rest, so any prior assignment must survive: overwriting it here
// hid every other value from the rules and let PB708 mistake the variable for
// a scalar — whose "fix", `name[i]=(val)`, bash rejects outright. As with
// mergeAppend, a merged Var keeps the first assignment's Pos and Assign as a
// stable identity for byte-offset edits. A subscript that is not a number
// literal still fixes the array type but places no value.
func (p *Package) recordIndexed(as *syntax.Assign) {
	name := as.Name.Value
	out := &Var{Name: name, Pos: as.Pos(), Assign: as, Array: true}
	if prev, ok := p.Vars[name]; ok {
		out.Pos, out.Assign = prev.Pos, prev.Assign
		out.Values = append([]string(nil), prev.Values...)
		out.ElemFor = append([]int(nil), prev.ElemFor...)
		out.ElemCount = prev.ElemCount
		out.CountUnknown = prev.CountUnknown
	}
	if idx, ok := AssignIndex(as); ok {
		for len(out.Values) <= idx {
			out.Values = append(out.Values, "")
			if out.ElemFor != nil {
				out.ElemFor = append(out.ElemFor, -1) // padding has no written element
			}
		}
		val, _ := RenderWord(as.Value, nil)
		if as.Append { // `name[i]+=val` concatenates onto the element
			out.Values[idx] += val
		} else {
			out.Values[idx] = val
			if out.ElemFor != nil {
				// The written element's text no longer produces this value.
				out.ElemFor[idx] = -1
			}
		}
		if idx >= out.ElemCount {
			out.ElemCount = idx + 1
		}
	}
	p.Vars[name] = out
}

// assignIndexMax caps how far an indexed assignment may extend Values.
// PKGBUILDs are untrusted input; `a[9999999999]=x` must not allocate a huge
// slice just to record one element.
const assignIndexMax = 4096

// AssignIndex returns the statically-known element index of an indexed
// assignment (`name[i]=val`). ok is false when the assignment has no
// subscript, or when the subscript is not a plain number literal within
// [0, assignIndexMax].
func AssignIndex(as *syntax.Assign) (int, bool) {
	if as == nil || as.Index == nil {
		return 0, false
	}
	w, isWord := as.Index.(*syntax.Word)
	if !isWord {
		return 0, false
	}
	s, dynamic := RenderWord(w, nil)
	if dynamic {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > assignIndexMax {
		return 0, false
	}
	return n, true
}

// Scalar returns the rendered value of a scalar variable, expanded against
// other known scalars.
func (p *Package) Scalar(name string) (string, bool) {
	v, ok := p.Vars[name]
	if !ok || v.Array || len(v.Values) != 1 {
		return "", false
	}
	return p.Expand(v.Values[0]), true
}

var varRef = regexp.MustCompile(`\$(\{[A-Za-z_][A-Za-z0-9_]*\}|[A-Za-z_][A-Za-z0-9_]*)`)

// Expand substitutes $name / ${name} references using known top-level
// variables. An unsubscripted reference to an array expands to its first
// element ($arr means ${arr[0]} in bash), which split PKGBUILDs rely on:
// pkgname=(a b); source=(${pkgname}.service) fetches a.service. Unknown
// references are left as-is.
func (p *Package) Expand(s string) string {
	for range 5 {
		if !strings.Contains(s, "$") {
			break
		}
		out := varRef.ReplaceAllStringFunc(s, func(m string) string {
			name := strings.Trim(m[1:], "{}")
			v, ok := p.Vars[name]
			if !ok {
				return m
			}
			if v.Array {
				if len(v.Values) == 0 {
					return ""
				}
				// "$pkgname" is "${pkgname[0]}" in bash, and bash brace-expands
				// an array assignment before indexing it: with
				// pkgname=(demo{,-vf}) element zero is "demo", not
				// "demo{,-vf}". Splicing the written form instead leaves braces
				// in the result that a later expansion re-expands, turning one
				// source into two.
				return ExpandBraces(v.Values[0])[0]
			}
			if len(v.Values) == 1 {
				return v.Values[0]
			}
			return m
		})
		if out == s {
			break
		}
		s = out
	}
	return s
}

// ValidInstallName reports whether an install= value names a plain file in
// the package directory: not "." or "..", no path separator, and equal to its
// own basename (which rules out parent traversal and absolute paths). It is
// the loader's boundary — a hostile value cannot steer pkglint into reading a
// file outside the package (traversal) or an unbounded device like /dev/zero
// (DoS) — and the rule that reports a missing scriptlet applies the same test
// so it never probes a path the loader would refuse.
func ValidInstallName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		name == filepath.Base(name) && !strings.ContainsAny(name, `/\`)
}

// installFiles returns the scriptlet filenames referenced by install= or
// declared per split package, deduplicated.
func (p *Package) installFiles() []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		if !ValidInstallName(name) {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if v, ok := p.Vars["install"]; ok {
		for _, val := range v.Values {
			add(p.Expand(val))
		}
	}
	if p.SrcInfo != nil {
		for _, val := range p.SrcInfo.All("install") {
			add(val)
		}
	}
	return out
}

// Units returns the PKGBUILD followed by any parsed scriptlets.
func (p *Package) Units() []Unit {
	return append([]Unit{p.PKGBUILD}, p.Scriptlets...)
}

var suppressRe = regexp.MustCompile(`#\s*pkglint:\s*ignore=([A-Z0-9, ]+)`)

// Directive is one inline "# pkglint: ignore=PB123[,PB456]" directive found in
// a single line of source, with the byte spans a rewriter needs to edit it.
type Directive struct {
	IDs      []string // rule IDs in written order, duplicates dropped
	Start    int      // offset of the directive's '#' within the line
	IDsStart int      // span of the rule-ID list, trailing spaces and
	IDsEnd   int      // commas trimmed
}

// ParseDirective returns the inline-ignore directive in one line of source (or
// one comment's raw text), if it carries one. Suppression parsing and the
// stale-directive rule both build on it, so the syntax that is honored and the
// syntax that is audited cannot drift apart.
func ParseDirective(line string) (Directive, bool) {
	m := suppressRe.FindStringSubmatchIndex(line)
	if m == nil {
		return Directive{}, false
	}
	d := Directive{Start: m[0], IDsStart: m[2], IDsEnd: m[3]}
	for d.IDsEnd > d.IDsStart {
		if c := line[d.IDsEnd-1]; c != ' ' && c != ',' {
			break
		}
		d.IDsEnd--
	}
	seen := map[string]bool{}
	for id := range strings.SplitSeq(line[d.IDsStart:d.IDsEnd], ",") {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			d.IDs = append(d.IDs, id)
		}
	}
	return d, len(d.IDs) > 0
}

func parseSuppressions(raw []byte) map[int]map[string]bool {
	out := map[int]map[string]bool{}
	n := 0
	for line := range strings.SplitSeq(string(raw), "\n") {
		n++
		d, ok := ParseDirective(line)
		if !ok {
			continue
		}
		ids := map[string]bool{}
		for _, id := range d.IDs {
			ids[id] = true
		}
		out[n] = ids
	}
	return out
}

// Suppressed reports whether ruleID is suppressed at path:line by a directive
// on that line or the line above it, within the same file. A directive in one
// file never suppresses findings in another.
func (p *Package) Suppressed(ruleID, path string, line int) bool {
	perLine, ok := p.Suppressions[path]
	if !ok {
		return false
	}
	for _, l := range []int{line, line - 1} {
		if ids, ok := perLine[l]; ok && ids[ruleID] {
			return true
		}
	}
	return false
}

// RenderWord returns an approximate textual form of a word. dynamic is true
// when the word contains constructs whose value cannot be determined
// statically (command substitution, process substitution, arithmetic,
// indirection, or parameter operations). Plain references to unknown
// variables render as "$name" and are not considered dynamic, so prefix
// checks like `$pkgdir/...` still work. vars, when non-nil, resolves simple
// parameter references.
func RenderWord(w *syntax.Word, vars map[string]string) (s string, dynamic bool) {
	if w == nil {
		return "", false
	}
	var b strings.Builder
	dyn := renderParts(&b, w.Parts, vars)
	return b.String(), dyn
}

// expandArrayRef statically expands an array element that is a whole-array
// reference — `"${files[@]}"` alone in its word, plain or with makepkg's
// prefix/suffix idiom (`"${files[@]/#/$url/}"` prepends, `"${files[@]/%/.sig}"`
// appends) or a literal-pattern replacement — into one value per element of the
// referenced array. Values keep unexpanded $refs (the prefix above stays
// "$url/") for Expand to resolve at use, exactly like directly written
// elements.
//
// ok reports a successful expansion. isRef reports that the element names a
// whole array even when it cannot be expanded — the caller then knows the
// rendered value count is not the array's real length. A reference whose
// operation preserves element count but hides content (glob replacements,
// prefix/suffix strips, case conversion) still expands, to one dynamic marker
// per element: the count is what checksum pairing needs. Static text around
// the reference is kept where bash puts it — "$url/${files[@]}" prefixes the
// first element only; the per-element idiom is the `/#/` replacement. A quoted
// "${files[*]}" joins into a single word and `${#files[@]}` is a count, so
// neither is a multi-element reference.
func (p *Package) expandArrayRef(w *syntax.Word) (vals []string, ok, isRef bool) {
	if w == nil {
		return nil, false, false
	}
	// Locate the word's single whole-array reference; everything around it must
	// render statically and becomes a prefix on the first element and a suffix
	// on the last, which is where bash attaches surrounding text in
	// "$url/${files[@]}". Two references, or dynamic surrounding text, is a
	// count we refuse to guess at.
	var pre, post strings.Builder
	var ref *syntax.ParamExp
	bad := false
	var scan func(parts []syntax.WordPart, inQuotes bool)
	scan = func(parts []syntax.WordPart, inQuotes bool) {
		for _, part := range parts {
			if bad {
				return
			}
			if dq, isDq := part.(*syntax.DblQuoted); isDq && !inQuotes {
				scan(dq.Parts, true)
				continue
			}
			if pe, isPe := part.(*syntax.ParamExp); isPe && isWholeArrayRef(pe, inQuotes) {
				if ref != nil {
					bad = true
					return
				}
				ref = pe
				continue
			}
			b := &pre
			if ref != nil {
				b = &post
			}
			if renderParts(b, []syntax.WordPart{part}, nil) {
				bad = true
				return
			}
		}
	}
	scan(w.Parts, false)
	if ref == nil {
		return nil, false, false
	}
	if bad {
		return nil, false, true
	}
	pe := ref
	if pe.Excl || pe.Names != 0 || pe.Slice != nil {
		return nil, false, true // index lists and slices change the count
	}
	src, known := p.Vars[pe.Param.Value]
	if !known || !src.Array || src.CountUnknown {
		return nil, false, true
	}
	// PKGBUILDs are untrusted input. Chained references double values per
	// assignment and replacements multiply content, so refuse anything past
	// the same per-assignment ceiling indexed writes use, and budget the
	// bytes produced below.
	if len(src.Values) > assignIndexMax {
		return nil, false, true
	}
	rewrite := func(s string) (string, bool) { return s, true }
	switch {
	case pe.Repl != nil:
		orig, dynO := RenderWord(pe.Repl.Orig, nil)
		with, dynW := RenderWord(pe.Repl.With, nil)
		switch {
		case dynO || dynW:
			rewrite = nil
		case !pe.Repl.All && orig == "#": // empty pattern anchored at start: prepend
			rewrite = func(s string) (string, bool) { return with + s, true }
		case !pe.Repl.All && orig == "%": // empty pattern anchored at end: append
			rewrite = func(s string) (string, bool) { return s + with, true }
		case orig != "" && !strings.ContainsAny(orig, `*?[#%\$`):
			if pe.Repl.All {
				rewrite = func(s string) (string, bool) {
					// Bound the output before building it: every match grows
					// the string by the replacement's surplus.
					grown := len(s) + strings.Count(s, orig)*len(with)
					if grown > arrayRefMaxBytes {
						return "", false
					}
					return strings.ReplaceAll(s, orig, with), true
				}
			} else {
				rewrite = func(s string) (string, bool) { return strings.Replace(s, orig, with, 1), true }
			}
		default:
			rewrite = nil // glob or anchored pattern: content unknown, count kept
		}
	case pe.Exp != nil:
		switch pe.Exp.Op {
		case syntax.RemSmallPrefix, syntax.RemLargePrefix, syntax.RemSmallSuffix, syntax.RemLargeSuffix,
			syntax.UpperFirst, syntax.UpperAll, syntax.LowerFirst, syntax.LowerAll:
			rewrite = nil // strips and case conversion preserve the count
		default:
			return nil, false, true // ${files[@]:-...} and friends change it
		}
	}
	if len(src.Values) == 0 {
		// An empty array expands to zero words — unless surrounding text keeps
		// one word alive, as "pre${a[@]}post" does in bash.
		if pre.Len() == 0 && post.Len() == 0 {
			return []string{}, true, true
		}
		return []string{pre.String() + post.String()}, true, true
	}
	total := 0
	vals = make([]string, len(src.Values))
	for i, s := range src.Values {
		if rewrite == nil {
			vals[i] = "\x00"
		} else {
			v, fits := rewrite(s)
			if !fits {
				return nil, false, true
			}
			vals[i] = v
		}
		if i == 0 {
			vals[i] = pre.String() + vals[i]
		}
		if i == len(src.Values)-1 {
			vals[i] += post.String()
		}
		if total += len(vals[i]); total > arrayRefMaxBytes {
			return nil, false, true
		}
	}
	return vals, true, true
}

// arrayRefMaxBytes caps the content one array-reference expansion may produce.
// No real PKGBUILD comes near it; a hostile one chaining replacements to
// multiply string content must not allocate its way out of static analysis.
const arrayRefMaxBytes = 1 << 20

// isWholeArrayRef reports whether the parameter expansion names every element
// of an array: ${a[@]} or unquoted ${a[*]}, in any of the operation forms
// expandArrayRef handles. ${#a[@]} is a count and a quoted "${a[*]}" joins into
// one word, so neither qualifies.
func isWholeArrayRef(pe *syntax.ParamExp, inQuotes bool) bool {
	if pe.Length || pe.Index == nil {
		return false
	}
	iw, isWord := pe.Index.(*syntax.Word)
	if !isWord {
		return false
	}
	idx, dyn := RenderWord(iw, nil)
	if dyn {
		return false
	}
	return idx == "@" || (idx == "*" && !inQuotes)
}

func renderParts(b *strings.Builder, parts []syntax.WordPart, vars map[string]string) bool {
	dynamic := false
	for _, part := range parts {
		switch x := part.(type) {
		case *syntax.Lit:
			b.WriteString(x.Value)
		case *syntax.SglQuoted:
			b.WriteString(x.Value)
		case *syntax.DblQuoted:
			if renderParts(b, x.Parts, vars) {
				dynamic = true
			}
		case *syntax.ParamExp:
			if x.Excl || x.Length || x.Width || x.Index != nil || x.Slice != nil || x.Repl != nil || x.Exp != nil || x.Names != 0 {
				b.WriteString("\x00")
				dynamic = true
				break
			}
			if vars != nil {
				if v, ok := vars[x.Param.Value]; ok {
					b.WriteString(v)
					break
				}
			}
			b.WriteString("$" + x.Param.Value)
		default:
			b.WriteString("\x00")
			dynamic = true
		}
	}
	return dynamic
}
