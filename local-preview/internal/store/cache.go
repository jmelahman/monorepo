package store

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// cacheKey identifies a resident artifact side for the access-time map.
func cacheKey(repo, side, hash string) string {
	return repo + "\x00" + side + "\x00" + hash
}

// NoteAccess records that an artifact side was just served or hydrated, so
// cache eviction reclaims the coldest artifacts first. Cheap and best-effort:
// a missing entry falls back to the directory's mtime.
func (s *Store) NoteAccess(repo, side, hash string) {
	if hash == "" {
		return
	}
	s.mu.Lock()
	s.accessed[cacheKey(repo, side, hash)] = time.Now()
	s.mu.Unlock()
}

// cacheEntry is one resident artifact side considered for eviction.
type cacheEntry struct {
	repo, side, hash string
	dir              string
	bytes            int64
	born             time.Time // publish time (dir mtime), the age-guard basis
	recency          time.Time // max(born, last access), the eviction order
}

// residentArtifacts enumerates every resident artifact side across all repos
// and the three sides, with its on-disk size and recency.
func (s *Store) residentArtifacts() []cacheEntry {
	var out []cacheEntry
	repos, err := os.ReadDir(s.artifactsDir)
	if err != nil {
		return nil
	}
	for _, repoEnt := range repos {
		if !repoEnt.IsDir() {
			continue
		}
		repo := repoEnt.Name()
		for _, side := range []string{"fe", "be", "dl"} {
			sideDir := filepath.Join(s.artifactsDir, repo, side)
			hashes, err := os.ReadDir(sideDir)
			if err != nil {
				continue
			}
			for _, h := range hashes {
				if !h.IsDir() {
					continue
				}
				dir := filepath.Join(sideDir, h.Name())
				born := time.Time{}
				if info, err := h.Info(); err == nil {
					born = info.ModTime()
				}
				recency := born
				s.mu.Lock()
				if at, ok := s.accessed[cacheKey(repo, side, h.Name())]; ok && at.After(recency) {
					recency = at
				}
				s.mu.Unlock()
				out = append(out, cacheEntry{
					repo:    repo,
					side:    side,
					hash:    h.Name(),
					dir:     dir,
					bytes:   s.cachedDirSize(dir, born),
					born:    born,
					recency: recency,
				})
			}
		}
	}
	return out
}

// ResidentArtifactBytes totals the bytes every resident artifact side occupies
// on local disk — the cache footprint that EvictCacheToWatermark trims and that
// usage reporting labels as resident (as opposed to durable-tier) bytes.
func (s *Store) ResidentArtifactBytes() int64 {
	var total int64
	for _, e := range s.residentArtifacts() {
		total += e.bytes
	}
	return total
}

// EvictCacheToWatermark reclaims local disk by deleting resident artifact
// directories, coldest first, until the resident footprint is at or below
// maxBytes. It treats local disk as a cache of the durable tier: a deleted
// directory is transparently re-hydrated on next serve, so unlike RemoveBackend
// the caller need NOT confirm no deploy still uses the hash — the deploy stays
// live, only its local copy goes away. State directories (mutable, never in the
// tier) are never touched.
//
// Safety rails:
//   - No-op when no durable tier is configured: local disk is then the only
//     copy and evicting it would be data loss, not caching.
//   - An artifact published within minAge is skipped, so a just-built side whose
//     asynchronous persist may still be in flight is never reclaimed before it
//     lands durably (the reconcile pass is the belt-and-suspenders net).
//   - protect(repo, side, hash) skips artifacts the caller pins — e.g. a side
//     with a live process, whose files may be bind-mounted into a running
//     container.
//
// Returns the number of bytes freed.
func (s *Store) EvictCacheToWatermark(maxBytes int64, minAge time.Duration, protect func(repo, side, hash string) bool) (int64, error) {
	if s.tier == nil {
		return 0, nil
	}
	entries := s.residentArtifacts()
	var total int64
	for _, e := range entries {
		total += e.bytes
	}
	if total <= maxBytes {
		return 0, nil
	}
	// Coldest first.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].recency.Before(entries[j].recency)
	})
	cutoff := time.Now().Add(-minAge)
	var freed int64
	for _, e := range entries {
		if total <= maxBytes {
			break
		}
		if minAge > 0 && e.born.After(cutoff) {
			continue // too fresh — persist may still be in flight
		}
		if protect != nil && protect(e.repo, e.side, e.hash) {
			continue
		}
		if err := os.RemoveAll(e.dir); err != nil {
			return freed, err
		}
		s.mu.Lock()
		delete(s.accessed, cacheKey(e.repo, e.side, e.hash))
		delete(s.sizeCache, e.dir)
		s.mu.Unlock()
		total -= e.bytes
		freed += e.bytes
	}
	return freed, nil
}

// SideDir returns the local artifact directory for a (repo, side, hash), where
// side is "fe", "be", or "dl"; empty for an unknown side.
func (s *Store) SideDir(repo, side, hash string) string {
	switch side {
	case "fe":
		return s.FrontendDir(repo, hash)
	case "be":
		return s.BackendDir(repo, hash)
	case "dl":
		return s.ArtifactDir(repo, hash)
	}
	return ""
}

// ResidentSide reports whether an artifact side is resident on local disk and,
// if so, its byte size and regular-file count — what a reconcile pass compares
// against the durable object's recorded metadata to verify integrity, and the
// source it re-persists from when the durable copy is missing.
func (s *Store) ResidentSide(repo, side, hash string) (dir string, bytes int64, files int, resident bool) {
	dir = s.SideDir(repo, side, hash)
	if dir == "" || !dirExists(dir) {
		return "", 0, 0, false
	}
	filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				bytes += info.Size()
				files++
			}
		}
		return nil
	})
	return dir, bytes, files, true
}

// cachedDirSize returns the recursive byte size of an artifact directory,
// memoized against its mtime. A published directory is immutable, so a matching
// mtime guarantees the cached size is still accurate; a rebuild-overwrite lands
// a new directory by rename with a fresh mtime, invalidating the entry.
func (s *Store) cachedDirSize(dir string, mtime time.Time) int64 {
	s.mu.Lock()
	if e, ok := s.sizeCache[dir]; ok && e.mtime.Equal(mtime) {
		s.mu.Unlock()
		return e.size
	}
	s.mu.Unlock()
	size := dirSizeOf(dir)
	s.mu.Lock()
	s.sizeCache[dir] = sizeCacheEntry{mtime: mtime, size: size}
	s.mu.Unlock()
	return size
}

// dirSizeOf sums regular-file sizes under root; 0 if root is missing.
func dirSizeOf(root string) int64 {
	var total int64
	filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
