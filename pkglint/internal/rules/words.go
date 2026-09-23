package rules

import (
	"regexp"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

func basename(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		return s[i+1:]
	}
	return s
}

func hasPrefixAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// hasVarRef reports whether a rendered word still contains an unresolved
// variable or dynamic marker.
func hasVarRef(s string) bool {
	return strings.ContainsAny(s, "$\x00")
}

// argOpaque reports whether the command's i'th argument still carries an
// expansion after rendering, which is pkglint saying it does not know what
// that word tells the command.
func argOpaque(c Command, i int) bool {
	return hasVarRef(c.Args[i]) || (i < len(c.ArgDyn) && c.ArgDyn[i])
}

// argWords returns the words the command's i'th argument contributes, split
// the way bash splits an unquoted expansion.
//
// An argument pkglint could not resolve is looked up before it is given up
// on: `cargo build $_cargo_flags` or `go build "${_goflags[@]}"` is rarely a
// command whose flags are unknowable, it is one whose flags are written a few
// lines up, and the assignments in scope are the best account of what the
// tool gets. The variables worth this are exactly the ones ordinary rendering
// cannot reach — a name assigned inside the function, an array, one a
// preceding phase changed — which is most of how real PKGBUILDs keep their
// build flags in one place. Words that are still expansions afterwards are
// kept as they rendered, so a caller can tell what is left unread.
func argWords(c Command, i int) []string {
	if argOpaque(c, i) && i < len(c.ArgWord) {
		if name := varRefName(c.ArgWord[i]); name != "" {
			at := -1
			if c.Stmt != nil {
				at = off(c.Stmt.Pos())
			}
			if words, ok := wordsInScope(c.Unit, name, c.Fn, at, c.Call); ok {
				return words
			}
		}
	}
	return strings.Fields(c.Args[i])
}

// varRefName returns the name of the variable a word is nothing but a
// reference to — `$flags`, `${flags}`, `"$flags"`, `"${flags[@]}"` — and ""
// for anything else. Only a whole reference stands for what the variable
// holds: `--root="$pkgdir/usr"` is text of its own around a value, and
// `${flags:-…}`, `${flags[0]}` or `${#flags[@]}` are operations on one that
// may hand the command something quite different from what was assigned.
func varRefName(w *syntax.Word) string {
	if w == nil || len(w.Parts) != 1 {
		return ""
	}
	part := w.Parts[0]
	if dq, ok := part.(*syntax.DblQuoted); ok {
		if len(dq.Parts) != 1 {
			return ""
		}
		part = dq.Parts[0]
	}
	pe, ok := part.(*syntax.ParamExp)
	if !ok || pe.Param == nil || pe.Excl || pe.Length || pe.Width ||
		pe.Slice != nil || pe.Repl != nil || pe.Exp != nil || pe.Names != 0 {
		return ""
	}
	if pe.Index != nil {
		iw, isWord := pe.Index.(*syntax.Word)
		if !isWord {
			return ""
		}
		idx, dyn := pkgbuild.RenderWord(iw, nil)
		if dyn || (idx != "@" && idx != "*") {
			return ""
		}
	}
	return pe.Param.Value
}

// hasOpaqueArg reports whether any of the command's arguments is opaque, so a
// fix that rewrites the argument list has nothing solid to rewrite against.
func hasOpaqueArg(c Command) bool {
	for i := range c.Args {
		if argOpaque(c, i) {
			return true
		}
	}
	return false
}

var assignWordRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

func isAssignWord(s string) bool { return assignWordRe.MatchString(s) }

// flattenPipe returns the statements of a pipeline in order, or nil when
// stmt is not a pipeline.
func flattenPipe(stmt *syntax.Stmt) []*syntax.Stmt {
	bc, ok := stmt.Cmd.(*syntax.BinaryCmd)
	if !ok || (bc.Op != syntax.Pipe && bc.Op != syntax.PipeAll) {
		return nil
	}
	var out []*syntax.Stmt
	var flatten func(s *syntax.Stmt)
	flatten = func(s *syntax.Stmt) {
		if b, ok := s.Cmd.(*syntax.BinaryCmd); ok && (b.Op == syntax.Pipe || b.Op == syntax.PipeAll) {
			flatten(b.X)
			flatten(b.Y)
			return
		}
		out = append(out, s)
	}
	flatten(stmt)
	return out
}

// pipelines yields every pipeline in a unit.
func pipelines(u *syntax.File) [][]*syntax.Stmt {
	var out [][]*syntax.Stmt
	seen := map[*syntax.Stmt]bool{}
	syntax.Walk(u, func(node syntax.Node) bool {
		stmt, ok := node.(*syntax.Stmt)
		if !ok || seen[stmt] {
			return true
		}
		if segs := flattenPipe(stmt); segs != nil {
			out = append(out, segs)
			for _, s := range segs {
				seen[s] = true // don't re-report inner segments of the same pipe
			}
		}
		return true
	})
	return out
}

// stmtCommandName resolves the command name of a pipeline segment,
// unwrapping simple wrappers like sudo or env.
func stmtCommandName(stmt *syntax.Stmt, vars map[string]string) string {
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return ""
	}
	c := (&Context{vars: vars}).newCommand(nil, "", stmt, call)
	return c.Name
}

// wordContainsCommand reports whether the word embeds a command substitution
// (or process substitution) that invokes one of names.
func wordContainsCommand(w *syntax.Word, names map[string]bool) bool {
	found := false
	syntax.Walk(w, func(node syntax.Node) bool {
		var stmts []*syntax.Stmt
		switch x := node.(type) {
		case *syntax.CmdSubst:
			stmts = x.Stmts
		case *syntax.ProcSubst:
			stmts = x.Stmts
		default:
			return true
		}
		for _, s := range stmts {
			syntax.Walk(s, func(n syntax.Node) bool {
				if call, ok := n.(*syntax.CallExpr); ok && len(call.Args) > 0 {
					name, _ := renderPlain(call.Args[0])
					if names[basename(name)] {
						found = true
					}
				}
				return !found
			})
		}
		return !found
	})
	return found
}

// renderPlain renders a word without variable resolution.
func renderPlain(w *syntax.Word) (string, bool) {
	var b strings.Builder
	dyn := false
	for _, part := range w.Parts {
		switch x := part.(type) {
		case *syntax.Lit:
			b.WriteString(x.Value)
		case *syntax.SglQuoted:
			b.WriteString(x.Value)
		case *syntax.DblQuoted:
			for _, p := range x.Parts {
				if lit, ok := p.(*syntax.Lit); ok {
					b.WriteString(lit.Value)
				} else {
					dyn = true
				}
			}
		default:
			dyn = true
		}
	}
	return b.String(), dyn
}
