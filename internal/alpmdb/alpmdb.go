// Package alpmdb reads pacman's local package database
// (/var/lib/pacman/local) without libalpm: the desc and files members are
// plain text. The dependency-inference rules use it to answer "which installed
// package owns this library or interpreter?" and "is that package reachable
// from this package's depends?" — the same questions namcap answers through
// libalpm. On systems without the database those rules simply do not run.
package alpmdb

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultRoot is where pacman keeps the local database on Arch systems.
const DefaultRoot = "/var/lib/pacman/local"

// Pkg is one installed package.
type Pkg struct {
	Name     string
	Version  string
	Depends  []string // as written, version constraints included
	Provides []string

	dir       string
	filesOnce sync.Once
	files     []string
}

// Files returns the package's file list (paths relative to /, directories
// with a trailing slash), loading it on first use.
func (p *Pkg) Files() []string {
	p.filesOnce.Do(func() {
		data, err := os.ReadFile(filepath.Join(p.dir, "files"))
		if err != nil {
			return
		}
		inFiles := false
		for line := range strings.Lines(string(data)) {
			line = strings.TrimRight(line, "\n")
			switch {
			case line == "%FILES%":
				inFiles = true
			case strings.HasPrefix(line, "%"):
				inFiles = false
			case inFiles && line != "":
				p.files = append(p.files, line)
			}
		}
	})
	return p.files
}

// DB is a loaded local database.
type DB struct {
	byName    map[string]*Pkg
	providers map[string][]string // provide name (no version) -> package names
	pkgs      []*Pkg

	ownersOnce sync.Once
	binOwner   map[string]string // "usr/bin/python3" -> owning package
	libOwner   map[string]string // 64-bit library basename ("libz.so.1") -> owning package
	libOwner32 map[string]string // 32-bit (usr/lib32) library basename -> owning package
	pathOwner  map[string]string // exact path (trailing slash stripped) -> owning package
}

// Load reads the database under root. A missing root returns (nil, nil):
// callers treat a nil DB as "no dependency resolution available".
func Load(root string) (*DB, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	db := &DB{byName: map[string]*Pkg{}, providers: map[string][]string{}}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		desc, err := os.ReadFile(filepath.Join(dir, "desc"))
		if err != nil {
			continue
		}
		p := parseDesc(desc)
		if p.Name == "" {
			continue
		}
		p.dir = dir
		db.byName[p.Name] = p
		db.pkgs = append(db.pkgs, p)
		db.providers[p.Name] = append(db.providers[p.Name], p.Name)
		for _, prov := range p.Provides {
			db.providers[DepName(prov)] = append(db.providers[DepName(prov)], p.Name)
		}
	}
	if len(db.pkgs) == 0 {
		return nil, nil
	}
	return db, nil
}

func parseDesc(data []byte) *Pkg {
	p := &Pkg{}
	section := ""
	for line := range strings.Lines(string(data)) {
		line = strings.TrimRight(line, "\n")
		if strings.HasPrefix(line, "%") && strings.HasSuffix(line, "%") {
			section = line
			continue
		}
		if line == "" {
			continue
		}
		switch section {
		case "%NAME%":
			p.Name = line
		case "%VERSION%":
			p.Version = line
		case "%DEPENDS%":
			p.Depends = append(p.Depends, line)
		case "%PROVIDES%":
			p.Provides = append(p.Provides, line)
		}
	}
	return p
}

// DepName strips the version constraint from a dependency or provides entry:
// "libfoo.so=1-64" -> "libfoo.so", "glibc>=2.38" -> "glibc".
func DepName(entry string) string {
	if i := strings.IndexAny(entry, "<>="); i >= 0 {
		entry = entry[:i]
	}
	return strings.TrimSpace(entry)
}

// Get returns the installed package with the given name, or nil.
func (db *DB) Get(name string) *Pkg {
	if db == nil {
		return nil
	}
	return db.byName[name]
}

// ProvidersOf returns the installed packages providing name (a package name
// or a provides capability such as a soname).
func (db *DB) ProvidersOf(name string) []string {
	if db == nil {
		return nil
	}
	return db.providers[name]
}

// Closure returns every package reachable from deps (package names or
// provided capabilities) through the depends graph, including the roots'
// providers themselves.
func (db *DB) Closure(deps []string) map[string]bool {
	out := map[string]bool{}
	if db == nil {
		return out
	}
	var todo []string
	for _, d := range deps {
		todo = append(todo, db.providers[DepName(d)]...)
	}
	for len(todo) > 0 {
		name := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if out[name] {
			continue
		}
		out[name] = true
		p := db.byName[name]
		if p == nil {
			continue
		}
		for _, d := range p.Depends {
			todo = append(todo, db.providers[DepName(d)]...)
		}
	}
	return out
}

// libDirs64 and libDirs32 are the directories whose members are indexed as
// libraries, split by ABI so a 64-bit binary's DT_NEEDED never resolves to
// the lib32 copy of the same soname (and vice versa).
var (
	libDirs64 = []string{"usr/lib/", "lib/", "lib64/", "usr/lib64/"}
	libDirs32 = []string{"usr/lib32/"}
)

// binDirs are the directories whose members are indexed as executables.
var binDirs = []string{"usr/bin/", "bin/", "usr/sbin/", "sbin/", "usr/local/bin/"}

// buildOwners scans every installed package's file list once, indexing the
// paths the dependency rules look up: executables by full path and libraries
// by basename.
func (db *DB) buildOwners() {
	db.ownersOnce.Do(func() {
		db.binOwner = map[string]string{}
		db.libOwner = map[string]string{}
		db.libOwner32 = map[string]string{}
		db.pathOwner = map[string]string{}
		index := func(m map[string]string, dirs []string, f, owner string) {
			for _, d := range dirs {
				if rest, ok := strings.CutPrefix(f, d); ok && !strings.Contains(rest, "/") {
					if _, taken := m[rest]; !taken {
						m[rest] = owner
					}
				}
			}
		}
		for _, p := range db.pkgs {
			for _, f := range p.Files() {
				isDir := strings.HasSuffix(f, "/")
				db.pathOwner[strings.TrimSuffix(f, "/")] = p.Name
				if isDir {
					continue
				}
				for _, d := range binDirs {
					if rest, ok := strings.CutPrefix(f, d); ok && !strings.Contains(rest, "/") {
						if _, taken := db.binOwner[f]; !taken {
							db.binOwner[f] = p.Name
						}
					}
				}
				if strings.Contains(path.Base(f), ".so") {
					index(db.libOwner, libDirs64, f, p.Name)
					index(db.libOwner32, libDirs32, f, p.Name)
				}
			}
		}
	})
}

// LibraryOwner returns the installed package shipping the given library
// (matched by soname/basename in the standard library directories). lib32
// selects the 32-bit index.
func (db *DB) LibraryOwner(soname string, lib32 bool) string {
	if db == nil {
		return ""
	}
	db.buildOwners()
	if lib32 {
		return db.libOwner32[soname]
	}
	return db.libOwner[soname]
}

// CommandOwner returns the installed package shipping the named command in a
// standard binary directory. cmd is a bare command name ("python3").
func (db *DB) CommandOwner(cmd string) string {
	if db == nil || cmd == "" || strings.Contains(cmd, "/") {
		return ""
	}
	db.buildOwners()
	for _, d := range binDirs {
		if owner, ok := db.binOwner[d+cmd]; ok {
			return owner
		}
	}
	return ""
}

// HasPath reports whether any installed package ships the exact path
// (relative to /, no trailing slash).
func (db *DB) HasPath(p string) bool { return db.PathOwner(p) != "" }

// PathOwner returns the installed package shipping the exact path (relative
// to /, no trailing slash), or "".
func (db *DB) PathOwner(p string) string {
	if db == nil {
		return ""
	}
	db.buildOwners()
	return db.pathOwner[strings.TrimSuffix(p, "/")]
}
