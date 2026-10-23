package main

// The feed is the site's answer to "what just landed that someone should
// read?": an Atom entry for each package base whose latest change introduced
// a security finding. The package pages already say all of this, but only to
// someone who goes looking; the feed is for triaging uploads as they arrive.
//
// An entry is keyed on the change, not the finding. A base emits only when
// its PKGBUILD moved — its LastModified differs from the prior record's — or
// when it is first seen and was modified within feedFirstSeenWindow. Re-lints
// a registry change forces emit nothing: a new rule's hits across the corpus
// are not uploads, and the rule's page already lists them. The corpus filling
// in from empty emits nothing either, for the same reason.
//
// What a triager has read and judged harmless goes in feedignore.txt, so the
// next upload of that base does not raise it again.

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jmelahman/pkglint/internal/rules"
)

// feedErrorRules are the error-severity rules whose findings are security
// signals rather than correctness ones. Every critical finding is fed; errors
// only from these, because most errors — PB70x's malformed fields, PB101's
// SKIP on every VCS source — would bury the handful worth reading.
// TestFeedErrorRulesExist holds each ID to the registry.
var feedErrorRules = map[string]bool{
	"PB104": true, // insecure-transport
	"PB116": true, // committed-binary
	"PB302": true, // eval
	"PB306": true, // unresolvable-command (${!indirection})
	"PB309": true, // hidden-unicode
	"PB310": true, // anonymized-fetch
	"PB402": true, // privilege-escalation
	"PB503": true, // unparseable-scriptlet
}

const (
	// feedFirstSeenWindow is how recently a base never scanned before must
	// have changed to be fed. It separates a fresh upload from the backlog
	// the corpus is still filling in, and is wider than a day so a run that
	// misses a few hours still catches the uploads in them.
	feedFirstSeenWindow = 48 * time.Hour
	// feedRetention and feedMaxEntries bound the history kept in feed.jsonl
	// and rendered into feed.xml.
	feedRetention  = 30 * 24 * time.Hour
	feedMaxEntries = 500
)

// feedEntry is one change to one package base that introduced security
// findings. It is persisted in feed.jsonl, so an entry keeps the time it was
// first seen across re-renders.
type feedEntry struct {
	ID   string `json:"id"`
	Base string `json:"base"`
	// Name is the package the base's report card is filed under, which for a
	// split package need not be the base. Blank in entries written before it
	// was recorded; see result.
	Name         string          `json:"name,omitempty"`
	Repo         string          `json:"repo"`
	Version      string          `json:"version"`
	LastModified int64           `json:"last_modified"`
	Seen         time.Time       `json:"seen"`
	Findings     []rules.Finding `json:"findings,omitempty"`
	Drift        []string        `json:"drift,omitempty"`
	// Err is why the scanner refused the snapshot. A PKGBUILD pkglint cannot
	// analyse is exactly what an upload hiding from triage would look like,
	// so a refusal is fed like a finding.
	Err string `json:"error,omitempty"`
}

// result is the entry as the siteResult its links are built from.
func (e feedEntry) result() siteResult {
	name := e.Name
	if name == "" {
		name = e.Base
	}
	return siteResult{Name: name, Base: e.Base, Repo: e.Repo}
}

// feedWorthy reports whether a finding belongs in the feed.
func feedWorthy(f rules.Finding) bool {
	return f.Severity == rules.Critical || (f.Severity == rules.Error && feedErrorRules[f.RuleID])
}

// findingKey identifies a finding across scans of the same base. The line is
// left out: an edit above a finding moves it without making it new. Keys are
// counted, not just collected — many rules word every hit the same way, so a
// second eval in a file that already had one shares the first one's key and
// is told apart only by there now being two.
func findingKey(f rules.Finding) string {
	return f.RuleID + "\x00" + f.Path + "\x00" + f.Message
}

// priorFindings is a base's previous findings by key, each key holding the
// severities it was reported at.
type priorFindings map[string][]rules.Severity

func newPriorFindings(fs []rules.Finding) priorFindings {
	p := priorFindings{}
	for _, f := range fs {
		k := findingKey(f)
		p[k] = append(p[k], f.Severity)
	}
	return p
}

// claim reports whether f was already known, consuming the prior finding that
// accounts for it. Severity counts: a rule with a range can report the same
// words at a worse level, and a hit that climbs from warning to critical is
// new news. The least severe prior at or above f's level is the one used up,
// leaving the worse ones for findings that need them.
func (p priorFindings) claim(f rules.Finding) bool {
	k := findingKey(f)
	best := -1
	for i, s := range p[k] {
		if s >= f.Severity && (best < 0 || s < p[k][best]) {
			best = i
		}
	}
	if best < 0 {
		return false
	}
	p[k] = append(p[k][:best], p[k][best+1:]...)
	return true
}

//go:embed feedignore.txt
var feedIgnoreText string

// feedIgnore is feedignore.txt parsed: base → rule → the worst severity a
// triager has judged harmless there.
var feedIgnore = mustParseFeedIgnore(feedIgnoreText)

// parseFeedIgnore reads the ignorelist: one "base rule severity reason" per
// line, # comments and blank lines skipped. Bases are exact names, never
// patterns — a pattern would let anyone upload a new base into its reach.
// The severity caps what is waived: a finding worse than the one triaged is
// fed, so a base whose harmless eval turns into an eval of a download is
// raised again.
func parseFeedIgnore(text string) (map[string]map[string]rules.Severity, error) {
	sevs := map[string]rules.Severity{}
	for _, s := range []rules.Severity{rules.Info, rules.Warn, rules.Error, rules.Critical} {
		sevs[s.String()] = s
	}
	out := map[string]map[string]rules.Severity{}
	for n, line := range strings.Split(text, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 4 {
			return nil, fmt.Errorf("feedignore.txt:%d: want \"base rule severity reason\"", n+1)
		}
		base, rule, sevName := fields[0], fields[1], fields[2]
		if !safeBase(base) {
			return nil, fmt.Errorf("feedignore.txt:%d: %q is not a package base", n+1, base)
		}
		sev, ok := sevs[sevName]
		if !ok {
			return nil, fmt.Errorf("feedignore.txt:%d: unknown severity %q", n+1, sevName)
		}
		if out[base] == nil {
			out[base] = map[string]rules.Severity{}
		}
		out[base][rule] = sev
	}
	return out, nil
}

func mustParseFeedIgnore(text string) map[string]map[string]rules.Severity {
	m, err := parseFeedIgnore(text)
	if err != nil {
		panic(err)
	}
	return m
}

// ignored reports whether a triager has waived f for base.
func ignored(base string, f rules.Finding) bool {
	sev, ok := feedIgnore[base][f.RuleID]
	return ok && f.Severity <= sev
}

// feedEntries returns this run's new entries: one per result whose base was
// re-scanned after a PKGBUILD change and came back refused, or with
// feed-worthy findings or provenance drift that its prior record did not
// have.
//
// A prior record linted under another rule registry is a weaker baseline:
// a rule added or re-tiered since never had the chance to report on it, so
// every one of its hits would look new and a release would flood the feed
// through whichever bases happened to be uploaded before their re-lint. Only
// critical findings are fed against such a baseline — the ones not worth
// missing on account of the noise.
func feedEntries(results []siteResult, prev, next map[string]stateRecord, now time.Time) []feedEntry {
	var out []feedEntry
	for _, r := range results {
		rec, ok := next[r.Base]
		if !ok || rec.repo() != r.Repo {
			continue
		}
		old, had := prev[r.Base]
		had = had && old.repo() == r.Repo
		switch {
		case had && rec.LastModified == old.LastModified:
			continue // unchanged, over budget, failed, or a rule-only re-lint
		case !had && now.Sub(time.Unix(rec.LastModified, 0)) > feedFirstSeenWindow:
			continue // the backlog, not an upload
		}
		stale := had && old.Rules != rulesFingerprint()
		before := newPriorFindings(old.Findings)
		var fresh []rules.Finding
		for _, f := range rec.Findings {
			if before.claim(f) || !feedWorthy(f) || ignored(r.Base, f) {
				continue
			}
			if stale && f.Severity < rules.Critical {
				continue
			}
			fresh = append(fresh, f)
		}
		if len(fresh) == 0 && len(rec.Drift) == 0 && rec.Err == "" {
			continue
		}
		out = append(out, feedEntry{
			ID:           fmt.Sprintf("tag:jamison.lahman.dev,2026:pkglint/%s/%s/%d", r.Repo, r.Base, rec.LastModified),
			Base:         r.Base,
			Name:         r.Name,
			Repo:         r.Repo,
			Version:      r.Version,
			LastModified: rec.LastModified,
			Seen:         now.UTC(),
			Findings:     fresh,
			Drift:        rec.Drift,
			Err:          rec.Err,
		})
	}
	return out
}

// mergeFeed adds this run's entries to the history and trims it: entries
// older than feedRetention go, then all but the newest feedMaxEntries. An ID
// already in the history keeps its first sighting: the feed is saved before
// the state, so a run that dies between the two re-emits its entries next
// time, and they must not come back re-dated.
func mergeFeed(history, added []feedEntry, now time.Time) []feedEntry {
	byID := make(map[string]bool, len(history))
	all := make([]feedEntry, 0, len(history)+len(added))
	for _, e := range append(history, added...) {
		if byID[e.ID] || now.Sub(e.Seen) > feedRetention {
			continue
		}
		byID[e.ID] = true
		all = append(all, e)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].Seen.Equal(all[j].Seen) {
			return all[i].Seen.After(all[j].Seen)
		}
		return all[i].ID < all[j].ID
	})
	if len(all) > feedMaxEntries {
		all = all[:feedMaxEntries]
	}
	return all
}

// loadFeed reads the feed history. As with loadState, a missing file is the
// first run and a malformed one is an error, not an empty history.
func loadFeed(path string) ([]feedEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []feedEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), maxStateLine)
	for line := 1; sc.Scan(); line++ {
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		var e feedEntry
		if err := json.Unmarshal(b, &e); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if !safeBase(e.Base) {
			return nil, fmt.Errorf("%s:%d: unsafe package base %q", path, line, e.Base)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return out, nil
}

// saveFeed writes the history one entry per line, newest first, through a
// temporary file as saveState does.
func saveFeed(path string, entries []feedEntry) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	Sub     string      `xml:"subtitle"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Author  atomPerson  `xml:"author"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomPerson struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Rel   string `xml:"rel,attr,omitempty"`
	Type  string `xml:"type,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
	Href  string `xml:"href,attr"`
}

type atomText struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

type atomEntry struct {
	Title      atomText       `xml:"title"`
	ID         string         `xml:"id"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Links      []atomLink     `xml:"link"`
	Categories []atomCategory `xml:"category"`
	Content    atomText       `xml:"content"`
}

// visible spells out format characters — bidi overrides, zero-widths — as
// \uXXXX. Messages carry text out of the PKGBUILD, and a feed reader would
// otherwise apply a PB309 finding's own bidi override to the line reporting
// it. Characters XML cannot carry at all are encoding/xml's to replace.
func visible(s string) string {
	if strings.IndexFunc(s, func(r rune) bool { return unicode.Is(unicode.Cf, r) }) < 0 {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Cf, r) {
			fmt.Fprintf(&b, `\u%04x`, r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// entryTitle summarises an entry as "base version (repo): 2 critical, 1 drift".
func entryTitle(e feedEntry) string {
	counts := map[rules.Severity]int{}
	for _, f := range e.Findings {
		counts[f.Severity]++
	}
	var parts []string
	for _, s := range []rules.Severity{rules.Critical, rules.Error} {
		if n := counts[s]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, s))
		}
	}
	if n := len(e.Drift); n > 0 {
		parts = append(parts, fmt.Sprintf("%d drift", n))
	}
	if e.Err != "" {
		parts = append([]string{"refused"}, parts...)
	}
	return fmt.Sprintf("%s %s (%s): %s", e.Base, e.Version, e.Repo, strings.Join(parts, ", "))
}

// writeAtom renders the history as an Atom feed. Every piece of PKGBUILD text
// goes out as type="text" content — never HTML — so a reader has nothing of
// the package's to interpret.
//
// The document is a function of the history alone, with no clock in it: the
// poll publishes only when feed.xml or the state changed, and a timestamp
// would change it every time. An empty feed is dated the epoch.
func writeAtom(path string, entries []feedEntry) error {
	updated := time.Unix(0, 0).UTC()
	if len(entries) > 0 {
		updated = entries[0].Seen
	}
	feed := atomFeed{
		Title:   "pkglint: new security findings",
		Sub:     "Arch package bases whose latest change introduced a critical finding, a security error, or provenance drift.",
		ID:      baseURL + "feed.xml",
		Updated: updated.Format(time.RFC3339),
		Author:  atomPerson{Name: "pkglint"},
		Links: []atomLink{
			{Rel: "self", Type: "application/atom+xml", Href: baseURL + "feed.xml"},
			{Rel: "alternate", Type: "text/html", Href: baseURL},
		},
	}
	for _, e := range entries {
		// The package's own page is the alternate because it always exists.
		// The report card may not yet: the poll feeds a base the hour it is
		// uploaded, and a base new to the site gets its card from the next
		// nightly render.
		r := e.result()
		ae := atomEntry{
			Title:     atomText{Type: "text", Body: visible(entryTitle(e))},
			ID:        e.ID,
			Published: e.Seen.Format(time.RFC3339),
			Updated:   e.Seen.Format(time.RFC3339),
			Links: []atomLink{
				{Rel: "alternate", Type: "text/html", Title: "package", Href: r.PageURL()},
				{Rel: "related", Title: "PKGBUILD", Href: r.SourceURL()},
				{Rel: "related", Type: "text/html", Title: "report card", Href: r.ReportURL()},
			},
			Categories: []atomCategory{{Term: e.Repo}},
		}
		var body strings.Builder
		if e.Err != "" {
			fmt.Fprintf(&body, "refused %s\n", e.Err)
			ae.Categories = append(ae.Categories, atomCategory{Term: "refused"})
		}
		ruleSeen := map[string]bool{}
		for _, f := range e.Findings {
			fmt.Fprintf(&body, "%s %s %s:%d %s\n", f.Severity, f.RuleID, f.Path, f.Line, f.Message)
			if !ruleSeen[f.RuleID] {
				ruleSeen[f.RuleID] = true
				ae.Links = append(ae.Links, atomLink{Rel: "related", Title: f.RuleID, Href: baseURL + "rules/" + f.RuleID + ".html"})
				ae.Categories = append(ae.Categories, atomCategory{Term: f.RuleID})
			}
		}
		for _, d := range e.Drift {
			fmt.Fprintf(&body, "drift %s\n", d)
		}
		if len(e.Drift) > 0 {
			ae.Categories = append(ae.Categories, atomCategory{Term: "drift"})
		}
		ae.Content = atomText{Type: "text", Body: visible(body.String())}
		feed.Entries = append(feed.Entries, ae)
	}

	b, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(xml.Header), append(b, '\n')...), 0o644)
}

// updateFeed folds this run's entries into the history at feedPath and
// renders the result to out/feed.xml.
func updateFeed(out, feedPath string, results []siteResult, prev, next map[string]stateRecord, now time.Time) error {
	history, err := loadFeed(feedPath)
	if err != nil {
		return fmt.Errorf("load feed: %w", err)
	}
	added := feedEntries(results, prev, next, now)
	entries := mergeFeed(history, added, now)
	if err := saveFeed(feedPath, entries); err != nil {
		return err
	}
	return writeAtom(filepath.Join(out, "feed.xml"), entries)
}
