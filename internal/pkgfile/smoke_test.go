package pkgfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadRealPackage exercises the loader against a real package from the
// host's pacman cache when one is available (any Arch machine); elsewhere it
// skips, keeping CI hermetic. The synthetic-archive tests carry the real
// assertions — this one exists to catch format drift against actual makepkg
// output.
func TestLoadRealPackage(t *testing.T) {
	matches, _ := filepath.Glob("/var/cache/pacman/pkg/*.pkg.tar.zst")
	if len(matches) == 0 {
		t.Skip("no pacman package cache on this host")
	}
	path := matches[0]
	pkg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	if pkg.Info.Name == "" || pkg.Info.Version == "" {
		t.Errorf("missing .PKGINFO metadata: %+v", pkg.Info)
	}
	if len(pkg.Entries) == 0 {
		t.Error("no entries read")
	}
	if !strings.Contains(filepath.Base(path), pkg.Info.Name) {
		t.Errorf("package name %q not in filename %q", pkg.Info.Name, filepath.Base(path))
	}
	if len(pkg.MTree) == 0 {
		t.Error("no .MTREE timestamps parsed")
	}
	for i := range pkg.Entries {
		e := &pkg.Entries[i]
		if e.IsELF && e.ELF == nil && e.Size <= maxELFSize {
			t.Errorf("%s: ELF magic but no parsed ELF info", e.Name)
		}
	}
}

func TestLoadRealPackageInventory(t *testing.T) {
	if os.Getenv("PKGLINT_SMOKE") == "" {
		t.Skip("set PKGLINT_SMOKE=1 for a verbose inventory pass over the package cache")
	}
	matches, _ := filepath.Glob("/var/cache/pacman/pkg/*.pkg.tar.zst")
	if len(matches) > 40 {
		matches = matches[:40]
	}
	for _, path := range matches {
		pkg, err := Load(path)
		if err != nil {
			t.Errorf("Load(%s): %v", path, err)
			continue
		}
		elfs, scripts := 0, 0
		for i := range pkg.Entries {
			if pkg.Entries[i].IsELF {
				elfs++
			}
			if pkg.Entries[i].IsScript {
				scripts++
			}
		}
		t.Logf("%s: %d entries, %d ELF, %d scripts, %d deps",
			pkg.Info.Name, len(pkg.Entries), elfs, scripts, len(pkg.Info.Depends))
	}
}
