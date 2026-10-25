package rules

import (
	"bytes"
	"slices"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

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

// quoteStyle is how a PKGBUILD writes plain array elements. The zero value is
// single quotes, the guidelines' template and the fallback when the file does
// not say.
type quoteStyle int

const (
	quoteSingle quoteStyle = iota
	quoteDouble
	quoteBare
)

// quoteFor is the quote entries are written with under q, single quotes
// unless every entry survives the other spelling. Bare entries must not split
// or redirect; double-quoted ones must not carry a backtick or a backslash,
// which validArrayEntries lets through because single quotes make both
// literal and double quotes do not.
func (q quoteStyle) quoteFor(entries []string) string {
	switch {
	case q == quoteDouble && !slices.ContainsFunc(entries, func(e string) bool { return strings.ContainsAny(e, "`\\") }):
		return `"`
	case q == quoteBare && bareArrayEntries(entries):
		return ""
	}
	return "'"
}

// arrayQuoteStyle is the convention the file's own top-level arrays follow,
// by majority vote of their elements, ties going to single quotes. Only an
// element that could have been written any of the three ways votes: one
// carrying an expansion or a `>` had its quoting forced, and says nothing
// about taste. Nor do the checksum arrays: their digests are bare-safe but
// written however updpkgsums or the upstream template spelled them, and with
// one per source they would outvote the dependency arrays a fix writes into.
func arrayQuoteStyle(pkg *pkgbuild.Package) quoteStyle {
	var votes [3]int
	for _, v := range pkg.Vars {
		if v.Assign == nil || v.Assign.Array == nil {
			continue
		}
		if field, _, _ := strings.Cut(v.Name, "_"); slices.Contains(hashAlgoSums, field) {
			continue
		}
		for _, el := range v.Assign.Array.Elems {
			if el.Value == nil || len(el.Value.Parts) != 1 {
				continue
			}
			switch p := el.Value.Parts[0].(type) {
			case *syntax.Lit:
				if bareArrayEntries([]string{p.Value}) {
					votes[quoteBare]++
				}
			case *syntax.SglQuoted:
				if !p.Dollar && bareArrayEntries([]string{p.Value}) {
					votes[quoteSingle]++
				}
			case *syntax.DblQuoted:
				if len(p.Parts) == 1 {
					if lit, ok := p.Parts[0].(*syntax.Lit); ok && bareArrayEntries([]string{lit.Value}) {
						votes[quoteDouble]++
					}
				}
			}
		}
	}
	best, tied := quoteSingle, false
	for _, q := range []quoteStyle{quoteDouble, quoteBare} {
		switch {
		case votes[q] > votes[best]:
			best, tied = q, false
		case votes[q] == votes[best]:
			tied = true
		}
	}
	if tied {
		return quoteSingle
	}
	return best
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

// bareArrayEntries reports whether every entry is a plain package name that
// needs no quoting to survive as one array element.
func bareArrayEntries(entries []string) bool {
	for _, e := range entries {
		if e == "" {
			return false
		}
		for _, r := range e {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("@._+-", r)) {
				return false
			}
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
		// Nothing in the array to match, so match the file.
		return Edit{Path: u.Path, Start: rparen, End: rparen,
			New: arrayEntryText(entries, arrayQuoteStyle(ctx.Pkg).quoteFor(entries), " "), Line: int(v.Pos.Line()), Desc: desc}, true
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
	// So does the quoting of the element before it, which is closer to hand
	// than the file's convention.
	style, sep := quoteBare, " "
	switch u.Raw[from] {
	case '\'':
		style = quoteSingle
	case '"':
		style = quoteDouble
	}
	if bytes.ContainsRune(u.Raw[at:rparen], '\n') {
		sep = "\n" + lineIndent(u.Raw, from)
	}
	return Edit{Path: u.Path, Start: at, End: at,
		New: sep + arrayEntryText(entries, style.quoteFor(entries), sep), Line: int(last.Value.Pos().Line()), Desc: desc}, true
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
		creates: creates, createTail: "\n", createQuote: arrayQuoteStyle(ctx.Pkg)}
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
// arrays, wrapped in the text of the anchor byte the edit claimed. The quote
// is picked per array at render time, because a merge can add an entry that
// cannot go bare to an array that could.
func createdArraysText(e Edit) string {
	var b strings.Builder
	b.WriteString(e.createHead)
	for _, c := range e.creates {
		b.WriteString("\n" + c.Field + "=(" + arrayEntryText(c.Entries, e.createQuote.quoteFor(c.Entries), " ") + ")")
	}
	b.WriteString(e.createTail)
	return b.String()
}

// editSpan is the byte range of one file an edit replaces.
type editSpan struct {
	path       string
	start, end int
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
	slots := map[editSpan]*slot{}
	var order []*slot
	absorbed := map[int]bool{}
	for i, e := range edits {
		if len(e.creates) == 0 || e.Group != "" {
			continue
		}
		k := editSpan{e.Path, e.Start, e.End}
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
