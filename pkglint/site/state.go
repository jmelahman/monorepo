package main

// The scan state is checked into the repository rather than left in a cache
// the CI runner may or may not still have. It is what makes a corpus this
// size tractable: a package base whose LastModified has not moved since the
// last run needs no snapshot download and no re-lint, so a nightly run pays
// for the few hundred bases that actually changed instead of all of them.
//
// LastModified comes out of the metadata dump, so validating a base costs
// nothing. The AUR's cgit does serve an ETag per snapshot, but using it would
// mean one conditional request per base per run to learn what the dump has
// already told us for free.

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/jmelahman/pkglint/internal/rules"
)

// stateRecord is everything a run would otherwise have to fetch and lint to
// rediscover about one package base. Fields the metadata dump carries — votes,
// description, maintainer, version — are deliberately absent: they are free on
// every run, and caching them would pin a package's vote count to whenever its
// PKGBUILD last changed.
type stateRecord struct {
	Base string `json:"base"`
	// Repo is the repository the base was scanned from. Absent from records
	// written before the official repositories joined the corpus, which were
	// all AUR; see repo. The AUR's LastModified and a repository's build date
	// are not on the same clock, so a record is fresh only against its own
	// repository.
	Repo         string          `json:"repo,omitempty"`
	LastModified int64           `json:"last_modified"`
	Grade        string          `json:"grade"`
	Findings     []rules.Finding `json:"findings"`
	Drift        []string        `json:"drift,omitempty"`
	Err          string          `json:"error,omitempty"`
	Fingerprint  sourceState     `json:"fingerprint"`

	// Rules is the fingerprint of the rule registry the findings were
	// produced under. A record from a different registry is stale even when
	// the PKGBUILD has not changed: without this, a pkglint release that adds
	// or recalibrates rules would leave most of the corpus graded by the old
	// ruleset forever, since re-lints are otherwise keyed on LastModified
	// alone.
	Rules string `json:"rules,omitempty"`
}

// repo is the record's repository, spelling out the AUR for the zero value.
func (r stateRecord) repo() string {
	if r.Repo == "" {
		return aurRepo
	}
	return r.Repo
}

// stateEpoch invalidates every state record when bumped. The registry
// fingerprint below catches rules being added, removed, or re-tiered on its
// own; bump this instead when rule *logic* changes in a way the registry
// shape cannot see and the whole corpus should re-lint.
const stateEpoch = 19

// rulesFingerprint identifies the current rule registry: the set of rule IDs
// with their declared severity ranges and fix levels, plus stateEpoch.
var rulesFingerprint = sync.OnceValue(func() string {
	var b strings.Builder
	fmt.Fprintf(&b, "epoch=%d", stateEpoch)
	for _, r := range rules.Registry() {
		s := r.Severities()
		fmt.Fprintf(&b, ";%s:%d:%d:%d", r.ID, s.Low, s.High, r.FixLevel)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("%x", sum[:8])
})

// loadState reads the checked-in scan state. A missing file is the ordinary
// first-run case, not an error; an unreadable or malformed one is reported,
// because silently treating it as empty would rescan the whole corpus and
// then overwrite the state that was merely unparseable.
func loadState(path string) (map[string]stateRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]stateRecord{}, nil
		}
		return nil, err
	}
	defer f.Close()

	state := map[string]stateRecord{}
	sc := bufio.NewScanner(f)
	// Records hold every finding for a package, which can run well past
	// bufio's default 64 KiB line ceiling.
	sc.Buffer(make([]byte, 0, 64<<10), maxStateLine)
	for line := 1; sc.Scan(); line++ {
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		var rec stateRecord
		if err := json.Unmarshal(b, &rec); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		// A record whose base is unusable as a path component cannot have come
		// from selectSeed, so treat the file as untrusted here too.
		if !safeBase(rec.Base) {
			return nil, fmt.Errorf("%s:%d: unsafe package base %q", path, line, rec.Base)
		}
		state[rec.Base] = rec
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return state, nil
}

// maxStateLine bounds a single state record. The state file is ours, but it is
// also the largest thing this program reads off disk, and a corrupt length
// should surface as an error rather than an allocation.
const maxStateLine = 8 << 20

// saveState writes the state as one JSON object per line, sorted by package
// base. Both properties are for git rather than for the program: sorted lines
// mean a run's diff is the handful of packages that changed, where a
// re-serialised JSON array would rewrite every line every night.
func saveState(path string, state map[string]stateRecord) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	bases := make([]string, 0, len(state))
	for base := range state {
		bases = append(bases, base)
	}
	sort.Strings(bases)

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, base := range bases {
		// Encode writes the trailing newline, which is the record separator.
		if err := enc.Encode(state[base]); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
