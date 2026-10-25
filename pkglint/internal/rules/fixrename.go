package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

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
