// Package orchard implements the subtree operations behind the commands.
package orchard

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

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

// PushOptions configure Push and PushTag.
type PushOptions struct {
	DryRun bool
	// Force overwrites the upstream ref, leased on its value when the push
	// starts, so an update that lands in between still fails the push.
	Force bool
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
	args := o.pushArgs()
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
	return o.Repo.Run(append(args, s.Remote, split+":"+ref)...)
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
	for _, line := range strings.Split(out, "\n") {
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

func (o *Orchard) pushArgs() []string {
	if o.NoVerify {
		return []string{"push", "--no-verify"}
	}
	return []string{"push"}
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
// tag to opts.Remote. It returns the tag.
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
	if _, err := o.Repo.Output("rev-parse", "--quiet", "--verify", "refs/tags/"+tag); err == nil {
		if !opts.Force {
			return "", fmt.Errorf("tag %s already exists; pass --force to move it", tag)
		}
		if err := o.checkUnpublished(s, version, rev); err != nil {
			return "", err
		}
	}

	if opts.Upstream {
		if err := o.Push(s, rev, PushOptions{}); err != nil {
			return "", err
		}
	} else {
		st, err := o.Status(s, rev)
		if err != nil {
			return "", err
		}
		if st.Ahead > 0 {
			return "", fmt.Errorf("%s's upstream lacks %d commit(s) of this release; publish them first with git orchard push %s, or pass --upstream", s.Prefix, st.Ahead, s.Prefix)
		}
	}

	message := opts.Message
	if message == "" {
		message = s.Prefix + " " + version
	}
	tagArgs := []string{"tag", "--annotate", "--message=" + message}
	if opts.Force {
		tagArgs = append(tagArgs, "--force")
	}
	if _, err := o.Repo.Output(append(tagArgs, tag, rev)...); err != nil {
		return "", err
	}
	if opts.Upstream {
		if err := o.PushTag(tag, PushOptions{Force: opts.Force}); err != nil {
			return tag, err
		}
	}
	if opts.Remote != "" {
		args := o.pushArgs()
		if opts.Force {
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
// upstream, which may predate the subtree.
func (o *Orchard) NextVersion(s config.Subtree, opts NextOptions) (string, error) {
	at, err := o.Repo.Output("tag", "--points-at", opts.Rev+"^{commit}", "--list", s.Prefix+"/v*")
	if err != nil {
		return "", err
	}
	// A pre-release can still be promoted.
	for _, tag := range strings.Fields(at) {
		v, err := semver.Parse(strings.TrimPrefix(tag, s.Prefix+"/"))
		if err == nil && v.Prefix == "" && (v.Stable() || v.PreRelease == opts.Suffix) {
			return "", fmt.Errorf("%s is already released as %s", opts.Rev, tag)
		}
	}
	versions, err := o.releases(s, opts.Remote)
	if err != nil {
		return "", err
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
			return latest.Base().String(), nil
		case opts.Suffix != "" && latest.Stable():
			inc = semver.Patch
		}
	}
	return latest.Next(inc, opts.Suffix, versions).String(), nil
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
	for _, line := range strings.Split(out, "\n") {
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
