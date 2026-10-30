// Package pins finds every copy of a pinned version in a repository — hook
// revs and additional_dependencies in pre-commit configs, action inputs in
// workflows — and rewrites the ones that differ from the pin's source.
//
// Copies are edited in place, byte for byte: the files are parsed only to
// find where each version is, so comments and formatting survive.
package pins

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"go.yaml.in/yaml/v3"

	"github.com/jmelahman/pinsync/internal/config"
)

var (
	// ErrNoSource is returned by Find for a pin whose source does not
	// define it.
	ErrNoSource = errors.New("source does not define the pin")
	// ErrUnfrozen is returned by Find for a hook rev that is a commit with
	// no `# frozen:` comment, which leaves no version to compare.
	ErrUnfrozen = errors.New("rev is a commit with no # frozen: comment")
)

// Copy is one place a pin's version is written.
type Copy struct {
	// File is the path of the file, relative to the repository root.
	File string
	// Pin is the name of the pin the copy follows.
	Pin string
	// Have is the version written, and Want the pin's. Neither has a
	// leading v.
	Have string
	Want string
	// Repo is the hook repository of a rev, which a new tag is resolved in.
	Repo string
	// value is where the version is written, a leading v included.
	value span
	// rev is where a hook rev pinned to a commit is written, when value is
	// the tag in its `# frozen:` comment.
	rev span
	// Line is the 1-based line of value.
	Line int
}

// Stale reports whether the copy differs from its pin.
func (c Copy) Stale() bool { return c.Have != c.Want }

type span struct{ start, end int }

// CommitFunc returns the commit tag names in the repository at url.
type CommitFunc func(ctx context.Context, url, tag string) (string, error)

// Find returns every copy of pins in files, which are relative to root,
// with Want set from each pin's source.
func Find(root string, files []string, pins []config.Pin) ([]Copy, error) {
	var copies []Copy
	for _, f := range files {
		kind := classify(f)
		if kind == none {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			return nil, err
		}
		doc, err := parse(f, src)
		if err != nil {
			return nil, err
		}
		var found []Copy
		if kind == preCommit {
			found, err = doc.preCommitCopies(pins)
		} else {
			found, err = doc.actionCopies(pins)
		}
		if err != nil {
			return nil, err
		}
		copies = append(copies, found...)
	}

	want := make(map[string]string, len(pins))
	for _, p := range pins {
		v, err := source(root, p, copies)
		if err != nil {
			return nil, fmt.Errorf("pins.%s: %w", p.Name, err)
		}
		want[p.Name] = v
	}
	for i := range copies {
		copies[i].Want = want[copies[i].Pin]
	}
	return copies, nil
}

// source returns the version p's source defines, without a leading v.
func source(root string, p config.Pin, copies []Copy) (string, error) {
	switch p.Source {
	case config.SourceUVLock:
		return locked(root, p.Package)
	case config.SourcePreCommit:
		for _, c := range copies {
			if c.Pin == p.Name && c.Repo != "" && isRootPreCommit(c.File) {
				return c.Have, nil
			}
		}
		return "", fmt.Errorf("%w: no %s in the root .pre-commit-config.yaml", ErrNoSource, p.Repo)
	default:
		return trimV(p.Version), nil
	}
}

// locked returns the version uv.lock, at root, locks pkg at.
func locked(root, pkg string) (string, error) {
	var lock struct {
		Package []struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
		} `toml:"package"`
	}
	if _, err := toml.DecodeFile(filepath.Join(root, "uv.lock"), &lock); err != nil {
		return "", err
	}
	var versions []string
	for _, p := range lock.Package {
		if normalize(p.Name) == normalize(pkg) && !slices.Contains(versions, p.Version) {
			versions = append(versions, p.Version)
		}
	}
	switch len(versions) {
	case 0:
		return "", fmt.Errorf("%w: uv.lock has no %s", ErrNoSource, pkg)
	case 1:
		return versions[0], nil
	default:
		return "", fmt.Errorf("uv.lock locks %s at several versions: %s", pkg, strings.Join(versions, ", "))
	}
}

// Apply rewrites the stale copies among copies to their pin's version,
// resolving the commit of each new hook rev with commit.
func Apply(ctx context.Context, root string, copies []Copy, commit CommitFunc) error {
	byFile := map[string][]Copy{}
	for _, c := range copies {
		if c.Stale() {
			byFile[c.File] = append(byFile[c.File], c)
		}
	}
	for f, cs := range byFile {
		if err := applyFile(ctx, filepath.Join(root, f), cs, commit); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
}

type edit struct {
	text string
	span
}

func applyFile(ctx context.Context, name string, copies []Copy, commit CommitFunc) error {
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	edits := make([]edit, 0, len(copies))
	for _, c := range copies {
		written := string(src[c.value.start:c.value.end])
		tag := strings.TrimSuffix(written, trimV(written)) + c.Want
		edits = append(edits, edit{span: c.value, text: tag})
		if c.rev.end > 0 {
			sha, err := commit(ctx, c.Repo, tag)
			if err != nil {
				return err
			}
			edits = append(edits, edit{span: c.rev, text: sha})
		}
	}
	// Back to front, so each edit leaves the offsets before it valid.
	slices.SortFunc(edits, func(a, b edit) int { return cmp.Compare(b.start, a.start) })
	for _, e := range edits {
		src = slices.Concat(src[:e.start], []byte(e.text), src[e.end:])
	}
	return os.WriteFile(name, src, info.Mode().Perm())
}

type kind int

const (
	none kind = iota
	preCommit
	workflow
)

func classify(f string) kind {
	base := path.Base(f)
	switch {
	case base == ".pre-commit-config.yaml" || base == ".pre-commit-config.yml":
		return preCommit
	case base == "action.yml" || base == "action.yaml":
		return workflow
	case strings.Contains("/"+path.Dir(f)+"/", "/.github/workflows/") &&
		(path.Ext(f) == ".yml" || path.Ext(f) == ".yaml"):
		return workflow
	}
	return none
}

func isRootPreCommit(f string) bool {
	return f == ".pre-commit-config.yaml" || f == ".pre-commit-config.yml"
}

type document struct {
	root  *yaml.Node
	name  string
	src   []byte
	lines []int // the offset each line starts at
}

func parse(name string, src []byte) (*document, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	lines := []int{0}
	for i, b := range src {
		if b == '\n' {
			lines = append(lines, i+1)
		}
	}
	return &document{root: &root, name: name, src: src, lines: lines}, nil
}

// span returns where the text of the scalar n is in the source.
func (d *document) span(n *yaml.Node) (span, error) {
	if n.Line < 1 || n.Line > len(d.lines) {
		return span{}, fmt.Errorf("%s: no line %d", d.name, n.Line)
	}
	// Columns count characters, not bytes.
	start := d.lines[n.Line-1]
	for range n.Column - 1 {
		_, size := utf8.DecodeRune(d.src[start:])
		start += size
	}
	if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
		start++
	}
	end := start + len(n.Value)
	if end > len(d.src) || string(d.src[start:end]) != n.Value {
		return span{}, fmt.Errorf("%s:%d: can't find %q as written", d.name, n.Line, n.Value)
	}
	return span{start: start, end: end}, nil
}

func (d *document) copyAt(n *yaml.Node, pin string, s span) Copy {
	return Copy{
		File:  d.name,
		Pin:   pin,
		Have:  trimV(string(d.src[s.start:s.end])),
		value: s,
		Line:  n.Line,
	}
}

var frozen = regexp.MustCompile(`#\s*frozen:\s*(\S+)`)

// preCommitCopies returns the copies in a pre-commit config: the revs of
// pinned hook repos and pinned additional_dependencies. A config that is a
// bare list is a fragment of one, such as a git-orchard profile, which is
// spliced under another config's `repos:`.
func (d *document) preCommitCopies(pins []config.Pin) ([]Copy, error) {
	repos := top(d.root)
	if repos.Kind != yaml.SequenceNode {
		repos = value(repos, "repos")
	}
	var copies []Copy
	for _, repo := range items(repos) {
		url, rev := value(repo, "repo"), value(repo, "rev")
		if url != nil && rev != nil {
			for _, p := range pins {
				if p.Repo == "" || !sameRepo(p.Repo, url.Value) {
					continue
				}
				c, err := d.revCopy(rev, p)
				if err != nil {
					return nil, err
				}
				copies = append(copies, c)
			}
		}
		for _, hook := range items(value(repo, "hooks")) {
			for _, dep := range items(value(hook, "additional_dependencies")) {
				found, err := d.depCopies(dep, pins)
				if err != nil {
					return nil, err
				}
				copies = append(copies, found...)
			}
		}
	}
	return copies, nil
}

func (d *document) revCopy(rev *yaml.Node, p config.Pin) (Copy, error) {
	s, err := d.span(rev)
	if err != nil {
		return Copy{}, err
	}
	lineEnd := bytes.IndexByte(d.src[s.end:], '\n')
	if lineEnd < 0 {
		lineEnd = len(d.src) - s.end
	}
	if m := frozen.FindSubmatchIndex(d.src[s.end : s.end+lineEnd]); m != nil {
		tag := span{start: s.end + m[2], end: s.end + m[3]}
		c := d.copyAt(rev, p.Name, tag)
		c.Repo, c.rev = p.Repo, s
		return c, nil
	}
	if isCommit(rev.Value) {
		return Copy{}, fmt.Errorf("%s:%d: %s: %w", d.name, rev.Line, p.Repo, ErrUnfrozen)
	}
	c := d.copyAt(rev, p.Name, s)
	c.Repo = p.Repo
	return c, nil
}

// depCopies returns the copies in one of a hook's additional_dependencies:
// name==version for Python, path@version for Go and npm.
func (d *document) depCopies(dep *yaml.Node, pins []config.Pin) ([]Copy, error) {
	i, sep := strings.Index(dep.Value, "=="), len("==")
	name := normalize(dep.Value[:max(i, 0)])
	if i <= 0 {
		i, sep = strings.LastIndex(dep.Value, "@"), len("@")
		name = dep.Value[:max(i, 0)]
	}
	if i <= 0 {
		return nil, nil
	}
	var copies []Copy
	for _, p := range pins {
		if name != p.Package && name != normalize(p.Package) {
			continue
		}
		s, err := d.span(dep)
		if err != nil {
			return nil, err
		}
		s.start += i + sep
		copies = append(copies, d.copyAt(dep, p.Name, s))
	}
	return copies, nil
}

// actionCopies returns the inputs of pinned actions, in any step of a
// workflow or composite action.
func (d *document) actionCopies(pins []config.Pin) ([]Copy, error) {
	var copies []Copy
	var walk func(n *yaml.Node) error
	walk = func(n *yaml.Node) error {
		if n.Kind == yaml.MappingNode {
			if uses := value(n, "uses"); uses != nil {
				for _, p := range pins {
					if p.Action == "" || !strings.HasPrefix(uses.Value, p.Action+"@") {
						continue
					}
					input := value(value(n, "with"), p.Input)
					// An expression is set elsewhere, and is not a version.
					if input == nil || input.Kind != yaml.ScalarNode || strings.Contains(input.Value, "${{") {
						continue
					}
					s, err := d.span(input)
					if err != nil {
						return err
					}
					copies = append(copies, d.copyAt(input, p.Name, s))
				}
			}
		}
		for _, c := range n.Content {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(d.root); err != nil {
		return nil, err
	}
	return copies, nil
}

// top returns the top-level node of a document.
func top(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

// value returns the value of key in the mapping m, or nil.
func value(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// items returns the items of the sequence s, or nil.
func items(s *yaml.Node) []*yaml.Node {
	if s == nil || s.Kind != yaml.SequenceNode {
		return nil
	}
	return s.Content
}

func sameRepo(a, b string) bool {
	clean := func(s string) string { return strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git") }
	return clean(a) == clean(b)
}

func isCommit(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func trimV(s string) string { return strings.TrimPrefix(s, "v") }

// normalize returns a Python package name in the form uv.lock records it.
func normalize(name string) string {
	return strings.NewReplacer("_", "-", ".", "-").Replace(strings.ToLower(name))
}
