package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jmelahman/pkglint/internal/rules"
)

// baseURL is where the generated site is published. Pages only need relative
// links, but a link preview does not: Open Graph consumers read the head
// without resolving it against the document, so the canonical URL and the card
// image have to be absolute, and the origin has to be known at render time.
const baseURL = "https://jamison.lahman.dev/pkglint/"

// rosterRows is how many rows the index server-renders. The corpus is far too
// large to emit in full: every row is eight DOM nodes, so the whole of it would
// cost a browser several hundred thousand elements and a filter that walks them
// all on every keystroke. A thousand rows is a page that paints immediately,
// reads as a complete roster, and still works with JavaScript off.
const rosterRows = 1000

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets
var assetFS embed.FS

var funcs = template.FuncMap{
	"sev":     func(s rules.Severity) string { return s.String() },
	"doc":     renderDoc,
	"scanErr": scanError,
	"join":    strings.Join,
}

// MaintainedBy is the roster name cell's tooltip: who maintains the package
// and who co-maintains it, or "" for a full orphan — or, for an official
// package, who maintains it on archlinux.org and who built it. It is one
// string built here rather than pieces assembled in the template, because
// site.js has to rebuild it verbatim for rows the server never sent — a
// change of wording here has a twin there.
func (r siteResult) MaintainedBy() string {
	var parts []string
	if r.Official() {
		if len(r.Maintainers) > 0 {
			parts = append(parts, "maintained by "+strings.Join(r.Maintainers, ", "))
		}
		if r.Packager != "" {
			parts = append(parts, "packaged by "+r.Packager)
		}
		return strings.Join(parts, ", ")
	}
	if r.Maintainer != "" {
		parts = append(parts, "maintained by "+r.Maintainer)
	}
	if len(r.CoMaintainers) > 0 {
		parts = append(parts, "co-maintained by "+strings.Join(r.CoMaintainers, ", "))
	}
	return strings.Join(parts, ", ")
}

// scanError strips pkglint's internal snapshot path out of a parse failure,
// leaving the file, line, column, and reason — the part a reader can act on.
// Errors arrive as "parse <snapshot>/PKGBUILD: <snapshot>/PKGBUILD:12:3: why".
func scanError(s string) string {
	if rest, ok := strings.CutPrefix(s, "parse "); ok {
		if i := strings.Index(rest, ": "); i >= 0 {
			s = rest[i+2:]
		}
	}
	head := s
	if i := strings.Index(head, ":"); i >= 0 {
		head = head[:i]
	}
	if i := strings.LastIndex(head, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// renderDoc escapes a rule's documentation and turns its `backtick` spans into
// code, which is how the rule docs are written for the terminal.
func renderDoc(s string) template.HTML {
	parts := strings.Split(s, "`")
	var b strings.Builder
	for i, p := range parts {
		esc := template.HTMLEscapeString(p)
		if i%2 == 1 && i != len(parts)-1 {
			b.WriteString("<code>" + esc + "</code>")
			continue
		}
		// An unmatched trailing backtick closes nothing, so it is prose rather
		// than the start of a span. Split has already eaten it — put it back,
		// or the sentence quietly loses a character.
		if i%2 == 1 {
			b.WriteByte('`')
		}
		b.WriteString(esc)
	}
	return template.HTML(b.String())
}

// clip shortens a description to n characters on a word boundary. Rule docs
// run to a paragraph, and a paragraph in a link preview is not read: X and
// Slack cut it wherever they run out of room, mid-word. Cutting it here at
// least ends on a word and says, with the ellipsis, that there is more.
func clip(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	cut := string(runes[:n])
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:—-") + "…"
}

// rubric states, per grade, the condition that produces it. It is shown beside
// the grade counts so the distribution explains its own axis.
var rubric = map[string]string{
	"A": "no warnings",
	"B": "1–2 warnings",
	"C": "3+ warnings",
	"D": "any error",
	"F": "any critical",
	"?": "not scanned",
}

// band is one grade's segment of the distribution bar, sized by package count.
type band struct {
	Grade  string
	Count  int
	Rubric string
	Style  template.CSS
}

// scope is the corpus, or one repository's share of it, as the index's
// headline counts and distribution bar see it. The index is rendered for the
// whole corpus and carries each repository's scope as JSON, so the script can
// redraw the numbers and the bar for whichever repositories are pressed.
type scope struct {
	Name     string         `json:"-"`
	Total    int            `json:"total"`
	Findings int            `json:"findings"`
	Fixable  int            `json:"fixable"`
	Drifted  int            `json:"drifted"`
	Grades   map[string]int `json:"grades"`
	Bands    []band         `json:"-"`
	Ticks    []tick         `json:"-"`
}

// tick is one labelled mark on the scale drawn under the distribution bar.
type tick struct {
	Label string
	Style template.CSS
}

// repoOrderOf lists the repositories present, in display order: the base
// system first, the AUR last, anything unfamiliar between.
func repoOrderOf[T any](byRepo map[string]T) []string {
	repos := make([]string, 0, len(byRepo))
	for repo := range byRepo {
		repos = append(repos, repo)
	}
	sort.Slice(repos, func(i, j int) bool {
		if ri, rj := repoRank(repos[i]), repoRank(repos[j]); ri != rj {
			return ri < rj
		}
		return repos[i] < repos[j]
	})
	return repos
}

// fileGroup collects a package's findings under the file they were found in,
// in first-seen order, so a page reads file by file rather than as one list.
type fileGroup struct {
	Path     string
	Findings []rules.Finding
}

// withRepos names the repository on every result. A result carries none only
// when it predates the official repositories — an older results.json, a test
// fixture — and those are all AUR results, which is what the roster's column,
// its filter and the package page all need spelled out rather than blank.
func withRepos(results []siteResult) []siteResult {
	out := make([]siteResult, len(results))
	for i, r := range results {
		if r.Repo == "" {
			r.Repo = aurRepo
		}
		out[i] = r
	}
	return out
}

func renderSite(out string, results []siteResult) error {
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	results = withRepos(results)
	ver, err := copyAssets(out)
	if err != nil {
		return err
	}

	registry := rules.Registry() // sorted by ID
	ruleIndex := map[string]rules.Rule{}
	for _, r := range registry {
		ruleIndex[r.ID] = r
	}

	tally := func(rs []siteResult) scope {
		sc := scope{Total: len(rs), Grades: map[string]int{}}
		for _, r := range rs {
			sc.Grades[r.Grade]++
			sc.Findings += len(r.Findings)
			if len(r.Drift) > 0 {
				sc.Drifted++
			}
			for _, f := range r.Findings {
				if rl, ok := ruleIndex[f.RuleID]; ok && rl.FixLevel.Fixable() {
					sc.Fixable++
				}
			}
		}
		sc.Bands = bands(sc.Grades, len(rs))
		sc.Ticks = ticks(len(rs))
		return sc
	}
	corpus := tally(results)

	// page wraps per-page data with what every template needs: the head's title
	// and description, the page's own absolute URL and the site's, the relative
	// path back to the site root (pages live at two depths), and an asset
	// version, so a stylesheet change can't be served from a stale cache.
	//
	// path is the page's URL relative to the site root, which is also what its
	// depth is counted from — index pages take their directory ("rules/"), so
	// they are canonicalised to the URL people actually link to.
	page := func(path, title, desc string, data map[string]any) map[string]any {
		data["Root"] = strings.Repeat("../", strings.Count(path, "/"))
		data["Ver"] = ver
		data["Title"] = title
		data["Desc"] = desc
		data["Site"] = baseURL
		data["URL"] = baseURL + path
		return data
	}

	// Only the head of the roster is server-rendered. results is votes-ordered,
	// so this is the most-installed slice of the corpus — the rows worth
	// spending bytes and DOM nodes on before a visitor has asked for anything.
	// The rest is reachable three ways: the alphabetical shards, the sitemap,
	// and roster.json, which site.js searches over once it lands.
	head := results
	if len(head) > rosterRows {
		head = head[:rosterRows]
	}

	// The repositories, each tallied on its own, in display order. With more
	// than one the index offers them as toggles, and the script sums the
	// pressed ones to redraw the headline counts and the distribution; the
	// page itself is rendered for the whole corpus, which is what a reader
	// without the script gets.
	var repos []scope
	byRepo := map[string][]siteResult{}
	for _, r := range results {
		byRepo[r.Repo] = append(byRepo[r.Repo], r)
	}
	for _, name := range repoOrderOf(byRepo) {
		sc := tally(byRepo[name])
		sc.Name = name
		repos = append(repos, sc)
	}
	repoJSON, err := json.Marshal(func() map[string]scope {
		m := map[string]scope{}
		for _, sc := range repos {
			m[sc.Name] = sc
		}
		return m
	}())
	if err != nil {
		return err
	}
	indexData := page("", "AUR Report Card",
		fmt.Sprintf("Static security and hygiene grades for %d AUR PKGBUILDs, generated by pkglint without ever sourcing them.", len(results)),
		map[string]any{
			"Results":   head,
			"Shown":     len(head),
			"Sort":      "votes", // run() ordered them
			"Corpus":    corpus,
			"Repos":     repos,
			"RepoJSON":  template.JS(repoJSON),
			"Total":     len(results),
			"Generated": time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		})
	if err := renderTo(tmpl, "index.html", filepath.Join(out, "index.html"), indexData); err != nil {
		return err
	}

	if err := writeRoster(out, results); err != nil {
		return err
	}
	if err := renderShards(tmpl, out, page, results); err != nil {
		return err
	}
	if err := writeSitemap(out, results, registry); err != nil {
		return err
	}

	for i, r := range registry {
		data := page("rules/"+r.ID+".html",
			fmt.Sprintf("%s: %s — AUR Report Card", r.ID, r.Name),
			clip(fmt.Sprintf("%s (%s): %s", r.ID, r.Name, r.Doc), 200),
			map[string]any{"Rule": r})
		if i > 0 {
			data["Prev"] = registry[i-1]
		}
		if i < len(registry)-1 {
			data["Next"] = registry[i+1]
		}
		if err := renderTo(tmpl, "rule.html", filepath.Join(out, "rules", r.ID+".html"), data); err != nil {
			return err
		}
	}

	rulesData := page("rules/", "Rule reference — AUR Report Card",
		"Every check pkglint runs on a PKGBUILD, grouped by concern, with a flagged and preferred example for each.",
		map[string]any{
			"Groups":         groupRules(registry),
			"FixSafe":        rules.FixSafe,
			"FixUnsafe":      rules.FixUnsafe,
			"WidestName":     widestName(registry),
			"WidestSeverity": widestSeverity(registry),
		})
	if err := renderTo(tmpl, "rulesindex.html", filepath.Join(out, "rules", "index.html"), rulesData); err != nil {
		return err
	}

	for _, r := range results {
		// selectSeed already filtered these; re-check at the write site so a
		// name arriving by any other route still cannot escape out/package.
		if !safeBase(r.Name) {
			log.Printf("skipping page for unsafe name %q", r.Name)
			continue
		}
		data := page("package/"+r.Name+".html",
			fmt.Sprintf("%s — AUR Report Card", r.Name),
			fmt.Sprintf("pkglint grade %s for %s/%s: %d finding(s) from static analysis.", r.Grade, r.Repo, r.Name, len(r.Findings)),
			map[string]any{"R": r, "Rules": ruleIndex, "Files": groupFindings(r.Findings)})
		if err := renderTo(tmpl, "package.html", filepath.Join(out, "package", r.Name+".html"), data); err != nil {
			return err
		}
	}
	return nil
}

// copyAssets writes the embedded stylesheet, script, and fonts under out and
// returns a short content hash of the two text assets, used to bust caches.
func copyAssets(out string) (string, error) {
	sum := sha256.New()
	err := fs.WalkDir(assetFS, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(out, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := assetFS.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, ".css") || strings.HasSuffix(p, ".js") {
			sum.Write(b)
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil))[:8], nil
}

// bands turns the grade counts into the segments of the distribution bar, in
// grade order and skipping grades nothing landed on.
func bands(counts map[string]int, total int) []band {
	var out []band
	for i, g := range []string{"A", "B", "C", "D", "F", "?"} {
		n := counts[g]
		if n == 0 {
			continue
		}
		out = append(out, band{
			Grade:  g,
			Count:  n,
			Rubric: rubric[g],
			Style:  template.CSS(fmt.Sprintf("flex-grow:%d;animation-delay:%dms", n, i*70)),
		})
	}
	return out
}

// ticks lays out a drafting scale under the distribution bar: round marks at a
// readable interval, plus the total at the right edge.
func ticks(total int) []tick {
	if total == 0 {
		return nil
	}
	step := 1
	for _, s := range []int{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000} {
		step = s
		if total/s <= 6 {
			break
		}
	}
	var out []tick
	mark := func(n int) {
		out = append(out, tick{
			Label: fmt.Sprint(n),
			Style: template.CSS(fmt.Sprintf("left:%.4f%%", float64(n)/float64(total)*100)),
		})
	}
	for n := 0; n < total; n += step {
		// Drop a mark that would collide with the total at the right edge.
		if total-n < step/2 {
			break
		}
		mark(n)
	}
	mark(total)
	return out
}

// groupFindings buckets a package's findings by file, preserving the order
// each file and finding first appeared in.
func groupFindings(findings []rules.Finding) []fileGroup {
	var out []fileGroup
	at := map[string]int{}
	for _, f := range findings {
		i, ok := at[f.Path]
		if !ok {
			i = len(out)
			at[f.Path] = i
			out = append(out, fileGroup{Path: f.Path})
		}
		out[i].Findings = append(out[i].Findings, f)
	}
	return out
}

// ruleGroup is a titled section of the rule reference. Code is the PB-hundreds
// prefix, shown in the page margin; Title names what the group protects.
type ruleGroup struct {
	Code  string
	Title string
	Rules []rules.Rule
}

// groupRules buckets the registry by the PB-hundreds prefix (PB1xx, PB2xx, …),
// in ID order. Registry() already returns rules sorted by ID.
func groupRules(all []rules.Rule) []ruleGroup {
	titles := []struct {
		prefix string
		title  string
	}{
		{"PB1", "Integrity & provenance"},
		{"PB2", "Hermeticity"},
		{"PB3", "Execution & obfuscation"},
		{"PB4", "Filesystem & privilege"},
		{"PB5", "Install scriptlets"},
		{"PB6", "Metadata consistency"},
		{"PB7", "Correctness & metadata"},
		{"PB8", "Built package"},
		{"PB9", "Style & conventions"},
	}
	var groups []ruleGroup
	for _, t := range titles {
		g := ruleGroup{Code: t.prefix + "xx", Title: t.title}
		for _, r := range all {
			if strings.HasPrefix(r.ID, t.prefix) {
				g.Rules = append(g.Rules, r)
			}
		}
		if len(g.Rules) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}

// widestName returns the longest rule name in the registry. The rule reference
// renders it invisibly in every table's Name header so the column is the same
// width in each category, not the width of that category's longest name.
func widestName(all []rules.Rule) string {
	var w string
	for _, r := range all {
		if len(r.Name) > len(w) {
			w = r.Name
		}
	}
	return w
}

// widestSeverity returns the registry's widest severity range, measured in
// label characters. It plays the same trick as widestName: the rule reference
// renders it invisibly in every table's Severity header so the column holds one
// width across all seven categories, including the ones where no rule
// escalates and the cells are a single badge. A range always renders two badges
// and so always beats a fixed severity, which makes the character count a fair
// proxy for the rendered width within each group.
func widestSeverity(all []rules.Rule) rules.SeverityRange {
	width := func(s rules.SeverityRange) int {
		n := len(s.Low.String())
		if s.Varies() {
			n += len("–") + len(s.High.String())
		}
		return n
	}
	var w rules.SeverityRange
	for _, r := range all {
		if s := r.Severities(); width(s) > width(w) {
			w = s
		}
	}
	return w
}

func renderTo(tmpl *template.Template, name, path string, data any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := tmpl.ExecuteTemplate(f, name, data); err != nil {
		f.Close()
		return fmt.Errorf("render %s: %w", name, err)
	}
	// Closed explicitly, not deferred: a write that fails at flush time would
	// otherwise publish a truncated page as a success.
	return f.Close()
}
