// Package rules implements pkglint's security and hygiene checks over
// statically parsed PKGBUILDs and install scriptlets.
package rules

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/pkgfile"
	"mvdan.cc/sh/v3/syntax"
)

// Severity of a finding.
type Severity int

const (
	Info Severity = iota
	Warn
	Error
	Critical
)

var severityNames = map[Severity]string{Info: "info", Warn: "warn", Error: "error", Critical: "critical"}

func (s Severity) String() string { return severityNames[s] }

// ParseSeverity converts a name to a Severity.
func ParseSeverity(name string) (Severity, error) {
	for sev, n := range severityNames {
		if n == name {
			return sev, nil
		}
	}
	return 0, fmt.Errorf("unknown severity %q", name)
}

// MarshalJSON renders severities as their names.
func (s Severity) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON reads back what MarshalJSON wrote. Without it a Finding is
// write-only over JSON: it encodes to a name and then refuses to decode,
// because the underlying int has no idea what "warn" means.
func (s *Severity) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		return err
	}
	sev, err := ParseSeverity(name)
	if err != nil {
		return err
	}
	*s = sev
	return nil
}

// SeverityRange is the span of severities one rule can report: Low is what it
// reports by default, High the worst it escalates to. The two are equal for
// the rules whose severity does not depend on what they find.
type SeverityRange struct {
	Low, High Severity
}

// Varies reports whether the rule's severity depends on what it found, i.e.
// whether the range covers more than one severity.
func (r SeverityRange) Varies() bool { return r.High > r.Low }

// Severities returns the range of severities the rule can report. It resolves
// an unset MaxSeverity, which is indistinguishable from Info (the zero value),
// to a fixed range at Severity.
func (r Rule) Severities() SeverityRange {
	if r.MaxSeverity > r.Severity {
		return SeverityRange{Low: r.Severity, High: r.MaxSeverity}
	}
	return SeverityRange{Low: r.Severity, High: r.Severity}
}

// Finding is a single reported issue.
type Finding struct {
	RuleID   string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Col      int      `json:"col"`
}

// Scope says which kind of input a rule analyzes.
type Scope int

const (
	// ScopePKGBUILD rules run over a package directory: the PKGBUILD and the
	// install scriptlets committed next to it.
	ScopePKGBUILD Scope = iota
	// ScopePackage rules run over a built package archive (.pkg.tar.*).
	ScopePackage
)

// Rule is a single check.
type Rule struct {
	ID    string
	Name  string // short slug, e.g. "unpinned-vcs-source"
	Scope Scope

	// Severity is what the rule reports. A handful of rules escalate on what
	// they find — an eval of a downloaded script is worse than a plain eval —
	// and set MaxSeverity to the worst they can reach; it is left unset when
	// the severity is fixed. Read the pair through Severities, which resolves
	// that zero value. Both are declarations *about* Check, not inputs to it:
	// the findings carry their own severity, and TestRuleSeveritiesAreDeclared
	// holds the two in agreement.
	Severity    Severity
	MaxSeverity Severity

	Doc   string // one-paragraph explanation, rendered on the report card site
	Check func(*Context) []Finding

	// FixLevel classifies the rule's auto-fix (FixNone when it has none); Fix
	// computes the edits. A FixSafe fix runs under --fix, a FixUnsafe fix only
	// under --unsafe-fix.
	FixLevel FixLevel
	Fix      Fixer

	// Bad and Good are illustrative PKGBUILD snippets shown on the report
	// card site: Bad is a construct the rule flags, Good the preferred
	// alternative. Attached from the examples table in Registry().
	Bad  string
	Good string
}

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
	fnVars map[string]map[string]string
	// localFuncs is the set of function names the file itself declares, keyed
	// by unit path, including those nested inside a compound statement — which
	// Unit.Functions does not carry. Rules that exempt a command by name need
	// it: the exemption has to lapse when the PKGBUILD supplies the body.
	localFuncs map[string]bool
	// funcDecls holds the bodies behind localFuncs, for the rules that judge a
	// call by what the function does rather than by its name. Only the
	// declarations whose commands collect() attributes to the function itself
	// are recorded: one nested inside another function keeps the enclosing
	// attribution, so its own body would look empty here, and an empty body
	// reads as harmless.
	funcDecls map[string]*syntax.FuncDecl
	// pureFn caches the verdict funcPurity computed for every declaration in
	// funcDecls.
	pureFn map[string]bool
	// suppUsed caches which inline-ignore directives actually suppress a
	// finding; see Context.suppressionUsage.
	suppUsed map[suppKey]bool

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
	ctx := &Context{Pkg: pkg, vars: map[string]string{}, fnVars: map[string]map[string]string{},
		localFuncs: map[string]bool{}, funcDecls: map[string]*syntax.FuncDecl{}}
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
			ctx.localFuncs[u.Path+"\x00"+name] = true
			ctx.funcDecls[u.Path+"\x00"+name] = u.Functions[name]
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
			ctx.localFuncs[u.Path+"\x00"+fd.Name.Value] = true
			if fn == "" {
				ctx.funcDecls[u.Path+"\x00"+fd.Name.Value] = fd
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
	key := u.Path + "\x00" + fn
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

func (ctx *Context) newCommand(u *pkgbuild.Unit, fn string, stmt *syntax.Stmt, call *syntax.CallExpr) Command {
	cmd := Command{Unit: u, Fn: fn, Stmt: stmt, Call: call}
	vars := ctx.varsFor(u, fn)
	args := call.Args
	for len(args) > 0 {
		name, dyn := pkgbuild.RenderWord(args[0], vars)
		if cmd.RawName == "" {
			cmd.RawName = name
		}
		if dyn || hasVarRef(name) {
			cmd.Dynamic = dyn
			cmd.Name = ""
			args = args[1:]
			break
		}
		base := basename(name)
		if wrappers[base] {
			args = args[1:]
			lookup := false
			// Skip the wrapper's flags and VAR=val words.
			for len(args) > 0 {
				s, d := pkgbuild.RenderWord(args[0], vars)
				if !d && (hasPrefixAny(s, "-") || isAssignWord(s)) {
					lookup = lookup || (base == "command" && commandLooksUp(s))
					args = args[1:]
					continue
				}
				break
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
	return ctx.localFuncs[c.Unit.Path+"\x00"+name]
}

// Commands returns every command; optional filters restrict the results.
func (ctx *Context) Commands() []Command { return ctx.cmds }

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

func (c Command) finding(id string, sev Severity, format string, args ...any) Finding {
	pos := c.Stmt.Pos()
	return Finding{
		RuleID:   id,
		Severity: sev,
		Message:  message(format, args...),
		Path:     c.Unit.Path,
		Line:     int(pos.Line()),
		Col:      int(pos.Col()),
	}
}

func findingAt(id string, sev Severity, path string, pos syntax.Pos, format string, args ...any) Finding {
	return Finding{
		RuleID:   id,
		Severity: sev,
		Message:  message(format, args...),
		Path:     path,
		Line:     int(pos.Line()),
		Col:      int(pos.Col()),
	}
}

// message formats a finding. Rendered words carry a NUL wherever the renderer
// hit something it could not resolve — a slice, a replacement, a command
// substitution — which rules test for but which has no business reaching a
// terminal, a JSON report, or a SARIF file. Reporting a source as
// "BaseX\x00.zip" reads like a corrupt filename rather than what it is: an
// expansion whose value is not knowable without running the PKGBUILD, so it
// gets shown as the ellipsis it means.
//
// Substitution happens on the arguments rather than the formatted result
// because %q escapes the NUL into a literal backslash-x-0-0 first, which no
// amount of scanning the output would then recognize.
func message(format string, args ...any) string {
	for i, a := range args {
		s, ok := a.(string)
		if !ok || !strings.Contains(s, "\x00") {
			continue
		}
		args[i] = ellipsize(s)
	}
	return ellipsize(fmt.Sprintf(format, args...))
}

// ellipsize replaces each run of unresolvable-expansion sentinels with one
// ellipsis. A run means adjacent expansions — `$a$b$c` renders as three — and
// "…" says everything "………" does about a stretch of text nothing can pin down.
func ellipsize(s string) string {
	if !strings.Contains(s, "\x00") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != 0 {
			b.WriteByte(s[i])
			continue
		}
		b.WriteString("…")
		for i+1 < len(s) && s[i+1] == 0 {
			i++
		}
	}
	return b.String()
}

// The registry is assembled once and shared. Building it means concatenating
// nine rule groups, attaching examples, and sorting a few hundred entries —
// cheap in isolation, but Run and its helpers ask for it several times per
// package, and over a corpus that dominated the linter's allocations.
//
// The state lives behind a sync.Once rather than in a package-level
// initializer because the rule groups reference functions that reach back into
// the registry (PB913 runs every rule), which as a var dependency would be an
// initialization cycle.
var (
	registryOnce sync.Once
	registryAll  []Rule          // every rule, ID order
	registryByID map[string]Rule // ID -> rule
)

func buildRegistry() {
	all := [][]Rule{integrityRules, hermeticRules, execRules, fsRules, scriptletRules, consistencyRules, correctnessRules, packageRules, styleRules}
	var out []Rule
	for _, group := range all {
		out = append(out, group...)
	}
	for i := range out {
		if ex, ok := examples[out[i].ID]; ok {
			out[i].Bad = ex.Bad
			out[i].Good = ex.Good
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	registryAll = out
	registryByID = make(map[string]Rule, len(out))
	for _, r := range out {
		registryByID[r.ID] = r
	}
}

// registry is the shared rule slice, for callers inside the package that only
// read it. Callers must not modify what it returns.
func registry() []Rule {
	registryOnce.Do(buildRegistry)
	return registryAll
}

// Registry is every rule, in ID order. It returns a fresh slice, so a caller
// is free to sort or filter it in place.
func Registry() []Rule {
	return slices.Clone(registry())
}

// Run executes every PKGBUILD-scope rule not in ignore and returns findings,
// dropping ones suppressed by inline directives. The result is totally ordered
// by (Path, Line, Col, RuleID, Message) and free of exact duplicates, so the
// same package always lints to the same list.
func Run(pkg *pkgbuild.Package, ignore map[string]bool) []Finding {
	ctx := NewContext(pkg)
	var out []Finding
	for _, rule := range registry() {
		if rule.Scope != ScopePKGBUILD || ignore[rule.ID] {
			continue
		}
		for _, f := range rule.Check(ctx) {
			if pkg.Suppressed(f.RuleID, f.Path, f.Line) {
				continue
			}
			out = append(out, f)
		}
	}
	return sortDedupe(out)
}

// RunPackage executes every package-scope rule over a built package archive,
// plus the scriptlet rules over the archive's .INSTALL if it has one. db may
// be nil, in which case the rules that need dependency resolution do not run.
func RunPackage(pf *pkgfile.Package, db *alpmdb.DB, ignore map[string]bool) []Finding {
	ctx := &Context{File: pf, DB: db, vars: map[string]string{}}
	var out []Finding
	for _, rule := range registry() {
		if rule.Scope != ScopePackage || ignore[rule.ID] {
			continue
		}
		out = append(out, rule.Check(ctx)...)
	}
	out = append(out, runPackageScriptlet(pf, ignore)...)
	return sortDedupe(out)
}

// runPackageScriptlet runs the PKGBUILD-scope rules over the archive's
// .INSTALL scriptlet, keeping only the findings anchored in the scriptlet
// itself. This gives built packages the same install-time analysis (network,
// persistence, obfuscation, hook redundancy) a package directory gets.
func runPackageScriptlet(pf *pkgfile.Package, ignore map[string]bool) []Finding {
	install := pf.Entry(".INSTALL")
	if install == nil || len(install.Data) == 0 {
		return nil
	}
	const path = ".INSTALL"
	pseudo := &pkgbuild.Package{
		Vars:         map[string]*pkgbuild.Var{},
		Suppressions: map[string]map[int]map[string]bool{},
	}
	// Rules walk every unit including the PKGBUILD; give the pseudo-package an
	// empty-but-parsed one so nothing trips over a nil AST.
	if empty, err := pkgbuild.ParseScriptlet("", nil); err == nil {
		empty.Scriptlet = false
		pseudo.PKGBUILD = empty
	}
	if unit, err := pkgbuild.ParseScriptlet(path, install.Data); err != nil {
		pseudo.ScriptletErrors = []pkgbuild.ScriptletError{{Path: path, Err: err.Error()}}
	} else {
		pseudo.Scriptlets = []pkgbuild.Unit{unit}
	}
	ctx := NewContext(pseudo)
	var out []Finding
	for _, rule := range registry() {
		if rule.Scope != ScopePKGBUILD || ignore[rule.ID] {
			continue
		}
		for _, f := range rule.Check(ctx) {
			// Rules that judge the (empty) pseudo-PKGBUILD report at other
			// paths; only the scriptlet's own findings are real here.
			if f.Path == path {
				out = append(out, f)
			}
		}
	}
	return out
}

func sortDedupe(out []Finding) []Finding {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Message < b.Message
	})
	// Drop exact duplicates (same rule + location + message). After a total
	// sort, duplicates are adjacent. This de-noises rules that can emit the
	// same finding twice (e.g. overlapping persistence-path hints).
	if len(out) > 1 {
		deduped := out[:1]
		for _, f := range out[1:] {
			last := deduped[len(deduped)-1]
			if f.RuleID == last.RuleID && f.Path == last.Path && f.Line == last.Line &&
				f.Col == last.Col && f.Message == last.Message {
				continue
			}
			deduped = append(deduped, f)
		}
		out = deduped
	}
	return out
}

// RuleByID returns the rule with the given ID.
func RuleByID(id string) (Rule, bool) {
	registryOnce.Do(buildRegistry)
	r, ok := registryByID[id]
	return r, ok
}

// Lookup resolves a rule from what a user typed on the command line: an ID in
// any case ("pb101"), or the rule's short name ("skipped-checksum"). IDs are
// matched first, so the two namespaces must not overlap and a name must
// identify one rule; TestRegistryIsWellFormed holds the registry to both.
func Lookup(q string) (Rule, bool) {
	q = strings.TrimSpace(q)
	if r, ok := RuleByID(strings.ToUpper(q)); ok {
		return r, true
	}
	name := strings.ToLower(q)
	for _, r := range registry() {
		if r.Name == name {
			return r, true
		}
	}
	return Rule{}, false
}

// suggestLimit caps how many near matches an unresolvable rule name offers.
const suggestLimit = 5

// Suggest returns the rules an argument Lookup could not resolve plausibly
// meant: IDs it prefixes ("pb10"), and names it appears in ("checksum"). It
// exists to make an error message helpful, not as a search API — at most
// suggestLimit rules come back, in ID order.
func Suggest(q string) []Rule {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []Rule
	for _, r := range registry() {
		if strings.HasPrefix(strings.ToLower(r.ID), q) || strings.Contains(r.Name, q) {
			out = append(out, r)
			if len(out) == suggestLimit {
				break
			}
		}
	}
	return out
}
