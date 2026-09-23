package main

// Reaching the whole corpus. The index server-renders only its head, so the
// rest of the packages need routes of their own: roster.json for the in-page
// search, alphabetical shards for people and crawlers following links, and a
// sitemap for crawlers that would rather be told.

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmelahman/pkglint/internal/rules"
)

// writeRoster emits the compact payload site.js searches over. It carries one
// array per package rather than an object, and only the fields the roster
// shows or filters on: spelling out keys 47,000 times costs more than the data.
// The maintainer and co-maintainers are here despite having no column, because
// the "@" query filters on them and would otherwise reach only the
// server-rendered head.
//
// Co-maintainers travel as one space-joined string, matching the row's
// data-comaintainers attribute — AUR usernames cannot hold a space, and one
// string is what site.js folds into its haystacks either way. The repository,
// the packager and an official package's maintainers follow: the repository
// has a column and a filter of its own, and the other two are an official
// package's answer to the "@" query, the maintainers space-joined like the
// co-maintainers — archweb usernames cannot hold a space either.
//
// Order is the index's order, so the client can render straight from it
// without re-sorting to reproduce the default view.
func writeRoster(out string, results []siteResult) error {
	rows := make([][]any, 0, len(results))
	for _, r := range results {
		drift := 0
		if len(r.Drift) > 0 {
			drift = 1
		}
		rows = append(rows, []any{r.Name, r.Grade, len(r.Findings), r.Votes, r.Description, r.Maintainer, drift, strings.Join(r.CoMaintainers, " "), r.Repo, r.Packager, strings.Join(r.Maintainers, " ")})
	}
	f, err := os.Create(filepath.Join(out, "roster.json"))
	if err != nil {
		return err
	}
	// Compact, unindented: this file is read by a program, and indenting it
	// would add a megabyte of whitespace to every visitor's download.
	if err := json.NewEncoder(f).Encode(rows); err != nil {
		f.Close()
		return err
	}
	// Closed explicitly, not deferred: a write that fails at flush time would
	// otherwise publish a truncated roster as a success.
	return f.Close()
}

// shardKey buckets a package by its first character: a single letter, or
// "0-9" for the digits. safeBase guarantees a leading ASCII alphanumeric, so
// there is always exactly one bucket and it is always a safe path component.
func shardKey(name string) string {
	c := name[0]
	if c >= '0' && c <= '9' {
		return "0-9"
	}
	return strings.ToLower(string(c))
}

// shard is one alphabetical page of the roster.
type shard struct {
	Key     string
	Results []siteResult
}

// groupShards buckets every result by first character and sorts each bucket by
// name. Alphabetical rather than by grade or votes deliberately: those move
// nightly, so a package would migrate between pages and its position would
// churn the committed diff, where a name is fixed for the life of the package.
func groupShards(results []siteResult) []shard {
	byKey := map[string][]siteResult{}
	for _, r := range results {
		if !safeBase(r.Name) {
			continue // already logged at the page-writing site
		}
		k := shardKey(r.Name)
		byKey[k] = append(byKey[k], r)
	}
	shards := make([]shard, 0, len(byKey))
	for k, rs := range byKey {
		sort.Slice(rs, func(i, j int) bool { return rs[i].Name < rs[j].Name })
		shards = append(shards, shard{Key: k, Results: rs})
	}
	sort.Slice(shards, func(i, j int) bool { return shards[i].Key < shards[j].Key })
	return shards
}

// pageFunc builds the per-page template data; renderShards takes it from
// renderSite rather than rebuilding the closure over the asset version.
type pageFunc func(path, title, desc string, data map[string]any) map[string]any

// renderShards writes roster/<key>.html for every bucket plus roster/index.html
// listing them. Without these the packages outside the index's head would have
// no inbound link anywhere on the site: not browsable without JavaScript, and
// never crawled.
func renderShards(tmpl *template.Template, out string, page pageFunc, results []siteResult) error {
	if err := os.MkdirAll(filepath.Join(out, "roster"), 0o755); err != nil {
		return err
	}
	shards := groupShards(results)
	keys := make([]string, 0, len(shards))
	for _, s := range shards {
		keys = append(keys, s.Key)
	}

	for i, s := range shards {
		data := page("roster/"+s.Key+".html",
			fmt.Sprintf("Packages: %s — AUR Report Card", s.Key),
			fmt.Sprintf("Every Arch Linux package base beginning with %q graded by pkglint: %d packages.", s.Key, len(s.Results)),
			map[string]any{
				"Results": s.Results,
				"Shown":   len(s.Results),
				"Total":   len(s.Results),
				"Key":     s.Key,
				"Keys":    keys,
				"Sort":    "name", // groupShards sorted them
			})
		if i > 0 {
			data["Prev"] = shards[i-1].Key
		}
		if i < len(shards)-1 {
			data["Next"] = shards[i+1].Key
		}
		if err := renderTo(tmpl, "shard.html", filepath.Join(out, "roster", s.Key+".html"), data); err != nil {
			return err
		}
	}

	index := page("roster/", "All packages — AUR Report Card",
		fmt.Sprintf("Every one of the %d Arch Linux package bases pkglint grades, listed alphabetically.", len(results)),
		// Key is empty rather than absent: shardnav compares it against every
		// letter, and a missing map key would compare a string against nil.
		map[string]any{"Shards": shards, "Keys": keys, "Total": len(results), "Key": ""})
	return renderTo(tmpl, "shardindex.html", filepath.Join(out, "roster", "index.html"), index)
}

// sitemapChunk is how many URLs go in one sitemap file. The sitemap protocol
// caps a single file at 50,000, which the corpus is already close enough to
// that a year's growth would silently truncate it, so the URLs are split and
// fronted by a sitemap index from the start.
const sitemapChunk = 25000

// writeSitemap lists every generated page, so a crawler reaches the whole
// corpus without having to walk the shards.
func writeSitemap(out string, results []siteResult, registry []rules.Rule) error {
	urls := make([]string, 0, len(results)+len(registry)+32)
	urls = append(urls, baseURL, baseURL+"roster/", baseURL+"rules/")
	for _, s := range groupShards(results) {
		urls = append(urls, baseURL+"roster/"+s.Key+".html")
	}
	for _, r := range registry {
		urls = append(urls, baseURL+"rules/"+r.ID+".html")
	}
	for _, r := range results {
		if !safeBase(r.Name) {
			continue
		}
		urls = append(urls, baseURL+"package/"+r.Name+".html")
	}

	var files []string
	for i := 0; i < len(urls); i += sitemapChunk {
		end := min(i+sitemapChunk, len(urls))
		name := fmt.Sprintf("sitemap-%d.xml", i/sitemapChunk+1)
		if err := writeURLSet(filepath.Join(out, name), urls[i:end]); err != nil {
			return err
		}
		files = append(files, name)
	}
	return writeSitemapIndex(filepath.Join(out, "sitemap.xml"), files)
}

func writeURLSet(path string, urls []string) error {
	return writeXML(path, func(w io.Writer) error {
		fmt.Fprint(w, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+"\n")
		for _, u := range urls {
			fmt.Fprint(w, "<url><loc>")
			if err := xml.EscapeText(w, []byte(u)); err != nil {
				return err
			}
			fmt.Fprint(w, "</loc></url>\n")
		}
		_, err := fmt.Fprint(w, "</urlset>\n")
		return err
	})
}

func writeSitemapIndex(path string, files []string) error {
	return writeXML(path, func(w io.Writer) error {
		fmt.Fprint(w, `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+"\n")
		for _, f := range files {
			fmt.Fprint(w, "<sitemap><loc>")
			if err := xml.EscapeText(w, []byte(baseURL+f)); err != nil {
				return err
			}
			fmt.Fprint(w, "</loc></sitemap>\n")
		}
		_, err := fmt.Fprint(w, "</sitemapindex>\n")
		return err
	})
}

func writeXML(path string, body func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(f, xml.Header); err != nil {
		f.Close()
		return err
	}
	if err := body(f); err != nil {
		f.Close()
		return err
	}
	// Closed explicitly, not deferred: a write that fails at flush time would
	// otherwise publish a truncated document as a success.
	return f.Close()
}
