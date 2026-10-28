// Package orchard implements the subtree operations behind the commands.
package orchard

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/git"
	"github.com/jmelahman/git-orchard/semver"
)

// Orchard is a repository and its subtree manifest.
type Orchard struct {
	Repo   git.Repo
	Config config.Config
	// NoVerify skips the hooks that git commit, merge and push would run,
	// like their --no-verify.
	NoVerify bool

	mu sync.Mutex
	// fetched holds the prefixes whose UpstreamRef this Orchard has fetched
	// or pushed.
	fetched map[string]bool
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
		return subtrees, nil //nolint:nilerr // an unknown since deliberately means every subtree
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
	if err := o.fetchSplits(s, rev); err != nil {
		return "", err
	}
	// With the repository, git subtree fetches any upstream commit it still
	// lacks, e.g. one the upstream branch no longer contains.
	split, err := o.Repo.Output("subtree", "split", "--quiet", "--prefix="+s.Prefix, rev, s.Remote)
	if err != nil {
		return "", err
	}
	if split == "" {
		return "", fmt.Errorf("%s has no history at %s", s.Prefix, rev)
	}
	return split, nil
}

// fetchSplits fetches the upstream branch of s if rev's history lacks any
// upstream commit its squashes were made from, which git subtree split needs.
// A fresh clone, like CI's, has none of them. Fetching the branch brings them
// in at once instead of git subtree's fetch per commit.
func (o *Orchard) fetchSplits(s config.Subtree, rev string) error {
	out, err := o.Repo.Output("log", "--grep=^git-subtree-dir: "+s.Prefix+"/*$",
		"--format=%(trailers:key=git-subtree-split,valueonly)", rev)
	if err != nil {
		return err
	}
	for split := range strings.FieldsSeq(out) {
		if _, err := o.Repo.Output("cat-file", "-e", split+"^{commit}"); err != nil {
			return o.fetchUpstream(s)
		}
	}
	return nil
}

// fetchUpstream fetches the upstream branch of s to UpstreamRef(s). It
// leaves FETCH_HEAD and maintenance alone, since other subtrees' fetches may
// be running at the same time.
func (o *Orchard) fetchUpstream(s config.Subtree) error {
	_, err := o.Repo.Output("fetch", "--quiet", "--no-tags", "--no-write-fetch-head", "--no-auto-maintenance",
		s.Remote, "+refs/heads/"+s.Branch+":"+UpstreamRef(s))
	if err == nil {
		o.markFetched(s)
	}
	return err
}

// PushOptions configure Push and PushTag.
type PushOptions struct {
	DryRun bool
	// Force overwrites the upstream ref, leased on its value when the push
	// starts, so an update that lands in between still fails the push.
	Force bool
	// Output receives git push's output; nil means stderr.
	Output io.Writer
}

// Push publishes rev's split of s to the upstream branch. Unless forced, an
// upstream with commits the monorepo lacks rejects the push until they are
// pulled in.
func (o *Orchard) Push(s config.Subtree, rev string, opts PushOptions) error {
	return o.push(s, rev, "refs/heads/"+s.Branch, opts)
}

// PushTag publishes a monorepo release tag, <prefix>/<name>, to the upstream
// of <prefix> as <name>.
func (o *Orchard) PushTag(tag string, opts PushOptions) error {
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
	return o.push(s, rev, "refs/tags/"+name, opts)
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

func (o *Orchard) push(s config.Subtree, rev, ref string, opts PushOptions) error {
	split, err := o.Split(s, rev)
	if err != nil {
		return err
	}
	out := opts.Output
	if out == nil {
		out = os.Stderr
	}
	verify := !o.NoVerify
	if verify {
		ran, err := o.prePush(s, rev, split, ref, out)
		if err != nil {
			return err
		}
		verify = !ran
	}
	args := pushArgs(verify)
	if opts.DryRun {
		args = append(args, "--dry-run")
	}
	if opts.Force {
		lease, err := o.lease(s.Remote, ref)
		if err != nil {
			return err
		}
		args = append(args, lease)
	}
	if err := o.Repo.RunTo(out, append(args, s.Remote, split+":"+ref)...); err != nil {
		return err
	}
	// Like a remote-tracking branch, the upstream ref follows the push, so
	// a later push in this run, e.g. release's tag, diffs against it.
	if ref == "refs/heads/"+s.Branch && !opts.DryRun {
		if _, err := o.Repo.Output("update-ref", UpstreamRef(s), split); err != nil {
			return err
		}
		o.markFetched(s)
	}
	return nil
}

// prePush runs the monorepo's pre-push hook for pushing split, rev's split of
// s, to ref upstream, and reports whether it did so in place of git push.
// Left to git push, the hook would get the split's commits, whose paths are
// relative to the prefix, and a URL for a remote name, which tools like
// pre-commit take to mean that nothing has been pushed yet and so check every
// file. Instead it gets a pair of commits that differ by exactly what the
// push changes upstream, at their monorepo paths: tip is rev's tree, and base
// is rev's tree with the prefix as the upstream has it, at the split's merge
// base with the upstream branch. Neither is referenced, so gc removes them.
//
// With no hook there is nothing to run, which also counts as done. A git
// without git hook list (or failing it otherwise) leaves the hook to git push.
func (o *Orchard) prePush(s config.Subtree, rev, split, ref string, out io.Writer) (bool, error) {
	if _, err := o.Repo.Output("hook", "list", "pre-push"); err != nil {
		return git.ExitCode(err) == 1, nil
	}
	upstream, err := o.upstream(s)
	if err != nil {
		return false, err
	}
	var merged string
	if upstream != "" {
		merged, err = o.Repo.Output("merge-base", split, upstream)
		if err != nil && git.ExitCode(err) != 1 {
			return false, err
		}
	}

	dir, err := os.MkdirTemp("", "git-orchard-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	// With no upstream branch or merge base, the upstream shares none of the
	// prefix, so base lacks it entirely.
	index := []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}
	steps := [][]string{
		{"read-tree", rev},
		{"rm", "-r", "--cached", "--quiet", "--ignore-unmatch", "--", ":(literal)" + s.Prefix},
	}
	if merged != "" {
		steps = append(steps, []string{"read-tree", "--prefix=" + s.Prefix + "/", merged})
	}
	for _, step := range steps {
		if _, err := o.Repo.OutputEnv(index, step...); err != nil {
			return false, err
		}
	}
	tree, err := o.Repo.OutputEnv(index, "write-tree")
	if err != nil {
		return false, err
	}
	// The commits are the hook's input, not history: they need no identity
	// of the user's, who may have none, as in CI, nor their signature.
	ident := []string{
		"GIT_AUTHOR_NAME=git-orchard", "GIT_AUTHOR_EMAIL=git-orchard",
		"GIT_COMMITTER_NAME=git-orchard", "GIT_COMMITTER_EMAIL=git-orchard",
	}
	commitTree := []string{"-c", "commit.gpgSign=false", "commit-tree", "-m", "git-orchard: " + s.Prefix}
	base, err := o.Repo.OutputEnv(ident, append(commitTree, tree)...)
	if err != nil {
		return false, err
	}
	tip, err := o.Repo.OutputEnv(ident, append(commitTree, "-p", base, rev+"^{tree}")...)
	if err != nil {
		return false, err
	}

	// The local ref is what's being pushed, e.g. refs/heads/master for HEAD,
	// or, like git push's for a commit, the commit.
	local, err := o.Repo.Output("rev-parse", "--symbolic-full-name", strings.TrimSuffix(rev, "^{commit}"))
	if err != nil || local == "" {
		local = tip
	}
	stdin := filepath.Join(dir, "stdin")
	if err := os.WriteFile(stdin, []byte(local+" "+tip+" "+ref+" "+base+"\n"), 0o600); err != nil {
		return false, err
	}
	return true, o.Repo.RunTo(out, "hook", "run", "--to-stdin="+stdin, "pre-push", "--", s.Remote, s.Remote)
}

// upstream returns UpstreamRef(s), fetched unless it already was by this
// Orchard, or an empty string if the upstream has no branch yet.
func (o *Orchard) upstream(s config.Subtree) (string, error) {
	o.mu.Lock()
	fetched := o.fetched[s.Prefix]
	o.mu.Unlock()
	if fetched {
		return UpstreamRef(s), nil
	}
	err := o.fetchUpstream(s)
	if err == nil {
		return UpstreamRef(s), nil
	}
	// ls-remote --exit-code exits 2 for a missing ref, unlike for a failure
	// to reach the remote, which the fetch's error explains better.
	if _, lsErr := o.Repo.Output("ls-remote", "--exit-code", s.Remote, "refs/heads/"+s.Branch); git.ExitCode(lsErr) == 2 {
		return "", nil
	}
	return "", err
}

func (o *Orchard) markFetched(s config.Subtree) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.fetched == nil {
		o.fetched = map[string]bool{}
	}
	o.fetched[s.Prefix] = true
}

// lease is a --force-with-lease on ref's current value on remote. Pushes go
// to URLs, which have no remote-tracking refs for a bare --force-with-lease
// to check against.
func (o *Orchard) lease(remote, ref string) (string, error) {
	current, _, err := o.lsRemote(remote, ref)
	if err != nil {
		return "", err
	}
	return "--force-with-lease=" + ref + ":" + current, nil
}

// lsRemote returns ref's value on remote and the commit it peels to, or
// empty strings if remote has no such ref.
func (o *Orchard) lsRemote(remote, ref string) (oid, commit string, err error) {
	out, err := o.Repo.Output("ls-remote", remote, ref, ref+"^{}")
	if err != nil {
		return "", "", err
	}
	for line := range strings.SplitSeq(out, "\n") {
		value, name, _ := strings.Cut(line, "\t")
		switch name {
		case ref:
			oid = value
		case ref + "^{}":
			commit = value
		}
	}
	if commit == "" {
		commit = oid
	}
	return oid, commit, nil
}

// UpstreamTag returns the commit the upstream of s has tagged name, or an
// empty string if it has no such tag.
func (o *Orchard) UpstreamTag(s config.Subtree, name string) (string, error) {
	_, commit, err := o.lsRemote(s.Remote, "refs/tags/"+name)
	return commit, err
}

// RemoteURL returns the URL of a monorepo remote, which may already be one.
func (o *Orchard) RemoteURL(remote string) string {
	if url, err := o.Repo.Output("remote", "get-url", remote); err == nil {
		return url
	}
	return remote
}

// pushArgs starts a git push, which runs the pre-push hook if verify.
func pushArgs(verify bool) []string {
	if verify {
		return []string{"push"}
	}
	return []string{"push", "--no-verify"}
}

// hooklessArgs prefixes a git command whose own hooks can't be skipped,
// like git subtree's merge, so that it runs none under NoVerify.
func (o *Orchard) hooklessArgs(args ...string) []string {
	if o.NoVerify {
		return append([]string{"-c", "core.hooksPath=" + os.DevNull}, args...)
	}
	return args
}

// Pull merges the upstream branch of s into its prefix.
func (o *Orchard) Pull(s config.Subtree, message string) error {
	args := o.hooklessArgs("subtree", "pull", "--prefix="+s.Prefix)
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

// Move moves s to prefix, in the working tree, the manifest and git's own
// configuration, and commits the move along with anything already staged, so
// references to the old prefix can be updated in the same commit.
//
// The commit carries git-subtree-dir and git-subtree-split trailers, as a
// squash does, naming the new prefix and the subtree's split, so splits of
// the new prefix continue the upstream's history and pushes stay
// fast-forwards. A fresh clone fetches that split from the upstream, so it
// must be there already: Move refuses a subtree with unpushed commits.
func (o *Orchard) Move(s config.Subtree, prefix, message string) error {
	prefix = config.Clean(prefix)
	if _, ok := o.Config.Lookup(prefix); ok {
		return fmt.Errorf("%s is already listed in %s", prefix, o.Config.Manifest)
	}
	if _, err := os.Lstat(filepath.Join(o.Repo.Dir, prefix)); err == nil {
		return fmt.Errorf("%s already exists", prefix)
	} else if !os.IsNotExist(err) {
		return err
	}
	// The move restages both, so changes there would be committed with it.
	dirty, err := o.Repo.Output("status", "--porcelain", "--", ":(literal)"+s.Prefix, ":(literal)"+o.Config.Manifest)
	if err != nil {
		return err
	}
	if dirty != "" {
		return fmt.Errorf("%s or %s has uncommitted changes; commit or stash them first", s.Prefix, o.Config.Manifest)
	}

	split, err := o.Split(s, "HEAD")
	if err != nil {
		return err
	}
	upstream, err := o.upstream(s)
	if err != nil {
		return err
	}
	if upstream != "" {
		_, err = o.Repo.Output("merge-base", "--is-ancestor", split, upstream)
	}
	if upstream == "" || git.ExitCode(err) == 1 {
		return fmt.Errorf("%s has commits its upstream lacks; push them first (git orchard push %s), since clones fetch the split from there", s.Prefix, s.Prefix)
	} else if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Join(o.Repo.Dir, prefix)), 0o755); err != nil {
		return err
	}
	if _, err := o.Repo.Output("mv", "--", s.Prefix, prefix); err != nil {
		return err
	}
	from, to := "subtree."+s.Prefix, "subtree."+prefix
	if _, err := o.Repo.Output("config", "--file", filepath.Join(o.Repo.Dir, o.Config.Manifest), "--rename-section", from, to); err != nil {
		return err
	}
	// An override in git's own configuration follows the subtree too. Exit
	// status 1 means there is none.
	_, err = o.Repo.Output("config", "--local", "--get-regexp", "^"+regexp.QuoteMeta(from)+`\.`)
	if err == nil {
		_, err = o.Repo.Output("config", "--local", "--rename-section", from, to)
	}
	if err != nil && git.ExitCode(err) != 1 {
		return err
	}
	if _, err := o.Repo.Output("add", "--", o.Config.Manifest); err != nil {
		return err
	}

	if message == "" {
		message = "Move " + s.Prefix + " to " + prefix
	}
	message += "\n\ngit-subtree-dir: " + prefix + "\ngit-subtree-split: " + split + "\n"
	args := []string{"commit", "--quiet", "--message=" + message}
	if o.NoVerify {
		args = append(args, "--no-verify")
	}
	if err := o.Repo.Run(args...); err != nil {
		return fmt.Errorf("%w; the move is staged: commit it with a message ending in\n\n  git-subtree-dir: %s\n  git-subtree-split: %s", err, prefix, split)
	}
	return nil
}

// ReleaseOptions configure Release.
type ReleaseOptions struct {
	// Rev is the monorepo revision to release.
	Rev string
	// Message annotates the tag; it defaults to "<prefix> <version>".
	Message string
	// Remote is the monorepo remote to push the tag to, where the mirror
	// action publishes it upstream. Empty only tags locally.
	Remote string
	// Upstream publishes the upstream branch and tag directly as well.
	Upstream bool
	// Force moves an existing tag, provided the upstream hasn't published it
	// at another commit: consumers of a published release (the Go module
	// proxy, release artifacts) won't follow it.
	Force bool
}

// Release tags opts.Rev as version of s, <prefix>/<version>, and pushes the
// tag to opts.Remote. It returns the tag. A tag already at opts.Rev is kept
// and published again, to finish a release whose publishing failed.
//
// The upstream branch must already contain the release, or the upstream tag
// would point at a commit outside its history; with opts.Upstream, the
// branch is published first.
func (o *Orchard) Release(s config.Subtree, version string, opts ReleaseOptions) (string, error) {
	if version == "" || strings.Contains(version, "/") {
		return "", fmt.Errorf("version %q must be non-empty and contain no slashes", version)
	}
	tag := s.Prefix + "/" + version
	if _, err := o.Repo.Output("check-ref-format", "refs/tags/"+tag); err != nil {
		return "", fmt.Errorf("%s is not a valid tag name", tag)
	}
	rev := opts.Rev + "^{commit}"
	commit, err := o.Repo.Output("rev-parse", "--verify", rev)
	if err != nil {
		return "", err
	}
	tagged, err := o.Repo.Output("rev-parse", "--quiet", "--verify", "refs/tags/"+tag+"^{commit}")
	exists := err == nil
	resume := exists && tagged == commit
	if exists && !resume {
		if !opts.Force {
			return "", fmt.Errorf("tag %s already exists at %.12s; pass --force to move it", tag, tagged)
		}
		if err := o.checkUnpublished(s, version, rev); err != nil {
			return "", err
		}
	}

	// Only a branch lacking the release is pushed, since one that has moved
	// on past it would be rewound.
	st, err := o.Status(s, rev)
	if err != nil {
		return "", err
	}
	if st.Ahead > 0 {
		if !opts.Upstream {
			return "", fmt.Errorf("%s's upstream lacks %d commit(s) of this release; publish them first with git orchard push %s, or pass --upstream", s.Prefix, st.Ahead, s.Prefix)
		}
		if err := o.Push(s, rev, PushOptions{}); err != nil {
			return "", err
		}
	}

	moving := exists && !resume
	if !resume {
		message := opts.Message
		if message == "" {
			message = s.Prefix + " " + version
		}
		tagArgs := []string{"tag", "--annotate", "--message=" + message}
		if moving {
			tagArgs = append(tagArgs, "--force")
		}
		if _, err := o.Repo.Output(append(tagArgs, tag, rev)...); err != nil {
			return "", err
		}
	}
	if opts.Upstream {
		if err := o.PushTag(tag, PushOptions{Force: opts.Force}); err != nil {
			return tag, err
		}
	}
	if opts.Remote != "" {
		args := pushArgs(!o.NoVerify)
		if moving {
			lease, err := o.lease(opts.Remote, "refs/tags/"+tag)
			if err != nil {
				return tag, err
			}
			args = append(args, lease)
		}
		if err := o.Repo.Run(append(args, opts.Remote, "refs/tags/"+tag)...); err != nil {
			return tag, err
		}
	}
	return tag, nil
}

// NextOptions configure NextVersion.
type NextOptions struct {
	// Rev is the monorepo revision to release.
	Rev string
	// Remote is a monorepo remote whose release tags count too; empty
	// counts only local ones.
	Remote    string
	Increment semver.Increment
	// Suffix makes the next version a pre-release, e.g. "rc".
	Suffix string
}

// NextVersion picks the version to release opts.Rev of s as, like tag: after
// the latest release, the patch version incremented (or opts.Increment), or
// with opts.Suffix, the next pre-release of that name. Where tag takes the
// latest release reachable from HEAD, this takes the latest overall. The
// releases are the <prefix>/v* tags here and on opts.Remote, and the v* tags
// upstream, which may predate the subtree. If opts.Rev is already released,
// it's that version, for Release to finish publishing, and released is true.
func (o *Orchard) NextVersion(s config.Subtree, opts NextOptions) (version string, released bool, err error) {
	at, err := o.Repo.Output("tag", "--points-at", opts.Rev+"^{commit}", "--list", s.Prefix+"/v*")
	if err != nil {
		return "", false, err
	}
	// A pre-release can still be promoted, or followed by another name's.
	var existing *semver.Version
	for tag := range strings.FieldsSeq(at) {
		v, err := semver.Parse(strings.TrimPrefix(tag, s.Prefix+"/"))
		if err == nil && v.Prefix == "" && (v.Stable() || v.PreRelease == opts.Suffix) &&
			(existing == nil || semver.Compare(v, *existing) > 0) {
			existing = &v
		}
	}
	if existing != nil {
		return existing.String(), true, nil
	}
	versions, err := o.releases(s, opts.Remote)
	if err != nil {
		return "", false, err
	}

	var latest, stable *semver.Version
	for i, v := range versions {
		if v.Stable() && (stable == nil || semver.Compare(v, *stable) > 0) {
			stable = &versions[i]
		}
		if (opts.Suffix == "" || v.PreRelease == opts.Suffix) && (latest == nil || semver.Compare(v, *latest) > 0) {
			latest = &versions[i]
		}
	}
	// A pre-release never starts from an older version than the latest
	// stable one.
	if opts.Suffix != "" && stable != nil && (latest == nil || semver.Compare(stable.Base(), latest.Base()) >= 0) {
		latest = stable
	}
	if latest == nil {
		latest = &semver.Version{}
	}
	// Unlike tag, a stable release follows its pre-releases rather than the
	// version after them, and a pre-release follows a stable release rather
	// than preceding it.
	inc := opts.Increment
	if inc == semver.Auto {
		switch {
		case opts.Suffix == "" && !latest.Stable():
			return latest.Base().String(), false, nil
		case opts.Suffix != "" && latest.Stable():
			inc = semver.Patch
		}
	}
	return latest.Next(inc, opts.Suffix, versions).String(), false, nil
}

// releases returns the versions s has been released as.
func (o *Orchard) releases(s config.Subtree, remote string) ([]semver.Version, error) {
	local, err := o.Repo.Output("tag", "--list", s.Prefix+"/v*")
	if err != nil {
		return nil, err
	}
	tags := strings.Fields(local)
	if remote != "" {
		remoteTags, err := o.remoteTags(remote)
		if err != nil {
			return nil, err
		}
		tags = append(tags, remoteTags...)
	}
	var names []string
	for _, tag := range tags {
		if name, ok := strings.CutPrefix(tag, s.Prefix+"/"); ok {
			names = append(names, name)
		}
	}
	upstream, err := o.remoteTags(s.Remote)
	if err != nil {
		return nil, err
	}
	names = append(names, upstream...)

	var versions []semver.Version
	for _, name := range names {
		if !strings.HasPrefix(name, "v") {
			continue
		}
		// A prefix left over is a nested subtree's release.
		if v, err := semver.Parse(name); err == nil && v.Prefix == "" {
			versions = append(versions, v)
		}
	}
	return versions, nil
}

// remoteTags lists the tags on remote.
func (o *Orchard) remoteTags(remote string) ([]string, error) {
	out, err := o.Repo.Output("ls-remote", "--tags", "--refs", remote)
	if err != nil {
		return nil, err
	}
	var tags []string
	for line := range strings.SplitSeq(out, "\n") {
		if _, ref, ok := strings.Cut(line, "\t"); ok {
			tags = append(tags, strings.TrimPrefix(ref, "refs/tags/"))
		}
	}
	return tags, nil
}

// checkUnpublished fails if the upstream of s already has version at a
// commit other than rev's split.
func (o *Orchard) checkUnpublished(s config.Subtree, version, rev string) error {
	_, published, err := o.lsRemote(s.Remote, "refs/tags/"+version)
	if err != nil || published == "" {
		return err
	}
	split, err := o.Split(s, rev)
	if err != nil {
		return err
	}
	if published != split {
		return fmt.Errorf("%s is already published upstream at %.12s; release a new version instead (git orchard push --tag --force overwrites it regardless)", version, published)
	}
	return nil
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
	if err := o.fetchUpstream(s); err != nil {
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
