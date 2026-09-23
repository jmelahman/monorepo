// Command fptriage ranks pkglint's rules by how often their findings look like
// false positives, so rule work goes where it pays off first.
//
// It is a maintainer tool, not part of pkglint: nothing here is imported by the
// pkglint command, and it is the only code in the module that talks to the
// TypeSafe API. It reads the package trees a site run leaves in its cache
// (snapshots/<base>@<lastmod>/ for the AUR, snapshots/<repo>/<base>@<lastmod>/
// for an official repository), lints them the same parse-only way `pkglint`
// does, samples findings per rule, and asks TypeSafe one typed question per
// finding: is it a false positive, and why was it reported. Verdicts are
// appended to -out as they arrive, which doubles as a cache, so a rerun only
// pays for findings it has not judged yet.
//
//	go run ./site -top 200 -budget 200 -out /tmp/site-preview
//	go run ./tools/fptriage -cache .cache -rules PB914 -per-rule 5 -dry-run
//	TYPESAFE_API_KEY=... go run ./tools/fptriage -cache .cache -max-requests 30
package main

import (
	"bufio"
	"cmp"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/rules"
	"github.com/jmelahman/typesafe-sdk-go"
)

// aurRepo names the AUR in output, as the site does for its state records.
const aurRepo = "aur"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "fptriage:", err)
		os.Exit(1)
	}
}

type options struct {
	cache       string
	rules       map[string]bool         // empty means every rule
	severities  map[rules.Severity]bool // empty means every severity
	perRule     int
	seed        uint64
	out         string
	maxRequests int
	jobs        int
	context     int
	dryRun      bool
}

func parseFlags(args []string, stderr io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("fptriage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.cache, "cache", ".cache", "site cache directory holding snapshots/")
	ruleList := fs.String("rules", "", "comma-separated rule IDs to judge (default: all)")
	sevList := fs.String("severity", "", "comma-separated finding severities to judge, e.g. error,critical (default: all)")
	fs.IntVar(&o.perRule, "per-rule", 15, "findings sampled per rule")
	fs.Uint64Var(&o.seed, "seed", 1, "sampling seed")
	fs.StringVar(&o.out, "out", "fptriage.jsonl", "verdicts file, appended to and reused as a cache")
	fs.IntVar(&o.maxRequests, "max-requests", 200, "most API requests this run")
	fs.IntVar(&o.jobs, "jobs", 4, "concurrent API requests")
	fs.IntVar(&o.context, "context", 10, "lines of context either side of the flagged line")
	fs.BoolVar(&o.dryRun, "dry-run", false, "print the requests instead of sending them")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() > 0 {
		return o, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if o.perRule < 1 || o.jobs < 1 || o.maxRequests < 0 || o.context < 0 {
		return o, errors.New("-per-rule and -jobs must be positive; -max-requests and -context must not be negative")
	}
	o.rules = map[string]bool{}
	for id := range strings.SplitSeq(*ruleList, ",") {
		if id = strings.TrimSpace(id); id != "" {
			o.rules[id] = true
		}
	}
	o.severities = map[rules.Severity]bool{}
	for name := range strings.SplitSeq(*sevList, ",") {
		if name = strings.TrimSpace(name); name != "" {
			sev, err := rules.ParseSeverity(name)
			if err != nil {
				return o, err
			}
			o.severities[sev] = true
		}
	}
	return o, nil
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return err
	}
	registry := map[string]rules.Rule{}
	for _, r := range rules.Registry() {
		registry[r.ID] = r
	}
	for id := range opts.rules {
		if _, ok := registry[id]; !ok {
			return fmt.Errorf("unknown rule %s", id)
		}
	}

	snaps, err := findSnapshots(opts.cache)
	if err != nil {
		return err
	}
	if len(snaps) == 0 {
		return fmt.Errorf("no snapshots under %s: run the site generator first", filepath.Join(opts.cache, "snapshots"))
	}
	cands, failed := lintAll(snaps, func(f rules.Finding) bool {
		return (len(opts.rules) == 0 || opts.rules[f.RuleID]) &&
			(len(opts.severities) == 0 || opts.severities[f.Severity])
	})
	if failed > 0 {
		fmt.Fprintf(stderr, "%d of %d snapshots failed to load and were skipped\n", failed, len(snaps))
	}
	sample := sampleByRule(cands, opts.perRule, opts.seed)

	verdicts, err := loadVerdicts(opts.out)
	if err != nil {
		return err
	}
	var pending []candidate
	for _, c := range sample {
		if _, ok := verdicts[c.key()]; !ok {
			pending = append(pending, c)
		}
	}
	if len(pending) > opts.maxRequests {
		fmt.Fprintf(stderr, "%d findings to judge; this run's budget covers %d\n", len(pending), opts.maxRequests)
		pending = pending[:opts.maxRequests]
	}

	if opts.dryRun {
		return printRequests(stdout, pending, registry, opts.context)
	}

	var judgeErr error
	if len(pending) > 0 {
		client, err := typesafe.New()
		if err != nil {
			return err
		}
		f, err := os.OpenFile(opts.out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		record := func(v verdict) error {
			verdicts[v.key()] = v
			b, err := json.Marshal(v)
			if err != nil {
				return err
			}
			_, err = f.Write(append(b, '\n'))
			return err
		}
		judgeErr = judgeAll(ctx, client, pending, registry, opts, record, stderr)
	}
	writeReport(stdout, sample, verdicts)
	return judgeErr
}

// A snapshot is one package tree in the site's cache.
type snapshot struct {
	Repo         string
	Base         string
	LastModified int64
	Dir          string
}

// findSnapshots lists the newest tree per base and repository under
// cache/snapshots. The site keeps a base's older trees until it prunes, and
// judging a stale one would spend a request on a PKGBUILD nobody publishes.
func findSnapshots(cache string) ([]snapshot, error) {
	root := filepath.Join(cache, "snapshots")
	newest := map[[2]string]snapshot{}
	var scan func(dir, repo string) error
	scan = func(dir, repo string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue // an interrupted download's .tar.gz
			}
			// The stamp follows the last @: AUR base names may contain one.
			at := strings.LastIndexByte(e.Name(), '@')
			if at < 0 {
				if repo == aurRepo {
					// An official repository's trees sit one level down.
					if err := scan(filepath.Join(dir, e.Name()), e.Name()); err != nil {
						return err
					}
				}
				continue
			}
			base := e.Name()[:at]
			lm, err := strconv.ParseInt(e.Name()[at+1:], 10, 64)
			if err != nil || base == "" {
				continue
			}
			s := snapshot{Repo: repo, Base: base, LastModified: lm, Dir: filepath.Join(dir, e.Name())}
			k := [2]string{repo, base}
			if old, seen := newest[k]; !seen || lm > old.LastModified {
				newest[k] = s
			}
		}
		return nil
	}
	if err := scan(root, aurRepo); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]snapshot, 0, len(newest))
	for _, s := range newest {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b snapshot) int {
		return cmp.Or(cmp.Compare(a.Repo, b.Repo), cmp.Compare(a.Base, b.Base))
	})
	return out, nil
}

// A candidate is one finding that could be judged.
type candidate struct {
	snapshot
	Finding rules.Finding
	File    string // Finding.Path relative to the snapshot's tree
}

func (c candidate) key() string {
	return findingKey(promptVersion, c.Repo, c.Base, c.LastModified, c.Finding.RuleID, c.File, c.Finding.Line, c.Finding.Message)
}

// findingKey identifies a verdict. The prompt version is part of it: a
// verdict answers the question as it was asked, so rewording the question
// has to send every finding back to be judged again.
func findingKey(prompt int, repo, base string, lastModified int64, rule, file string, line int, message string) string {
	return strings.Join([]string{strconv.Itoa(prompt), repo, base, strconv.FormatInt(lastModified, 10), rule, file, strconv.Itoa(line), message}, "\x00")
}

// lintAll lints every snapshot exactly as `pkglint` would — parsed, never run —
// so the findings match the text on disk rather than whatever a state file
// last recorded for the base. keep picks the findings worth judging; it sees
// each finding's own severity, which for an escalating rule is not always the
// rule's base severity.
func lintAll(snaps []snapshot, keep func(rules.Finding) bool) (cands []candidate, failed int) {
	for _, s := range snaps {
		pkg, err := pkgbuild.Load(s.Dir)
		if err != nil {
			failed++
			continue
		}
		for _, f := range rules.Run(pkg, nil) {
			if !keep(f) {
				continue
			}
			file, err := filepath.Rel(s.Dir, f.Path)
			if err != nil {
				file = f.Path
			}
			cands = append(cands, candidate{snapshot: s, Finding: f, File: file})
		}
	}
	return cands, failed
}

// sampleByRule keeps up to n findings per rule, ordered by rule. Each finding
// ranks by a hash of its key, not by a shuffle, so a finding keeps its place as
// the corpus grows: a rerun over a bigger cache mostly reselects findings the
// verdicts file already has, rather than paying to judge a fresh draw.
func sampleByRule(cands []candidate, n int, seed uint64) []candidate {
	byRule := map[string][]candidate{}
	for _, c := range cands {
		byRule[c.Finding.RuleID] = append(byRule[c.Finding.RuleID], c)
	}
	type ranked struct {
		c    candidate
		key  string
		rank uint64
	}
	var out []candidate
	for _, id := range slices.Sorted(maps.Keys(byRule)) {
		group := make([]ranked, 0, len(byRule[id]))
		for _, c := range byRule[id] {
			k := c.key()
			h := fnv.New64a()
			h.Write(binary.LittleEndian.AppendUint64(nil, seed))
			h.Write([]byte(k))
			group = append(group, ranked{c, k, h.Sum64()})
		}
		slices.SortFunc(group, func(a, b ranked) int {
			return cmp.Or(cmp.Compare(a.rank, b.rank), strings.Compare(a.key, b.key))
		})
		for _, r := range group[:min(n, len(group))] {
			out = append(out, r.c)
		}
	}
	return out
}

// A verdict is one judged finding, as stored in the verdicts file.
type verdict struct {
	Repo            string  `json:"repo"`
	Base            string  `json:"base"`
	LastModified    int64   `json:"last_modified"`
	Rule            string  `json:"rule"`
	File            string  `json:"file"`
	Line            int     `json:"line"`
	Message         string  `json:"message"`
	FP              float64 `json:"fp"`
	Cause           string  `json:"cause"`
	CauseConfidence float64 `json:"cause_confidence"`
	RequestID       string  `json:"request_id,omitempty"`
	Prompt          int     `json:"prompt"` // promptVersion the verdict answered
}

func (v verdict) key() string {
	return findingKey(v.Prompt, v.Repo, v.Base, v.LastModified, v.Rule, v.File, v.Line, v.Message)
}

func loadVerdicts(path string) (map[string]verdict, error) {
	out := map[string]verdict{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for line := 1; sc.Scan(); line++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var v verdict
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		out[v.key()] = v
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return out, nil
}

func requestFor(c candidate, registry map[string]rules.Rule, around int) (typesafe.SystemOneRequest, error) {
	raw, err := os.ReadFile(filepath.Join(c.Dir, c.File))
	if err != nil {
		return typesafe.SystemOneRequest{}, err
	}
	return buildRequest(registry[c.Finding.RuleID], c, excerpt(raw, c.Finding.Line, around)), nil
}

func printRequests(w io.Writer, pending []candidate, registry map[string]rules.Rule, around int) error {
	for _, c := range pending {
		req, err := requestFor(c, registry, around)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "# %s %s/%s %s:%d\n", c.Finding.RuleID, c.Repo, c.Base, c.File, c.Finding.Line)
		// Unescaped, so the excerpt's > marker reads as it is sent.
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(req); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "# %d requests\n", len(pending))
	return nil
}

// judgeAll judges pending findings concurrently, recording each verdict as it
// arrives so an interrupted run keeps what it paid for. A failed request is
// reported and skipped, except for a rejected credential, which would fail
// every request after it the same way.
func judgeAll(ctx context.Context, client *typesafe.Client, pending []candidate, registry map[string]rules.Rule, opts options, record func(verdict) error, stderr io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu       sync.Mutex
		failures int
		fatal    error
		wg       sync.WaitGroup
	)
	work := make(chan candidate)
	for range opts.jobs {
		wg.Go(func() {
			for c := range work {
				v, err := judgeCandidate(ctx, client, c, registry, opts.context)
				mu.Lock()
				switch {
				case err != nil && ctx.Err() != nil:
					// Cancelled under us; the cause is reported once below.
				case err != nil:
					failures++
					fmt.Fprintf(stderr, "%s %s/%s %s:%d: %v\n", c.Finding.RuleID, c.Repo, c.Base, c.File, c.Finding.Line, err)
					var authErr *typesafe.AuthenticationError
					var permErr *typesafe.PermissionDeniedError
					if fatal == nil && (errors.As(err, &authErr) || errors.As(err, &permErr)) {
						fatal = err
						cancel()
					}
				default:
					if err := record(v); err != nil && fatal == nil {
						fatal = err
						cancel()
					}
				}
				mu.Unlock()
			}
		})
	}
feed:
	for _, c := range pending {
		select {
		case work <- c:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()

	if fatal != nil {
		return fatal
	}
	if failures > 0 {
		return fmt.Errorf("%d of %d requests failed", failures, len(pending))
	}
	return ctx.Err()
}

func judgeCandidate(ctx context.Context, client *typesafe.Client, c candidate, registry map[string]rules.Rule, around int) (verdict, error) {
	req, err := requestFor(c, registry, around)
	if err != nil {
		return verdict{}, err
	}
	fp, cause, confidence, requestID, err := judgeOne(ctx, client, req)
	if err != nil {
		return verdict{}, err
	}
	return verdict{
		Repo: c.Repo, Base: c.Base, LastModified: c.LastModified,
		Rule: c.Finding.RuleID, File: c.File, Line: c.Finding.Line, Message: c.Finding.Message,
		FP: fp, Cause: cause, CauseConfidence: confidence, RequestID: requestID, Prompt: promptVersion,
	}, nil
}

// likelyFP is the P(fp) at which a verdict counts as a likely false positive
// in the report. It is a reading aid, not a decision: every example still
// needs a human look before a rule changes.
const likelyFP = 0.7

// examplesPerRule is how many of a rule's most likely false positives the
// report lists under the table.
const examplesPerRule = 3

type ruleStats struct {
	Rule     string
	Judged   []verdict
	MeanFP   float64
	Likely   int
	TopCause string
}

// writeReport summarizes this run's sample — the verdicts file may hold
// findings from other runs and rule filters, which are left out.
func writeReport(w io.Writer, sample []candidate, verdicts map[string]verdict) {
	byRule := map[string]*ruleStats{}
	unjudged := 0
	for _, c := range sample {
		v, ok := verdicts[c.key()]
		if !ok {
			unjudged++
			continue
		}
		s := byRule[v.Rule]
		if s == nil {
			s = &ruleStats{Rule: v.Rule}
			byRule[v.Rule] = s
		}
		s.Judged = append(s.Judged, v)
	}
	var stats []*ruleStats
	for _, s := range byRule {
		causes := map[string]int{}
		for _, v := range s.Judged {
			s.MeanFP += v.FP
			if v.FP >= likelyFP {
				s.Likely++
			}
			causes[v.Cause]++
		}
		s.MeanFP /= float64(len(s.Judged))
		for cause, n := range causes {
			if n > causes[s.TopCause] || n == causes[s.TopCause] && cause < s.TopCause {
				s.TopCause = cause
			}
		}
		slices.SortFunc(s.Judged, func(a, b verdict) int {
			return cmp.Or(cmp.Compare(b.FP, a.FP), strings.Compare(a.key(), b.key()))
		})
		stats = append(stats, s)
	}
	slices.SortFunc(stats, func(a, b *ruleStats) int {
		return cmp.Or(cmp.Compare(b.MeanFP, a.MeanFP), cmp.Compare(a.Rule, b.Rule))
	})

	fmt.Fprintf(w, "# False-positive triage\n\n")
	if len(stats) == 0 {
		fmt.Fprintf(w, "No judged findings.\n")
	} else {
		fmt.Fprintf(w, "| Rule | Judged | Mean P(fp) | P(fp) ≥ %.1f | Top cause |\n|---|---:|---:|---:|---|\n", likelyFP)
		for _, s := range stats {
			fmt.Fprintf(w, "| %s | %d | %.2f | %d | %s |\n", s.Rule, len(s.Judged), s.MeanFP, s.Likely, s.TopCause)
		}
		fmt.Fprintf(w, "\n## Most likely false positives\n")
		for _, s := range stats {
			fmt.Fprintf(w, "\n### %s\n\n", s.Rule)
			for _, v := range s.Judged[:min(examplesPerRule, len(s.Judged))] {
				fmt.Fprintf(w, "- %.2f %s — %s/%s %s:%d — %s\n", v.FP, v.Cause, v.Repo, v.Base, v.File, v.Line, v.Message)
			}
		}
	}
	if unjudged > 0 {
		fmt.Fprintf(w, "\n%d sampled findings are not judged yet; rerun with a larger -max-requests.\n", unjudged)
	}
}
