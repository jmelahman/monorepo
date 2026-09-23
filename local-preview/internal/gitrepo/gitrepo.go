// Package gitrepo manages the orchestrator's mirror clones and exposes the
// git plumbing the pipeline needs: ref resolution, tree listing (for content
// addressing), file reads at a commit, ancestry walks, and tree extraction.
// It is implemented in-process on go-git — no system git binary is required —
// and has no database dependency.
package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// go-git's stock "file" transport spawns git-upload-pack, which would keep
// the system-git dependency alive. Serve local clone/fetch sources in-process
// instead.
func init() {
	gitclient.InstallProtocol("file", server.NewClient(gitDirLoader{}))
}

// gitDirLoader resolves a local path — working tree, bare repo, or linked
// worktree — to its real git directory and serves it to the transport layer.
type gitDirLoader struct{}

func (gitDirLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	dir, err := resolveGitDir(ep.Path)
	if err != nil {
		return nil, transport.ErrRepositoryNotFound
	}
	return filesystem.NewStorage(osfs.New(dir), cache.NewObjectLRUDefault()), nil
}

// resolveGitDir maps path to the directory that actually holds refs and
// objects: path itself for a bare repo, path/.git for a working tree, and the
// main clone's git dir for a linked worktree (whose .git is a file pointing
// at a per-worktree dir that in turn defers to a commondir).
func resolveGitDir(path string) (string, error) {
	dotGit := filepath.Join(path, ".git")
	if fi, err := os.Stat(dotGit); err == nil {
		if fi.IsDir() {
			path = dotGit
		} else {
			raw, err := os.ReadFile(dotGit)
			if err != nil {
				return "", err
			}
			target := strings.TrimSpace(strings.TrimPrefix(string(raw), "gitdir:"))
			if !filepath.IsAbs(target) {
				target = filepath.Join(path, target)
			}
			path = target
		}
	}
	if raw, err := os.ReadFile(filepath.Join(path, "commondir")); err == nil {
		common := strings.TrimSpace(string(raw))
		if !filepath.IsAbs(common) {
			common = filepath.Join(path, common)
		}
		path = common
	}
	if _, err := os.Stat(filepath.Join(path, "objects")); err != nil {
		return "", fmt.Errorf("%s is not a git repository", path)
	}
	return filepath.Clean(path), nil
}

// nameRE restricts repo names to a single lowercase RFC 1035 DNS label, so
// that <label>-<repo> is a legal label too. Names may contain hyphens; the
// proxy resolves that split against the registry.
var nameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidateName rejects repo names that can't serve as a DNS label.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid repo name %q: must be a lowercase DNS label (letters, digits, hyphens)", name)
	}
	return nil
}

// TreeEntry is one blob in a commit's recursive tree listing.
type TreeEntry struct {
	Mode string
	Type string
	OID  string
	Path string
}

// Manager owns the directory of mirror clones.
type Manager struct {
	dir string
}

// NewManager returns a Manager rooted at reposDir (created lazily).
func NewManager(reposDir string) *Manager {
	return &Manager{dir: reposDir}
}

// Repo is a handle to one mirror clone.
type Repo struct {
	Name string
	Path string
}

func (m *Manager) repoPath(name string) string {
	return filepath.Join(m.dir, name+".git")
}

// Open returns a handle for an already-registered repo.
func (m *Manager) Open(name string) Repo {
	return Repo{Name: name, Path: m.repoPath(name)}
}

// mirrorRefSpec keeps refs/heads in sync on Fetch, unlike a plain bare
// clone whose fetch refspec is unset.
const mirrorRefSpec = config.RefSpec("+refs/*:refs/*")

// Add registers a repo by mirror-cloning source (a local path or clone URL).
// Any mirror already on disk at that name is replaced: the repos table is the
// authority on name ownership (callers check it before Add), so a pre-existing
// clone here is an orphan — a deletion whose on-disk cleanup failed or raced a
// fetch — and must not block re-registration. The clone lands in a temp dir
// and is swapped into place so a failed clone never destroys an existing
// mirror. progress, when non-nil, receives the transport's human-readable
// clone progress ("Counting objects: …").
func (m *Manager) Add(ctx context.Context, name, source string, progress io.Writer) (Repo, error) {
	if err := ValidateName(name); err != nil {
		return Repo{}, err
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return Repo{}, fmt.Errorf("create repos dir: %w", err)
	}
	// Absolutize local paths so the clone keeps working regardless of the
	// server's cwd.
	if st, err := os.Stat(source); err == nil && st.IsDir() {
		abs, err := filepath.Abs(source)
		if err != nil {
			return Repo{}, fmt.Errorf("resolve source path: %w", err)
		}
		source = abs
	}
	// The dot prefix keeps temp clones out of the `<name>.git` namespace
	// (names are DNS labels, which can't start with a dot).
	tmp, err := os.MkdirTemp(m.dir, "."+name+"-*.tmp")
	if err != nil {
		return Repo{}, fmt.Errorf("create clone dir: %w", err)
	}
	_, err = git.PlainCloneContext(ctx, tmp, true, &git.CloneOptions{
		URL:      source,
		Mirror:   true,
		Progress: progress,
	})
	if errors.Is(err, transport.ErrEmptyRemoteRepository) {
		// A source with no commits yet: set up the empty mirror by hand
		// (clone cleans up after itself on error) so refs arrive on a later
		// Fetch, matching `git clone --mirror` of an empty repo.
		err = initEmptyMirror(tmp, source)
	}
	if err != nil {
		os.RemoveAll(tmp)
		return Repo{}, fmt.Errorf("clone %s: %w", source, err)
	}
	dest := m.repoPath(name)
	if err := os.RemoveAll(dest); err != nil {
		os.RemoveAll(tmp)
		return Repo{}, fmt.Errorf("remove stale mirror: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.RemoveAll(tmp)
		return Repo{}, fmt.Errorf("move clone into place: %w", err)
	}
	return Repo{Name: name, Path: dest}, nil
}

func initEmptyMirror(dest, source string) error {
	repo, err := git.PlainInit(dest, true)
	if err != nil {
		return err
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name:   "origin",
		URLs:   []string{source},
		Fetch:  []config.RefSpec{mirrorRefSpec},
		Mirror: true,
	})
	return err
}

// Remove deletes the mirror clone for name. A missing clone is not an
// error.
func (m *Manager) Remove(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	return os.RemoveAll(m.repoPath(name))
}

func (r Repo) open() (*git.Repository, error) {
	repo, err := git.PlainOpen(r.Path)
	if err != nil {
		return nil, fmt.Errorf("open repo %s: %w", r.Path, err)
	}
	return repo, nil
}

// Fetch updates all refs from origin, pruning deleted ones.
// Fetch mirrors the source's refs into the clone, pruning refs the source
// no longer advertises. changed reports whether any ref moved: when the
// advertisement matches the mirror exactly, the fetch and prune are skipped
// entirely — callers use this to avoid graph work on quiet polls.
func (r Repo) Fetch(ctx context.Context) (changed bool, err error) {
	repo, err := r.open()
	if err != nil {
		return false, err
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		return false, fmt.Errorf("fetch %s: %w", r.Name, err)
	}
	// List first: the advertised refs decide whether anything moved, drive
	// pruning below, and let an unreachable source fail before any ref is
	// touched.
	advertised, err := remote.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return false, nil
		}
		return false, fmt.Errorf("fetch %s: %w", r.Name, err)
	}
	live := make(map[plumbing.ReferenceName]plumbing.Hash, len(advertised))
	for _, ref := range advertised {
		if ref.Type() == plumbing.HashReference && strings.HasPrefix(string(ref.Name()), "refs/") {
			live[ref.Name()] = ref.Hash()
		}
	}
	local, err := r.hashRefs(repo)
	if err != nil {
		return false, fmt.Errorf("fetch %s: %w", r.Name, err)
	}
	if maps.Equal(local, live) {
		return false, nil
	}
	err = remote.FetchContext(ctx, &git.FetchOptions{Force: true})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return false, fmt.Errorf("fetch %s: %w", r.Name, err)
	}
	// Prune: drop local refs the remote no longer advertises (the --prune
	// half of `git fetch --prune` under a +refs/*:refs/* mirror refspec).
	after, err := r.hashRefs(repo)
	if err != nil {
		return true, fmt.Errorf("fetch %s: %w", r.Name, err)
	}
	for name := range after {
		if _, ok := live[name]; !ok {
			if err := repo.Storer.RemoveReference(name); err != nil {
				return true, fmt.Errorf("fetch %s: prune %s: %w", r.Name, name, err)
			}
		}
	}
	return true, nil
}

// hashRefs returns the repo's refs/* hash references by name.
func (r Repo) hashRefs(repo *git.Repository) (map[plumbing.ReferenceName]plumbing.Hash, error) {
	iter, err := repo.References()
	if err != nil {
		return nil, err
	}
	refs := map[plumbing.ReferenceName]plumbing.Hash{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() == plumbing.HashReference && strings.HasPrefix(string(ref.Name()), "refs/") {
			refs[ref.Name()] = ref.Hash()
		}
		return nil
	})
	return refs, err
}

// ResolveRef resolves a branch, tag, or (abbreviated) sha to a full commit
// sha, fetching once and retrying if the ref isn't known locally yet.
func (r Repo) ResolveRef(ctx context.Context, ref string) (string, error) {
	sha, err := r.revParse(ctx, ref)
	if err == nil {
		return sha, nil
	}
	if _, ferr := r.Fetch(ctx); ferr != nil {
		return "", fmt.Errorf("resolve %q: %w (fetch failed: %v)", ref, err, ferr)
	}
	sha, err = r.revParse(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", ref, err)
	}
	return sha, nil
}

func (r Repo) revParse(_ context.Context, ref string) (string, error) {
	repo, err := r.open()
	if err != nil {
		return "", err
	}
	hash, err := resolveCommit(repo, ref)
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}

// resolveCommit resolves ref (branch, tag, HEAD, full or abbreviated sha)
// and peels annotated tags down to the commit they tag.
func resolveCommit(repo *git.Repository, ref string) (plumbing.Hash, error) {
	h, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return plumbing.ZeroHash, err
	}
	hash := *h
	for {
		obj, err := repo.Object(plumbing.AnyObject, hash)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		switch o := obj.(type) {
		case *object.Commit:
			return hash, nil
		case *object.Tag:
			hash = o.Target
		default:
			return plumbing.ZeroHash, fmt.Errorf("%q is not a commit", ref)
		}
	}
}

// CommitMeta is display metadata for one commit.
type CommitMeta struct {
	AuthorName  string
	AuthorEmail string
}

// CommitMeta returns the author of sha.
func (r Repo) CommitMeta(_ context.Context, sha string) (CommitMeta, error) {
	repo, err := r.open()
	if err != nil {
		return CommitMeta{}, err
	}
	commit, err := commitAt(repo, sha)
	if err != nil {
		return CommitMeta{}, fmt.Errorf("commit meta %s: %w", sha, err)
	}
	return CommitMeta{AuthorName: commit.Author.Name, AuthorEmail: commit.Author.Email}, nil
}

func commitAt(repo *git.Repository, sha string) (*object.Commit, error) {
	hash, err := resolveCommit(repo, sha)
	if err != nil {
		return nil, err
	}
	return repo.CommitObject(hash)
}

// IsBranch reports whether name is a branch head in the mirror.
func (r Repo) IsBranch(_ context.Context, name string) bool {
	repo, err := r.open()
	if err != nil {
		return false
	}
	_, err = repo.Reference(plumbing.NewBranchReferenceName(name), false)
	return err == nil
}

// BranchesPointingAt returns the branches whose tip is sha, in refname
// order. Commits that are no longer any branch's tip return none.
func (r Repo) BranchesPointingAt(_ context.Context, sha string) ([]string, error) {
	tips, err := r.branchTips(sha)
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, t := range tips {
		branches = append(branches, t.Branch)
	}
	return branches, nil
}

// BranchTip is one branch head in the mirror.
type BranchTip struct {
	Branch string
	SHA    string
}

// BranchTips returns every branch head in the mirror, in refname order.
func (r Repo) BranchTips(_ context.Context) ([]BranchTip, error) {
	return r.branchTips("")
}

// branchTips lists branch heads sorted by refname; a non-empty at filters to
// branches whose tip is exactly that commit.
func (r Repo) branchTips(at string) ([]BranchTip, error) {
	repo, err := r.open()
	if err != nil {
		return nil, err
	}
	var want plumbing.Hash
	if at != "" {
		h, err := resolveCommit(repo, at)
		if err != nil {
			return nil, fmt.Errorf("branches pointing at %s: %w", at, err)
		}
		want = h
	}
	iter, err := repo.Branches()
	if err != nil {
		return nil, err
	}
	var tips []BranchTip
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if at != "" && ref.Hash() != want {
			return nil
		}
		tips = append(tips, BranchTip{Branch: ref.Name().Short(), SHA: ref.Hash().String()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(tips, func(i, j int) bool { return tips[i].Branch < tips[j].Branch })
	return tips, nil
}

// UnreachableSHAs returns the subset of candidates no longer reachable from
// any of tips — commits that have fallen out of the repo's branch history
// because an unmerged branch was deleted or a tip was force-pushed past them.
// Every argument is a full or abbreviated object name; tips are the surviving
// branch heads (see BranchTips), gathered after a fetch --prune.
//
// Reachability is one ancestry walk from the tips; a candidate the walk never
// visits is unreachable. Candidates the mirror has already gc'd are skipped
// rather than reported, keeping the result conservative — they're left for a
// later pass. With no surviving tips every present candidate is unreachable.
// Candidates come back in input order; each match is returned once.
func (r Repo) UnreachableSHAs(ctx context.Context, candidates, tips []string) ([]string, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	repo, err := r.open()
	if err != nil {
		return nil, err
	}

	type candidate struct {
		orig string
		hash plumbing.Hash
	}
	var cands []candidate
	seen := make(map[string]bool, len(candidates))
	remaining := make(map[plumbing.Hash]int)
	for _, c := range candidates {
		if seen[c] {
			continue // dedupe; each candidate reported at most once
		}
		seen[c] = true
		h, err := resolveCommit(repo, c)
		if err != nil {
			continue // gc'd or unknown: conservatively not unreachable
		}
		cands = append(cands, candidate{orig: c, hash: h})
		remaining[h]++
	}
	if len(cands) == 0 {
		return nil, nil
	}

	// Walk ancestry from every surviving tip, stopping early once all
	// candidates have been proven reachable.
	reachable := make(map[plumbing.Hash]bool)
	var queue []plumbing.Hash
	for _, t := range tips {
		if h, err := resolveCommit(repo, t); err == nil {
			queue = append(queue, h)
		}
	}
	for len(queue) > 0 && len(remaining) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		h := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if reachable[h] {
			continue
		}
		reachable[h] = true
		delete(remaining, h)
		commit, err := repo.CommitObject(h)
		if err != nil {
			continue // shallow or corrupt edge: treat as a dead end
		}
		queue = append(queue, commit.ParentHashes...)
	}

	var unreachable []string
	for _, c := range cands {
		if !reachable[c.hash] {
			unreachable = append(unreachable, c.orig)
		}
	}
	return unreachable, nil
}

// ReadFile returns the content of path at sha.
func (r Repo) ReadFile(_ context.Context, sha, path string) ([]byte, error) {
	repo, err := r.open()
	if err != nil {
		return nil, err
	}
	commit, err := commitAt(repo, sha)
	if err != nil {
		return nil, fmt.Errorf("read %s:%s: %w", sha, path, err)
	}
	f, err := commit.File(path)
	if err != nil {
		return nil, fmt.Errorf("read %s:%s: %w", sha, path, err)
	}
	rd, err := f.Blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("read %s:%s: %w", sha, path, err)
	}
	defer rd.Close()
	content, err := io.ReadAll(rd)
	if err != nil {
		return nil, fmt.Errorf("read %s:%s: %w", sha, path, err)
	}
	return content, nil
}

// LsTree lists all blobs at sha, optionally restricted to a subtree.
// Submodule (commit) entries are skipped — submodules are out of scope.
// Entries come back in git's sorted tree order, which content hashing relies
// on (trees store entries sorted, and the walk is depth-first).
func (r Repo) LsTree(_ context.Context, sha, sub string) ([]TreeEntry, error) {
	repo, err := r.open()
	if err != nil {
		return nil, err
	}
	commit, err := commitAt(repo, sha)
	if err != nil {
		return nil, fmt.Errorf("ls-tree %s: %w", sha, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("ls-tree %s: %w", sha, err)
	}
	walker := object.NewTreeWalker(tree, true, nil)
	defer walker.Close()
	var entries []TreeEntry
	for {
		name, entry, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ls-tree %s: %w", sha, err)
		}
		if entry.Mode == filemode.Dir || entry.Mode == filemode.Submodule {
			continue
		}
		if sub != "" && sub != "." && name != sub && !strings.HasPrefix(name, sub+"/") {
			continue
		}
		entries = append(entries, TreeEntry{
			Mode: fmt.Sprintf("%06o", uint32(entry.Mode)),
			Type: "blob",
			OID:  entry.Hash.String(),
			Path: name,
		})
	}
	return entries, nil
}

// FirstParentAncestry returns up to limit ancestors of sha (excluding sha
// itself), nearest first, following only first parents. Merge commits
// therefore walk the branch that received the merge.
func (r Repo) FirstParentAncestry(ctx context.Context, sha string, limit int) ([]string, error) {
	repo, err := r.open()
	if err != nil {
		return nil, err
	}
	commit, err := commitAt(repo, sha)
	if err != nil {
		return nil, fmt.Errorf("ancestry of %s: %w", sha, err)
	}
	var ancestors []string
	for len(ancestors) < limit && commit.NumParents() > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		parent := commit.ParentHashes[0]
		ancestors = append(ancestors, parent.String())
		commit, err = repo.CommitObject(parent)
		if err != nil {
			return nil, fmt.Errorf("ancestry of %s: %w", sha, err)
		}
	}
	return ancestors, nil
}

// Archive extracts the full tree at sha into dest. Stateless by design: it
// reads only the object database and needs no cleanup bookkeeping, so
// concurrent extractions are always safe.
func (r Repo) Archive(ctx context.Context, sha, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create archive dest: %w", err)
	}
	root, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("archive dest: %w", err)
	}
	repo, err := r.open()
	if err != nil {
		return err
	}
	commit, err := commitAt(repo, sha)
	if err != nil {
		return fmt.Errorf("archive %s: %w", sha, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("archive %s: %w", sha, err)
	}
	walker := object.NewTreeWalker(tree, true, nil)
	defer walker.Close()
	for {
		name, entry, err := walker.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("archive %s: %w", sha, err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(name))
		switch entry.Mode {
		case filemode.Dir, filemode.Submodule:
			err = os.MkdirAll(target, 0o755)
		case filemode.Symlink:
			err = extractSymlink(repo, entry.Hash, root, target)
		default:
			err = extractBlob(repo, entry.Hash, target, entry.Mode)
		}
		if err != nil {
			return fmt.Errorf("archive %s: %s: %w", sha, name, err)
		}
	}
}

// extractSymlink recreates a committed symlink whose blob content is the link
// target. The target is fully attacker-controlled (a repo owner commits it), so
// it is validated before creation the same way store.ExtractTar validates tar
// symlinks: an absolute target always escapes and is rejected, and a relative
// target is refused if it resolves outside root. Without this a committed
// symlink to /etc/passwd (or a ../ escape) would be recreated verbatim and then
// followed by the frontend file server / artifact publisher, disclosing host
// files to unauthenticated preview visitors.
func extractSymlink(repo *git.Repository, hash plumbing.Hash, root, target string) error {
	linkTarget, err := blobContent(repo, hash)
	if err != nil {
		return err
	}
	link := string(linkTarget)
	if filepath.IsAbs(link) || strings.HasPrefix(link, "/") {
		return fmt.Errorf("absolute symlink target %q", link)
	}
	resolved := filepath.Join(filepath.Dir(target), filepath.FromSlash(link))
	if !withinRoot(root, resolved) {
		return fmt.Errorf("symlink target %q escapes archive root", link)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	os.Remove(target)
	return os.Symlink(link, target)
}

// withinRoot reports whether p is root itself or lies beneath it. Both must be
// clean absolute paths (Archive derives target from an absolute root).
func withinRoot(root, p string) bool {
	return p == root || strings.HasPrefix(p, root+string(os.PathSeparator))
}

func extractBlob(repo *git.Repository, hash plumbing.Hash, target string, mode filemode.FileMode) error {
	blob, err := repo.BlobObject(hash)
	if err != nil {
		return err
	}
	rd, err := blob.Reader()
	if err != nil {
		return err
	}
	defer rd.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	perm := os.FileMode(0o644)
	if mode == filemode.Executable {
		perm = 0o755
	}
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, rd); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func blobContent(repo *git.Repository, hash plumbing.Hash) ([]byte, error) {
	blob, err := repo.BlobObject(hash)
	if err != nil {
		return nil, err
	}
	rd, err := blob.Reader()
	if err != nil {
		return nil, err
	}
	defer rd.Close()
	return io.ReadAll(rd)
}
