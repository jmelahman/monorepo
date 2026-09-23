// Package hashkey computes the content-address of a commit's frontend and
// backend partitions. The digest covers the relevant preview.toml section
// (so build-command changes bust the cache) plus the filtered git tree
// entries (mode, blob OID, path) — never file contents, which git has
// already hashed.
package hashkey

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/manifest"
)

// Frontend hashes the frontend partition: entries under fe.Path. extra is
// environment context resolved outside the manifest section (the repo's
// devcontainer, when this side would use it); nil contributes nothing, so
// digests predate-and-postdate identically for repos without one.
func Frontend(fe manifest.Frontend, extra []byte, entries []gitrepo.TreeEntry) (string, error) {
	sel := filter(entries, func(p string) bool { return under(p, fe.Path) })
	if len(sel) == 0 {
		return "", fmt.Errorf("frontend.path %q matches no files at this commit", fe.Path)
	}
	return digest(fe, extra, sel)
}

// Backend hashes the backend partition: entries under be.Path, minus the
// frontend subtree, minus be.Exclude patterns.
func Backend(be manifest.Backend, frontendPath string, extra []byte, entries []gitrepo.TreeEntry) (string, error) {
	sel := filter(entries, func(p string) bool {
		return under(p, be.Path) && !under(p, frontendPath) && !excluded(p, be.Exclude)
	})
	if len(sel) == 0 {
		return "", fmt.Errorf("backend partition matches no files at this commit (path %q)", be.Path)
	}
	return digest(be, extra, sel)
}

// Artifact hashes a downloadable-artifact partition: entries under a.Path,
// minus the frontend subtree, minus a.Exclude patterns — the same partition
// rule as Backend. The artifact's name is deliberately not part of the
// digest: renaming an entry with an unchanged section doesn't rebuild.
func Artifact(a manifest.Artifact, frontendPath string, extra []byte, entries []gitrepo.TreeEntry) (string, error) {
	sel := filter(entries, func(p string) bool {
		return under(p, a.Path) && !under(p, frontendPath) && !excluded(p, a.Exclude)
	})
	if len(sel) == 0 {
		return "", fmt.Errorf("artifact partition matches no files at this commit (path %q)", a.Path)
	}
	return digest(a, extra, sel)
}

func filter(entries []gitrepo.TreeEntry, keep func(string) bool) []gitrepo.TreeEntry {
	var out []gitrepo.TreeEntry
	for _, e := range entries {
		if keep(e.Path) {
			out = append(out, e)
		}
	}
	return out
}

// under reports whether slash-path p is inside root ("." = whole tree).
func under(p, root string) bool {
	return root == "." || p == root || strings.HasPrefix(p, root+"/")
}

// excluded matches p against exclude patterns: "dir/" prefixes, or glob
// patterns applied to the full path and the base name ("*.md" matches
// markdown anywhere in the tree).
func excluded(p string, patterns []string) bool {
	for _, pat := range patterns {
		if dir, ok := strings.CutSuffix(pat, "/"); ok {
			if under(p, dir) {
				return true
			}
			continue
		}
		if ok, _ := path.Match(pat, p); ok {
			return true
		}
		if ok, _ := path.Match(pat, path.Base(p)); ok {
			return true
		}
	}
	return false
}

// digest hashes the manifest section (canonical JSON — struct field order is
// fixed), the extra environment blob when present, then the sorted tree
// entries. Entries arrive in git ls-tree order, which is already
// deterministic.
func digest(section any, extra []byte, entries []gitrepo.TreeEntry) (string, error) {
	h := sha256.New()
	cfg, err := json.Marshal(section)
	if err != nil {
		return "", fmt.Errorf("marshal manifest section: %w", err)
	}
	h.Write(cfg)
	h.Write([]byte{0})
	if len(extra) > 0 {
		h.Write(extra)
		h.Write([]byte{0})
	}
	for _, e := range entries {
		fmt.Fprintf(h, "%s %s %s\t%s\x00", e.Mode, e.Type, e.OID, e.Path)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
