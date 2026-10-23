package main

// The nightly run grades the corpus; the poll keeps the feed current between
// runs. Every half hour it reads the AUR's list of recently modified
// packages, resolves them with a single RPC call, scans the bases that moved
// since the state last saw them, and folds what they introduced into
// feed.xml. It renders nothing else: report cards, badges and the roster
// catch up at the next nightly run.
//
// It shares the nightly run's state and feed history, so the two never
// disagree about what is new: a base the poll scanned is reused by the night,
// and a base the poll missed — the list is only the last hundred changes —
// is fed by the night with the same entry ID it would have had.

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"maps"
	neturl "net/url"
	"os"
	"time"
)

// The AUR's recently-modified list and its RPC. Vars so tests can point them
// at a local server.
var (
	aurModifiedURL = "https://aur.archlinux.org/rss/modified"
	aurInfoURL     = "https://aur.archlinux.org/rpc/v5/info"
)

// maxPollBody bounds the RSS document and the RPC answer, each a few tens of
// kilobytes in practice.
const maxPollBody = 4 << 20

// recentAURNames returns the package names on the AUR's recently-modified
// list, newest first, without repeats. Only titles are read: everything else
// a package needs comes from the RPC, which is authoritative for it.
func recentAURNames() ([]string, error) {
	resp, err := get(aurModifiedURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var doc struct {
		Items []struct {
			Title string `xml:"title"`
		} `xml:"channel>item"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, maxPollBody)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("GET %s: %w", aurModifiedURL, err)
	}
	var names []string
	seen := map[string]bool{}
	for _, it := range doc.Items {
		if !safeBase(it.Title) {
			log.Printf("poll: skipping unsafe package name %q", it.Title)
			continue
		}
		if !seen[it.Title] {
			seen[it.Title] = true
			names = append(names, it.Title)
		}
	}
	return names, nil
}

// aurInfo resolves package names to their metadata in one RPC request — the
// list holds at most a hundred, well inside what one request may name.
func aurInfo(names []string) ([]metaPackage, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var body struct {
		Type    string        `json:"type"`
		Error   string        `json:"error"`
		Results []metaPackage `json:"results"`
	}
	if err := getJSON(aurInfoURL+"?"+neturl.Values{"arg[]": names}.Encode(), &body); err != nil {
		return nil, err
	}
	if body.Type == "error" {
		return nil, fmt.Errorf("AUR RPC: %s", body.Error)
	}
	return body.Results, nil
}

// pollAUR is the half-hourly run: scan what the AUR changed since the state
// last looked, update the feed history at feedPath and out/feed.xml, and save
// the state. A base whose record is from an official repository is left
// alone, and so is one on the official list at officialPath that the state
// has not reached yet — the official package shadows it, as in selectSeed.
func pollAUR(out, cache, statePath, feedPath, officialPath string) error {
	for _, dir := range []string{out, cache} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	names, err := recentAURNames()
	if err != nil {
		return err
	}
	metas, err := aurInfo(names)
	if err != nil {
		return err
	}
	prev, err := loadState(statePath)
	if err != nil {
		return fmt.Errorf("load scan state: %w", err)
	}
	official, err := loadOfficialBases(officialPath)
	if err != nil {
		return err
	}

	next := maps.Clone(prev)
	var results []siteResult
	seen := map[string]bool{}
	var current, failed int
	for _, m := range metas {
		m.Repo = ""
		if !safeBase(m.PackageBase) {
			log.Printf("poll: skipping package base with unsafe name %q", m.PackageBase)
			continue
		}
		// A split package's siblings share one base, and one scan.
		if seen[m.PackageBase] {
			continue
		}
		seen[m.PackageBase] = true
		if official[m.PackageBase] {
			current++
			continue
		}
		if old, ok := prev[m.PackageBase]; ok && (old.repo() != aurRepo || old.LastModified == m.LastModified) {
			current++
			continue
		}
		res, st, err := scanOne(m, cache, prev)
		if err != nil {
			// Not persisted: the nightly run, or the next poll, retries it.
			log.Printf("poll: %s: %v", m.PackageBase, err)
			failed++
		}
		if st != nil {
			next[m.PackageBase] = *st
			results = append(results, res)
		}
	}
	log.Printf("poll: %d listed, %d bases scanned (%d failed), %d already current",
		len(names), len(results), failed, current)

	if err := updateFeed(out, feedPath, results, prev, next, time.Now()); err != nil {
		return err
	}
	return saveState(statePath, next)
}
