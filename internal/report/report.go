// Package report aggregates rule findings into graded package reports and
// renders them for humans and machines.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jmelahman/pkglint/internal/rules"
)

// PackageReport is the lint result for one package.
type PackageReport struct {
	Name     string          `json:"name"`
	Path     string          `json:"path"`
	Grade    string          `json:"grade"`
	Findings []rules.Finding `json:"findings"`
	Err      string          `json:"error,omitempty"`
}

// Grade condenses findings into a letter. The scale is deliberately simple:
// any critical -> F, any error -> D, warns pull an otherwise clean package
// down to B or C, info-only stays A.
func Grade(findings []rules.Finding) string {
	warns := 0
	worst := rules.Info
	for _, f := range findings {
		if f.Severity > worst {
			worst = f.Severity
		}
		if f.Severity == rules.Warn {
			warns++
		}
	}
	switch {
	case worst == rules.Critical:
		return "F"
	case worst == rules.Error:
		return "D"
	case warns >= 3:
		return "C"
	case warns >= 1:
		return "B"
	default:
		return "A"
	}
}

// New builds a report for a package path.
func New(path string, findings []rules.Finding) PackageReport {
	if findings == nil {
		findings = []rules.Finding{}
	}
	name := filepath.Base(path)
	if name == "PKGBUILD" {
		name = filepath.Base(filepath.Dir(path))
	}
	// filepath.Dir(".") is "." again, so a relative path like "." or "PKGBUILD"
	// needs an absolute path to recover the containing directory's name.
	if name == "." || name == string(filepath.Separator) || name == "" {
		if abs, err := filepath.Abs(path); err == nil {
			target := abs
			if filepath.Base(abs) == "PKGBUILD" {
				target = filepath.Dir(abs)
			}
			name = filepath.Base(target)
		}
	}
	return PackageReport{Name: name, Path: path, Grade: Grade(findings), Findings: findings}
}

// NewError builds a report for a package that failed to load. It reuses New's
// name derivation and guarantees a non-nil Findings slice (so JSON emits [] not
// null, consistent with successful reports).
func NewError(path string, loadErr error) PackageReport {
	r := New(path, nil)
	r.Grade = "?"
	r.Err = loadErr.Error()
	return r
}

// sanitize renders untrusted text safe for a terminal by escaping control
// characters, so PKGBUILD-derived content (paths, messages) cannot inject ANSI
// escapes, carriage returns, or title-setting sequences into the report.
func sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case unicode.IsControl(r): // C1 controls (U+0080–U+009F), etc.
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ANSI SGR sequences RenderText brackets tokens with when color is on.
const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiDim       = "\x1b[2m"
	ansiRed       = "\x1b[31m"
	ansiGreen     = "\x1b[32m"
	ansiYellow    = "\x1b[33m"
	ansiMagenta   = "\x1b[35m"
	ansiCyan      = "\x1b[36m"
	ansiBoldRed   = "\x1b[1;31m"
	ansiBoldGreen = "\x1b[1;32m"
)

// styler colorizes trusted report tokens; styler(false) passes text through
// unchanged, so plain rendering is byte-identical to the pre-color output.
type styler bool

func (s styler) wrap(code, text string) string {
	if !s {
		return text
	}
	return code + text + ansiReset
}

func (s styler) severity(sev rules.Severity) string {
	var code string
	switch sev {
	case rules.Info:
		code = ansiCyan
	case rules.Warn:
		code = ansiYellow
	case rules.Error:
		code = ansiRed
	default: // Critical
		code = ansiBoldRed
	}
	return s.wrap(code, sev.String())
}

func (s styler) grade(g string) string {
	var code string
	switch g {
	case "A":
		code = ansiBoldGreen
	case "B":
		code = ansiGreen
	case "C":
		code = ansiYellow
	case "D":
		code = ansiRed
	case "F":
		code = ansiBoldRed
	default: // "?" — failed to load
		code = ansiMagenta
	}
	return s.wrap(code, g)
}

// RenderText writes the human-readable report. Untrusted, PKGBUILD-derived
// fields (name, error, path, message) are sanitized; severity, rule ID, and
// grade are trusted enum/registry values. Color is applied only around those
// trusted tokens (and the already-sanitized name), so untrusted content can
// neither carry its own escapes nor break out of ours.
//
// Packages with no findings are silent unless verbose is set; a summary at the
// end accounts for every package either way, followed by the auto-fix tally
// when any finding has a fix.
func RenderText(w io.Writer, reports []PackageReport, color, verbose bool) {
	s := styler(color)
	printed := 0
	for _, r := range reports {
		if r.Err == "" && len(r.Findings) == 0 && !verbose {
			continue
		}
		if printed > 0 {
			fmt.Fprintln(w)
		}
		printed++
		if r.Err != "" {
			fmt.Fprintf(w, "%s: %s %s\n",
				s.wrap(ansiBold, sanitize(r.Name)), s.wrap(ansiBoldRed, "error:"), sanitize(r.Err))
			continue
		}
		fmt.Fprintf(w, "%s: grade %s", s.wrap(ansiBold, sanitize(r.Name)), s.grade(r.Grade))
		if len(r.Findings) == 0 {
			fmt.Fprintf(w, ", no findings\n")
			continue
		}
		fmt.Fprintf(w, ", %d finding(s)\n", len(r.Findings))
		for _, f := range r.Findings {
			if f.Line == 0 {
				// Package-archive findings locate a member, not a line.
				fmt.Fprintf(w, "  %s: %s %s %s\n",
					sanitize(f.Path), s.severity(f.Severity), s.wrap(ansiDim, "["+f.RuleID+"]"), sanitize(f.Message))
				continue
			}
			fmt.Fprintf(w, "  %s:%d:%d: %s %s %s\n",
				sanitize(f.Path), f.Line, f.Col, s.severity(f.Severity), s.wrap(ansiDim, "["+f.RuleID+"]"), sanitize(f.Message))
		}
	}
	if printed > 0 {
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%s\n", summarize(reports))
	if fixes := summarizeFixes(reports); fixes != "" {
		fmt.Fprintf(w, "%s\n", fixes)
	}
}

// summarize condenses the run into one line: how many packages were linted and
// how they split across clean / with findings / failed to load (zero-count
// groups are omitted).
func summarize(reports []PackageReport) string {
	clean, flawed, failed := 0, 0, 0
	for _, r := range reports {
		switch {
		case r.Err != "":
			failed++
		case len(r.Findings) > 0:
			flawed++
		default:
			clean++
		}
	}
	noun := "packages"
	if len(reports) == 1 {
		noun = "package"
	}
	var parts []string
	if clean > 0 {
		parts = append(parts, fmt.Sprintf("%d clean", clean))
	}
	if flawed > 0 {
		parts = append(parts, fmt.Sprintf("%d with findings", flawed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed to load", failed))
	}
	out := fmt.Sprintf("%d %s linted", len(reports), noun)
	if len(parts) > 0 {
		out += ": " + strings.Join(parts, ", ")
	}
	return out
}

// summarizeFixes counts the findings whose rule declares an auto-fix and names
// the flag that applies each group, so the run ends by saying how much of it a
// rerun could repair. --unsafe-fix implies --fix, hence "more": its count is
// what the unsafe level adds on top of the safe one.
//
// The tally reads the rule registry, not the fixers, so it counts findings a
// fix *targets*. A few fixes need state the linter does not have — sources on
// disk for a digest, a probe of the https host — and stay unapplied; the count
// is an upper bound for those, which is why it advertises flags rather than
// promising a number of rewrites. Empty when nothing is fixable.
func summarizeFixes(reports []PackageReport) string {
	index := make(map[string]rules.Rule)
	for _, r := range rules.Registry() {
		index[r.ID] = r
	}
	// Two tallies, because the two scopes are repaired by different commands:
	// a built-package finding has no PKGBUILD in front of `pkglint --fix` to
	// rewrite, and advertising that flag for it would send the user to a run
	// that changes nothing.
	var safe, unsafe, buildSafe, buildUnsafe int
	for _, r := range reports {
		for _, f := range r.Findings {
			rule, ok := index[f.RuleID]
			if !ok {
				continue
			}
			s, u := &safe, &unsafe
			if rule.Scope == rules.ScopePackage {
				s, u = &buildSafe, &buildUnsafe
			}
			switch rule.FixLevel {
			case rules.FixSafe:
				*s++
			case rules.FixUnsafe:
				*u++
			}
		}
	}
	var parts []string
	if p := fixPhrase(safe, unsafe, ""); p != "" {
		parts = append(parts, p)
	}
	if p := fixPhrase(buildSafe, buildUnsafe, "build "); p != "" {
		parts = append(parts, p)
	}
	return strings.Join(parts, "; ")
}

// fixPhrase is one tally's sentence. prefix is the command the flags belong
// to, empty for the root command.
func fixPhrase(safe, unsafe int, prefix string) string {
	switch {
	case safe > 0 && unsafe > 0:
		return fmt.Sprintf("%d finding(s) fixable with %s--fix, %d more with %s--unsafe-fix", safe, prefix, unsafe, prefix)
	case safe > 0:
		return fmt.Sprintf("%d finding(s) fixable with %s--fix", safe, prefix)
	case unsafe > 0:
		return fmt.Sprintf("%d finding(s) fixable with %s--unsafe-fix", unsafe, prefix)
	}
	return ""
}

// RenderJSON writes reports as a JSON array.
func RenderJSON(w io.Writer, reports []PackageReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(reports)
}

// SARIF 2.1.0 (https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)
// is the interchange format consumed by GitHub code scanning and other tools.
// Only the subset pkglint populates is modeled here.
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool        sarifTool         `json:"tool"`
	Results     []sarifResult     `json:"results"`
	Invocations []sarifInvocation `json:"invocations,omitempty"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string         `json:"id"`
	Name             string         `json:"name,omitempty"`
	ShortDescription sarifText      `json:"shortDescription"`
	FullDescription  sarifText      `json:"fullDescription"`
	Properties       map[string]any `json:"properties,omitempty"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID     string          `json:"ruleId"`
	RuleIndex  int             `json:"ruleIndex"`
	Level      string          `json:"level"`
	Message    sarifText       `json:"message"`
	Locations  []sarifLocation `json:"locations"`
	Properties map[string]any  `json:"properties,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
}

type sarifInvocation struct {
	ExecutionSuccessful        bool                `json:"executionSuccessful"`
	ToolExecutionNotifications []sarifNotification `json:"toolExecutionNotifications,omitempty"`
}

// sarifNotification is a notification object: the schema names the payload
// field "message", not "text" (the message object nested inside it is what
// carries "text").
type sarifNotification struct {
	Level   string    `json:"level"`
	Message sarifText `json:"message"`
}

// sarifLevel maps a pkglint severity onto SARIF's level enum
// (none|note|warning|error). SARIF has no level above "error", so Critical
// collapses onto it; RenderSARIF preserves the distinction in properties.
func sarifLevel(s rules.Severity) string {
	switch s {
	case rules.Info:
		return "note"
	case rules.Warn:
		return "warning"
	default: // Error, Critical
		return "error"
	}
}

// RenderSARIF writes findings in SARIF 2.1.0, suitable for GitHub code
// scanning. Untrusted text needs no sanitizing here: encoding/json escapes
// control characters itself (unlike RenderText, which writes to a terminal).
func RenderSARIF(w io.Writer, reports []PackageReport) error {
	reg := rules.Registry()
	idx := make(map[string]int, len(reg))
	driverRules := make([]sarifRule, len(reg))
	for i, r := range reg {
		idx[r.ID] = i
		driverRules[i] = sarifRule{
			ID:               r.ID,
			Name:             r.Name,
			ShortDescription: sarifText{Text: r.Name},
			FullDescription:  sarifText{Text: r.Doc},
		}
	}

	run := sarifRun{
		Tool: sarifTool{Driver: sarifDriver{
			Name:           "pkglint",
			InformationURI: "https://github.com/jmelahman/pkglint",
			Rules:          driverRules,
		}},
		Results:     []sarifResult{}, // non-nil → emits [] not null
		Invocations: []sarifInvocation{{ExecutionSuccessful: true}},
	}

	for _, rep := range reports {
		if rep.Err != "" {
			run.Invocations[0].ExecutionSuccessful = false
			run.Invocations[0].ToolExecutionNotifications = append(
				run.Invocations[0].ToolExecutionNotifications,
				sarifNotification{Level: "error", Message: sarifText{Text: rep.Path + ": " + rep.Err}})
			continue
		}
		for _, f := range rep.Findings {
			ri, ok := idx[f.RuleID]
			if !ok {
				ri = -1 // finding from a rule not in the registry (shouldn't happen)
			}
			line := f.Line
			if line < 1 {
				line = 1 // SARIF requires startLine >= 1
			}
			run.Results = append(run.Results, sarifResult{
				RuleID:    f.RuleID,
				RuleIndex: ri,
				Level:     sarifLevel(f.Severity),
				Message:   sarifText{Text: f.Message},
				Locations: []sarifLocation{{PhysicalLocation: sarifPhysical{
					ArtifactLocation: sarifArtifact{URI: f.Path},
					Region:           sarifRegion{StartLine: line, StartColumn: f.Col},
				}}},
				Properties: map[string]any{"severity": f.Severity.String()},
			})
		}
	}

	doc := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs:    []sarifRun{run},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// ExceedsThreshold reports whether any finding is at or above sev.
func ExceedsThreshold(reports []PackageReport, sev rules.Severity) bool {
	for _, r := range reports {
		if r.Err != "" {
			return true
		}
		for _, f := range r.Findings {
			if f.Severity >= sev {
				return true
			}
		}
	}
	return false
}

// rulesWidth is the column --rules wraps at: the classic terminal width, so
// the listing reads in a default-size window and in a pager. ruleIndent is
// what a rule's body — documentation, prose, snippets — hangs under, and
// ruleLabelIndent the shallower indent of the labels heading those blocks in
// `explain`.
const (
	rulesWidth      = 80
	ruleIndent      = "      "
	ruleLabelIndent = "  "
)

// RenderRules writes the rule listing behind --rules: one header line per
// rule — ID, name, the severity it reports (a range when it escalates), and
// the flag that auto-fixes it, if any — followed by its documentation wrapped
// to rulesWidth. Every token is registry data, so color needs no sanitizing.
func RenderRules(w io.Writer, all []rules.Rule, color bool) {
	s := styler(color)
	for i, r := range all {
		if i > 0 {
			fmt.Fprintln(w)
		}
		renderRuleHeader(w, s, r)
		renderRuleDoc(w, r)
	}
}

// renderRuleHeader writes a rule's identity line: ID, name, the severity it
// reports (a range when it escalates), and the flag that auto-fixes it, if
// any. --rules and `explain` share it so the two cannot drift apart.
func renderRuleHeader(w io.Writer, s styler, r rules.Rule) {
	fmt.Fprintf(w, "%s %s  %s", s.wrap(ansiBold, r.ID), r.Name, s.severityRange(r.Severities()))
	if flag := r.FixCommand(); flag != "" {
		code := ansiGreen
		if !r.FixLevel.Safe() {
			code = ansiYellow
		}
		fmt.Fprintf(w, "  %s", s.wrap(code, flag))
	}
	fmt.Fprintln(w)
}

// renderRuleDoc writes a rule's documentation, wrapped under the indent the
// rest of the listing uses.
func renderRuleDoc(w io.Writer, r rules.Rule) {
	renderIndented(w, wrapWords(r.Doc, rulesWidth-len(ruleIndent)))
}

// renderIndented writes already-wrapped lines under ruleIndent.
func renderIndented(w io.Writer, lines []string) {
	for _, line := range lines {
		if line == "" {
			// A blank separator inside a block is blank: indenting it would
			// leave trailing whitespace on the line.
			fmt.Fprintln(w)
			continue
		}
		fmt.Fprintf(w, "%s%s\n", ruleIndent, line)
	}
}

// RenderRuleDetail writes one rule's reference page behind `pkglint explain`:
// the --rules header and documentation, what the rule is checked against, the
// flagged-versus-preferred example pair the report card site shows, the
// auto-fix invocation when the rule has one, and the directive that suppresses
// it. Everything printed is registry data, so — as in RenderRules — color
// needs no sanitizing. The snippets are written verbatim, however long their
// lines: they are code, and wrapping them would break what they demonstrate.
func RenderRuleDetail(w io.Writer, r rules.Rule, color bool) {
	s := styler(color)
	renderRuleHeader(w, s, r)
	renderRuleDoc(w, r)

	applies := "the PKGBUILD and the install scriptlets committed beside it."
	if r.Scope == rules.ScopePackage {
		applies = "the built package archive (*.pkg.tar.*), not the PKGBUILD — lint one " +
			"directly, or run 'pkglint build' to produce it."
	}
	renderRuleSection(w, s, ansiBold, "Applies to", wrapWords(applies, rulesWidth-len(ruleIndent)))

	if r.Bad != "" {
		renderRuleSection(w, s, ansiRed, "Flagged", snippetLines(r.Bad))
	}
	if r.Good != "" {
		renderRuleSection(w, s, ansiGreen, "Preferred", snippetLines(r.Good))
	}
	if flag := r.FixCommand(); flag != "" {
		fix := fmt.Sprintf("'pkglint %s' rewrites this in place", flag)
		if !r.FixLevel.Safe() {
			fix += " — the rewrite is mechanical but changes what the build does, so read the result"
		}
		fix += ". Add --diff to see the change without writing it."
		renderRuleSection(w, s, ansiBold, "Auto-fix", wrapWords(fix, rulesWidth-len(ruleIndent)))
	}
	renderRuleSection(w, s, ansiBold, "Suppress", suppressLines(r))
}

// suppressLines is the Suppress block: the directive that silences one
// finding, then the prose under it. Only a PKGBUILD-scope rule gets the inline
// form — a built-package finding is anchored at a path inside the archive,
// where there is no line of source for a directive to sit on, so --ignore is
// the only thing that turns it off.
func suppressLines(r rules.Rule) []string {
	directive, prose := "# pkglint: ignore="+r.ID, fmt.Sprintf(
		"on the finding's line or the line above it, in that file. '--ignore %s' turns "+
			"the rule off for a whole run instead.", r.ID)
	if r.Scope == rules.ScopePackage {
		directive, prose = "pkglint --ignore "+r.ID, "turns the rule off for a whole run. An "+
			"inline '# pkglint: ignore=' directive cannot reach this finding: it is reported "+
			"against a file in the built archive, not a line of the PKGBUILD."
		if r.FixLevel.Fixable() {
			// The fix does land in the PKGBUILD, so there the directive has
			// somewhere to go — on the line the rewrite would touch.
			prose += " The directive does decline the auto-fix, though: put it on the array " +
				"the fix would rewrite."
		}
	}
	return append([]string{directive, ""}, wrapWords(prose, rulesWidth-len(ruleIndent))...)
}

// renderRuleSection writes one labelled block of an `explain` page: a blank
// line, the label, and the body under the shared indent.
func renderRuleSection(w io.Writer, s styler, code, label string, body []string) {
	fmt.Fprintf(w, "\n%s%s\n", ruleLabelIndent, s.wrap(code, label))
	renderIndented(w, body)
}

// snippetLines splits an example into the lines to print. A snippet is code:
// its own line breaks are the ones that matter, and the trailing newline a raw
// string literal may end with is not a blank line to reproduce.
func snippetLines(snippet string) []string {
	return strings.Split(strings.TrimRight(snippet, "\n"), "\n")
}

// severityRange renders a rule's severity span, coloring each end on its own
// so "warn-critical" reads as both a warning and a critical at a glance.
func (s styler) severityRange(sr rules.SeverityRange) string {
	if !sr.Varies() {
		return s.severity(sr.Low)
	}
	return s.severity(sr.Low) + "-" + s.severity(sr.High)
}

// wrapWords greedily fills lines of at most width characters (measured in
// runes), breaking on whitespace only; a single word longer than width gets a
// line of its own rather than being split.
func wrapWords(text string, width int) []string {
	var lines []string
	var line strings.Builder
	lineLen := 0
	for _, word := range strings.Fields(text) {
		wl := utf8.RuneCountInString(word)
		if lineLen > 0 && lineLen+1+wl > width {
			lines = append(lines, line.String())
			line.Reset()
			lineLen = 0
		}
		if lineLen > 0 {
			line.WriteByte(' ')
			lineLen++
		}
		line.WriteString(word)
		lineLen += wl
	}
	if lineLen > 0 {
		lines = append(lines, line.String())
	}
	return lines
}
