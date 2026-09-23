package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/pkgfile"
	"mvdan.cc/sh/v3/syntax"
)

// FixLevel classifies how aggressive a rule's auto-fix is.
type FixLevel int

const (
	// FixNone means the rule has no auto-fix.
	FixNone FixLevel = iota
	// FixSafe fixes are deterministic rewrites that preserve intended behavior
	// or restore a security default. They are applied by --fix.
	FixSafe
	// FixUnsafe fixes are mechanical but behavior-changing, so they need
	// review. They are applied only by --unsafe-fix (which implies --fix).
	FixUnsafe
)

// FixCommand is the pkglint invocation that applies this rule's auto-fix,
// empty when it has none. It is the bare flag for a PKGBUILD-scope rule, and
// `build --fix` for a package-scope one: that finding comes out of a built
// archive, and `pkglint --fix` has only the PKGBUILD to read, so `build` — the
// one command that holds both — is where the fix can be applied at all.
func (r Rule) FixCommand() string {
	flag := r.FixLevel.Flag()
	if flag == "" || r.Scope != ScopePackage {
		return flag
	}
	return "build " + flag
}

// Fixable reports whether the level has an auto-fix at all.
func (l FixLevel) Fixable() bool { return l == FixSafe || l == FixUnsafe }

// Safe reports whether the fix is behavior-preserving (applied by --fix).
func (l FixLevel) Safe() bool { return l == FixSafe }

// Flag is the CLI flag that applies fixes at this level (empty when none).
func (l FixLevel) Flag() string {
	switch l {
	case FixSafe:
		return "--fix"
	case FixUnsafe:
		return "--unsafe-fix"
	default:
		return ""
	}
}

// Edit is a byte-range replacement within a single unit's Raw source. Start
// and End are byte offsets into that unit's Raw; Start == End is an insertion.
type Edit struct {
	Path   string
	Start  int
	End    int
	New    string
	Line   int // 1-based line of the change, for inline-suppression checks
	RuleID string
	Desc   string // human-readable description of what changed
	// Group ties edits that only make sense together, so an all-or-nothing
	// rewrite cannot be applied in part. A rename is the case that needs it: a
	// declaration renamed without one of its references — because another
	// rule's edit overlapped that byte range, or an inline directive covered
	// its line — leaves a PKGBUILD referring to a variable nothing sets. Empty
	// means the edit stands alone, which is true of every single-site fix.
	Group string

	// creates, with the anchor text createHead/createTail wrapped around it,
	// describes an edit that writes whole array assignments into a PKGBUILD
	// that had none. Keeping the parts rather than only the rendered New is
	// what lets mergeCreations fold two rules reaching for the same absent
	// array into one assignment. Empty on every other edit.
	creates                []arrayCreate
	createHead, createTail string
}

// FixEnv carries capabilities a fixer needs but must not perform itself (for
// example, network access). The caller supplies it only when a fix is
// requested.
type FixEnv struct {
	// ResolveRef maps a VCS URL and a mutable ref (a tag or branch name) to an
	// immutable commit hash. It is nil when resolution is unavailable (e.g.
	// offline); a fixer that needs it then emits no edit and the finding
	// stands. Implementations run `git ls-remote` or similar — network I/O the
	// rules package deliberately delegates to the caller.
	ResolveRef func(url, ref string) (string, error)

	// LocalDigest hashes a source file that is *already* on disk, given the
	// package directory and the local filename makepkg gives the source. It
	// returns an error when the file is not there, and the fixer that needs it
	// then emits no edit and the finding stands.
	//
	// It never downloads. A digest cannot be derived from PKGBUILD text the
	// way a commit hash can be derived from a ref, so PB102's fix only repairs
	// packages whose sources have been fetched already — deliberately, since
	// fetching would mean pkglint issuing requests to URLs it read out of an
	// untrusted file. Implementations look in the package directory and
	// makepkg's source cache and nowhere else; the rules package does no file
	// I/O of its own.
	LocalDigest func(dir, filename string) (Digests, error)

	// ProbeHTTPS reports whether an https URL is actually served, returning nil
	// when it is and an error describing the refusal otherwise. It is nil when
	// probing is unavailable (offline), and a fixer that needs it then emits no
	// edit and the finding stands.
	//
	// It exists so the transport fixes can be checked rather than hoped: only
	// the server knows whether it answers on https, and PB104's rewrite is a
	// claim about the server. Implementations issue a headers-only request and
	// read no body — what comes back is used solely to decide whether the URL
	// resolves, never as input to anything. That distinction is what keeps this
	// inside pkglint's "never act on what the file says" line: the PKGBUILD
	// chooses an address to knock on, and nothing more.
	//
	// A successful probe means the URL exists over https, not that it serves
	// the same bytes. Nothing but the checksum can say that, which is why the
	// fix stays unsafe even when the probe passes.
	ProbeHTTPS func(url string) error
}

// Digests are hashes of one local source file, all computed from a single read
// so they provably describe the same bytes — which is what lets a fixer check
// a weak digest and emit a strong one for the same content. A field is empty
// when the implementation did not compute that algorithm.
type Digests struct {
	MD5    string
	SHA1   string
	SHA256 string
}

// Fixer computes edits that resolve a rule's findings. It returns nil for
// occurrences it cannot fix safely.
type Fixer func(*Context, *FixEnv) []Edit

// FixResult is the outcome of applying edits to one unit.
type FixResult struct {
	Path     string
	Original []byte
	Fixed    []byte
	Applied  []Edit // in original file order
}

// Changed reports whether the fix altered the unit.
func (r FixResult) Changed() bool { return !bytes.Equal(r.Original, r.Fixed) }

// CollectEdits runs every eligible fixer and returns the edits it proposes,
// excluding rules in ignore, occurrences suppressed inline, and fixes above
// the requested level.
func CollectEdits(ctx *Context, ignore map[string]bool, level FixLevel, env *FixEnv) []Edit {
	return collectEdits(ctx, ignore, level, env, ScopePKGBUILD)
}

// collectEdits is CollectEdits restricted to one scope. The two scopes are
// never collected together: a PKGBUILD-scope fix answers a finding the file
// alone produced and belongs to `--fix`, while a package-scope fix answers one
// only a build could produce and belongs to `pkglint build --fix`. Running
// both from the build would silently make it the more thorough `--fix`, on a
// PKGBUILD whose static findings were already reported and gated.
func collectEdits(ctx *Context, ignore map[string]bool, level FixLevel, env *FixEnv, scope Scope) []Edit {
	if env == nil {
		env = &FixEnv{}
	}
	var edits []Edit
	// A suppressed edit that belongs to a group waives the whole group: the
	// rest of it would rewrite half a rename, and a directive on a
	// declaration's line is a maintainer declining that rename entirely.
	waived := map[string]bool{}
	for _, rule := range registry() {
		if rule.Scope != scope || rule.Fix == nil || rule.FixLevel == FixNone || rule.FixLevel > level || ignore[rule.ID] {
			continue
		}
		for _, e := range rule.Fix(ctx, env) {
			e.RuleID = rule.ID
			// e.Path is the unit the edit rewrites: usually the PKGBUILD, but
			// command-driven fixers also edit scriptlets. Check the directive in
			// that file, not a fixed one.
			if ctx.Pkg.Suppressed(rule.ID, e.Path, e.Line) {
				if e.Group != "" {
					waived[e.Group] = true
				}
				continue
			}
			edits = append(edits, e)
		}
	}
	// Merging last, on what survived: an edit waived by a directive must not
	// reach the array it wanted through a neighbour it was folded into.
	return mergeCreations(dropGroups(edits, waived))
}

// dropGroups removes every edit belonging to one of the named groups, keeping
// ungrouped edits and the edits of groups that survived intact.
func dropGroups(edits []Edit, groups map[string]bool) []Edit {
	if len(groups) == 0 {
		return edits
	}
	out := edits[:0]
	for _, e := range edits {
		if e.Group != "" && groups[e.Group] {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Fix computes and applies auto-fixes for a package at the given level,
// returning one FixResult per unit that had edits applied.
func Fix(pkg *pkgbuild.Package, ignore map[string]bool, level FixLevel, env *FixEnv) []FixResult {
	return applyByUnit(pkg, CollectEdits(NewContext(pkg), ignore, level, env))
}

// FixPackage computes and applies the package-scope auto-fixes for pkg from the
// archive it built, returning one FixResult per unit that had edits applied.
// db is the pacman local database the package-scope rules resolve against; a
// nil db stands the fixes down exactly as it stands the rules down.
//
// pf must be the archive pkg produced. Nothing here verifies that a build
// happened — only `pkglint build` can say so — but the fixers check that the
// archive is the package the PKGBUILD declares before writing anything, so a
// mismatched pair yields no edits rather than a wrong one. See fixpkg.go.
func FixPackage(pkg *pkgbuild.Package, pf *pkgfile.Package, db *alpmdb.DB, ignore map[string]bool, level FixLevel, env *FixEnv) []FixResult {
	ctx := NewContext(pkg)
	ctx.File = pf
	ctx.DB = db
	return applyByUnit(pkg, collectEdits(ctx, ignore, level, env, ScopePackage))
}

// applyByUnit applies the edits to the units they address, returning one
// FixResult per unit that had edits.
func applyByUnit(pkg *pkgbuild.Package, edits []Edit) []FixResult {
	byPath := map[string][]Edit{}
	for _, e := range edits {
		byPath[e.Path] = append(byPath[e.Path], e)
	}
	var results []FixResult
	for _, u := range pkg.Units() {
		es := byPath[u.Path]
		if len(es) == 0 {
			continue
		}
		fixed, applied := ApplyEdits(u.Raw, es)
		results = append(results, FixResult{Path: u.Path, Original: u.Raw, Fixed: fixed, Applied: applied})
	}
	return results
}

// ApplyEdits applies edits (all addressing the same raw source) and returns
// the rewritten bytes plus the edits actually applied, in file order.
// Overlapping edits are resolved by keeping the earlier-starting one.
//
// An edit dropped that way takes the rest of its Group with it, and the
// selection then runs again: a group is all-or-nothing, and removing one
// frees the byte ranges its members had claimed for whatever they displaced.
// Each round drops at least one edit, so the loop terminates.
func ApplyEdits(raw []byte, edits []Edit) (result []byte, applied []Edit) {
	sorted := append([]Edit(nil), edits...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Start != sorted[j].Start {
			return sorted[i].Start < sorted[j].Start
		}
		return sorted[i].End < sorted[j].End
	})
	var kept []Edit
	for {
		kept = selectEdits(raw, sorted)
		broken := brokenGroups(sorted, kept)
		if len(broken) == 0 {
			break
		}
		sorted = dropGroups(sorted, broken)
	}
	out := raw
	for i := len(kept) - 1; i >= 0; i-- {
		e := kept[i]
		var buf []byte
		buf = append(buf, out[:e.Start]...)
		buf = append(buf, e.New...)
		buf = append(buf, out[e.End:]...)
		out = buf
	}
	return out, kept
}

// selectEdits picks the applicable, non-overlapping edits from a start-sorted
// list, keeping the earlier-starting one of any overlapping pair.
func selectEdits(raw []byte, sorted []Edit) []Edit {
	var kept []Edit
	lastEnd := 0
	for _, e := range sorted {
		if e.Start < 0 || e.End > len(raw) || e.Start > e.End {
			continue
		}
		if len(kept) > 0 && e.Start < lastEnd {
			continue // overlaps an already-kept edit
		}
		kept = append(kept, e)
		lastEnd = e.End
	}
	return kept
}

// brokenGroups names the groups that lost a member during selection, and so
// must be abandoned rather than half-applied.
func brokenGroups(sorted, kept []Edit) map[string]bool {
	total := map[string]int{}
	for _, e := range sorted {
		if e.Group != "" {
			total[e.Group]++
		}
	}
	if len(total) == 0 {
		return nil
	}
	for _, e := range kept {
		if e.Group != "" {
			total[e.Group]--
		}
	}
	var broken map[string]bool
	for g, missing := range total {
		if missing > 0 {
			if broken == nil {
				broken = map[string]bool{}
			}
			broken[g] = true
		}
	}
	return broken
}

// off is the byte offset of a position, as an int.
func off(p syntax.Pos) int { return int(p.Offset()) }

// wordByValue returns the first argument word of c whose statically rendered
// value equals val, or nil.
func wordByValue(c Command, val string) *syntax.Word {
	if val == "" {
		return nil
	}
	for _, w := range c.Call.Args {
		if s, _ := pkgbuild.RenderWord(w, nil); s == val {
			return w
		}
	}
	return nil
}

// --- shared flag-insertion primitives ----------------------------------------

// appendFlagAt returns where a flag appended to c belongs, whether that place
// is in front of a `--` separator, and whether there is a place at all.
//
// The end of the call is the right spot for a tool that reads its own flags
// anywhere in the argument list, and it is the one placement that works on a
// command with an unreadable word in the middle, since the edit never has to
// know what the words in between say. Past a `--` the words belong to the
// program the tool execs rather than to the tool, so a flag appended there is
// handed to the wrong process; it goes in front of the separator instead. A
// separator that arrived through an expansion has no place in the text to
// insert before, and the caller then emits nothing so the finding stands.
// That holds for a variable pkglint can read as much as one it cannot: an
// argument's rendered value is split into the words bash would make of it,
// the way cargoWords does, so `_rest='-- extra'` is a separator too.
//
// Generalized from fixCargoLocked, which asks exactly this question of cargo.
func appendFlagAt(c Command) (at int, beforeSep bool, ok bool) {
	for _, a := range c.Args {
		if !slices.Contains(strings.Fields(a), "--") {
			continue
		}
		w := wordByValue(c, "--")
		if w == nil {
			return 0, true, false
		}
		return off(w.Pos()), true, true
	}
	return off(c.Call.End()), false, true
}

// appendFlagEdit builds the edit that adds flag to c at appendFlagAt's
// placement, or reports false when there is nowhere to put it.
func appendFlagEdit(c Command, flag, desc string) (Edit, bool) {
	at, beforeSep, ok := appendFlagAt(c)
	if !ok {
		return Edit{}, false
	}
	text := " " + flag
	if beforeSep {
		text = flag + " "
	}
	return Edit{
		Path:  c.Unit.Path,
		Start: at,
		End:   at,
		New:   text,
		Line:  int(c.Stmt.Pos().Line()),
		Desc:  desc,
	}, true
}

// hiddenFlagWords reports whether c carries a word that could be a flag
// pkglint cannot read: one that is nothing but a variable reference, which
// bash splits into however many words the variable holds. The flag a fixer is
// about to insert may already be in there, and inserting it twice is at best
// noise and at worst — for a define whose last spelling wins — a silent
// override of what the PKGBUILD asked for.
//
// A word that merely contains an expansion is not a hidden flag: `-S
// "$srcdir/pkg"` and `--root="$pkgdir/usr"` are flags whose *values* are
// unreadable, which says nothing about which flags are present. Treating every
// expansion as a possible flag would cost these fixes most of what they can
// fix, since real build invocations are full of $srcdir paths. This is the
// same distinction cargoFlagsHidden draws for cargo.
func hiddenFlagWords(c Command) bool {
	for i, w := range c.ArgWord {
		if argOpaque(c, i) && varRefName(w) != "" {
			return true
		}
	}
	return false
}

// fixLockfileFlags appends the lockfile flag to every unpinned invocation of
// the lockfileFlags entries registered under id, declining where the flag
// could already be hiding in a variable, where it would change the command's
// meaning, or where there is no place in the text to put it.
func fixLockfileFlags(ctx *Context, id string) []Edit {
	var edits []Edit
	for _, l := range lockfileFlags {
		if l.id != id {
			continue
		}
		fixSubs := l.fixSubs
		if fixSubs == nil {
			fixSubs = l.subs
		}
		for _, c := range ctx.CommandsNamed(l.commands...) {
			sub, ok := l.unpinned(c)
			if !ok || !slices.Contains(fixSubs, sub) {
				continue
			}
			if hiddenFlagWords(c) {
				continue
			}
			edit, ok := appendFlagEdit(c, l.flag, l.desc(c.Name, sub))
			if !ok {
				continue
			}
			edits = append(edits, edit)
		}
	}
	return edits
}

// lockfileFixer is the Fixer for a rule whose only fix is fixLockfileFlags.
func lockfileFixer(id string) Fixer {
	return func(ctx *Context, _ *FixEnv) []Edit { return fixLockfileFlags(ctx, id) }
}

// replaceWordTailEdit rewrites the trailing occurrence of old inside a word,
// leaving the rest of the word — a `-D` prefix, a `:STRING` type, the quotes
// around the whole thing — exactly as written. It reports false when old is
// not what the word ends on (bar closing quotes), which is the fixer saying
// the word is not the shape it was told to expect.
func replaceWordTailEdit(u *pkgbuild.Unit, w *syntax.Word, old, new, desc string) (Edit, bool) {
	start, end := off(w.Pos()), off(w.End())
	if start < 0 || end > len(u.Raw) || start >= end {
		return Edit{}, false
	}
	raw := string(u.Raw[start:end])
	i := strings.LastIndex(raw, old)
	if i < 0 || strings.Trim(raw[i+len(old):], `"'`) != "" {
		return Edit{}, false
	}
	return Edit{
		Path:  u.Path,
		Start: start + i,
		End:   start + i + len(old),
		New:   new,
		Line:  int(w.Pos().Line()),
		Desc:  desc,
	}, true
}

// --- shared array-element primitives -----------------------------------------

// The dependency, provides and options rules all want one of two edits: put a
// name into a top-level array, or take one out. Both have to reckon with the
// same thing — pkgbuild.Var is a *merged* view of every assignment to a name,
// while an edit can only touch the one assignment that has source text here —
// so the guard is shared rather than restated per rule.

// rewritableArray returns the array assignment of name that a fixer may edit,
// or nil.
//
// It declines an array that is not one plain literal: a `+=` merge or an
// indexed write contributes values whose source text is elsewhere, and a
// whole-array reference that could not be expanded (CountUnknown) means the
// elements written here are not the elements makepkg will see. Rewriting the
// first assignment of either would be an edit made against half the data.
func rewritableArray(pkg *pkgbuild.Package, name string) *pkgbuild.Var {
	v := pkg.Vars[name]
	if v == nil || !v.Array || v.CountUnknown || v.Assign == nil || v.Assign.Array == nil {
		return nil
	}
	if v.ElemCount != len(v.Assign.Array.Elems) {
		return nil
	}
	return v
}

// arrayEntryText quotes entries for an array literal, joined by sep.
func arrayEntryText(entries []string, quote, sep string) string {
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = quote + e + quote
	}
	return strings.Join(parts, sep)
}

// arrayAnchors are the fields a created array is written after: the ones every
// PKGBUILD declares, in the order the guidelines' template puts them. The last
// one present wins, so the new line lands in the metadata block rather than
// above the Maintainer comment — which is where offset 0 would put it.
var arrayAnchors = []string{"pkgbase", "pkgname", "pkgver", "pkgrel", "arch", "url", "license", "depends"}

// validArrayEntries reports whether the fixer can render entries faithfully.
// It writes them between quotes it chose, so a value carrying whitespace, a
// quote or an expansion of its own is one it must decline.
func validArrayEntries(entries []string) bool {
	if len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if e == "" || strings.ContainsAny(e, "'\"$ \t\n") {
			return false
		}
	}
	return true
}

// addArrayElemsEdit puts entries into the top-level array field, creating the
// array when it is absent. It reports false when there is no place to write
// that the whole file agrees on.
func addArrayElemsEdit(ctx *Context, field string, entries []string, desc string) (Edit, bool) {
	if !validArrayEntries(entries) {
		return Edit{}, false
	}
	u := &ctx.Pkg.PKGBUILD
	if ctx.Pkg.Vars[field] == nil {
		return newArraysEdit(ctx, []string{field}, entries, desc)
	}
	// Top-level control flow also assigns this name before makepkg reads it,
	// so which value survives is not knowable without running the file.
	if ctx.Pkg.ConditionalVars[field] {
		return Edit{}, false
	}
	v := rewritableArray(ctx.Pkg, field)
	if v == nil {
		return Edit{}, false
	}
	elems := v.Assign.Array.Elems
	rparen := off(v.Assign.Array.Rparen)
	if len(elems) == 0 {
		if rparen <= 0 || rparen > len(u.Raw) {
			return Edit{}, false
		}
		return Edit{Path: u.Path, Start: rparen, End: rparen,
			New: arrayEntryText(entries, "'", " "), Line: int(v.Pos.Line()), Desc: desc}, true
	}
	last := elems[len(elems)-1]
	if last.Value == nil || last.Index != nil {
		return Edit{}, false // an indexed write, or a value with no text
	}
	at, from := off(last.Value.End()), off(last.Value.Pos())
	if at <= 0 || at > rparen || rparen > len(u.Raw) {
		return Edit{}, false
	}
	// One element per line stays one element per line: an array written that
	// way is usually long enough that appending on the closing line would run
	// past anyone's column limit.
	quote, sep := "'", " "
	if c := u.Raw[from]; c == '"' {
		quote = `"`
	}
	if bytes.ContainsRune(u.Raw[at:rparen], '\n') {
		sep = "\n" + lineIndent(u.Raw, from)
	}
	return Edit{Path: u.Path, Start: at, End: at,
		New: sep + arrayEntryText(entries, quote, sep), Line: int(last.Value.Pos().Line()), Desc: desc}, true
}

// arrayCreate is one `field=(entries…)` assignment a creating edit writes. It
// rides along on the Edit so two rules reaching for the same absent array can
// be folded into one assignment; see mergeCreations.
type arrayCreate struct {
	Field   string
	Entries []string
}

// newArraysEdit writes whole array assignments on the line after the last
// anchor field the PKGBUILD declares, as one edit.
//
// One edit, not one per field, because the edit claims the anchor's line
// ending rather than inserting beside it: an insertion at a zero-width point
// would let a second array-creating fix insert at the same point, and two
// `makedepends=()` lines are not an addition — the second is an assignment
// that drops whatever the first declared. Claiming a byte makes that a
// collision the machinery can see, and mergeCreations settles it.
func newArraysEdit(ctx *Context, fields []string, entries []string, desc string) (Edit, bool) {
	if !validArrayEntries(entries) || len(fields) == 0 {
		return Edit{}, false
	}
	creates := make([]arrayCreate, 0, len(fields))
	for _, f := range fields {
		if ctx.Pkg.Vars[f] != nil || ctx.Pkg.ConditionalVars[f] {
			return Edit{}, false
		}
		creates = append(creates, arrayCreate{Field: f, Entries: entries})
	}
	u := &ctx.Pkg.PKGBUILD
	at := -1
	for _, name := range arrayAnchors {
		v := ctx.Pkg.Vars[name]
		if v == nil || v.Assign == nil {
			continue
		}
		if end := off(v.Assign.End()); end > at {
			at = end
		}
	}
	if at <= 0 || at > len(u.Raw) {
		return Edit{}, false
	}
	// Past anything else on the anchor's closing line — a trailing comment
	// belongs to the line it was written on, not to the new assignment.
	for at < len(u.Raw) && u.Raw[at] != '\n' {
		at++
	}
	e := Edit{Path: u.Path, Start: at, End: at + 1, Line: lineOf(u.Raw, at), Desc: desc,
		creates: creates, createTail: "\n"}
	if at >= len(u.Raw) {
		// The anchor is the last line and the file has no trailing newline:
		// claim its final byte and write it back, so this edit still holds
		// ground a second array-creating fix would have to overlap.
		e.Start, e.End, e.createHead, e.createTail = at-1, at, string(u.Raw[at-1]), ""
	}
	e.New = createdArraysText(e)
	return e, true
}

// createdArraysText renders an array-creating edit's replacement: the assigned
// arrays, wrapped in the text of the anchor byte the edit claimed.
func createdArraysText(e Edit) string {
	var b strings.Builder
	b.WriteString(e.createHead)
	for _, c := range e.creates {
		b.WriteString("\n" + c.Field + "=(" + arrayEntryText(c.Entries, "'", " ") + ")")
	}
	b.WriteString(e.createTail)
	return b.String()
}

// mergeCreations folds array-creating edits that claim the same anchor byte
// into one assignment block.
//
// Two rules reaching for an absent makedepends — a git source needing its
// client and a cmake build needing cmake — each want to write the assignment,
// and only one can. Merging is what lets a single --fix run settle both
// instead of applying one and leaving the other's finding for the next run.
//
// Grouped edits are left out of the merge. A group is all-or-nothing, and a
// merged edit has no way to carry one contributor's group without imposing it
// on the rest: a waiver on the group would then take an unrelated rule's
// dependency with it. Such an edit keeps colliding, which costs a run rather
// than correctness.
func mergeCreations(edits []Edit) []Edit {
	type slot struct {
		at     int      // the contributing edit that keeps the anchor
		fields []string // in first-seen order, so the output does not shuffle
		by     map[string][]string
		ids    []string
		descs  []string
	}
	slots := map[string]*slot{}
	var order []*slot
	absorbed := map[int]bool{}
	for i, e := range edits {
		if len(e.creates) == 0 || e.Group != "" {
			continue
		}
		k := fmt.Sprintf("%s\x00%d\x00%d", e.Path, e.Start, e.End)
		s := slots[k]
		if s == nil {
			s = &slot{at: i, by: map[string][]string{}}
			slots[k] = s
			order = append(order, s)
		} else {
			absorbed[i] = true
		}
		for _, c := range e.creates {
			if _, seen := s.by[c.Field]; !seen {
				s.fields = append(s.fields, c.Field)
			}
			for _, entry := range c.Entries {
				// Two rules asking for the same package — a Rust build whose
				// sources are also a git checkout — declare it once.
				if !slices.Contains(s.by[c.Field], entry) {
					s.by[c.Field] = append(s.by[c.Field], entry)
				}
			}
		}
		s.ids = append(s.ids, e.RuleID)
		s.descs = append(s.descs, e.Desc)
	}
	if len(absorbed) == 0 {
		return edits
	}
	for _, s := range order {
		if len(s.ids) < 2 {
			continue
		}
		e := edits[s.at]
		e.creates = nil
		for _, f := range s.fields {
			e.creates = append(e.creates, arrayCreate{Field: f, Entries: s.by[f]})
		}
		e.RuleID = strings.Join(s.ids, ",")
		e.Desc = strings.Join(s.descs, "; ")
		e.New = createdArraysText(e)
		edits[s.at] = e
	}
	out := edits[:0]
	for i, e := range edits {
		if !absorbed[i] {
			out = append(out, e)
		}
	}
	return out
}

// lineOf is the 1-based line the byte at o sits on.
func lineOf(raw []byte, o int) int {
	if o > len(raw) {
		o = len(raw)
	}
	return 1 + bytes.Count(raw[:o], []byte("\n"))
}

// wordValueCount is how many of v's rendered values the written element w
// produced. More than one means a brace group — `depends=(python-{foo,bar})` —
// whose word cannot be deleted for just one of the names it expands to.
func wordValueCount(v *pkgbuild.Var, w *syntax.Word) int {
	n := 0
	for _, e := range varElems(v) {
		if e.Word == w {
			n++
		}
	}
	return n
}

// removeArrayElemEdits deletes written elements from a top-level array.
//
// When every element goes the whole assignment goes with them: `provides=()`
// is valid bash, but a rule that called the entries dead metadata did not ask
// for an empty array to be left behind in their place. Words the array does
// not own, and brace groups that stand for more names than the caller named,
// are skipped rather than guessed at.
func removeArrayElemEdits(u *pkgbuild.Unit, v *pkgbuild.Var, words []*syntax.Word, desc string) []Edit {
	if v == nil || v.Assign == nil || v.Assign.Array == nil || v.CountUnknown {
		return nil
	}
	if v.ElemCount != len(v.Assign.Array.Elems) {
		return nil
	}
	own := map[*syntax.Word]bool{}
	for _, e := range v.Assign.Array.Elems {
		// An indexed write puts its value at a position other rules and other
		// writes count on; deleting the text would renumber the array.
		if e.Value != nil && e.Index == nil {
			own[e.Value] = true
		}
	}
	drop := map[*syntax.Word]bool{}
	for _, w := range words {
		if w == nil || !own[w] || wordValueCount(v, w) != 1 {
			continue
		}
		drop[w] = true
	}
	if len(drop) == 0 {
		return nil
	}
	if len(drop) == len(v.Assign.Array.Elems) {
		start, end := lineCut(u.Raw, off(v.Assign.Pos()), off(v.Assign.End()))
		return []Edit{{Path: u.Path, Start: start, End: end, New: "",
			Line: int(v.Assign.Pos().Line()), Desc: desc}}
	}
	var edits []Edit
	for _, e := range v.Assign.Array.Elems {
		if e.Value == nil || !drop[e.Value] {
			continue
		}
		start, end := arrayElemCut(u.Raw, off(e.Value.Pos()), off(e.Value.End()))
		edits = append(edits, Edit{Path: u.Path, Start: start, End: end, New: "",
			Line: int(e.Value.Pos().Line()), Desc: desc})
	}
	return edits
}

// arrayElemCut widens the byte range of an array element to what deleting it
// should take: its whole line when it had one to itself, else the run of
// spaces on one side of it, so the elements that remain stay separated by
// exactly one space.
func arrayElemCut(raw []byte, start, end int) (int, int) {
	ls := lineStart(raw, start)
	if strings.TrimSpace(string(raw[ls:start])) == "" {
		rest := end
		for rest < len(raw) && (raw[rest] == ' ' || raw[rest] == '\t') {
			rest++
		}
		if rest < len(raw) && raw[rest] == '\n' {
			return ls, rest + 1
		}
	}
	s := start
	for s > 0 && (raw[s-1] == ' ' || raw[s-1] == '\t') {
		s--
	}
	if s < start {
		return s, end
	}
	e := end
	for e < len(raw) && (raw[e] == ' ' || raw[e] == '\t') {
		e++
	}
	return start, e
}

// lineCut widens a byte range to the whole line, newline included, when only
// indentation precedes it — so deleting a statement does not leave the blank
// line it stood on. Anything sharing the line keeps it.
func lineCut(raw []byte, start, end int) (int, int) {
	ls := start
	for ls > 0 && (raw[ls-1] == ' ' || raw[ls-1] == '\t') {
		ls--
	}
	if ls != 0 && raw[ls-1] != '\n' {
		return start, end
	}
	start = ls
	for end < len(raw) && (raw[end] == ' ' || raw[end] == '\t') {
		end++
	}
	if end < len(raw) && raw[end] == '\n' {
		end++
	}
	return start, end
}

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
	perElem := map[string]int{}
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

func elemKey(arch string, idx int) string { return arch + "\x00" + strconv.Itoa(idx) }

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
func sourceElems(u *pkgbuild.Unit) map[string]*syntax.Word {
	out := map[string]*syntax.Word{}
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
	var order []string
	groups := map[string][]pkgbuild.SourceEntry{}
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

// --- PB203: cargo without --locked -----------------------------------------

// fixCargoLocked adds --locked to the cargo commands PB203 reports, at the end
// of the command. Appending, rather than inserting after the subcommand, is
// what lets it fix a command with an unreadable value in it — `cargo fetch
// --target "$CARCH-…"`, the guidelines' own prepare() — since the edit never
// has to know what the words in between say. cargoUnlockedCommands has already
// left out the commands whose flags pkglint cannot read.
//
// The end is the wrong place once the command has a `--`, since past it the
// words are the called program's: appending to `cargo rustc -- -C
// target-cpu=native` hands rustc a --locked it rejects, trading the finding for
// a build that no longer runs. The flag goes in front of the separator
// instead — and where the separator arrives through a variable there is no
// place in the text to put it, so the finding stands.
func fixCargoLocked(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range cargoUnlockedCommands(ctx) {
		// cargoHasSeparator sees a `--` that arrives through a variable;
		// appendFlagAt only sees one written out. A separator with no word
		// spelling it has nowhere in the text to write in front of, so the
		// finding stands.
		if cargoHasSeparator(c) && wordByValue(c, "--") == nil {
			continue
		}
		edit, ok := appendFlagEdit(c, "--locked",
			fmt.Sprintf("add --locked to `cargo %s`", c.Subcommand()))
		if !ok {
			continue
		}
		edits = append(edits, edit)
	}
	return edits
}

// --- PB940: cargo test/check --release -------------------------------------

// fixCargoCheckRelease deletes the release flag from cargo test/check so the
// run keeps the debug assertions and overflow checks it exists to exercise. It
// removes whichever spelling the command used, `--release` or `-r`, and only
// one written literally before the `--` separator: one that arrives through a
// variable is not ours to rewrite, and one after `--` is the test binary's
// argument, which cargoReleaseFlag already declines to report.
func fixCargoCheckRelease(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range ctx.CommandsNamed("cargo") {
		sub := c.Subcommand()
		flag := cargoReleaseFlag(c)
		if (sub != "test" && sub != "check") || flag == "" {
			continue
		}
		w := wordByValue(c, flag)
		if w == nil {
			continue // written as an expansion: the finding stands
		}
		start, end := flagCut(c.Unit.Raw, off(w.Pos()), off(w.End()))
		edits = append(edits, Edit{
			Path:  c.Unit.Path,
			Start: start,
			End:   end,
			New:   "",
			Line:  int(w.Pos().Line()),
			Desc:  fmt.Sprintf("drop %s from `cargo %s` (keeps debug assertions and overflow checks on)", flag, sub),
		})
	}
	return edits
}

// flagCut widens the byte range of a flag word to what deleting it should
// actually take: the whitespace in front of it, and — when the flag sits alone
// on a continuation line — that whole line, including the backslash that would
// otherwise be left continuing into nothing or dangling off the line above.
func flagCut(raw []byte, start, end int) (int, int) {
	for start > 0 && (raw[start-1] == ' ' || raw[start-1] == '\t') {
		start--
	}
	if start == 0 || raw[start-1] != '\n' {
		return start, end
	}
	rest := end
	for rest < len(raw) && (raw[rest] == ' ' || raw[rest] == '\t') {
		rest++
	}
	switch {
	case rest+1 < len(raw) && raw[rest] == '\\' && raw[rest+1] == '\n':
		// More words follow: take this line and its newline.
		return start, rest + 2
	case rest >= len(raw) || raw[rest] == '\n':
		// Last line of the call: the previous line's `\` continued into it and
		// must go too, or the command runs on into whatever comes next.
		cut := start - 1 // the newline
		if cut > 0 && raw[cut-1] == '\\' {
			cut--
			for cut > 0 && (raw[cut-1] == ' ' || raw[cut-1] == '\t') {
				cut--
			}
			return cut, rest
		}
	}
	return start, end
}

// --- PB942: cargo build without --release ----------------------------------

// fixCargoBuildRelease inserts --release into the build() cargo build, right
// after the subcommand word: cargo reads its own flags anywhere ahead of the
// `--` separator, and putting it there keeps it out of whatever the PKGBUILD
// hands the compiler on the far side.
//
// Two shapes are declined rather than guessed at. A command whose words do not
// all render — `cargo build --features "${myfeatures[@]}"` — is one the rule
// reports (cargoDevProfileBuilds has already stood down where the unreadable
// word could be a profile flag) but the fix will not rewrite: an argument list
// pkglint cannot read in full is not one to edit blind. And a PKGBUILD that
// reaches into a debug/ directory is reading the artifact this flag moves:
// cargo writes the dev profile to target/debug and the release one to
// target/release, so the flag alone would leave package() copying a path that
// no longer exists. Repointing it is a rewrite of package(), not a flag
// insertion, so the finding stands.
func fixCargoBuildRelease(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range cargoDevProfileBuilds(ctx) {
		if hasOpaqueArg(c) || namesDevArtifact(c.Unit) {
			continue
		}
		w := wordByValue(c, "build")
		if w == nil {
			continue // the subcommand itself came through an expansion
		}
		at := off(w.End())
		edits = append(edits, Edit{
			Path:  c.Unit.Path,
			Start: at,
			End:   at,
			New:   " --release",
			Line:  int(c.Stmt.Pos().Line()),
			Desc:  "insert --release into `cargo build` (ships the optimized profile)",
		})
	}
	return edits
}

// devArtifactRe matches a reference to a debug/ directory, which in a Rust
// build tree is cargo's dev-profile output: target/debug, the
// target/<triple>/debug of a --target build, or the same under whatever
// CARGO_TARGET_DIR the PKGBUILD picked. Matching the directory rather than any
// particular target path costs the odd unrelated debug/ (Arch's own
// /usr/lib/debug among them) a fix it could have had, which is the cheap side
// of the trade: the alternative is emitting one that stops the build.
var devArtifactRe = regexp.MustCompile(`/debug\b`)

// namesDevArtifact reports whether the unit names a debug/ path anywhere,
// source text included — a fix that has to be sure cannot limit itself to the
// arguments it managed to resolve.
func namesDevArtifact(u *pkgbuild.Unit) bool {
	return u != nil && devArtifactRe.Match(u.Raw)
}

// --- PB204: implicit go module downloads -----------------------------------

// fixGoDownloads gives the build its modules ahead of time: a prepare() that
// runs `go mod download`, entered through the same cd the build step uses so
// it lands in the same module. That is the finding message's first remedy, it
// works for sources that ship no vendor directory (a git tag rarely does),
// and goVendored recognizes the download step, so the finding clears for the
// right reason. The old edit — appending -mod=vendor to the build line —
// looked cheaper but pointed at a vendor directory that usually does not
// exist, and a later -mod flag on the same line overrode it silently anyway.
func fixGoDownloads(ctx *Context, _ *FixEnv) []Edit {
	if goVendored(ctx) {
		return nil
	}
	for _, c := range ctx.CommandsNamed("go") {
		if !c.InBuildPhase() || goCommandVendored(ctx, c) {
			continue
		}
		switch c.Subcommand() {
		case "build", "install", "test", "run":
		default:
			continue
		}
		// One structural edit fixes every flagged command in the file, so the
		// first one carries it.
		if edit, ok := goModDownloadEdit(ctx, c); ok {
			return []Edit{edit}
		}
		return nil
	}
	return nil
}

// goModDownloadEdit inserts `go mod download` into prepare() — extending the
// function if the PKGBUILD has one, writing a fresh one directly above c's
// function otherwise. The download gets -modcacherw (unless GOFLAGS already
// carries it) so the cache the fix creates stays removable, as PB916 asks.
func goModDownloadEdit(ctx *Context, c Command) (Edit, bool) {
	u := c.Unit
	indent := lineIndent(u.Raw, off(c.Stmt.Pos()))
	line := int(c.Stmt.Pos().Line())
	// GOFLAGS is only read from prepare()'s own environment, so an export
	// in build() — the guidelines' usual home for it — does not cover a
	// download that runs a phase earlier.
	download := func(at int) string {
		for _, v := range assignmentsInScope(u, "GOFLAGS", "prepare", at, nil) {
			if strings.Contains(v, "-modcacherw") {
				return "go mod download\n"
			}
		}
		return "go mod download -modcacherw\n"
	}

	if fd := u.Functions["prepare"]; fd != nil {
		block, ok := fd.Body.Cmd.(*syntax.Block)
		if !ok {
			return Edit{}, false
		}
		at := lineStart(u.Raw, off(block.Rbrace))
		if at <= off(block.Lbrace) {
			return Edit{}, false // a one-line prepare(); nowhere to put a new line
		}
		var b strings.Builder
		// prepare() may do its work at $srcdir's root; without a cd of its
		// own the download must still land in the module the build compiles.
		if !fnHasCommand(ctx, u, "prepare", "cd") {
			b.WriteString(cdLine(ctx, u, c.Fn))
		}
		b.WriteString(indent + download(at))
		return Edit{
			Path: u.Path, Start: at, End: at, New: b.String(), Line: line,
			Desc: "add `go mod download` to prepare() so the build needn't fetch",
		}, true
	}

	fd := u.Functions[c.Fn]
	if fd == nil {
		return Edit{}, false
	}
	at := lineStart(u.Raw, off(fd.Pos()))
	var b strings.Builder
	b.WriteString("prepare() {\n")
	b.WriteString(cdLine(ctx, u, c.Fn))
	b.WriteString(indent + download(at))
	b.WriteString("}\n\n")
	return Edit{
		Path: u.Path, Start: at, End: at, New: b.String(), Line: line,
		Desc: "add prepare() with `go mod download` so " + c.Fn + "() needn't fetch",
	}, true
}

// cdLine returns fn's opening cd statement as a full source line — indent and
// any `|| exit` guard included — or "" when fn has none or its cd shares the
// line with another command. prepare() must land in the directory the build
// step runs in, and copying the exact line is how it provably does.
func cdLine(ctx *Context, u *pkgbuild.Unit, fn string) string {
	for _, c := range ctx.Commands() {
		if c.Unit != u || c.Fn != fn || c.Name != "cd" {
			continue
		}
		start := lineStart(u.Raw, off(c.Stmt.Pos()))
		end := off(c.Stmt.End())
		for end < len(u.Raw) && u.Raw[end] != '\n' {
			end++
		}
		line := string(u.Raw[start:end])
		rest := line[off(c.Stmt.End())-start:]
		if rest != "" && !cdGuardRe.MatchString(rest) {
			return ""
		}
		return line + "\n"
	}
	return ""
}

// cdGuardRe matches the failure guard a cd is allowed to share its line with.
var cdGuardRe = regexp.MustCompile(`^\s*\|\|\s*(exit|return)\b[^|&;]*$`)

func fnHasCommand(ctx *Context, u *pkgbuild.Unit, fn, name string) bool {
	for _, c := range ctx.Commands() {
		if c.Unit == u && c.Fn == fn && c.Name == name {
			return true
		}
	}
	return false
}

// lineStart returns the offset of the first byte of the line containing o.
func lineStart(raw []byte, o int) int {
	for o > 0 && raw[o-1] != '\n' {
		o--
	}
	return o
}

// lineIndent returns the leading whitespace of the line containing o.
func lineIndent(raw []byte, o int) string {
	start := lineStart(raw, o)
	end := start
	for end < len(raw) && (raw[end] == ' ' || raw[end] == '\t') {
		end++
	}
	return string(raw[start:end])
}

// --- PB914/PB915/PB916: Arch Go guideline build flags ------------------------

// goFlagOrder is the guidelines' spelling of the build flags, and the order
// an inserted one is slotted into: after the last flag that precedes it here
// and is already present, else before the first that follows it, else first.
var goFlagOrder = []string{"-buildmode", "-trimpath", "-mod=", "-modcacherw"}

// goFlagSlot returns where flag goes among words, which are the flag words
// already present in their textual order: the index of the word to insert
// after, or -1 to insert before words[0] (or, with no words, wherever the
// caller's front is).
func goFlagSlot(words []string, flag string) int {
	rank := func(w string) int {
		for i, p := range goFlagOrder {
			if strings.HasPrefix(w, p) {
				return i
			}
		}
		return -1
	}
	mine := rank(flag)
	after := -1
	for i, w := range words {
		if r := rank(w); r >= 0 && r < mine {
			after = i
		}
	}
	return after
}

// goFlagEdits inserts flag into every command in cmds that neither passes it
// nor inherits it from a GOFLAGS assignment, where the PKGBUILD keeps such
// flags: in the GOFLAGS export the command inherits, in the array it spreads
// onto the command, or on the command itself — as a continuation line of its
// own when that is how the command lays its flags out. One edit covers every
// command that shares the assignment.
func goFlagEdits(ctx *Context, cmds []Command, flagPrefix, flag string) []Edit {
	var edits []Edit
	seen := map[int]bool{}
	for _, c := range cmds {
		if goFlagAddressed(goFlags(ctx, c), c, flagPrefix) {
			continue
		}
		edit, ok := goFlagEdit(c, flag)
		if !ok || seen[edit.Start] {
			continue
		}
		seen[edit.Start] = true
		edit.Line = int(c.Stmt.Pos().Line())
		edits = append(edits, edit)
	}
	return edits
}

// goFlagEdit picks the one place flag belongs for c. The GOFLAGS export in
// scope wins when there is one with literal text to extend — that is the
// guidelines' home for these flags and the edit a maintainer would make — an
// array the command (or that export) expands is next, and the command line
// is the fallback.
func goFlagEdit(c Command, flag string) (Edit, bool) {
	at := -1
	if c.Stmt != nil {
		at = off(c.Stmt.Pos())
	}
	// The command's own `GOFLAGS=… go build` prefix is not the shared home
	// the edit is after — and PB205's fix may be removing that very prefix
	// in the same pass — so it is passed over for the command line.
	own := map[*syntax.Assign]bool{}
	for _, as := range c.Call.Assigns {
		own[as] = true
	}
	var goflags *syntax.Assign
	scanAssignments(c.Unit, "GOFLAGS", c.Fn, at, c.Call, false, func(as *syntax.Assign) {
		if as.Value != nil && !own[as] {
			goflags = as
		}
	})
	if goflags != nil {
		if name := varRefName(goflags.Value); name != "" && name != "GOFLAGS" {
			if arr := arrayInScope(c, name, at); arr != nil {
				return goFlagIntoArray(c, arr, flag)
			}
		} else if edit, ok := goFlagIntoString(c, goflags.Value, flag); ok {
			return edit, true
		}
	}
	for i, w := range c.ArgWord {
		if !argOpaque(c, i) {
			continue
		}
		if name := varRefName(w); name != "" {
			if arr := arrayInScope(c, name, at); arr != nil {
				return goFlagIntoArray(c, arr, flag)
			}
		}
	}
	return goFlagIntoCommand(c, flag)
}

// arrayInScope returns the last array assignment to name that c can see, or
// nil when the name is not assigned as an array literal.
func arrayInScope(c Command, name string, at int) *syntax.Assign {
	var arr *syntax.Assign
	scanAssignments(c.Unit, name, c.Fn, at, c.Call, true, func(as *syntax.Assign) {
		if as.Array != nil {
			arr = as
		}
	})
	return arr
}

// goFlagIntoString inserts flag into the literal text of a GOFLAGS value,
// slotted by goFlagOrder among the flags the text spells out. A value with
// no literal text at all has nowhere to put it.
func goFlagIntoString(c Command, w *syntax.Word, flag string) (Edit, bool) {
	// The literal runs of the value, each with the offset its text starts at.
	type run struct {
		text string
		base int
	}
	var runs []run
	for _, part := range w.Parts {
		switch x := part.(type) {
		case *syntax.Lit:
			runs = append(runs, run{x.Value, off(x.Pos())})
		case *syntax.SglQuoted:
			runs = append(runs, run{x.Value, off(x.Pos()) + 1})
		case *syntax.DblQuoted:
			for _, p := range x.Parts {
				if lit, ok := p.(*syntax.Lit); ok {
					runs = append(runs, run{lit.Value, off(lit.Pos())})
				}
			}
		}
	}
	if len(runs) == 0 {
		return Edit{}, false
	}
	// The flag words the text spells out, with where each ends and starts.
	var words []string
	var starts, ends []int
	for _, r := range runs {
		for i := 0; i < len(r.text); {
			if r.text[i] == ' ' {
				i++
				continue
			}
			j := i
			for j < len(r.text) && r.text[j] != ' ' {
				j++
			}
			if r.text[i] == '-' {
				words = append(words, r.text[i:j])
				starts = append(starts, r.base+i)
				ends = append(ends, r.base+j)
			}
			i = j
		}
	}
	var start int
	var text string
	switch slot := goFlagSlot(words, flag); {
	case slot >= 0:
		start, text = ends[slot], " "+flag
	case len(words) > 0:
		start, text = starts[0], flag+" "
	default:
		start, text = runs[0].base, flag
		if strings.TrimSpace(runs[0].text) != "" {
			text += " "
		}
	}
	return Edit{
		Path:  c.Unit.Path,
		Start: start,
		End:   start,
		New:   text,
		Desc:  fmt.Sprintf("insert %s into GOFLAGS", flag),
	}, true
}

// goFlagIntoArray adds flag as an element of a flag array, slotted by
// goFlagOrder and laid out the way the array's elements are: one per line
// when they are, inline otherwise.
func goFlagIntoArray(c Command, as *syntax.Assign, flag string) (Edit, bool) {
	elems := as.Array.Elems
	var words []string
	for _, el := range elems {
		s, _ := pkgbuild.RenderWord(el.Value, nil)
		words = append(words, s)
	}
	raw := c.Unit.Raw
	perLine := len(elems) > 0 && elems[0].Pos().Line() != as.Array.Lparen.Line()
	var start int
	var text string
	switch slot := goFlagSlot(words, flag); {
	case slot >= 0:
		start = off(elems[slot].End())
		if perLine {
			text = "\n" + lineIndent(raw, off(elems[slot].Pos())) + flag
		} else {
			text = " " + flag
		}
	case len(elems) > 0:
		start = off(elems[0].Pos())
		if perLine {
			text = flag + "\n" + lineIndent(raw, start)
		} else {
			text = flag + " "
		}
	default:
		start, text = off(as.Array.Lparen)+1, flag
	}
	return Edit{
		Path:  c.Unit.Path,
		Start: start,
		End:   start,
		New:   text,
		Desc:  fmt.Sprintf("insert %s into %s", flag, as.Name.Value),
	}, true
}

// goFlagIntoCommand puts flag on the command line itself, slotted by
// goFlagOrder among the flags already there. A flag that sits on a
// continuation line of its own gets the new one as another such line;
// otherwise the flag goes right after its neighbour, or after the verb.
func goFlagIntoCommand(c Command, flag string) (Edit, bool) {
	verb := c.Subcommand()
	if verb == "mod" {
		if verb = secondSubcommand(c); verb == "" {
			return Edit{}, false
		}
	}
	vw := wordByValue(c, verb)
	if vw == nil {
		return Edit{}, false
	}
	// The flag words after the verb, up to the first non-flag argument,
	// which is where go stops reading flags. A flag's value (`-o build`) ends
	// the run early, which only costs a slot further back in the list.
	var flagWords []*syntax.Word
	var words []string
	past := false
	for _, w := range c.Call.Args {
		if !past {
			past = w == vw
			continue
		}
		s, _ := pkgbuild.RenderWord(w, nil)
		if !strings.HasPrefix(s, "-") {
			break
		}
		flagWords = append(flagWords, w)
		words = append(words, s)
	}
	raw := c.Unit.Raw
	ownLine := func(w *syntax.Word) bool {
		return w.Pos().Line() != vw.Pos().Line() && strings.TrimSpace(string(raw[lineStart(raw, off(w.Pos())):off(w.Pos())])) == ""
	}
	var start int
	var text string
	switch slot := goFlagSlot(words, flag); {
	case slot >= 0:
		w := flagWords[slot]
		start = off(w.End())
		if ownLine(w) {
			text = " \\\n" + lineIndent(raw, off(w.Pos())) + flag
		} else {
			text = " " + flag
		}
	case len(flagWords) > 0 && ownLine(flagWords[0]):
		start = off(flagWords[0].Pos())
		text = flag + " \\\n" + lineIndent(raw, start)
	default:
		start, text = off(vw.End()), " "+flag
	}
	return Edit{
		Path:  c.Unit.Path,
		Start: start,
		End:   start,
		New:   text,
		Desc:  fmt.Sprintf("insert %s into `go %s`", flag, c.Subcommand()),
	}, true
}

func fixGoPIE(ctx *Context, _ *FixEnv) []Edit {
	return goFlagEdits(ctx, goBuildCommands(ctx), "-buildmode", "-buildmode=pie")
}

func fixGoTrimpath(ctx *Context, _ *FixEnv) []Edit {
	return goFlagEdits(ctx, goBuildCommands(ctx), "-trimpath", "-trimpath")
}

func fixGoModcacheRW(ctx *Context, _ *FixEnv) []Edit {
	return goFlagEdits(ctx, goModuleCommands(ctx), "-modcacherw", "-modcacherw")
}

// --- PB205: re-enable Go module verification -------------------------------

func fixGoEnvWeakening(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	// Inline command prefixes: `GOSUMDB=off go build ...`.
	for _, c := range ctx.Commands() {
		for _, as := range c.Call.Assigns {
			if isGoWeakeningAssign(as) {
				edits = append(edits, removeAssignEdit(c.Unit, as))
			}
		}
	}
	// Standalone top-level assignment statements: `GOSUMDB=off`, `export GOSUMDB=off`.
	u := &ctx.Pkg.PKGBUILD
	for _, stmt := range u.TopLevel {
		switch cmd := stmt.Cmd.(type) {
		case *syntax.CallExpr:
			if len(cmd.Args) == 0 {
				edits = append(edits, assignStmtEdits(u, stmt, cmd.Assigns)...)
			}
		case *syntax.DeclClause:
			edits = append(edits, assignStmtEdits(u, stmt, cmd.Args)...)
		}
	}
	return edits
}

func isGoWeakeningAssign(as *syntax.Assign) bool {
	if as == nil || as.Name == nil {
		return false
	}
	_, bad := goEnvWeakens(as.Name.Value, assignValue(as))
	return bad
}

func assignValue(as *syntax.Assign) string {
	if as == nil || as.Value == nil {
		return ""
	}
	s, _ := renderPlain(as.Value)
	return s
}

// assignStmtEdits removes the Go-weakening assignments in a pure assignment
// statement: the whole statement if all its assignments are weakening, else
// each weakening assignment individually.
func assignStmtEdits(u *pkgbuild.Unit, stmt *syntax.Stmt, assigns []*syntax.Assign) []Edit {
	var bad []*syntax.Assign
	for _, as := range assigns {
		if isGoWeakeningAssign(as) {
			bad = append(bad, as)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	if len(bad) == len(assigns) {
		return []Edit{removeStmtLine(u, stmt, bad[0].Name.Value)}
	}
	edits := make([]Edit, 0, len(bad))
	for _, as := range bad {
		edits = append(edits, removeAssignEdit(u, as))
	}
	return edits
}

func removeAssignEdit(u *pkgbuild.Unit, as *syntax.Assign) Edit {
	start, end := off(as.Pos()), off(as.End())
	for end < len(u.Raw) && (u.Raw[end] == ' ' || u.Raw[end] == '\t') {
		end++
	}
	return Edit{
		Path:  u.Path,
		Start: start,
		End:   end,
		New:   "",
		Line:  int(as.Pos().Line()),
		Desc:  fmt.Sprintf("remove %s (re-enables Go module verification)", as.Name.Value),
	}
}

func removeStmtLine(u *pkgbuild.Unit, stmt *syntax.Stmt, name string) Edit {
	start, end := lineCut(u.Raw, off(stmt.Pos()), off(stmt.End()))
	return Edit{
		Path:  u.Path,
		Start: start,
		End:   end,
		New:   "",
		Line:  int(stmt.Pos().Line()),
		Desc:  fmt.Sprintf("remove %s assignment (re-enables Go module verification)", name),
	}
}

// --- PB206: npm/yarn lockfile-faithful install -----------------------------

func fixNpmCI(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range ctx.CommandsNamed("npm") {
		sub := c.Subcommand()
		if (sub != "install" && sub != "i") || namesPackages(c) {
			continue // `npm install <pkg>` is not equivalent to `npm ci`
		}
		w := wordByValue(c, sub)
		if w == nil {
			continue
		}
		edits = append(edits, Edit{
			Path:  c.Unit.Path,
			Start: off(w.Pos()),
			End:   off(w.End()),
			New:   "ci",
			Line:  int(c.Stmt.Pos().Line()),
			Desc:  fmt.Sprintf("replace `npm %s` with `npm ci` (installs exactly the committed lockfile)", sub),
		})
	}
	return append(edits, fixLockfileFlags(ctx, "PB206")...)
}

// --- PB403: drop setuid/setgid mode bits -----------------------------------

func fixSetuid(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, c := range ctx.CommandsNamed("chmod") {
		for _, a := range c.Args {
			if !setuidNumericMode(a) {
				continue
			}
			w := wordByValue(c, a)
			if w == nil {
				break
			}
			cleaned := clearSetuidBits(a)
			if cleaned == a {
				break
			}
			edits = append(edits, Edit{
				Path:  c.Unit.Path,
				Start: off(w.Pos()),
				End:   off(w.End()),
				New:   cleaned,
				Line:  int(c.Stmt.Pos().Line()),
				Desc:  fmt.Sprintf("drop setuid/setgid bit: chmod %s → %s", a, cleaned),
			})
			break
		}
	}
	for _, c := range ctx.CommandsNamed("install") {
		mode, w, replacement := installModeArg(c)
		if !setuidNumericMode(mode) || w == nil || replacement == mode {
			continue
		}
		orig, _ := pkgbuild.RenderWord(w, nil)
		edits = append(edits, Edit{
			Path:  c.Unit.Path,
			Start: off(w.Pos()),
			End:   off(w.End()),
			New:   replacement,
			Line:  int(c.Stmt.Pos().Line()),
			Desc:  fmt.Sprintf("drop setuid/setgid bit: install %s → %s", orig, replacement),
		})
	}
	return edits
}

func clearSetuidBits(mode string) string {
	v, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return mode
	}
	v &^= 0o6000 // clear setuid and setgid, keep sticky and permission bits
	return fmt.Sprintf("%04o", v)
}

// --- PB902: prefix a custom variable with an underscore ---------------------

// varNameRe matches the variable names this fix will rewrite: a plain
// lowercase shell identifier. Anything else is left alone rather than
// guessed at.
var varNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// fixUnprefixedCustomVars renames each flagged custom variable to its
// underscored spelling, declaration and references together.
//
// Unlike every other fix here, this one is not a single-site rewrite: a
// declaration renamed without its references is a PKGBUILD that reads a
// variable nothing sets, which is worse than the finding. So each rename is
// emitted as one Group — all of its edits land or none of them do — and the
// fixer proves it can see every occurrence before emitting anything.
//
// That proof is the whole design. Every name-bearing construct is claimed
// explicitly (assignments, parameter expansions, `for` iterators, arithmetic
// operands, `unset`), and then any *other* mention of the name in the file —
// a bare word, a single-quoted string, a `$name` spelled in literal text the
// shell won't expand here (a `trap`/`sh -c` string, an escaped `\$name`, a
// quoted heredoc) — vetoes the rename, because a mention this code cannot
// classify is one it cannot promise to rewrite. Constructs that can reach a
// variable this walk cannot follow — a name computed at run time (`eval`,
// `${!x}`, `declare -n`), or another file pulled into the same shell with
// `source`/`.` — veto every rename in the file for the same reason.
//
// It stays an unsafe fix even so. A variable exported to the build's
// environment is refused outright — a Makefile reading $foo would silently
// stop seeing it — but a rename still rewrites a name the maintainer chose,
// and it is the sort of change that wants an eye on it.
func fixUnprefixedCustomVars(ctx *Context, _ *FixEnv) []Edit {
	u := &ctx.Pkg.PKGBUILD
	if dynamicNaming(u) {
		return nil
	}
	vars := unprefixedCustomVars(ctx)
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names) // one deterministic edit order per package
	var edits []Edit
	for _, name := range names {
		edits = append(edits, renameVarEdits(ctx, u, name)...)
	}
	return edits
}

// renameVarEdits is one variable's rename, or nil when it cannot be made
// completely.
func renameVarEdits(ctx *Context, u *pkgbuild.Unit, name string) []Edit {
	target := "_" + name
	if !varNameRe.MatchString(name) {
		return nil
	}
	// The underscored spelling has to be free: renaming onto a name already in
	// use would merge two variables into one.
	if ctx.Pkg.Vars[target] != nil || mentions(u, target) {
		return nil
	}
	sites, ok := renameSites(u, name)
	if !ok || len(sites) == 0 {
		return nil
	}
	group := "PB902:" + name
	edits := make([]Edit, 0, len(sites))
	for i, pos := range sites {
		line := int(pos.Line())
		// A directive anywhere in the rename waives all of it (CollectEdits
		// drops the group), so the check here is only about not proposing an
		// edit a maintainer already declined.
		if ctx.Pkg.Suppressed("PB902", u.Path, line) {
			return nil
		}
		desc := fmt.Sprintf("rename reference $%s → $%s", name, target)
		if i == 0 {
			desc = fmt.Sprintf("rename %s → %s (%d occurrence(s))", name, target, len(sites))
		}
		edits = append(edits, Edit{
			Path:  u.Path,
			Start: off(pos),
			End:   off(pos) + len(name),
			New:   target,
			Line:  line,
			Desc:  desc,
			Group: group,
		})
	}
	return edits
}

// renameSites returns the position of every token naming name, in source
// order, and reports whether the set is provably complete.
//
// It is not complete — and the caller must then leave the finding alone —
// when the name is exported (renaming it changes what child processes see),
// when `unset -f` treats it as a function, or when it turns up somewhere this
// walk cannot account for.
func renameSites(u *pkgbuild.Unit, name string) ([]syntax.Pos, bool) {
	claimed := map[uint]bool{}
	var sites []syntax.Pos
	ok := true
	// A reference spelled inside literal text — `trap 'echo $name' EXIT`, an
	// escaped `\$name`, a quoted heredoc handed to a shell — is one the shell
	// expands later, after a rename would have moved the variable out from
	// under it.
	refRe := literalRefRe(name)
	claim := func(lit *syntax.Lit) {
		if lit == nil || lit.Value != name || claimed[lit.Pos().Offset()] {
			return
		}
		claimed[lit.Pos().Offset()] = true
		sites = append(sites, lit.Pos())
	}
	// Arithmetic reads and writes a variable by bare name — `((count++))`,
	// `$((count + 1))` — so operands are claimed wholesale within any
	// arithmetic context. Nested walks reach them before the outer walk sees
	// the same literals, which is what keeps them from being mistaken for
	// unaccountable mentions.
	claimArithm := func(n syntax.Node) {
		syntax.Walk(n, func(node syntax.Node) bool {
			if w, isWord := node.(*syntax.Word); isWord {
				if lit, single := singleLit(w); single {
					claim(lit)
				}
			}
			return true
		})
	}
	syntax.Walk(u.File, func(node syntax.Node) bool {
		if !ok {
			return false
		}
		switch x := node.(type) {
		case *syntax.Assign:
			if x.Name == nil || x.Name.Value != name {
				return true
			}
			claim(x.Name)
		case *syntax.CallExpr:
			// `FOO=1 make` puts FOO in that command's environment, where a
			// rename would change what the command reads.
			if len(x.Args) > 0 {
				for _, as := range x.Assigns {
					if as.Name != nil && as.Name.Value == name {
						ok = false
						return false
					}
				}
			}
			ok = claimUnset(x, name, claim)
			return ok
		case *syntax.DeclClause:
			if declExports(x) && declNames(x, name) {
				ok = false
				return false
			}
		case *syntax.ParamExp:
			claim(x.Param)
		case *syntax.WordIter:
			claim(x.Name)
		case *syntax.ArithmExp:
			claimArithm(x.X)
		case *syntax.ArithmCmd:
			claimArithm(x.X)
		case *syntax.LetClause:
			for _, e := range x.Exprs {
				claimArithm(e)
			}
		case *syntax.CStyleLoop:
			for _, e := range []syntax.ArithmExpr{x.Init, x.Cond, x.Post} {
				if e != nil {
					claimArithm(e)
				}
			}
		case *syntax.Lit:
			// Every literal spelling of the name that no case above claimed:
			// a `[[ -v name ]]` test, a `read name`, a command argument that
			// merely happens to match. Which of those it is cannot be told
			// apart reliably, so the rename stands down. So does a `$name`
			// hiding in literal text (escapes, quoted heredoc bodies land
			// here too).
			if (x.Value == name && !claimed[x.Pos().Offset()]) || refRe.MatchString(x.Value) {
				ok = false
				return false
			}
		case *syntax.SglQuoted:
			if x.Value == name || refRe.MatchString(x.Value) {
				ok = false
				return false
			}
		}
		return true
	})
	if !ok {
		return nil, false
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].Offset() < sites[j].Offset() })
	return sites, true
}

// claimUnset claims the operands of `unset name` / `unset -v name`, and
// reports whether the rename may go ahead: `unset -f name` names a function,
// not the variable, and renaming its operand would unset nothing. A
// dynamically assembled argument could be either — `unset $w` unsets whatever
// $w names — so any of those stands the rename down too.
func claimUnset(c *syntax.CallExpr, name string, claim func(*syntax.Lit)) bool {
	if len(c.Args) == 0 {
		return true
	}
	if v, dyn := renderPlain(c.Args[0]); dyn || v != "unset" {
		return true
	}
	functions := false
	for _, w := range c.Args[1:] {
		v, dyn := renderPlain(w)
		if dyn {
			return false
		}
		if strings.HasPrefix(v, "-") && strings.ContainsRune(v, 'f') {
			functions = true
		}
	}
	for _, w := range c.Args[1:] {
		lit, single := singleLit(w)
		if !single || lit.Value != name {
			continue
		}
		if functions {
			return false
		}
		claim(lit)
	}
	return true
}

// singleLit returns w's sole literal part, if that is all it has.
func singleLit(w *syntax.Word) (*syntax.Lit, bool) {
	if w == nil || len(w.Parts) != 1 {
		return nil, false
	}
	lit, ok := w.Parts[0].(*syntax.Lit)
	return lit, ok
}

// declExports reports whether a declaration puts its variables in the
// environment: `export`, or `declare`/`typeset` carrying -x.
func declExports(d *syntax.DeclClause) bool {
	if d.Variant != nil && d.Variant.Value == "export" {
		return true
	}
	return declFlag(d, 'x')
}

// declNames reports whether the declaration mentions name.
func declNames(d *syntax.DeclClause, name string) bool {
	for _, as := range d.Args {
		if as.Name != nil && as.Name.Value == name {
			return true
		}
		if as.Name == nil && as.Value != nil {
			if v, _ := renderPlain(as.Value); v == name {
				return true
			}
		}
	}
	return false
}

// declFlag reports whether the declaration carries a short flag, bundled
// spellings (-gx) included.
func declFlag(d *syntax.DeclClause, flag byte) bool {
	for _, as := range d.Args {
		if as.Name != nil || as.Value == nil {
			continue
		}
		v, _ := renderPlain(as.Value)
		if !strings.HasPrefix(v, "-") || strings.HasPrefix(v, "--") {
			continue
		}
		if strings.IndexByte(v[1:], flag) >= 0 {
			return true
		}
	}
	return false
}

// dynamicNaming reports whether the file can reach a variable this walk
// cannot follow: a name computed at run time — `eval`, indirect expansion
// (`${!x}`), namerefs (`declare -n`) — or another file pulled into the same
// shell with `source`/`.`, whose code sees every variable without a trace
// here. Any of them makes every rename in the file unprovable, so none is
// offered.
func dynamicNaming(u *pkgbuild.Unit) bool {
	found := false
	syntax.Walk(u.File, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.ParamExp:
			if x.Excl && !arrayKeysParam(x) {
				found = true
			}
		case *syntax.DeclClause:
			if declFlag(x, 'n') {
				found = true
			}
		case *syntax.CallExpr:
			if len(x.Args) > 0 {
				if v, _ := renderPlain(x.Args[0]); v == "eval" || v == "source" || v == "." {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// arrayKeysParam reports whether p is `${!arr[@]}` / `${!arr[*]}`, the
// array-indices form. Alone among the `${!...}` spellings it reads the named
// array itself rather than a variable named at run time, so it renames like
// any other reference.
func arrayKeysParam(p *syntax.ParamExp) bool {
	if p.Names != 0 {
		return false
	}
	w, ok := p.Index.(*syntax.Word)
	if !ok {
		return false
	}
	lit, single := singleLit(w)
	return single && (lit.Value == "@" || lit.Value == "*")
}

// literalRefRe matches a `$name` or `${name` reference spelled inside literal
// text, with a boundary so `$namelier` does not count.
func literalRefRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`\$\{?` + regexp.QuoteMeta(name) + `([^a-zA-Z0-9_]|$)`)
}

// mentions reports whether name appears anywhere in the file, in any of the
// spellings a rename would have to care about — a bare or quoted word, or a
// `$name` reference spelled in literal text that would start resolving to the
// renamed variable.
func mentions(u *pkgbuild.Unit, name string) bool {
	refRe := literalRefRe(name)
	found := false
	syntax.Walk(u.File, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.Lit:
			if x.Value == name || refRe.MatchString(x.Value) {
				found = true
			}
		case *syntax.SglQuoted:
			if x.Value == name || refRe.MatchString(x.Value) {
				found = true
			}
		}
		return !found
	})
	return found
}
