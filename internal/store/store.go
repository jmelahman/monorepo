// Package store owns on-disk content-addressed artifacts and mutable backend
// state directories.
//
// Every publish builds into tmp/ and lands with a same-filesystem os.Rename,
// so "the directory exists" always means "the content is complete" — a crash
// mid-build can never leave a half-written artifact behind. tmp/ is never
// authoritative and is swept at startup.
package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// artifactListTTL bounds staleness of cached artifact listings. A published
// artifact directory never changes (content-addressed, landed by rename), so
// the TTL only bounds how long a listing can outlive eviction of its
// directory — it exists so deploy list views don't re-walk every artifact
// directory on every poll.
const artifactListTTL = 10 * time.Second

// Store manages artifacts under artifactsDir, state under stateDir, and
// staging under tmpDir. All three must live on the same filesystem so
// renames are atomic.
type Store struct {
	artifactsDir string
	stateDir     string
	tmpDir       string

	// tier is the optional durable artifact backend. nil keeps local disk
	// authoritative (today's single-node default); non-nil makes local disk a
	// cache that Hydrate refills and EvictCacheToWatermark reclaims.
	tier ArtifactTier
	// maxExtractBytes caps the decompressed size ExtractTar writes before
	// aborting with ErrArchiveTooLarge — the ceiling on an untrusted upload's
	// (or a hydrated object's) expansion. 0 disables the cap. Set via
	// SetMaxExtractBytes.
	maxExtractBytes int64
	// sf deduplicates concurrent hydrations of the same content-address.
	sf singleflight.Group

	mu       sync.Mutex
	artLists map[string]artListEntry
	// accessed records the last time a resident artifact side was served or
	// hydrated, so cache eviction can reclaim the coldest first. Keyed by
	// cacheKey(repo, side, hash). Absent entries fall back to directory mtime.
	accessed map[string]time.Time
	// sizeCache memoizes the recursive byte size of an artifact directory,
	// keyed by path and validated by mtime. Published dirs are immutable (they
	// land by rename, and a rebuild-overwrite changes the mtime), so a hit is
	// always accurate — this keeps a cache sweep from re-walking every
	// artifact's tens of thousands of files on every pass.
	sizeCache map[string]sizeCacheEntry
}

type sizeCacheEntry struct {
	mtime time.Time
	size  int64
}

type artListEntry struct {
	files   []ArtifactFile
	expires time.Time
}

// New returns a Store over the three roots (created lazily).
func New(artifactsDir, stateDir, tmpDir string) *Store {
	return &Store{
		artifactsDir: artifactsDir,
		stateDir:     stateDir,
		tmpDir:       tmpDir,
		artLists:     map[string]artListEntry{},
		accessed:     map[string]time.Time{},
		sizeCache:    map[string]sizeCacheEntry{},
	}
}

// SetMaxExtractBytes sets the cap ExtractTar applies to the decompressed size
// of an extracted tar (0 disables it). It backs both the upload and hydrate
// paths, so a single knob bounds every extraction this Store performs.
func (s *Store) SetMaxExtractBytes(n int64) {
	s.maxExtractBytes = n
}

// ExtractTar extracts a tar(.gz) stream into destDir under this Store's
// configured decompression cap (see SetMaxExtractBytes). It is the entry point
// upload and hydrate use so both honor the same ceiling; the package-level
// ExtractTar takes an explicit cap for callers that manage their own.
func (s *Store) ExtractTar(r io.Reader, destDir string) (int64, error) {
	return ExtractTar(r, destDir, s.maxExtractBytes)
}

// ExtractTarPayload is ExtractTar under the payload symlink policy — backend
// artifact trees only. See the package-level ExtractTarPayload for why.
func (s *Store) ExtractTarPayload(r io.Reader, destDir string) (int64, error) {
	return ExtractTarPayload(r, destDir, s.maxExtractBytes)
}

// FrontendDir returns the artifact path for a frontend hash.
func (s *Store) FrontendDir(repo, hash string) string {
	return filepath.Join(s.artifactsDir, repo, "fe", hash)
}

// BackendDir returns the artifact path for a backend hash.
func (s *Store) BackendDir(repo, hash string) string {
	return filepath.Join(s.artifactsDir, repo, "be", hash)
}

// ArtifactDir returns the artifact path for a downloadable-artifact hash.
func (s *Store) ArtifactDir(repo, hash string) string {
	return filepath.Join(s.artifactsDir, repo, "dl", hash)
}

// HasFrontend reports whether the frontend artifact is published.
func (s *Store) HasFrontend(repo, hash string) bool { return dirExists(s.FrontendDir(repo, hash)) }

// HasBackend reports whether the backend artifact is published.
func (s *Store) HasBackend(repo, hash string) bool { return dirExists(s.BackendDir(repo, hash)) }

// HasArtifact reports whether the downloadable artifact is published.
func (s *Store) HasArtifact(repo, hash string) bool { return dirExists(s.ArtifactDir(repo, hash)) }

// RemoveRepo deletes every published artifact and state directory for a
// repo (repo deletion).
func (s *Store) RemoveRepo(repo string) error {
	if err := os.RemoveAll(filepath.Join(s.artifactsDir, repo)); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.stateDir, repo))
}

// RemoveBackend deletes a backend artifact's published build and its mutable
// state directory. Called when the last deploy referencing be_hash is
// removed; artifacts are content-addressed and shared, so the caller must
// confirm no surviving deploy still uses the hash first.
func (s *Store) RemoveBackend(repo, beHash string) error {
	if err := os.RemoveAll(s.BackendDir(repo, beHash)); err != nil {
		return err
	}
	return os.RemoveAll(s.StateDirPath(repo, beHash))
}

// RemoveFrontend deletes a published frontend artifact (static bundle or
// process-mode build), subject to the same shared-hash caution as
// RemoveBackend.
func (s *Store) RemoveFrontend(repo, feHash string) error {
	return os.RemoveAll(s.FrontendDir(repo, feHash))
}

// RemoveArtifact deletes a published downloadable artifact, subject to the
// same shared-hash caution as RemoveBackend.
func (s *Store) RemoveArtifact(repo, hash string) error {
	return os.RemoveAll(s.ArtifactDir(repo, hash))
}

// PublishFrontend moves srcDir into place as the frontend artifact.
// Idempotent: if the artifact already exists it is kept as-is unless
// overwrite is set (the --rebuild path).
func (s *Store) PublishFrontend(repo, hash, srcDir string, overwrite bool) error {
	return s.publish(s.FrontendDir(repo, hash), srcDir, overwrite)
}

// PublishBackend moves srcDir into place as the backend artifact.
func (s *Store) PublishBackend(repo, hash, srcDir string, overwrite bool) error {
	return s.publish(s.BackendDir(repo, hash), srcDir, overwrite)
}

// PublishArtifactDir publishes an already-staged flat downloadable-artifact
// directory (every entry a regular file addressed by base name), landing it
// with the same atomic rename as every other publish. Unlike
// PublishArtifactFiles it does no per-file staging — the caller has already
// materialized the final layout, as when a downloadable artifact is hydrated
// from the durable tier.
func (s *Store) PublishArtifactDir(repo, hash, srcDir string, overwrite bool) error {
	return s.publish(s.ArtifactDir(repo, hash), srcDir, overwrite)
}

// PublishArtifactFiles publishes the named build outputs (slash paths
// relative to builtDir) as a downloadable artifact. Files are staged flat —
// downloads are addressed by base name; the manifest rejects duplicate base
// names — then land with the same atomic rename as every other publish.
func (s *Store) PublishArtifactFiles(repo, hash, builtDir string, files []string, overwrite bool) error {
	staging, err := os.MkdirTemp(s.ensureTmp(), "dl-")
	if err != nil {
		return fmt.Errorf("create artifact staging: %w", err)
	}
	defer os.RemoveAll(staging)
	for _, f := range files {
		src := filepath.Join(builtDir, filepath.FromSlash(f))
		// Lstat, not Stat: a declared artifact file must be a genuine build
		// output, never a symlink. os.Stat would follow the link and happily
		// publish the target's content — a committed symlink to a host file
		// (/etc/passwd, cloud credentials) then leaks via artifact download.
		info, err := os.Lstat(src)
		if err != nil {
			return fmt.Errorf("artifact file %q was not produced by the build", f)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact file %q is a symlink", f)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("artifact file %q is not a regular file", f)
		}
		if err := copyFile(src, filepath.Join(staging, filepath.Base(f)), info.Mode().Perm()); err != nil {
			return fmt.Errorf("stage artifact file %q: %w", f, err)
		}
	}
	return s.publish(s.ArtifactDir(repo, hash), staging, overwrite)
}

// ArtifactFile describes one downloadable file in a published artifact.
type ArtifactFile struct {
	Name string
	Size int64
}

// ListArtifactFiles returns a published artifact's files sorted by name; nil
// when the artifact directory doesn't exist (unbuilt or evicted). Listings
// are served from a short-TTL cache: published directories are immutable, so
// a hit is always accurate for a directory that still exists, and a missing
// directory (never cached) is re-probed every call so files appear as soon
// as the artifact publishes. Callers must not mutate the returned slice.
func (s *Store) ListArtifactFiles(repo, hash string) []ArtifactFile {
	key := repo + "\x00" + hash
	s.mu.Lock()
	if e, ok := s.artLists[key]; ok && time.Now().Before(e.expires) {
		s.mu.Unlock()
		return e.files
	}
	s.mu.Unlock()

	entries, err := os.ReadDir(s.ArtifactDir(repo, hash))
	if err != nil {
		return nil
	}
	out := make([]ArtifactFile, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, ArtifactFile{Name: e.Name(), Size: info.Size()})
	}
	s.mu.Lock()
	s.artLists[key] = artListEntry{files: out, expires: time.Now().Add(artifactListTTL)}
	s.mu.Unlock()
	return out
}

func (s *Store) publish(dest, srcDir string, overwrite bool) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create artifact parent: %w", err)
	}
	if dirExists(dest) {
		if !overwrite {
			os.RemoveAll(srcDir)
			return nil
		}
		// Move the old artifact aside first so the window with no artifact
		// present is as small as possible, then discard it.
		trash, err := os.MkdirTemp(s.ensureTmp(), "trash-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(trash)
		if err := os.Rename(dest, filepath.Join(trash, "old")); err != nil {
			return fmt.Errorf("displace old artifact: %w", err)
		}
	}
	if err := os.Rename(srcDir, dest); err != nil {
		// A concurrent publisher of the same hash may have won the rename;
		// content-addressing makes their copy equivalent to ours.
		if dirExists(dest) {
			os.RemoveAll(srcDir)
			return nil
		}
		return fmt.Errorf("publish artifact: %w", err)
	}
	return nil
}

// NewScratchDir creates a staging directory under tmp/. The cleanup func is
// safe to call after a successful rename out of the scratch dir.
func (s *Store) NewScratchDir(purpose string) (string, func(), error) {
	dir, err := os.MkdirTemp(s.ensureTmp(), purpose+"-")
	if err != nil {
		return "", nil, fmt.Errorf("create scratch dir: %w", err)
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}

// StateDirPath returns the mutable state directory for a backend artifact.
func (s *Store) StateDirPath(repo, beHash string) string {
	return filepath.Join(s.stateDir, repo, beHash)
}

// HasStateDir reports whether the state dir exists.
func (s *Store) HasStateDir(repo, beHash string) bool {
	return dirExists(s.StateDirPath(repo, beHash))
}

// InitFreshStateDir creates an empty state dir (the no-deployed-ancestor
// case). Idempotent.
func (s *Store) InitFreshStateDir(repo, beHash string) error {
	return os.MkdirAll(s.StateDirPath(repo, beHash), 0o755)
}

// ForkStateDir copies the src backend's state dir to dst: recursive copy
// into tmp/, then one atomic rename. No-op if dst already exists. The caller
// is responsible for quiescing whatever writes to src first.
func (s *Store) ForkStateDir(repo, srcBeHash, dstBeHash string) error {
	dst := s.StateDirPath(repo, dstBeHash)
	if dirExists(dst) {
		return nil
	}
	src := s.StateDirPath(repo, srcBeHash)
	if !dirExists(src) {
		return fmt.Errorf("fork state: source state dir for %s does not exist", srcBeHash)
	}
	staging, err := os.MkdirTemp(s.ensureTmp(), "fork-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	copied := filepath.Join(staging, "state")
	if err := copyDir(src, copied); err != nil {
		return fmt.Errorf("fork state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(copied, dst); err != nil {
		if dirExists(dst) {
			return nil
		}
		return fmt.Errorf("fork state: %w", err)
	}
	return nil
}

// SweepTmp removes staging entries older than maxAge — orphans from a crash.
func (s *Store) SweepTmp(maxAge time.Duration) error {
	entries, err := os.ReadDir(s.tmpDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.RemoveAll(filepath.Join(s.tmpDir, e.Name()))
		}
	}
	return nil
}

func (s *Store) ensureTmp() string {
	os.MkdirAll(s.tmpDir, 0o755)
	return s.tmpDir
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// copyDir recursively copies src to dst, preserving file modes. Regular
// files, directories, and symlinks only.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			return copyFile(p, target, info.Mode().Perm())
		default:
			return fmt.Errorf("unsupported file type %s in state dir: %s", info.Mode(), p)
		}
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
