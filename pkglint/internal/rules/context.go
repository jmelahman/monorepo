package rules

import (
	"maps"
	"sort"
	"strings"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/pkgfile"
	"mvdan.cc/sh/v3/syntax"
)

// The analysis engine every PKGBUILD rule reads through: the Context built
// once per package, and the commands, variables and function bodies it
// resolves from the parse.

// fnKey names a function within one file of the package: path is the unit's
// path, name the function's.
type fnKey struct{ path, name string }

// Context carries the package under analysis plus precomputed command
// information shared by rules. A rule run has exactly one of Pkg (a PKGBUILD
// package directory) and File (a built package archive) to work from — which
// one is what Scope says. Both are set only for the package-scope fixers,
// which read a finding out of the archive and write the answer into the
// PKGBUILD that produced it; see fixpkg.go for why that pairing is narrow.
type Context struct {
	Pkg  *pkgbuild.Package
	File *pkgfile.Package // built package under analysis, for ScopePackage rules
	DB   *alpmdb.DB       // pacman local database; nil when unavailable
	cmds []Command
	vars map[string]string
	// fnVars caches the per-function variants of vars, keyed by unit path and
	// function name. Two things make a function's view differ from the file's:
	// makepkg rebinds pkgname inside package_<name>(), and a function that
	// assigns a name of its own leaves the top-level value stale.
	fnVars map[fnKey]map[string]string
	// localFuncs is the set of function names the file itself declares, keyed
	// by unit path, including those nested inside a compound statement — which
	// Unit.Functions does not carry. Rules that exempt a command by name need
	// it: the exemption has to lapse when the PKGBUILD supplies the body.
	localFuncs map[fnKey]bool
	// funcDecls holds the bodies behind localFuncs, for the rules that judge a
	// call by what the function does rather than by its name. Only the
	// declarations whose commands collect() attributes to the function itself
	// are recorded: one nested inside another function keeps the enclosing
	// attribution, so its own body would look empty here, and an empty body
	// reads as harmless.
	funcDecls map[fnKey]*syntax.FuncDecl
	// allFuncDecls holds every declaration of each name, nested ones
	// included, for the rules that follow a call into whatever body it may
	// run, whatever the commands inside are attributed to.
	allFuncDecls map[fnKey][]*syntax.FuncDecl
	// pureFn caches the verdict funcPurity computed for every declaration in
	// funcDecls.
	pureFn map[fnKey]bool
	// suppUsed caches which inline-ignore directives actually suppress a
	// finding; see Context.suppressionUsage.
	suppUsed map[suppKey]bool

	byStmt map[*syntax.Stmt]int // lazily built index of cmds by statement

	pkgFacts *packageFacts // lazily computed facts shared by package rules
}

// Command is one resolved command invocation anywhere in a unit (including
// inside command substitutions).
type Command struct {
	Unit    *pkgbuild.Unit
	Fn      string // enclosing function name; "" for top-level code
	Stmt    *syntax.Stmt
	Call    *syntax.CallExpr
	Name    string // resolved command name; "" when not statically resolvable
	RawName string // rendered first word, for messages
	Dynamic bool   // command name contains unresolvable constructs
	Args    []string
	ArgDyn  []bool
	ArgWord []*syntax.Word // the word each argument was rendered from
}

// NewContext precomputes shared state for rules.
func NewContext(pkg *pkgbuild.Package) *Context {
	ctx := &Context{Pkg: pkg, vars: map[string]string{}, fnVars: map[fnKey]map[string]string{},
		localFuncs: map[fnKey]bool{}, funcDecls: map[fnKey]*syntax.FuncDecl{},
		allFuncDecls: map[fnKey][]*syntax.FuncDecl{}}
	for name, v := range pkg.Vars {
		// Scalars render to exactly one value; for arrays, bash expands an
		// unsubscripted $name to the first element (and an empty array to "").
		if len(v.Values) > 0 {
			ctx.vars[name] = pkg.Expand(v.Values[0])
		} else if v.Array {
			ctx.vars[name] = ""
		}
	}
	units := pkg.Units()
	for i := range units {
		u := &units[i]
		names := make([]string, 0, len(u.Functions))
		for name := range u.Functions {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			ctx.localFuncs[fnKey{u.Path, name}] = true
			ctx.funcDecls[fnKey{u.Path, name}] = u.Functions[name]
			ctx.allFuncDecls[fnKey{u.Path, name}] = append(ctx.allFuncDecls[fnKey{u.Path, name}], u.Functions[name])
			ctx.collect(u, name, u.Functions[name].Body)
		}
		for _, stmt := range u.TopLevel {
			ctx.collect(u, "", stmt)
		}
	}
	return ctx
}

func (ctx *Context) collect(u *pkgbuild.Unit, fn string, root *syntax.Stmt) {
	syntax.Walk(root, func(node syntax.Node) bool {
		// parseUnit only lifts a FuncDecl into Unit.Functions when it sits
		// directly in the file's statement list, so one declared inside a
		// compound statement — the idiom of picking between two build() bodies
		// with a top-level `if` — stays in TopLevel. It is still a function:
		// its body runs when makepkg calls the phase, not when the file is
		// sourced. Recurse under the declared name so the commands inside are
		// attributed to it and not to the enclosing scope, which for a
		// top-level `if` would be "" and make every one of them look like
		// code that runs on source.
		if fd, ok := node.(*syntax.FuncDecl); ok && fd.Name != nil {
			ctx.localFuncs[fnKey{u.Path, fd.Name.Value}] = true
			ctx.allFuncDecls[fnKey{u.Path, fd.Name.Value}] = append(ctx.allFuncDecls[fnKey{u.Path, fd.Name.Value}], fd)
			if fn == "" {
				ctx.funcDecls[fnKey{u.Path, fd.Name.Value}] = fd
				ctx.collect(u, fd.Name.Value, fd.Body)
				return false
			}
			// Declared inside another function, though, the body only exists
			// while that function runs — a `_dl() { curl …; }` in build() is
			// build()'s network access however it is spelled — so its commands
			// keep the enclosing attribution and the phase rules that key on
			// it. Only the top-level `if` idiom above re-homes commands.
			return true
		}
		stmt, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		ctx.cmds = append(ctx.cmds, ctx.newCommand(u, fn, stmt, call))
		return true
	})
}

// wrappers whose next non-flag argument is the real command.
var wrappers = map[string]bool{
	"env": true, "nice": true, "ionice": true, "nohup": true, "timeout": true,
	"stdbuf": true, "eatmydata": true, "setsid": true, "command": true,
	"exec": true, "builtin": true, "sudo": true, "doas": true,
	// busybox runs the applet its first argument names, so `busybox wget`
	// is wget. It takes no options ahead of an applet: a leading switch
	// (--install, --list) is busybox's own and runs none; see busyboxSelf.
	"busybox": true,
	// Proxy launchers run their command with its traffic redirected, so
	// `torsocks wget` is wget; PB310 reports the launcher itself.
	"torsocks": true, "proxychains": true, "proxychains4": true, "tsocks": true,
}

// busyboxSelf reports whether a busybox command line runs busybox itself rather
// than an applet: nothing follows it, or the next word is one of its own
// switches. Skipping those as wrapper flags would leave `busybox --install -s
// /bin` naming a command "bin".
func busyboxSelf(base string, next []string) bool {
	return base == "busybox" && (len(next) == 0 || strings.HasPrefix(next[0], "-"))
}

// commandLooksUp reports whether a flag word passed to `command` selects one of
// its -v/-V lookup forms, which describe where a name would resolve rather than
// running it. They are short options only: `command` takes no long ones, so a
// `--` word is the end-of-options marker and the next word is a real command.
func commandLooksUp(flag string) bool {
	if len(flag) < 2 || flag[0] != '-' || flag[1] == '-' {
		return false
	}
	return strings.ContainsAny(flag[1:], "vV")
}

// varsFor returns the variable map for rendering words inside fn, which differs
// from the file-level map in two ways.
//
// makepkg runs each split's package_<name>() with pkgname rebound to that
// split, so inside those functions $pkgname is the split's own name rather than
// the pkgname array's first element.
//
// And a function that assigns a name the file also assigns makes the
// file-level value stale: `_installDir=/usr/share/$pkgname` at the top,
// `_installDir=$pkgdir$_installDir` in package(), and every later use means the
// staged path, not the live one. Rendering those uses against the top-level
// value does not produce an approximation — it produces a string that never
// exists during the build, which is how a rule ends up reporting a write to
// /usr/share that only ever lands under $pkgdir. Dropping the name renders the
// use as `$_installDir`, which is what it honestly is: a value only running the
// PKGBUILD would know.
func (ctx *Context) varsFor(u *pkgbuild.Unit, fn string) map[string]string {
	if fn == "" {
		return ctx.vars // top-level code: the file-level values are the live ones
	}
	key := fnKey{u.Path, fn}
	if m, ok := ctx.fnVars[key]; ok {
		return m
	}
	m := ctx.buildFnVars(u, fn)
	ctx.fnVars[key] = m
	return m
}

func (ctx *Context) buildFnVars(u *pkgbuild.Unit, fn string) map[string]string {
	stale := map[string]bool{}
	mark := func(names map[string]bool) {
		for n := range names {
			if _, ok := ctx.vars[n]; ok {
				stale[n] = true
			}
		}
	}
	// The function's own `local`s count — they shadow the file-level value for
	// the rest of its body — but an earlier phase's died with that function,
	// so by the time fn runs the file-level value is live again.
	mark(assignedIn(u, fn, true))
	for _, name := range precedingPhases(fn) {
		mark(assignedIn(u, name, false))
	}
	split := ctx.splitName(fn)
	if len(stale) == 0 && split == "" {
		return ctx.vars
	}
	m := make(map[string]string, len(ctx.vars))
	maps.Copy(m, ctx.vars)
	for n := range stale {
		delete(m, n)
	}
	if split != "" {
		// makepkg binds pkgname immediately before calling the function, so this
		// holds however the file wrote the array.
		m["pkgname"] = split
	}
	return m
}

// splitName returns the split package fn builds, or "" if fn is not a
// package_<name>() for a name the pkgname array declares.
func (ctx *Context) splitName(fn string) string {
	split, ok := strings.CutPrefix(fn, "package_")
	if !ok {
		return ""
	}
	v, ok := ctx.Pkg.Vars["pkgname"]
	if !ok || !v.Array {
		return ""
	}
	for _, val := range v.Values {
		if ctx.Pkg.Expand(val) == split {
			return split
		}
	}
	return ""
}

// buildPhases are the functions makepkg runs, in order, in one shell before it
// reaches package(). An assignment in an earlier one is still in effect in a
// later one — which is why a name build() rewrote is no longer the file's.
var buildPhases = []string{"prepare", "build", "check"}

// precedingPhases returns the phases makepkg has already run by the time it
// calls fn. A helper function the PKGBUILD calls itself has no fixed place in
// that order, so only its own assignments count for it.
func precedingPhases(fn string) []string {
	switch {
	case fn == "package" || strings.HasPrefix(fn, "package_"):
		return buildPhases
	case fn == "check":
		return buildPhases[:2]
	case fn == "build":
		return buildPhases[:1]
	}
	return nil
}

// assignedIn returns the names assigned anywhere inside fn's body, including
// inside conditionals and loops: what matters is that the value can change, not
// whether this particular run changes it. With includeLocals false, `local`
// declarations are skipped — those bindings do not outlive the function, so
// they cannot make its value stale for anyone called later.
func assignedIn(u *pkgbuild.Unit, fn string, includeLocals bool) map[string]bool {
	fd := u.Functions[fn]
	if fd == nil {
		return nil
	}
	var out map[string]bool
	syntax.Walk(fd.Body, func(n syntax.Node) bool {
		if d, ok := n.(*syntax.DeclClause); ok && !includeLocals &&
			d.Variant != nil && d.Variant.Value == "local" {
			return false
		}
		a, ok := n.(*syntax.Assign)
		if !ok || a.Name == nil {
			return true
		}
		if out == nil {
			out = map[string]bool{}
		}
		out[a.Name.Value] = true
		return true
	})
	return out
}

// wrapperValueFlags lists, per wrapper, the options that take their value as
// the next word. Skipping only the flag left the value standing where the
// command should be: `nice -n 10 curl` resolved to a command called "10".
// Only the separate-word spelling needs listing; `-n10` and `--signal=KILL`
// are one word and already skipped as flags.
var wrapperValueFlags = map[string]map[string]bool{
	"timeout": {"-s": true, "--signal": true, "-k": true, "--kill-after": true},
	"nice":    {"-n": true, "--adjustment": true},
	"ionice":  {"-c": true, "--class": true, "-n": true, "--classdata": true},
	"stdbuf":  {"-i": true, "-o": true, "-e": true},
	"env":     {"-u": true, "--unset": true, "-C": true, "--chdir": true},
	"exec":    {"-a": true},
	"sudo": {"-u": true, "-g": true, "-h": true, "-p": true, "-C": true, "-D": true,
		"-R": true, "-T": true, "-U": true, "-r": true, "-t": true},
	"doas": {"-u": true, "-C": true},
	"torsocks": {"-u": true, "--user": true, "-p": true, "--pass": true,
		"-a": true, "--address": true, "-P": true, "--port": true},
	"proxychains": {"-f": true}, "proxychains4": {"-f": true},
}

// wrapperOperands is how many positional words a wrapper consumes before the
// command it runs: timeout's duration.
var wrapperOperands = map[string]int{"timeout": 1}

type varState int

const (
	varUnassigned varState = iota // the file never assigns it: $CC, $MAKE
	varResolved                   // every assignment in scope names one command
	varOpaque                     // assigned, but dynamically or inconsistently
)

// commandVarValue resolves a command name written as nothing but a variable
// reference — `$d`, `"${d}"` — from the assignments in scope. varsFor drops a
// name its function assigns, rightly, since the file-level value is stale
// there; but that left `d=curl; $d … | sh` with no name for any rule to
// match and nothing reporting that it had none.
//
// A name the file never assigns is the environment's ($CC, $MAKE) and stays
// as it was. One every assignment agrees on resolves to that command. Anything
// else — a command substitution, two branches naming different tools — is a
// command pkglint cannot name, which PB306 exists to say.
//
// Some assignments say nothing about the name and are skipped: a declaration
// with no value (`export CC`, `local tool`) and a self-default with an empty
// default (`CC="${CC:-}"`). A self-default with a word (`d="${d:-curl}"`)
// names that word, since it is what runs when the environment is silent. A
// wrapper-led value (`env LC_ALL=C make`) names the command its options lead
// to; array first elements are brace-expanded as everywhere else.
//
// A name spelled from several pieces — `$a$b`, `${c}rl` — resolves piece by
// piece and joins: `a=bu; b=n; $a$b` is bun. Each piece must be one word; one
// assigned to something unreadable makes the whole name PB306's, and one the
// file never assigns leaves it the environment's, as a lone `$CC` would. A
// word with a literal `/` is a path to a file the build runs, so an
// unreadable piece of it leaves the name as unresolved as before.
func (ctx *Context) commandVarValue(u *pkgbuild.Unit, fn string, stmt *syntax.Stmt, call *syntax.CallExpr, w *syntax.Word) (string, varState) {
	if u == nil {
		return "", varUnassigned
	}
	at := -1
	if stmt != nil {
		at = off(stmt.Pos())
	}
	ref := varRefName(w)
	if ref == "" {
		return ctx.compositeVarValue(u, fn, at, call, w)
	}
	return varValue(u, ref, fn, at, call, nameInFields)
}

// compositeVarValue is commandVarValue for a word of several parts.
func (ctx *Context) compositeVarValue(u *pkgbuild.Unit, fn string, at int, call *syntax.CallExpr, w *syntax.Word) (string, varState) {
	if w == nil {
		return "", varUnassigned
	}
	vars := ctx.varsFor(u, fn)
	oneWord := func(fields []string) (string, bool) { return fields[0], len(fields) == 1 }
	var parts []syntax.WordPart
	for _, p := range w.Parts {
		if dq, ok := p.(*syntax.DblQuoted); ok {
			parts = append(parts, dq.Parts...)
		} else {
			parts = append(parts, p)
		}
	}
	var b strings.Builder
	state, path := varResolved, false
	for _, p := range parts {
		pw := &syntax.Word{Parts: []syntax.WordPart{p}}
		s, d := pkgbuild.RenderWord(pw, vars)
		if !d && !hasVarRef(s) {
			b.WriteString(s)
			path = path || strings.Contains(s, "/")
			continue
		}
		st := varOpaque
		var v string
		if ref := varRefName(pw); !d && ref != "" {
			v, st = varValue(u, ref, fn, at, call, oneWord)
		}
		switch {
		case st == varOpaque:
			state = varOpaque
		case st == varUnassigned && state == varResolved:
			state = varUnassigned // keep looking: a later piece may still be opaque
		}
		b.WriteString(v)
	}
	if state == varOpaque && path {
		// `"$_stage0/bin/llvm-profdata"`, `./setup_${ver}.run`: a path is a
		// file the build runs, not a tool pkglint failed to name.
		return "", varUnassigned
	}
	if state != varResolved {
		return "", state
	}
	return b.String(), varResolved
}

// varValue resolves ref from the assignments in scope at position at, taking
// from each assignment's words the one pick names.
func varValue(u *pkgbuild.Unit, ref, fn string, at int, call *syntax.CallExpr, pick func([]string) (string, bool)) (string, varState) {
	// commandVarMaxAssigns: a name assigned more often than this is not one
	// pkglint will reconcile; it also keeps N assignments × N uses from being
	// quadratic again.
	const commandVarMaxAssigns = 64
	if len(u.AssignsTo(ref)) > commandVarMaxAssigns {
		return "", varOpaque
	}
	state, value := varUnassigned, ""
	scanAssignments(u, ref, fn, at, call, true, func(as *syntax.Assign) {
		if state == varOpaque {
			return
		}
		if as.Naked {
			return // a declaration with no value neither names a command nor hides one
		}
		var selfDef *syntax.Word
		if as.Value != nil {
			if def, ok := selfDefaultWord(as.Value, ref); ok {
				if def == nil || len(def.Parts) == 0 {
					return // `${CC:-}`: nothing to name, the environment's as before
				}
				selfDef = def
			}
		}
		var first *syntax.Word
		switch {
		case as.Append || as.Index != nil:
		case selfDef != nil:
			first = selfDef // what runs whenever the environment is silent
		case as.Value != nil:
			first = as.Value
		case as.Array != nil && len(as.Array.Elems) > 0:
			first = as.Array.Elems[0].Value
		}
		if first == nil {
			state = varOpaque
			return
		}
		s, d := pkgbuild.RenderWord(first, nil)
		if !d && as.Value == nil && selfDef == nil {
			s = pkgbuild.ExpandBraces(s)[0]
		}
		fields := strings.Fields(s)
		if d || hasVarRef(s) || len(fields) == 0 {
			state = varOpaque
			return
		}
		name, ok := pick(fields)
		if !ok {
			state = varOpaque
			return
		}
		if state == varResolved && name != value {
			state = varOpaque
			return
		}
		state, value = varResolved, name
	})
	return value, state
}

// selfDefaultWord recognises w as exactly `${name:-…}` (or `-`, `=`, `:=`),
// optionally double-quoted, and returns its default word (nil when empty). Such
// a line keeps whatever the environment supplied and otherwise runs the
// default, so it names its default; with no default it says nothing, like a
// name the file never assigns.
func selfDefaultWord(w *syntax.Word, name string) (def *syntax.Word, ok bool) {
	if w == nil || len(w.Parts) != 1 {
		return nil, false
	}
	part := w.Parts[0]
	if dq, ok := part.(*syntax.DblQuoted); ok {
		if len(dq.Parts) != 1 {
			return nil, false
		}
		part = dq.Parts[0]
	}
	pe, ok := part.(*syntax.ParamExp)
	if !ok || pe.Param == nil || pe.Param.Value != name || pe.Exp == nil {
		return nil, false
	}
	switch pe.Exp.Op {
	case syntax.DefaultUnset, syntax.DefaultUnsetOrNull, syntax.AssignUnset, syntax.AssignUnsetOrNull:
		return pe.Exp.Word, true
	}
	return nil, false
}

// nameInFields returns the command a value's words run: the first word, or
// for a wrapper-led value (`env LC_ALL=C make`) the word its options lead
// to. ok is false when the wrapper's options use up the value, which leaves
// the command among the caller's own arguments.
func nameInFields(fields []string) (name string, ok bool) {
	for len(fields) > 0 {
		base := basename(unescapeName(fields[0]))
		if !wrappers[base] || len(fields) == 1 {
			return fields[0], true // a lone wrapper is skipped by newCommand's own loop
		}
		if busyboxSelf(base, fields[1:]) {
			return fields[0], true
		}
		fields = fields[1:]
		for len(fields) > 0 && (hasPrefixAny(fields[0], "-") || isAssignWord(fields[0])) {
			w := fields[0]
			if base == "command" && commandLooksUp(w) {
				return "command", true
			}
			fields = fields[1:]
			if wrapperValueFlags[base][w] && len(fields) > 0 {
				fields = fields[1:]
			}
		}
		for n := wrapperOperands[base]; n > 0 && len(fields) > 0; n-- {
			fields = fields[1:]
		}
	}
	return "", false
}

func (ctx *Context) newCommand(u *pkgbuild.Unit, fn string, stmt *syntax.Stmt, call *syntax.CallExpr) Command {
	cmd := Command{Unit: u, Fn: fn, Stmt: stmt, Call: call}
	vars := ctx.varsFor(u, fn)
	args := call.Args
	for len(args) > 0 {
		name, dyn := pkgbuild.RenderWord(args[0], vars)
		if cmd.RawName == "" {
			cmd.RawName = name
		}
		if !dyn && hasVarRef(name) {
			if resolved, state := ctx.commandVarValue(u, fn, stmt, call, args[0]); state == varResolved {
				name = resolved
			} else if state == varOpaque {
				dyn = true // assigned, but to nothing readable: PB306's case
			}
		}
		if dyn || hasVarRef(name) {
			cmd.Dynamic = dyn
			cmd.Name = ""
			args = args[1:]
			break
		}
		base := basename(unescapeName(name))
		if wrappers[base] && base == "busybox" {
			var next []string
			if len(args) > 1 {
				s, _ := pkgbuild.RenderWord(args[1], vars)
				next = []string{s}
			}
			if busyboxSelf(base, next) {
				cmd.Name = base
				args = args[1:]
				break
			}
		}
		if wrappers[base] {
			args = args[1:]
			lookup := false
			// Skip the wrapper's flags and VAR=val words.
			for len(args) > 0 {
				s, d := pkgbuild.RenderWord(args[0], vars)
				if !d && (hasPrefixAny(s, "-") || isAssignWord(s)) {
					lookup = lookup || (base == "command" && commandLooksUp(s))
					args = args[1:]
					if wrapperValueFlags[base][s] && len(args) > 0 {
						args = args[1:]
					}
					continue
				}
				break
			}
			for n := wrapperOperands[base]; n > 0 && len(args) > 0; n-- {
				args = args[1:]
			}
			if lookup {
				// `command -v cc` prints where cc would be found and runs
				// nothing. Resolving through to the argument made every probe
				// for a tool read as an invocation of it — `M4="$(command -v
				// m4)"` was reported as m4 executing at the PKGBUILD's top
				// level, and a `command -v yarn` in build() as yarn fetching.
				cmd.Name = "command"
				break
			}
			continue
		}
		cmd.Name = base
		args = args[1:]
		break
	}
	for _, w := range args {
		s, dyn := pkgbuild.RenderWord(w, vars)
		cmd.Args = append(cmd.Args, s)
		cmd.ArgDyn = append(cmd.ArgDyn, dyn)
		cmd.ArgWord = append(cmd.ArgWord, w)
	}
	return cmd
}

// definesFunc reports whether the unit the command came from declares a
// function of that name, at any nesting depth.
func (ctx *Context) definesFunc(c Command, name string) bool {
	return ctx.localFuncs[fnKey{c.Unit.Path, name}]
}

// Commands returns every command; optional filters restrict the results.
func (ctx *Context) Commands() []Command { return ctx.cmds }

// commandAt returns the Command collected for stmt. Rules that walk the
// syntax tree themselves must use it rather than resolve the statement
// again: only collect knows which function a statement belongs to, and a
// name resolved without that is resolved against the wrong variables.
func (ctx *Context) commandAt(stmt *syntax.Stmt) (Command, bool) {
	if ctx.byStmt == nil {
		ctx.byStmt = make(map[*syntax.Stmt]int, len(ctx.cmds))
		for i, c := range ctx.cmds {
			if _, dup := ctx.byStmt[c.Stmt]; !dup {
				ctx.byStmt[c.Stmt] = i
			}
		}
	}
	i, ok := ctx.byStmt[stmt]
	if !ok {
		return Command{}, false
	}
	return ctx.cmds[i], true
}

// CommandsNamed returns commands with a resolved name in names.
func (ctx *Context) CommandsNamed(names ...string) []Command {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	var out []Command
	for _, c := range ctx.cmds {
		if set[c.Name] {
			out = append(out, c)
		}
	}
	return out
}

// HasArg reports whether the command has an argument exactly equal to s.
func (c Command) HasArg(s string) bool {
	for _, a := range c.Args {
		if a == s {
			return true
		}
	}
	return false
}

// Subcommand returns the first argument that does not look like a flag.
func (c Command) Subcommand() string {
	for _, a := range c.Args {
		if !hasPrefixAny(a, "-") {
			return a
		}
	}
	return ""
}

// InBuildPhase reports whether the command runs in build(), check(),
// package() or a split package_*() function of a PKGBUILD.
func (c Command) InBuildPhase() bool {
	if c.Unit.Scriptlet {
		return false
	}
	return c.Fn == "build" || c.Fn == "check" || c.Fn == "package" || hasPrefixAny(c.Fn, "package_")
}
