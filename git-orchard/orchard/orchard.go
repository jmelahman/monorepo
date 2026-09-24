// Package orchard implements the subtree operations behind the commands.
package orchard

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/git"
)

// Orchard is a repository and its subtree manifest.
type Orchard struct {
	Repo   git.Repo
	Config config.Config
}

// Open loads the orchard containing dir.
func Open(dir string) (*Orchard, error) {
	repo, err := git.Open(dir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(repo)
	if err != nil {
		return nil, err
	}
	return &Orchard{Repo: repo, Config: cfg}, nil
}

var zeroOID = regexp.MustCompile(`^0+$`)

// ChangedSince narrows subtrees to those with changes between since and rev.
// A since that doesn't name a commit here (the all-zero "before" of a new
// branch, or history rewritten by a force push) means every subtree.
func (o *Orchard) ChangedSince(subtrees []config.Subtree, since, rev string) ([]config.Subtree, error) {
	if zeroOID.MatchString(since) {
		return subtrees, nil
	}
	if _, err := o.Repo.Output("rev-parse", "--verify", "--quiet", since+"^{commit}"); err != nil {
		return subtrees, nil
	}
	out, err := o.Repo.Output("diff", "--name-only", "--no-renames", since, rev)
	if err != nil {
		return nil, err
	}
	files := strings.Split(out, "\n")
	var changed []config.Subtree
	for _, s := range subtrees {
		for _, f := range files {
			if strings.HasPrefix(f, s.Prefix+"/") {
				changed = append(changed, s)
				break
			}
		}
	}
	return changed, nil
}

// Split returns the commit that is rev's history of s as its own repository.
// Splits are deterministic, so the same monorepo history always yields the
// same commits; that is what lets a push to the upstream fast-forward.
func (o *Orchard) Split(s config.Subtree, rev string) (string, error) {
	split, err := o.Repo.Output("subtree", "split", "--quiet", "--prefix="+s.Prefix, rev)
	if err != nil {
		return "", err
	}
	if split == "" {
		return "", fmt.Errorf("%s has no history at %s", s.Prefix, rev)
	}
	return split, nil
}

// Push publishes rev's split of s to the upstream branch. It never forces: an
// upstream with commits the monorepo lacks rejects the push until they are
// pulled in.
func (o *Orchard) Push(s config.Subtree, rev string, dryRun bool) error {
	return o.push(s, rev, "refs/heads/"+s.Branch, dryRun)
}

// PushTag publishes a monorepo release tag, <prefix>/<name>, to the upstream
// of <prefix> as <name>.
func (o *Orchard) PushTag(tag string, dryRun bool) error {
	tag = strings.TrimPrefix(tag, "refs/tags/")
	prefix, name, ok := SplitTag(tag)
	if !ok {
		return fmt.Errorf("tag %s is not of the form <prefix>/<name>", tag)
	}
	s, ok := o.Config.Lookup(prefix)
	if !ok {
		return fmt.Errorf("tag %s: %s is not a subtree listed in %s", tag, prefix, o.Config.Manifest)
	}
	rev := "refs/tags/" + tag + "^{commit}"
	if _, err := o.Repo.Output("rev-parse", "--quiet", "--verify", rev); err != nil {
		return fmt.Errorf("tag %s doesn't exist; create it first, e.g. git tag -a -m %q %s", tag, tag, tag)
	}
	return o.push(s, rev, "refs/tags/"+name, dryRun)
}

// SplitTag splits a release tag at its last slash, so nested prefixes work:
// templates/golang-template/v1.0.0 is v1.0.0 of templates/golang-template.
func SplitTag(tag string) (prefix, name string, ok bool) {
	i := strings.LastIndex(tag, "/")
	if i <= 0 || i == len(tag)-1 {
		return "", "", false
	}
	return tag[:i], tag[i+1:], true
}

func (o *Orchard) push(s config.Subtree, rev, ref string, dryRun bool) error {
	split, err := o.Split(s, rev)
	if err != nil {
		return err
	}
	args := []string{"push"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	return o.Repo.Run(append(args, s.Remote, split+":"+ref)...)
}

// Pull merges the upstream branch of s into its prefix.
func (o *Orchard) Pull(s config.Subtree, message string) error {
	args := []string{"subtree", "pull", "--prefix=" + s.Prefix}
	if o.Config.Squash {
		args = append(args, "--squash")
	}
	if message != "" {
		args = append(args, "--message="+message)
	}
	return o.Repo.Run(append(args, s.Remote, s.Branch)...)
}

// Add imports the upstream branch of s at its prefix and records it in the
// manifest.
func (o *Orchard) Add(s config.Subtree, message string) error {
	if _, ok := o.Config.Lookup(s.Prefix); ok {
		return fmt.Errorf("%s is already listed in %s", s.Prefix, o.Config.Manifest)
	}
	args := []string{"subtree", "add", "--prefix=" + s.Prefix}
	if o.Config.Squash {
		args = append(args, "--squash")
	}
	if message != "" {
		args = append(args, "--message="+message)
	}
	if err := o.Repo.Run(append(args, s.Remote, s.Branch)...); err != nil {
		return err
	}
	return config.Add(o.Repo, o.Config.Manifest, s)
}

// Status compares a subtree's split with its upstream branch.
type Status struct {
	// Ahead counts commits in the monorepo that the upstream lacks, and
	// Behind the reverse.
	Ahead, Behind int
}

func (s Status) String() string {
	switch {
	case s.Ahead == 0 && s.Behind == 0:
		return "up to date"
	case s.Behind == 0:
		return fmt.Sprintf("%d ahead", s.Ahead)
	case s.Ahead == 0:
		return fmt.Sprintf("%d behind", s.Behind)
	}
	return fmt.Sprintf("diverged: %d ahead, %d behind", s.Ahead, s.Behind)
}

// UpstreamRef is where Status fetches the upstream of s.
func UpstreamRef(s config.Subtree) string {
	return "refs/orchard/upstream/" + s.Prefix
}

// Status fetches the upstream branch of s and compares it with rev's split.
func (o *Orchard) Status(s config.Subtree, rev string) (Status, error) {
	ref := UpstreamRef(s)
	if _, err := o.Repo.Output("fetch", "--quiet", "--no-tags", s.Remote, "+refs/heads/"+s.Branch+":"+ref); err != nil {
		return Status{}, err
	}
	split, err := o.Split(s, rev)
	if err != nil {
		return Status{}, err
	}
	out, err := o.Repo.Output("rev-list", "--left-right", "--count", split+"..."+ref)
	if err != nil {
		return Status{}, err
	}
	var st Status
	counts := strings.Fields(out)
	if len(counts) != 2 {
		return Status{}, fmt.Errorf("unexpected rev-list output %q", out)
	}
	if st.Ahead, err = strconv.Atoi(counts[0]); err != nil {
		return Status{}, err
	}
	if st.Behind, err = strconv.Atoi(counts[1]); err != nil {
		return Status{}, err
	}
	return st, nil
}
