package rules

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB913 audits the audit trail itself: an inline "# pkglint: ignore=" that no
// longer suppresses anything is a claim about the code that stopped being
// true. The check and its fixer live together here because they share the
// directive scanner and the usage computation below.

const staleIgnoreID = "PB913"

// allRules is bound to the registry accessor in init rather than called
// directly: the PB913 check runs every registered rule, and referencing the
// registry from styleRules' own initializer would be an initialization cycle.
// The indirection through a variable is what breaks it — the dependency
// analysis follows function references out of an initializer, but stops at a
// variable whose own initializer is empty.
var allRules func() []Rule

func init() { allRules = registry }

// knownRuleIDs is the set of registered rule IDs. PB913 asks for it once per
// package, so it is built once and shared read-only; callers must not write to
// what they get back.
var (
	knownIDsOnce sync.Once
	knownIDs     map[string]bool
)

func knownRuleIDs() map[string]bool {
	knownIDsOnce.Do(func() {
		knownIDs = make(map[string]bool, len(allRules()))
		for _, r := range allRules() {
			knownIDs[r.ID] = true
		}
	})
	return knownIDs
}

// suppKey names one rule ID of one inline-ignore directive.
type suppKey struct {
	path string
	line int
	id   string
}

// suppressionUsage reports which inline-ignore directives earn their keep: the
// (path, line, id) of every directive entry that suppresses at least one
// finding. It re-runs the PKGBUILD-scope rules over the package — unsuppressed
// findings and suppressed ones alike land here, which is why Run's own results
// cannot answer the question — and is cached on the Context because the
// stale-directive check and its fixer both need it.
//
// Only the rules some directive actually names are re-run. A finding
// contributes a key only when the package suppresses that rule ID on that
// line, so a rule no directive mentions cannot produce one however many
// findings it reports, and running it is pure cost. That makes this a no-op
// for the overwhelming majority of packages, which carry no directives at all.
func (ctx *Context) suppressionUsage() map[suppKey]bool {
	if ctx.suppUsed != nil {
		return ctx.suppUsed
	}
	ctx.suppUsed = map[suppKey]bool{}
	named := suppressedIDs(ctx.Pkg)
	for _, rule := range allRules() {
		if rule.Scope != ScopePKGBUILD || rule.ID == staleIgnoreID || !named[rule.ID] {
			continue
		}
		for _, f := range rule.Check(ctx) {
			perLine := ctx.Pkg.Suppressions[f.Path]
			for _, l := range []int{f.Line, f.Line - 1} {
				if perLine[l][f.RuleID] {
					ctx.suppUsed[suppKey{f.Path, l, f.RuleID}] = true
				}
			}
		}
	}
	return ctx.suppUsed
}

// suppressedIDs is every rule ID named by an inline-ignore directive anywhere
// in the package.
func suppressedIDs(pkg *pkgbuild.Package) map[string]bool {
	out := map[string]bool{}
	for _, perLine := range pkg.Suppressions {
		for _, ids := range perLine {
			for id := range ids {
				out[id] = true
			}
		}
	}
	return out
}

// ignoreDirective is one inline-ignore directive backed by a real comment in a
// parsed unit. All offsets are byte offsets into that unit's Raw.
type ignoreDirective struct {
	line int
	col  int      // 1-based column of the directive's '#'
	ids  []string // rule IDs in written order

	commentStart, commentEnd int // the whole comment, '#' through end of line
	idsStart, idsEnd         int // the rule-ID list, trailing separators trimmed

	// sole is true when the comment carries nothing but the directive, so
	// removing the comment whole loses no prose; ownLine when only whitespace
	// precedes the comment on its line.
	sole    bool
	ownLine bool
}

// ignoreDirectives scans a unit's comments for inline-ignore directives.
// Working from the AST rather than raw lines means directive-shaped text that
// is not actually a comment (inside a heredoc or a string) is never touched:
// the suppression parser honors such text line-by-line, but rewriting it could
// change what the build does, so it is left for a human.
func ignoreDirectives(u *pkgbuild.Unit) []ignoreDirective {
	if u.File == nil {
		return nil
	}
	var out []ignoreDirective
	syntax.Walk(u.File, func(node syntax.Node) bool {
		c, ok := node.(*syntax.Comment)
		if !ok {
			return true
		}
		cs, ce := off(c.Pos()), off(c.End())
		if cs < 0 || ce > len(u.Raw) || cs >= ce {
			return true
		}
		raw := string(u.Raw[cs:ce])
		d, found := pkgbuild.ParseDirective(raw)
		if !found {
			return true
		}
		ls := lineStart(u.Raw, cs)
		out = append(out, ignoreDirective{
			line:         int(c.Pos().Line()),
			col:          int(c.Pos().Col()) + d.Start,
			ids:          d.IDs,
			commentStart: cs,
			commentEnd:   ce,
			idsStart:     cs + d.IDsStart,
			idsEnd:       cs + d.IDsEnd,
			sole:         d.Start == 0 && strings.Trim(raw[d.IDsEnd:], " \t,") == "",
			ownLine:      strings.Trim(string(u.Raw[ls:cs]), " \t") == "",
		})
		return true
	})
	return out
}

// staleDirectiveIDs splits a directive's rule IDs into the ones still earning
// their suppression and the stale ones: IDs that are not pkglint rules at all,
// or that match no finding on the directive's own or next line.
func (ctx *Context) staleDirectiveIDs(path string, d ignoreDirective) (kept, stale []string) {
	known := knownRuleIDs()
	for _, id := range d.ids {
		if known[id] && ctx.suppressionUsage()[suppKey{path, d.line, id}] {
			kept = append(kept, id)
		} else {
			stale = append(stale, id)
		}
	}
	return kept, stale
}

// honoredDirectives yields the unit's directives the suppression parser
// actually recorded, i.e. the ones that can suppress findings. It also spares
// the full-registry usage run for the common package with no directives.
func honoredDirectives(pkg *pkgbuild.Package, u *pkgbuild.Unit) []ignoreDirective {
	perLine := pkg.Suppressions[u.Path]
	if len(perLine) == 0 {
		return nil
	}
	var out []ignoreDirective
	for _, d := range ignoreDirectives(u) {
		if perLine[d.line] != nil {
			out = append(out, d)
		}
	}
	return out
}

func checkStaleIgnores(ctx *Context) []Finding {
	known := knownRuleIDs()
	var out []Finding
	for _, u := range ctx.Pkg.Units() {
		for _, d := range honoredDirectives(ctx.Pkg, &u) {
			for _, id := range d.ids {
				if !known[id] {
					out = append(out, Finding{RuleID: staleIgnoreID, Severity: Warn,
						Path: u.Path, Line: d.line, Col: d.col,
						Message: fmt.Sprintf("ignore directive names %s, which is not a pkglint rule; it suppresses nothing", id)})
					continue
				}
				if !ctx.suppressionUsage()[suppKey{u.Path, d.line, id}] {
					out = append(out, Finding{RuleID: staleIgnoreID, Severity: Warn,
						Path: u.Path, Line: d.line, Col: d.col,
						Message: fmt.Sprintf("ignore directive for %s matches no finding on this or the next line; remove it", id)})
				}
			}
		}
	}
	return out
}

// fixStaleIgnores deletes what checkStaleIgnores flags: the whole comment when
// every ID is stale and the comment says nothing else, or just the stale IDs
// when live ones remain. A stale directive tangled into a longer comment is
// reported but not rewritten — what the surrounding prose should become is the
// author's call.
func fixStaleIgnores(ctx *Context, _ *FixEnv) []Edit {
	var edits []Edit
	for _, u := range ctx.Pkg.Units() {
		for _, d := range honoredDirectives(ctx.Pkg, &u) {
			kept, stale := ctx.staleDirectiveIDs(u.Path, d)
			switch {
			case len(stale) == 0:
			case len(kept) > 0:
				sep := ","
				if strings.Contains(string(u.Raw[d.idsStart:d.idsEnd]), ", ") {
					sep = ", "
				}
				edits = append(edits, Edit{
					Path:  u.Path,
					Start: d.idsStart,
					End:   d.idsEnd,
					New:   strings.Join(kept, sep),
					Line:  d.line,
					Desc:  fmt.Sprintf("drop %s from ignore directive (no matching finding)", strings.Join(stale, ", ")),
				})
			case d.sole:
				start, end := d.commentStart, d.commentEnd
				if d.ownLine {
					start = lineStart(u.Raw, start)
					if end < len(u.Raw) && u.Raw[end] == '\n' {
						end++
					}
				} else {
					for start > 0 && (u.Raw[start-1] == ' ' || u.Raw[start-1] == '\t') {
						start--
					}
				}
				edits = append(edits, Edit{
					Path:  u.Path,
					Start: start,
					End:   end,
					New:   "",
					Line:  d.line,
					Desc:  fmt.Sprintf("remove stale ignore directive (%s)", strings.Join(stale, ", ")),
				})
			}
		}
	}
	return edits
}

// --- --add-ignores: annotate every current finding --------------------------

// AddIgnoreEdits computes the edits that suppress every finding Run still
// reports with an inline "# pkglint: ignore=" directive: extending a directive
// that already covers the finding's site when one exists, otherwise inserting
// a fresh comment line above the finding. Findings that cannot be annotated
// safely — no parsed unit to edit, a site inside a heredoc or a multi-line
// word, a backslash-continued line — are skipped and keep being reported.
// PB913 findings are skipped too: the answer to a stale directive is removing
// it, not a second directive ignoring the reminder.
func AddIgnoreEdits(pkg *pkgbuild.Package, ignore map[string]bool) []Edit {
	units := pkg.Units()
	unitByPath := map[string]*pkgbuild.Unit{}
	for i := range units {
		unitByPath[units[i].Path] = &units[i]
	}

	// One site per (path, line), rule IDs deduped in Run's sorted order.
	type site struct {
		path string
		line int
	}
	newIDs := map[site][]string{}
	var sites []site
	for _, f := range Run(pkg, ignore) {
		if f.RuleID == staleIgnoreID || unitByPath[f.Path] == nil || f.Line < 1 {
			continue
		}
		s := site{f.Path, f.Line}
		if len(newIDs[s]) == 0 {
			sites = append(sites, s)
		}
		if !slices.Contains(newIDs[s], f.RuleID) {
			newIDs[s] = append(newIDs[s], f.RuleID)
		}
	}
	if len(sites) == 0 {
		return nil
	}

	directiveAt := map[string]map[int]ignoreDirective{}
	blockedAt := map[string]map[int]bool{}
	for path, u := range unitByPath {
		perLine := map[int]ignoreDirective{}
		for _, d := range ignoreDirectives(u) {
			perLine[d.line] = d
		}
		directiveAt[path] = perLine
		blockedAt[path] = insertBlockedLines(u)
	}

	// A directive can cover two sites (its own line and the next), so merges
	// accumulate per directive before becoming one edit each.
	merged := map[site][]string{}
	var mergeOrder, insertOrder []site
	for _, s := range sites {
		var target site
		found := false
		for _, l := range []int{s.line - 1, s.line} {
			if _, ok := directiveAt[s.path][l]; ok {
				target, found = site{s.path, l}, true
				break
			}
		}
		if !found {
			if !insertableAt(unitByPath[s.path], blockedAt[s.path], s.line) {
				continue
			}
			insertOrder = append(insertOrder, s)
			continue
		}
		if len(merged[target]) == 0 {
			mergeOrder = append(mergeOrder, target)
		}
		merged[target] = append(merged[target], newIDs[s]...)
	}

	var edits []Edit
	for _, t := range mergeOrder {
		u, d := unitByPath[t.path], directiveAt[t.path][t.line]
		old := string(u.Raw[d.idsStart:d.idsEnd])
		sep := ","
		if strings.Contains(old, ", ") {
			sep = ", "
		}
		edits = append(edits, Edit{
			Path:   u.Path,
			Start:  d.idsStart,
			End:    d.idsEnd,
			New:    old + sep + strings.Join(merged[t], sep),
			Line:   d.line,
			RuleID: strings.Join(merged[t], ","),
			Desc:   fmt.Sprintf("extend ignore directive to cover %s", strings.Join(merged[t], ", ")),
		})
	}
	for _, s := range insertOrder {
		u := unitByPath[s.path]
		at, ok := lineOffset(u.Raw, s.line)
		if !ok {
			continue
		}
		ids := strings.Join(newIDs[s], ",")
		edits = append(edits, Edit{
			Path:   u.Path,
			Start:  at,
			End:    at,
			New:    lineIndent(u.Raw, at) + "# pkglint: ignore=" + ids + "\n",
			Line:   s.line,
			RuleID: ids,
			Desc:   fmt.Sprintf("add ignore directive for %s", strings.Join(newIDs[s], ", ")),
		})
	}
	return edits
}

// AddIgnores computes and applies the --add-ignores edits, returning one
// FixResult per unit that changed.
func AddIgnores(pkg *pkgbuild.Package, ignore map[string]bool) []FixResult {
	return applyByUnit(pkg, AddIgnoreEdits(pkg, ignore))
}

// insertBlockedLines marks the lines of u that a fresh comment line must not
// be inserted above, because the insertion point is inside something that is
// not plain script: a heredoc body (the comment would become output) or a
// word spilling over from the previous line (a multi-line quoted string).
func insertBlockedLines(u *pkgbuild.Unit) map[int]bool {
	blocked := map[int]bool{}
	if u.File == nil {
		return blocked
	}
	syntax.Walk(u.File, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.Redirect:
			if x.Hdoc != nil {
				// Body lines plus the delimiter line: inserting above any of
				// them lands inside the document.
				for l := int(x.Hdoc.Pos().Line()); l <= int(x.Hdoc.End().Line())+1; l++ {
					blocked[l] = true
				}
			}
		case *syntax.Word:
			for l := int(x.Pos().Line()) + 1; l <= int(x.End().Line()); l++ {
				blocked[l] = true
			}
		}
		return true
	})
	return blocked
}

// insertableAt reports whether a comment line can be inserted above line n:
// n must exist, not be blocked, and not continue the previous line (a
// trailing backslash would turn the comment into the command's tail).
func insertableAt(u *pkgbuild.Unit, blocked map[int]bool, n int) bool {
	at, ok := lineOffset(u.Raw, n)
	if !ok || blocked[n] {
		return false
	}
	if at >= 2 && u.Raw[at-1] == '\n' && u.Raw[at-2] == '\\' {
		return false
	}
	return true
}

// lineOffset returns the byte offset of the start of 1-based line n.
func lineOffset(raw []byte, n int) (int, bool) {
	if n < 1 {
		return 0, false
	}
	at := 0
	for l := 1; l < n; l++ {
		i := bytes.IndexByte(raw[at:], '\n')
		if i < 0 {
			return 0, false
		}
		at += i + 1
	}
	if at >= len(raw) {
		return 0, false
	}
	return at, true
}
