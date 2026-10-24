package rules

import (
	"fmt"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// RunCheck is what makepkg's command line says about running check(): the
// last of --check and --nocheck wins, and neither leaves it to options=().
type RunCheck int

const (
	CheckDefault RunCheck = iota // neither flag: options=(!check) decides
	CheckForced                  // --check
	CheckSkipped                 // --nocheck
)

// NameProblem reports what is wrong with name as a package name, by makepkg's
// own rule (lint_one_pkgname): ASCII letters, digits and @._+- only, neither
// empty nor starting with - or .; "" when nothing is. PB701 and `pkglint
// build`'s dependency sync share it, so there is one definition.
func NameProblem(name string) string { return nameProblem(name) }

// SyncDeps returns the dependency names makepkg -s would ask pacman for when
// it builds pkg on carch — what `pkglint build` installs in a container on
// makepkg's behalf, because makepkg itself cannot get root there. It reads the
// parse, never the file run: the values are the ones every rule here sees.
//
// The fields are makepkg's own: the top-level depends and makedepends, and
// checkdepends when makepkg would run check() (a check function, and neither
// --nocheck nor options=(!check) unless --check overrides it), each with its
// _$carch variant merged in, as merge_arch_attrs does whatever arch= says. A
// split package's depends inside package_<name>() are deliberately not read.
// makepkg -s does not install them either — they are metadata for the
// archive, read only after the build — and they routinely name the PKGBUILD's
// own sibling packages, which no repository has yet.
//
// Version constraints are cut ("foo>=1.2" -> "foo", "libfoo.so=1-64" ->
// "libfoo.so"); duplicates are dropped and first-written order kept. Names are
// returned as written and not validated: the caller is the one handing them to
// a process, so the check that matters is made there (NameProblem).
//
// Guessing is the one thing this must not do. A list missing an entry fails
// the build with "missing dependencies" half an hour into a compile at best,
// and an entry rendered wrongly installs something nobody asked for. So the
// parse's answer is only taken when nothing at the top level could make bash
// disagree with it (settled), and every field it cannot know is an error
// naming it, not a skip.
func SyncDeps(pkg *pkgbuild.Package, carch string, run RunCheck) ([]string, error) {
	fields := []string{"depends", "makedepends"}
	_, hasCheck := pkg.PKGBUILD.Functions["check"]
	wantCheck := hasCheck && run != CheckSkipped
	fields = append(fields, "checkdepends")
	var names []string
	for _, f := range fields {
		names = append(names, f, f+"_"+carch)
	}
	if err := settled(pkg, append(slices.Clone(names), "options")); err != nil {
		return nil, err
	}
	if wantCheck && run == CheckDefault {
		skip, err := optionsSkipCheck(pkg)
		if err != nil {
			return nil, err
		}
		wantCheck = !skip
	}

	var out []string
	seen := map[string]bool{}
	for _, name := range names {
		if strings.HasPrefix(name, "checkdepends") && !wantCheck {
			continue
		}
		v := pkg.Vars[name]
		if v == nil {
			continue
		}
		if v.CountUnknown {
			return nil, fmt.Errorf("%s holds an array reference or word-splitting expansion that cannot be resolved without running the PKGBUILD", name)
		}
		for _, e := range varElems(v) {
			val, ok := staticVal(pkg, e.Value)
			if !ok {
				return nil, fmt.Errorf("%s entry %q is not statically known (a substitution or an unset variable), so it cannot be resolved without running the PKGBUILD", name, printable(e.Value))
			}
			// Unquoted, bash would split this into several names; quoted,
			// it is one name no package has. Either way it is not a list
			// to hand to pacman as it stands.
			if strings.ContainsAny(val, " \t\n") {
				return nil, fmt.Errorf("%s entry %q contains whitespace, so it is not one package name", name, printable(val))
			}
			dep := val
			if i := strings.IndexAny(dep, "<>="); i >= 0 {
				dep = dep[:i]
			}
			if !seen[dep] {
				seen[dep] = true
				out = append(out, dep)
			}
		}
	}
	return out, nil
}

// optionsSkipCheck reports whether options=() holds !check. The array must
// be statically known: whether checkdepends are installed hangs on it.
func optionsSkipCheck(pkg *pkgbuild.Package) (bool, error) {
	v := pkg.Vars["options"]
	if v == nil {
		return false, nil
	}
	if v.CountUnknown {
		return false, fmt.Errorf("options holds an array reference that cannot be resolved without running the PKGBUILD, and decides whether checkdepends are needed")
	}
	for _, e := range varElems(v) {
		val, ok := staticVal(pkg, e.Value)
		if !ok {
			return false, fmt.Errorf("options entry %q is not statically known, and decides whether checkdepends are needed", printable(e.Value))
		}
		if strings.TrimPrefix(val, `\`) == "!check" {
			return true, nil
		}
	}
	return false, nil
}

// settled checks that the parse's value of every name in fields — and of
// every variable those values are built from, transitively — is the value
// bash would hold when makepkg reads the metadata. The parse models plain
// assignments and += in file order; it does not model anything else that can
// write a variable, so anything else at the top level that could is refused:
//
//   - eval, source/., mapfile/readarray, unset, alias, trap and enable,
//     anywhere at the top level, and a command whose name is not a literal
//     (it could be any of those);
//   - a call to one of the file's own functions outside a subshell, since
//     its body could assign anything;
//   - declare/typeset/local/export/readonly naming one of the variables, or
//     with -n (a nameref aliases one name to another);
//   - read, printf -v, getopts and the like given one of the names, an
//     indexed write, a for-loop variable, a temporary `name=x cmd`
//     assignment, or an assignment inside a subshell (which bash discards
//     and a parse could still record);
//   - ${name=word}/${name:=word} and arithmetic assignment, wherever written;
//   - assignment under control flow, or a referenced variable assigned after
//     the assignment that reads it (the parse uses its final value; bash, the
//     one current at the time).
//
// Function bodies are not walked: makepkg calls them only after resolving
// dependencies, and nothing at the top level may call them (above).
func settled(pkg *pkgbuild.Package, fields []string) error {
	u := &pkg.PKGBUILD
	tracked := map[string]bool{}
	work := slices.Clone(fields)
	for len(work) > 0 {
		name := work[0]
		work = work[1:]
		if tracked[name] {
			continue
		}
		tracked[name] = true
		if pkg.ConditionalVars[name] {
			return fmt.Errorf("%s is assigned under top-level control flow (an if, case or loop), so its value cannot be read without running the PKGBUILD", name)
		}
		if v := pkg.Vars[name]; v != nil && v.CountUnknown && !slices.Contains(fields, name) {
			return fmt.Errorf("%s, which a dependency list is built from, holds an array reference that cannot be resolved without running the PKGBUILD", name)
		}
		for _, site := range u.AssignsTo(name) {
			if site.Fn != "" {
				continue
			}
			refs, err := references(site.Assign)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			for _, ref := range refs {
				if ref == name {
					continue
				}
				for _, other := range u.AssignsTo(ref) {
					if other.Fn == "" && other.Assign.Pos().Offset() > site.Assign.Pos().Offset() {
						return fmt.Errorf("%s reads %s, which is assigned again after it, so its value cannot be read without running the PKGBUILD", name, ref)
					}
				}
				work = append(work, ref)
			}
		}
	}

	funcs := map[string]bool{}
	syntax.Walk(u.File, func(n syntax.Node) bool {
		if fd, ok := n.(*syntax.FuncDecl); ok {
			funcs[fd.Name.Value] = true
		}
		return true
	})
	s := &topScan{tracked: tracked, funcs: funcs}
	for _, stmt := range u.TopLevel {
		s.walk(stmt, false)
		if s.err != nil {
			return s.err
		}
	}
	return nil
}

// references returns the variables an assignment's value reads.
func references(as *syntax.Assign) ([]string, error) {
	var refs []string
	var err error
	walkAssignValue(as, func(n syntax.Node) bool {
		pe, ok := n.(*syntax.ParamExp)
		if !ok || err != nil {
			return err == nil
		}
		if pe.Excl {
			err = fmt.Errorf("an indirect reference (${!…}) cannot be resolved without running the PKGBUILD")
			return false
		}
		if pe.Param != nil {
			refs = append(refs, pe.Param.Value)
		}
		return true
	})
	return refs, err
}

func walkAssignValue(as *syntax.Assign, fn func(syntax.Node) bool) {
	if as.Index != nil {
		syntax.Walk(as.Index, fn)
	}
	if as.Value != nil {
		syntax.Walk(as.Value, fn)
	}
	if as.Array != nil {
		syntax.Walk(as.Array, fn)
	}
}

// refusedBuiltins can write variables in ways the parse does not follow.
var refusedBuiltins = map[string]bool{
	"eval": true, "source": true, ".": true, "mapfile": true, "readarray": true,
	"unset": true, "alias": true, "trap": true, "enable": true,
	// Reached as commands only through `builtin declare` and the like; written
	// plainly they are DeclClauses.
	"declare": true, "typeset": true, "local": true, "export": true, "readonly": true,
}

// topScan walks the top-level statements for anything settled refuses.
type topScan struct {
	tracked map[string]bool
	funcs   map[string]bool
	err     error
}

func (s *topScan) fail(format string, args ...any) {
	if s.err == nil {
		s.err = fmt.Errorf(format+", so the dependency lists cannot be read without running the PKGBUILD", args...)
	}
}

func (s *topScan) walk(root syntax.Node, inSub bool) {
	syntax.Walk(root, func(n syntax.Node) bool {
		if s.err != nil {
			return false
		}
		switch x := n.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst, *syntax.Subshell:
			// What runs in a subshell cannot change the parent's
			// variables; only an assignment there is checked, since bash
			// discards it and the parse might not.
			if n != root && !inSub {
				s.walk(n, true)
				return false
			}
		case *syntax.FuncDecl:
			// A definition under control flow: its body runs only when
			// called, which the CallExpr case refuses. Whether check()
			// exists decides whether checkdepends are installed.
			if x.Name.Value == "check" {
				s.fail("check() is defined under top-level control flow")
			}
			return false
		case *syntax.Assign:
			if x.Name != nil && s.tracked[x.Name.Value] && (inSub || x.Index != nil) {
				s.fail("%s is assigned in a subshell or by index", x.Name.Value)
			}
		case *syntax.CallExpr:
			if !inSub {
				s.call(x)
			}
		case *syntax.DeclClause:
			if !inSub {
				s.decl(x)
			}
		case *syntax.ParamExp:
			if x.Exp != nil && (x.Exp.Op == syntax.AssignUnset || x.Exp.Op == syntax.AssignUnsetOrNull) && !inSub {
				s.fail("${…=…} assigns a variable as a side effect of expanding it")
			}
		case *syntax.BinaryArithm:
			// Assgn sits apart from the compound operators in the token
			// enum, which run contiguously from += to **=.
			if (x.Op == syntax.Assgn || x.Op >= syntax.AddAssgn && x.Op <= syntax.PowAssgn) && !inSub {
				s.fail("arithmetic assignment at the top level")
			}
		case *syntax.UnaryArithm:
			if (x.Op == syntax.Inc || x.Op == syntax.Dec) && !inSub {
				s.fail("arithmetic assignment at the top level")
			}
		case *syntax.ForClause:
			if wi, ok := x.Loop.(*syntax.WordIter); ok && s.tracked[wi.Name.Value] && !inSub {
				s.fail("%s is a for-loop variable", wi.Name.Value)
			}
		}
		return true
	})
}

func (s *topScan) call(x *syntax.CallExpr) {
	if len(x.Args) == 0 {
		return
	}
	for _, as := range x.Assigns {
		if as.Name != nil && s.tracked[as.Name.Value] {
			s.fail("%s is assigned for a single command", as.Name.Value)
			return
		}
	}
	args := x.Args
	for {
		name := args[0].Lit()
		if name == "" {
			s.fail("a command at the top level has a name that is not a literal")
			return
		}
		if (name == "command" || name == "builtin") && len(args) > 1 {
			args = args[1:]
			for len(args) > 1 && strings.HasPrefix(args[0].Lit(), "-") {
				args = args[1:]
			}
			continue
		}
		if refusedBuiltins[name] {
			s.fail("the top level runs %s", name)
			return
		}
		if s.funcs[name] {
			s.fail("the top level calls the PKGBUILD's own function %s", name)
			return
		}
		break
	}
	// read -a depends, printf -v depends, getopts x depends, -vdepends...
	for _, w := range args[1:] {
		lit := w.Lit()
		for i := range len(lit) {
			rest := lit[i:]
			if j := strings.IndexAny(rest, "[="); j >= 0 {
				rest = rest[:j]
			}
			if s.tracked[rest] {
				s.fail("a command at the top level is handed the name %s", rest)
				return
			}
		}
	}
}

func (s *topScan) decl(x *syntax.DeclClause) {
	if x.Variant.Value == "nameref" {
		s.fail("the top level declares a nameref")
		return
	}
	for _, as := range x.Args {
		if as.Name == nil {
			if opt := as.Value.Lit(); (strings.HasPrefix(opt, "-") || strings.HasPrefix(opt, "+")) && strings.Contains(opt, "n") {
				s.fail("the top level declares a nameref")
				return
			}
			continue
		}
		if s.tracked[as.Name.Value] {
			s.fail("%s is set by %s", as.Name.Value, x.Variant.Value)
			return
		}
	}
}

// printable renders the NUL marker RenderWord leaves for a construct it could
// not render as something a terminal shows.
func printable(s string) string {
	return strings.ReplaceAll(s, "\x00", "…")
}
