package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNoLock is returned by Check for a project without a lockfile, which
// is what pins the versions a check regenerates from.
var ErrNoLock = errors.New("no lockfile; run vinder sync")

// errSwap marks a Sync that failed after it began changing the output
// directory, rather than before.
var errSwap = errors.New("replacing the vendor directory")

// Sync regenerates the project's vendor tree in place, updating the lockfile
// to match package.json. It replaces each configured <out>/<key>/
// directory and the manifest, removes the directories of keys the previous
// manifest had and the config no longer does, and leaves anything else in
// the output directory alone.
func (p *Project) Sync(ctx context.Context) error {
	// Generate next to the output so the swap below is a rename.
	gen, err := os.MkdirTemp(p.Dir, ".vinder-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(gen)

	if _, err := p.Generate(ctx, gen, false); err != nil {
		return err
	}

	if err := p.swap(gen); err != nil {
		return fmt.Errorf("%w: %w", errSwap, err)
	}
	p.synced = true
	return nil
}

// swap moves the tree generated in gen into the output directory.
func (p *Project) swap(gen string) error {
	out := p.OutDir()
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, key := range p.staleKeys() {
		if err := os.RemoveAll(filepath.Join(out, key)); err != nil {
			return err
		}
	}
	for _, key := range p.Keys() {
		if err := os.RemoveAll(filepath.Join(out, key)); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(gen, key), filepath.Join(out, key)); err != nil {
			return err
		}
	}
	return os.Rename(filepath.Join(gen, ManifestFile), filepath.Join(out, ManifestFile))
}

// Check regenerates the vendor tree from the locked versions and compares it
// with the one on disk. It returns one line per file that differs, is
// missing or should not be there, as paths relative to the project; none
// means the vendor tree is up to date.
func (p *Project) Check(ctx context.Context) ([]string, error) {
	pm, err := p.packageManager()
	if err != nil {
		return nil, err
	}
	if pm.lockfile(p.Dir) == "" {
		return nil, fmt.Errorf("%s: %w", p.Dir, ErrNoLock)
	}
	gen, err := os.MkdirTemp("", "vinder-check-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(gen)

	if _, err := p.Generate(ctx, gen, true); err != nil {
		return nil, err
	}
	want, err := listFiles(gen, append(p.Keys(), ManifestFile))
	if err != nil {
		return nil, err
	}
	got, err := listFiles(p.OutDir(), append(append(p.Keys(), p.staleKeys()...), ManifestFile))
	if err != nil {
		return nil, err
	}

	all := map[string]bool{}
	for f := range want {
		all[f] = true
	}
	for f := range got {
		all[f] = true
	}
	var diffs []string
	for _, f := range sortedKeys(all) {
		rel := filepath.ToSlash(filepath.Join(p.Config.Out, f))
		switch {
		case !got[f]:
			diffs = append(diffs, "missing  "+rel)
		case !want[f]:
			diffs = append(diffs, "extra    "+rel)
		default:
			a, err := os.ReadFile(filepath.Join(gen, f))
			if err != nil {
				return nil, err
			}
			b, err := os.ReadFile(filepath.Join(p.OutDir(), f))
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(a, b) {
				diffs = append(diffs, "modified "+rel)
			}
		}
	}
	return diffs, nil
}

// staleKeys are the keys in the manifest on disk that the config no longer
// has. Keys that do not name a directory under the output directory are
// dropped: the manifest is a committed file, and sync removes what this
// returns.
func (p *Project) staleKeys() []string {
	old, err := ReadManifest(filepath.Join(p.OutDir(), ManifestFile))
	if err != nil {
		return nil
	}
	var stale []string
	for key := range old.Deps {
		if _, ok := p.Config.Deps[key]; !ok && validKey(key) {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	return stale
}

// listFiles returns the files under root/name for each name, as paths
// relative to root. Names that do not exist are skipped.
func listFiles(root string, names []string) (map[string]bool, error) {
	files := map[string]bool{}
	for _, name := range names {
		err := filepath.WalkDir(filepath.Join(root, name), func(file string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) && file == filepath.Join(root, name) {
				return fs.SkipAll
			}
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, file)
			files[filepath.ToSlash(rel)] = true
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// Discover returns every directory under root holding a vinder.toml,
// skipping node_modules and hidden directories.
func Discover(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if file != root && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == ConfigFile {
			dirs = append(dirs, filepath.Dir(file))
		}
		return nil
	})
	return dirs, err
}
