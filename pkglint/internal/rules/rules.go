// Package rules implements pkglint's security and hygiene checks over
// statically parsed PKGBUILDs and install scriptlets.
package rules

import (
	"encoding/json"
	"fmt"
	"iter"
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
	// alternative.
	Bad  string
	Good string

	// Refs are the upstream documents the rule enforces — a section of the
	// Arch package guidelines, a man page — shown on the rule's reference
	// page. Empty when the rule is pkglint's own policy: a citation claims
	// the document says what the rule says, so a rule nothing upstream backs
	// goes without one.
	Refs []Ref
}

// Ref is one upstream document a rule cites.
type Ref struct {
	Title string // e.g. "Rust package guidelines"
	URL   string
}

// ReferenceURL is where the report card site publishes the rule reference:
// one page per rule, which every surface that names a rule links to.
const ReferenceURL = "https://jamison.lahman.dev/pkglint/rules/"

// URL is the rule's page in the published rule reference.
func (r Rule) URL() string {
	return ReferenceURL + r.ID + ".html"
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
// initializer because PB913 runs every rule: a var built from the groups would
// depend on itself through checkStaleIgnores. The groups are functions, not
// vars, for the same reason.
var (
	registryOnce sync.Once
	registryAll  []Rule          // every rule, ID order
	registryByID map[string]Rule // ID -> rule
)

func buildRegistry() {
	all := [][]Rule{integrityRules(), hermeticRules(), execRules(), fsRules(), scriptletRules(), consistencyRules(), correctnessRules(), packageRules(), styleRules()}
	var out []Rule
	for _, group := range all {
		out = append(out, group...)
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

// rulesIn yields the registered rules of one scope, in ID order, skipping the
// IDs in ignore.
func rulesIn(scope Scope, ignore map[string]bool) iter.Seq[Rule] {
	return func(yield func(Rule) bool) {
		for _, r := range registry() {
			if r.Scope == scope && !ignore[r.ID] && !yield(r) {
				return
			}
		}
	}
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
	for rule := range rulesIn(ScopePKGBUILD, ignore) {
		for _, f := range rule.Check(ctx) {
			if pkg.Suppressed(f.RuleID, f.Path, f.Line) {
				continue
			}
			out = append(out, f)
		}
	}
	// Past the expansion budget, references stay unexpanded and every rule
	// that reads them goes quiet — which a file could arrange on purpose. Say so.
	if pkg.ExpandExhausted() && !ignore["PB307"] && !pkg.Suppressed("PB307", pkg.PKGBUILD.Path, 1) {
		out = append(out, Finding{RuleID: "PB307", Severity: Warn, Path: pkg.PKGBUILD.Path, Line: 1, Col: 1,
			Message: "variable references in this PKGBUILD expand to more than 32 MiB; pkglint stopped " +
				"expanding them, so values that depend on them were not analyzed"})
	}
	return sortDedupe(out)
}

// RunPackage executes every package-scope rule over a built package archive,
// plus the scriptlet rules over the archive's .INSTALL if it has one. db may
// be nil, in which case the rules that need dependency resolution do not run.
func RunPackage(pf *pkgfile.Package, db *alpmdb.DB, ignore map[string]bool) []Finding {
	ctx := &Context{File: pf, DB: db, vars: map[string]string{}}
	var out []Finding
	for rule := range rulesIn(ScopePackage, ignore) {
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
	if install == nil || (len(install.Data) == 0 && install.Size == 0) {
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
	switch unit, err := pkgbuild.ParseScriptlet(path, install.Data); {
	case len(install.Data) == 0:
		// The archive reader keeps a member's bytes only up to its cap; a
		// larger .INSTALL was never read, and no rule has seen it.
		pseudo.ScriptletErrors = []pkgbuild.ScriptletError{{Path: path,
			Err: fmt.Sprintf("%d bytes is too large to analyze", install.Size)}}
	case err != nil:
		pseudo.ScriptletErrors = []pkgbuild.ScriptletError{{Path: path, Err: err.Error()}}
	default:
		pseudo.Scriptlets = []pkgbuild.Unit{unit}
	}
	ctx := NewContext(pseudo)
	var out []Finding
	for rule := range rulesIn(ScopePKGBUILD, ignore) {
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
