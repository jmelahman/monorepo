// Package worktree turns linked worktrees, made with `git worktree add`, into
// standalone repositories.
package worktree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmelahman/git-orchard/git"
)

// inProgress are the files in a worktree's git directory that mark an
// operation that isn't finished.
var inProgress = []string{
	"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG",
	"rebase-merge", "rebase-apply", "sequencer",
}

// skipKeys are the config keys that describe the repository's layout and
// format rather than settings, which the clone has its own values for.
var skipKeys = map[string]bool{
	"core.bare":                    true,
	"core.worktree":                true,
	"core.repositoryformatversion": true,
	"extensions.objectformat":      true,
	"extensions.refstorage":        true,
	"extensions.worktreeconfig":    true,
}

// Detach converts the linked worktree containing dir into a standalone
// repository with its own .git directory, and removes it from the worktrees
// of the repository it was linked to. It returns the worktree's top level.
//
// The new repository has all of the original's refs, config, hooks and
// excludes, and the worktree's HEAD, index and per-worktree config, so
// staged and unstaged changes carry over. Objects are hardlinked where the
// filesystem allows, as for a local git clone.
func Detach(dir string) (string, error) {
	repo, err := git.Open(dir)
	if err != nil {
		return "", err
	}
	top := repo.Dir
	gitDir, err := repo.Output("rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	common, err := repo.Output("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if gitDir == common || filepath.Dir(gitDir) != filepath.Join(common, "worktrees") {
		return "", fmt.Errorf("%s is not a linked worktree", top)
	}
	if exists(filepath.Join(gitDir, "locked")) {
		return "", fmt.Errorf("%s is locked; run git worktree unlock first", top)
	}
	for _, name := range inProgress {
		if exists(filepath.Join(gitDir, name)) {
			return "", fmt.Errorf("%s has an operation in progress (%s); finish or abort it first", top, name)
		}
	}
	if exists(filepath.Join(gitDir, "modules")) {
		return "", fmt.Errorf("%s has submodules, whose repositories live in %s; detaching them isn't supported", top, gitDir)
	}

	tmp, err := os.MkdirTemp(top, ".git-detach-")
	if err != nil {
		return "", err
	}
	done := false
	defer func() {
		if !done {
			_ = os.RemoveAll(tmp)
		}
	}()
	if err := build(repo, tmp, gitDir, common); err != nil {
		return "", err
	}

	dotGit := filepath.Join(top, ".git")
	link, err := os.ReadFile(dotGit)
	if err != nil {
		return "", err
	}
	if err := os.Remove(dotGit); err != nil {
		return "", err
	}
	restore := func() {
		_ = os.RemoveAll(dotGit)
		_ = os.WriteFile(dotGit, link, 0o644)
	}
	if err := os.Rename(tmp, dotGit); err != nil {
		restore()
		return "", err
	}
	if _, err := repo.Output("status", "--porcelain"); err != nil {
		restore()
		return "", fmt.Errorf("detached repository doesn't work, so %s is still linked: %w", top, err)
	}
	done = true

	if err := os.RemoveAll(gitDir); err != nil {
		return top, fmt.Errorf("detached %s, but couldn't remove it from %s's worktrees: %w", top, common, err)
	}
	return top, nil
}

// build makes a repository at tmp with everything from the common git
// directory, common, and the worktree's own git directory, gitDir.
func build(repo git.Repo, tmp, gitDir, common string) error {
	if _, err := repo.Output("clone", "--quiet", "--mirror", common, tmp); err != nil {
		return err
	}
	clone := git.Repo{Dir: tmp}
	cfg := filepath.Join(tmp, "config")
	if _, err := clone.Output("config", "--file", cfg, "--remove-section", "remote.origin"); err != nil {
		return err
	}

	extensions, err := copyConfig(filepath.Join(common, "config"), cfg)
	if err != nil {
		return err
	}
	// Per-worktree config applies to this worktree alone, which is now the
	// repository's only one, so it goes in the shared config.
	if exists(filepath.Join(gitDir, "config.worktree")) {
		ext, err := copyConfig(filepath.Join(gitDir, "config.worktree"), cfg)
		if err != nil {
			return err
		}
		extensions = extensions || ext
	}
	if extensions {
		if _, err := clone.Output("config", "--file", cfg, "core.repositoryformatversion", "1"); err != nil {
			return err
		}
	}
	if _, err := clone.Output("config", "--file", cfg, "core.bare", "false"); err != nil {
		return err
	}

	if ref, err := repo.Output("symbolic-ref", "--quiet", "HEAD"); err == nil {
		if _, err := clone.Output("symbolic-ref", "HEAD", ref); err != nil {
			return err
		}
	} else {
		head, err := repo.Output("rev-parse", "--verify", "HEAD")
		if err != nil {
			return err
		}
		if _, err := clone.Output("update-ref", "--no-deref", "HEAD", head); err != nil {
			return err
		}
	}

	// The worktree's own state.
	names := []string{"index", "logs/HEAD", "info/sparse-checkout"}
	shared, err := filepath.Glob(filepath.Join(gitDir, "sharedindex.*"))
	if err != nil {
		return err
	}
	for _, path := range shared {
		names = append(names, filepath.Base(path))
	}
	for _, name := range names {
		if err := copyIfExists(filepath.Join(gitDir, name), filepath.Join(tmp, name)); err != nil {
			return err
		}
	}
	// The repository's, which a clone doesn't copy.
	for _, name := range []string{"info/exclude", "info/attributes"} {
		if err := copyIfExists(filepath.Join(common, name), filepath.Join(tmp, name)); err != nil {
			return err
		}
	}
	if exists(filepath.Join(common, "hooks")) {
		if err := os.RemoveAll(filepath.Join(tmp, "hooks")); err != nil {
			return err
		}
		if err := copyDir(filepath.Join(common, "hooks"), filepath.Join(tmp, "hooks")); err != nil {
			return err
		}
	}
	return nil
}

// copyConfig adds the settings in the config file src to the config file
// dst, except those in skipKeys, and reports whether any were extensions.
func copyConfig(src, dst string) (bool, error) {
	r := git.Repo{Dir: filepath.Dir(dst)}
	out, err := r.Output("config", "--file", src, "--list", "--null")
	if err != nil {
		return false, err
	}
	extensions := false
	for entry := range strings.SplitSeq(out, "\x00") {
		if entry == "" {
			continue
		}
		key, value, ok := strings.Cut(entry, "\n")
		if !ok {
			value = "true" // a bare key, like `[core] ignorecase`
		}
		if skipKeys[strings.ToLower(key)] {
			continue
		}
		if strings.HasPrefix(strings.ToLower(key), "extensions.") {
			extensions = true
		}
		if _, err := r.Output("config", "--file", dst, "--add", key, value); err != nil {
			return false, err
		}
	}
	return extensions, nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func copyIfExists(src, dst string) error {
	info, err := os.Lstat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return copyFile(src, dst, info.Mode())
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if mode&fs.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode.Perm())
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target, info.Mode())
	})
}
