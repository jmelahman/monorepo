package rules

import (
	"bytes"
	"sort"

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
	createQuote            quoteStyle
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

// maxFixRounds bounds how many times Fix re-lints its own output. Real
// packages settle in two rounds; the cap is for a pair of fixers that would
// undo each other, which must stop rather than spin.
const maxFixRounds = 8

// Fix computes and applies auto-fixes for a package at the given level.
//
// Every fixer in a round works from the same parse, so an edit that overlaps
// another one is dropped (ApplyEdits) and a fixer whose rewrite only becomes
// possible after another's cannot see it. Fix therefore re-parses what a round
// produced and fixes again, until a round changes nothing: a single --fix
// leaves the file where a second one would, which is what keeps the fix hook
// from rewriting the same file on two commits in a row.
//
// The result holds one FixResult per unit per round that changed it, in round
// order. Each one's Applied edits are offsets into its own Original, so they
// render against it; the last result for a path is that unit's final content.
func Fix(pkg *pkgbuild.Package, ignore map[string]bool, level FixLevel, env *FixEnv) []FixResult {
	env = memoizeEnv(env)
	var all []FixResult
	for round := 0; round < maxFixRounds; round++ {
		results := applyByUnit(pkg, CollectEdits(NewContext(pkg), ignore, level, env))
		contents := map[string][]byte{}
		for _, r := range results {
			if r.Changed() {
				all = append(all, r)
				contents[r.Path] = r.Fixed
			}
		}
		if len(contents) == 0 {
			break
		}
		next, err := pkg.Reload(contents)
		if err != nil {
			// What the round wrote no longer loads. It stands, since the
			// round's own edits were each checked; nothing is built on it.
			break
		}
		pkg = next
	}
	return all
}

// memoizeEnv wraps env's capabilities so that each question is asked once per
// Fix: a later round re-runs the fixers whose edit an earlier round declined,
// and a probe that timed out, or a ref that did not resolve, would otherwise
// be retried once per round.
func memoizeEnv(env *FixEnv) *FixEnv {
	if env == nil {
		return nil
	}
	out := *env
	if f := env.ResolveRef; f != nil {
		type answer struct {
			hash string
			err  error
		}
		seen := map[[2]string]answer{}
		out.ResolveRef = func(url, ref string) (string, error) {
			k := [2]string{url, ref}
			a, ok := seen[k]
			if !ok {
				a.hash, a.err = f(url, ref)
				seen[k] = a
			}
			return a.hash, a.err
		}
	}
	if f := env.ProbeHTTPS; f != nil {
		seen := map[string]error{}
		out.ProbeHTTPS = func(url string) error {
			err, ok := seen[url]
			if !ok {
				err = f(url)
				seen[url] = err
			}
			return err
		}
	}
	return &out
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
