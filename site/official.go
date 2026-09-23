package main

// The official repositories — core, extra, multilib — are the second source of
// PKGBUILDs beside the AUR. They have no metadata dump, but they have
// something as good: the pacman sync database every mirror serves, one desc
// entry per package naming its pkgbase, its version, its build date, and who
// built it. That is the index. The PKGBUILD itself lives on the package's
// GitLab project, where devtools tags every release with the version it was
// built from, so the tag's archive is the tree the mirror's package came out
// of — fetched once per base and cached by build date, exactly as an AUR
// snapshot is cached by LastModified.
//
// Nothing here changes what happens to a PKGBUILD once it is on disk: it is
// parsed, never sourced, whichever repository it came from.

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// aurRepo names the AUR wherever a repository name is expected. It is the
// zero value's meaning too: metadata, results and state records that predate
// the official repositories carry no repository, and they are all AUR.
const aurRepo = "aur"

// gitlabPackages is the namespace every official package's repository lives
// under. Package pages link to it, and the tag archives come out of it.
const gitlabPackages = "https://gitlab.archlinux.org/archlinux/packaging/packages/"

// Vars, not consts, so tests can point them at a local server.
var (
	// syncDBURL locates a repository's sync database, arguments repo and repo
	// again. geo.mirror.pkgbuild.com is the project's own geo-routed mirror;
	// the x86_64 database also lists every "any" package, so it is the whole
	// repository.
	syncDBURL = "https://geo.mirror.pkgbuild.com/%s/os/x86_64/%s.db"

	// officialSnapshotURL is a release tag's archive: project, tag, project,
	// tag. A project that does not exist answers this with a 200 and a sign-in
	// page rather than a 404, which is why downloadOnce refuses HTML.
	officialSnapshotURL = gitlabPackages + "%s/-/archive/%s/%s-%s.tar.gz"
)

// repoRe is the shape of a repository name as pacman spells them. The name
// becomes a cache filename, a URL segment and a CSS class, so it is validated
// at the flag rather than trusted at each of those.
var repoRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// parseRepos reads the -repos flag: a comma-separated list of official
// repositories, in the order they should fill in. Empty means none.
func parseRepos(s string) ([]string, error) {
	var repos []string
	for _, r := range strings.Split(s, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if r == aurRepo || !repoRe.MatchString(r) {
			return nil, fmt.Errorf("invalid repository name %q", r)
		}
		if !slices.Contains(repos, r) {
			repos = append(repos, r)
		}
	}
	return repos, nil
}

// repoOrder ranks the repositories for display: the base system first, the AUR
// last. Anything not named here — an unusual -repos value — sits between them.
var repoOrder = []string{"core", "extra", "multilib"}

// repoRank orders repositories for sorting. site.js carries the same ranking
// for the rows it sorts client-side.
func repoRank(repo string) int {
	if repo == aurRepo {
		return len(repoOrder) + 1
	}
	if i := slices.Index(repoOrder, repo); i >= 0 {
		return i
	}
	return len(repoOrder)
}

// loadOfficial downloads (or reuses a same-day cached copy of) each
// repository's sync database and returns one metaPackage per package base.
func loadOfficial(cache string, repos []string) ([]metaPackage, error) {
	var all []metaPackage
	for _, repo := range repos {
		p := filepath.Join(cache, repo+".db")
		if !fresh(p) {
			if err := download(fmt.Sprintf(syncDBURL, repo, repo), p); err != nil {
				return nil, fmt.Errorf("download %s sync database: %w", repo, err)
			}
		}
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		pkgs, err := parseSyncDB(f, repo)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		log.Printf("%s: %d package bases", repo, len(pkgs))
		all = append(all, pkgs...)
	}
	if len(all) == 0 {
		return all, nil
	}
	maintainers, err := loadMaintainers(cache)
	if err != nil {
		return nil, err
	}
	for i := range all {
		all[i].Maintainers = maintainers[all[i].PackageBase]
	}
	return all, nil
}

// The sync database says who built a package but not who maintains it. That
// relation lives on archlinux.org, whose package search answers in JSON: 250
// packages a page, each with its maintainers by archweb username. The whole
// of archlinux.org is some 64 pages — a minute at the throttle — fetched once
// a day and folded to one entry per pkgbase, which is the level the relation
// is kept at: every package of a base names the same people.
//
// The query names no repository. archweb spells them in title case and
// rejects the pacman spelling ("repo=core" is an invalid form, answered with
// no results rather than an error), and a base's maintainers do not depend on
// which repository carries it, so a filter would save two pages at the price
// of guessing how archweb spells "KDE-Unstable". Page is the one argument.
var archwebSearchURL = "https://archlinux.org/packages/search/json/?limit=250&page=%d"

// maxArchwebPages bounds the paging. archlinux.org says how many pages it
// has and fetchMaintainers believes it — up to here, six times the corpus.
// Past that the number is wrong, and a run that kept believing it would
// spend its deadline on archlinux.org instead of on PKGBUILDs.
const maxArchwebPages = 400

// archwebPage is one page of the package search, reduced to what is read of
// it: whether the query was accepted, how many pages there are, and each
// package's base and maintainers.
type archwebPage struct {
	Valid    bool `json:"valid"`
	NumPages int  `json:"num_pages"`
	Results  []struct {
		PkgBase     string   `json:"pkgbase"`
		Maintainers []string `json:"maintainers"`
	} `json:"results"`
}

// loadMaintainers returns each official package base's maintainers, from a
// same-day cached copy or by paging through archlinux.org's package search.
func loadMaintainers(cache string) (map[string][]string, error) {
	p := filepath.Join(cache, "maintainers.json")
	if fresh(p) {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var byBase map[string][]string
		if err := json.Unmarshal(b, &byBase); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		return byBase, nil
	}
	byBase, err := fetchMaintainers()
	if err != nil {
		return nil, fmt.Errorf("archlinux.org package search: %w", err)
	}
	// Written whole and renamed into place, as download does: a run killed
	// mid-write must not leave a fresh-looking half of a file for its rerun
	// to fail on.
	if err := writeJSON(p+".tmp", byBase); err != nil {
		return nil, err
	}
	if err := os.Rename(p+".tmp", p); err != nil {
		return nil, err
	}
	log.Printf("archlinux.org: maintainers for %d package bases", len(byBase))
	return byBase, nil
}

// fetchMaintainers pages through the search, stopping where archlinux.org
// says the pages end. That is the only stop there is: a page past the last is
// answered with the last page over again, not with an error or an empty one,
// so a loop that ran until nothing came back would never finish.
func fetchMaintainers() (map[string][]string, error) {
	byBase := map[string][]string{}
	for page, last := 1, 1; page <= last; page++ {
		var pg archwebPage
		if err := getJSON(fmt.Sprintf(archwebSearchURL, page), &pg); err != nil {
			return nil, err
		}
		if !pg.Valid {
			return nil, fmt.Errorf("page %d: query rejected", page)
		}
		if pg.NumPages > maxArchwebPages {
			return nil, fmt.Errorf("page %d: %d pages claimed, more than the %d this reads", page, pg.NumPages, maxArchwebPages)
		}
		// Re-read from every page rather than trusted from the first: a
		// package added or dropped mid-sweep moves the end.
		last = pg.NumPages
		for _, r := range pg.Results {
			if _, seen := byBase[r.PkgBase]; seen || r.PkgBase == "" {
				continue
			}
			// Sorted: archweb lists them in no particular order, and the
			// page should not reword itself from one night to the next over
			// nothing.
			byBase[r.PkgBase] = slices.Sorted(slices.Values(r.Maintainers))
		}
	}
	return byBase, nil
}

// maxDescBytes bounds one desc entry. A real one is a few kilobytes.
const maxDescBytes = 1 << 20

// parseSyncDB reads a pacman sync database — a gzipped tar of
// <name>-<version>/desc files — and folds its packages into one entry per
// pkgbase, which is what the report card grades. Split packages share a
// PKGBUILD and normally a build; when the mirror is mid-update they can carry
// different versions, and the newest build is the one whose tag names the
// PKGBUILD the mirror is serving, so it wins.
func parseSyncDB(r io.Reader, repo string) ([]metaPackage, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(io.LimitReader(gz, maxMetaDecompressed))

	type entry struct {
		meta  metaPackage
		named string // the description of the package named after its base, if any
	}
	byBase := map[string]*entry{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != "desc" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxDescBytes))
		if err != nil {
			return nil, err
		}
		fields := descFields(data)
		name := fields.first("%NAME%")
		base := fields.first("%BASE%")
		if base == "" {
			base = name
		}
		if name == "" || base == "" {
			continue
		}
		built, _ := strconv.ParseInt(fields.first("%BUILDDATE%"), 10, 64)
		m := metaPackage{
			Name:         name,
			PackageBase:  base,
			Version:      fields.first("%VERSION%"),
			Description:  fields.first("%DESC%"),
			URL:          fields.first("%URL%"),
			Packager:     packagerName(fields.first("%PACKAGER%")),
			LastModified: built,
			Repo:         repo,
		}
		e, ok := byBase[base]
		if !ok {
			e = &entry{meta: m}
			byBase[base] = e
		} else if built > e.meta.LastModified || (built == e.meta.LastModified && name < e.meta.Name) {
			e.meta = m
		}
		if name == base {
			e.named = m.Description
		}
	}

	pkgs := make([]metaPackage, 0, len(byBase))
	for _, e := range byBase {
		m := e.meta
		// The roster shows one line per base: the description of the package
		// that carries the base's name reads as the base's own, where a split
		// package's first alphabetical member ("linux-docs") does not.
		if e.named != "" {
			m.Description = e.named
		}
		m.Name = m.PackageBase
		pkgs = append(pkgs, m)
	}
	slices.SortFunc(pkgs, func(a, b metaPackage) int { return strings.Compare(a.PackageBase, b.PackageBase) })
	return pkgs, nil
}

// fields is a parsed desc entry: each %SECTION% and the lines under it.
type fields map[string][]string

func (f fields) first(key string) string {
	if v := f[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// descFields parses pacman's desc format: a %KEY% line opens a section and
// every non-empty line until the next one belongs to it.
func descFields(data []byte) fields {
	out := fields{}
	section := ""
	for line := range strings.Lines(string(data)) {
		line = strings.TrimRight(line, "\r\n")
		if len(line) > 2 && strings.HasPrefix(line, "%") && strings.HasSuffix(line, "%") {
			section = line
			continue
		}
		if line == "" || section == "" {
			continue
		}
		out[section] = append(out[section], line)
	}
	return out
}

// packagerName is the person behind a %PACKAGER% line, which makepkg writes as
// "Name <email>". The address is dropped: it is not something to publish
// beside every package a person has ever built, and the name is what the
// roster's "@" query is for.
func packagerName(s string) string {
	if i := strings.IndexByte(s, '<'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// The project-name rules below are devtools' gitlab_project_name_to_path,
// which is how the packaging namespace got its paths: GitLab reserves some
// names and refuses some characters, so a pkgbase is not always its project.
var (
	projectJoin    = regexp.MustCompile(`([a-zA-Z0-9]+)\+([a-zA-Z]+)`)
	projectSpecial = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	projectRuns    = regexp.MustCompile(`[_-]{2,}`)
)

// gitlabProject is the GitLab path of a pkgbase's packaging project:
// "libsigc++" lives at libsigcplusplus, "tree" at unix-tree.
func gitlabProject(base string) string {
	p := projectJoin.ReplaceAllString(base, "$1-$2") // a single '+' between words becomes '-'
	p = strings.ReplaceAll(p, "+", "plus")           // any other '+' is spelled out
	p = projectSpecial.ReplaceAllString(p, "-")      // GitLab allows only [A-Za-z0-9_.-]
	p = projectRuns.ReplaceAllString(p, "-")         // and no runs of separators
	if p == "tree" {
		p = "unix-tree" // reserved by GitLab
	}
	return p
}

// gitlabTag is the release tag devtools cuts for a package version
// (get_tag_from_pkgver): git refuses ':' and '~', so an epoch's colon becomes
// a dash and a tilde a dot — "1:1.10.0-2" is tagged 1-1.10.0-2.
func gitlabTag(version string) string {
	tag := strings.Replace(version, ":", "-", 1)
	return strings.ReplaceAll(tag, "~", ".")
}

// tagRe is what a tag derived from a well-formed version looks like. Anything
// else is not a URL segment this program should build.
var tagRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// snapshotSource is where a base's current PKGBUILD tree is fetched from: the
// AUR's cgit snapshot, or the release tag's archive on GitLab.
func snapshotSource(m metaPackage) (string, error) {
	if m.repo() == aurRepo {
		return fmt.Sprintf(snapshotURL, m.PackageBase), nil
	}
	tag := gitlabTag(m.Version)
	if !tagRe.MatchString(tag) {
		return "", fmt.Errorf("version %q does not name a release tag", m.Version)
	}
	project := gitlabProject(m.PackageBase)
	return fmt.Sprintf(officialSnapshotURL, project, tag, project, tag), nil
}

// Official reports whether the package comes from an official repository
// rather than the AUR. Templates branch on it: an official package has
// maintainers, a packager and a GitLab project where an AUR package has a
// maintainer, co-maintainers, votes and an AUR page.
func (r siteResult) Official() bool {
	return r.Repo != "" && r.Repo != aurRepo
}

// PageURL is the package's page on the repository that carries it.
func (r siteResult) PageURL() string {
	if r.Official() {
		return "https://archlinux.org/pkgbase/" + r.Base + "/"
	}
	return "https://aur.archlinux.org/pkgbase/" + r.Base
}

// SourceURL is where the PKGBUILD this page grades is kept.
func (r siteResult) SourceURL() string {
	if r.Official() {
		return gitlabPackages + gitlabProject(r.Base)
	}
	return "https://aur.archlinux.org/cgit/aur.git/tree/PKGBUILD?h=" + r.Base
}
