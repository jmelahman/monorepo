package rules

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

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
		b.WriteString(prepareCd(ctx, u, block, c.Fn, indent))
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
	if cds := fnCommands(ctx, u, c.Fn, "cd"); len(cds) > 0 {
		b.WriteString(cdLine(cds[0], indent))
	}
	b.WriteString(indent + download(at))
	b.WriteString("}\n\n")
	return Edit{
		Path: u.Path, Start: at, End: at, New: b.String(), Line: line,
		Desc: "add prepare() with `go mod download` so " + c.Fn + "() needn't fetch",
	}, true
}

// cdLine returns cd as a full source line — indent and any `|| exit` guard
// included — or "" for the zero Command (fn has no cd). prepare() must land
// in the directory the build step runs in, and copying the exact line is how
// it provably does. A cd that shares its line with another command
// (`cd "$pkgname" && make generate`) is rewritten alone, from its words as
// written, so the neighbor is not run twice.
func cdLine(cd Command, indent string) string {
	if cd.Stmt == nil {
		return ""
	}
	u := cd.Unit
	start := lineStart(u.Raw, off(cd.Stmt.Pos()))
	end := off(cd.Stmt.End())
	for end < len(u.Raw) && u.Raw[end] != '\n' {
		end++
	}
	line := string(u.Raw[start:end])
	if rest := line[off(cd.Stmt.End())-start:]; rest != "" && !cdGuardRe.MatchString(rest) {
		return indent + "cd " + cdTarget(cd) + "\n"
	}
	return line + "\n"
}

// prepareCd returns the lines that take the end of an existing prepare() to
// the directory fn compiles in, so a download appended there lands in the
// module the build reads. A prepare() that never changes directory works at
// $srcdir's root, where fn's opening cd is the one to copy. One whose only
// directory change is an unconditional, top-level cd naming the same place
// as fn's needs nothing. Anywhere else — dms-shell's prepare() cds into the
// checkout while build() compiles in its core/ subdirectory, or the cd sits
// in a subshell or an if and may not have happened — the download first
// returns to $srcdir, where makepkg starts every phase and so where fn's own
// cd is relative to.
func prepareCd(ctx *Context, u *pkgbuild.Unit, prepare *syntax.Block, fn, indent string) string {
	var build Command
	if cds := fnCommands(ctx, u, fn, "cd"); len(cds) > 0 {
		build = cds[0]
	}
	moves := fnCommands(ctx, u, "prepare", "cd", "pushd", "popd")
	switch {
	case len(moves) == 0:
		return cdLine(build, indent)
	case len(moves) == 1 && moves[0].Name == "cd" && build.Stmt != nil &&
		topLevelStmt(prepare, moves[0].Stmt) && cdTarget(moves[0]) == cdTarget(build):
		return ""
	}
	return indent + "cd \"$srcdir\"\n" + cdLine(build, indent)
}

// topLevelStmt reports whether s always runs when block does, in the shell
// that runs block: it is one of block's statements, or the left-hand side of
// one's && / || chain. A pipeline, subshell or conditional runs it elsewhere
// or not at all, and a cd there says nothing about the directory afterwards.
func topLevelStmt(block *syntax.Block, s *syntax.Stmt) bool {
	for _, x := range block.Stmts {
		for x != nil {
			if x == s {
				return true
			}
			bin, ok := x.Cmd.(*syntax.BinaryCmd)
			if !ok || (bin.Op != syntax.AndStmt && bin.Op != syntax.OrStmt) {
				break
			}
			x = bin.X
		}
	}
	return false
}

// cdTarget is a cd's argument words as written, so two cds compare equal
// only when they name their directory the same way.
func cdTarget(c Command) string {
	var parts []string
	for _, w := range c.ArgWord {
		parts = append(parts, string(c.Unit.Raw[off(w.Pos()):off(w.End())]))
	}
	return strings.Join(parts, " ")
}

// cdGuardRe matches the failure guard a cd is allowed to share its line with.
var cdGuardRe = regexp.MustCompile(`^\s*\|\|\s*(exit|return)\b[^|&;]*$`)

// fnCommands returns the invocations of the named commands in u's function
// fn, in source order.
func fnCommands(ctx *Context, u *pkgbuild.Unit, fn string, names ...string) []Command {
	var out []Command
	for _, c := range ctx.Commands() {
		if c.Unit == u && c.Fn == fn && slices.Contains(names, c.Name) {
			out = append(out, c)
		}
	}
	return out
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
//
// With shared false every edit goes on the command line, for a flag a shared
// home would also hand to commands that must not get it.
func goFlagEdits(ctx *Context, cmds []Command, flagPrefix, flag string, shared bool) []Edit {
	var edits []Edit
	seen := map[int]bool{}
	for _, c := range cmds {
		if goFlagAddressed(goFlags(ctx, c), c, flagPrefix) {
			continue
		}
		edit, ok := goFlagEdit(c, flag)
		if !shared {
			edit, ok = goFlagIntoCommand(c, flag)
		}
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
	return goFlagEdits(ctx, goBuildCommands(ctx), "-buildmode", "-buildmode=pie", true)
}

func fixGoTrimpath(ctx *Context, _ *FixEnv) []Edit {
	return goFlagEdits(ctx, goBuildCommands(ctx), "-trimpath", "-trimpath", true)
}

func fixGoModcacheRW(ctx *Context, _ *FixEnv) []Edit {
	// go rejects -modcacherw with module mode off, so beside a GOPATH-mode
	// command a GOFLAGS export or flags array the two share cannot carry it.
	return goFlagEdits(ctx, goModuleCommands(ctx), "-modcacherw", "-modcacherw", !anyGoModulesOff(ctx))
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
