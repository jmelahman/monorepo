package cli

import (
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
)

type linkResult int

const (
	linkOK linkResult = iota
	linkCreated
	linkRelinked
	linkConflict
	linkError
)

// cmdLink symlinks every entry of .config/ into the repository root.
// A missing .config/ is a no-op so the hook is harmless in repos that
// don't use undot yet.
func cmdLink(a mem.Allocator, dryRun bool) int {
	fi, err := os.Lstat(configDir)
	if err != nil {
		return 0
	}
	if !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "undot: %s exists but is not a directory\n", configDir)
		return 1
	}
	entries, rerr := os.ReadDir(a, configDir)
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "undot: cannot read %s: %s\n", configDir, rerr.Error())
		return 1
	}
	loadIgnoreRules(a)
	fails := 0
	for _, e := range entries {
		if e.Name == confFileName {
			// The conf file is never symlinked into the root; treat it
			// as an implicit nolink entry and clean up any stray link.
			if !removeNolinkLink(e.Name, dryRun) {
				fails++
			}
			continue
		}
		if isNolink(e.Name) {
			if !removeNolinkLink(e.Name, dryRun) {
				fails++
			}
			continue
		}
		r := ensureLink(e.Name, dryRun)
		if r == linkConflict || r == linkError {
			fails++
		}
	}
	if fails > 0 {
		return 1
	}
	return 0
}

// ensureLink makes the root entry `name` a symlink to .config/name.
// A correct link is left alone; a stale or wrong link is replaced; a real
// file or directory is never touched.
func ensureLink(name string, dryRun bool) linkResult {
	target := configDir + "/" + name
	result := linkCreated
	fi, err := os.Lstat(name)
	if err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			fmt.Fprintf(os.Stderr, "undot: %s already exists and is not a symlink; skipping (remove it or move it into %s/)\n", name, configDir)
			return linkConflict
		}
		var buf [os.MaxPathLen]byte
		dest, rerr := os.Readlink(buf[:], name)
		if rerr == nil && dest == target {
			return linkOK
		}
		result = linkRelinked
		if !dryRun {
			if os.Remove(name) != nil {
				fmt.Fprintf(os.Stderr, "undot: cannot remove stale link %s\n", name)
				return linkError
			}
		}
	}
	if dryRun {
		fmt.Printf("undot: [dry-run] would link %s -> %s\n", name, target)
		return result
	}
	if serr := os.Symlink(target, name); serr != nil {
		fmt.Fprintf(os.Stderr, "undot: cannot link %s -> %s: %s\n", name, target, serr.Error())
		return linkError
	}
	fmt.Printf("undot: linked %s -> %s\n", name, target)
	return result
}

// removeNolinkLink cleans up a root symlink that undot previously
// created for an entry now marked nolink, so checkouts transition on their
// own. Anything at the root path that isn't exactly our symlink is left
// alone. Returns false only on a failed removal.
func removeNolinkLink(name string, dryRun bool) bool {
	target := configDir + "/" + name
	fi, err := os.Lstat(name)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return true
	}
	var buf [os.MaxPathLen]byte
	dest, rerr := os.Readlink(buf[:], name)
	if rerr != nil || dest != target {
		return true
	}
	if dryRun {
		fmt.Printf("undot: [dry-run] would remove %s (nolink)\n", name)
		return true
	}
	if os.Remove(name) != nil {
		fmt.Fprintf(os.Stderr, "undot: cannot remove %s (nolink)\n", name)
		return false
	}
	fmt.Printf("undot: removed %s (nolink: reference %s directly)\n", name, target)
	return true
}
