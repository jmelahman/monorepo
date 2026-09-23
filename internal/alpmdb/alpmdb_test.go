package alpmdb

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeDB writes a local-database layout into a temp dir.
func fakeDB(t *testing.T, pkgs map[string]struct{ desc, files string }) *DB {
	t.Helper()
	root := t.TempDir()
	for dir, content := range pkgs {
		d := filepath.Join(root, dir)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "desc"), []byte(content.desc), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "files"), []byte(content.files), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	db, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func testDB(t *testing.T) *DB {
	return fakeDB(t, map[string]struct{ desc, files string }{
		"glibc-2.38-1": {
			desc:  "%NAME%\nglibc\n\n%VERSION%\n2.38-1\n\n%PROVIDES%\nlibc.so=6-64\n",
			files: "%FILES%\nusr/\nusr/lib/\nusr/lib/libc.so.6\n",
		},
		"zlib-1.3-1": {
			desc:  "%NAME%\nzlib\n\n%VERSION%\n1.3-1\n\n%DEPENDS%\nglibc\n\n%PROVIDES%\nlibz.so=1-64\n",
			files: "%FILES%\nusr/\nusr/lib/\nusr/lib/libz.so.1\nusr/lib/libz.so.1.3\n",
		},
		"python-3.12-1": {
			desc:  "%NAME%\npython\n\n%VERSION%\n3.12-1\n\n%DEPENDS%\nzlib\n\n%PROVIDES%\npython3\n",
			files: "%FILES%\nusr/\nusr/bin/\nusr/bin/python3\nusr/lib/python3.12/\n",
		},
	})
}

func TestLoadMissingRootIsNil(t *testing.T) {
	db, err := Load(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil || db != nil {
		t.Fatalf("missing root: db=%v err=%v, want nil/nil", db, err)
	}
	if db.Get("glibc") != nil || db.LibraryOwner("libz.so.1", false) != "" || db.HasPath("usr/bin/sh") {
		t.Error("nil DB methods must be safe no-ops")
	}
}

func TestOwnersAndClosure(t *testing.T) {
	db := testDB(t)
	if got := db.LibraryOwner("libz.so.1", false); got != "zlib" {
		t.Errorf("LibraryOwner(libz.so.1) = %q", got)
	}
	if got := db.CommandOwner("python3"); got != "python" {
		t.Errorf("CommandOwner(python3) = %q", got)
	}
	if db.CommandOwner("missing") != "" {
		t.Error("unknown command should have no owner")
	}
	if !db.HasPath("usr/lib/python3.12") {
		t.Error("HasPath should match directories with the slash stripped")
	}

	closure := db.Closure([]string{"python"})
	for _, want := range []string{"python", "zlib", "glibc"} {
		if !closure[want] {
			t.Errorf("closure missing %s: %v", want, closure)
		}
	}
	// Provides names resolve to their providers.
	if c := db.Closure([]string{"libz.so=1-64"}); !c["zlib"] || !c["glibc"] {
		t.Errorf("provides-rooted closure wrong: %v", c)
	}
	// Version constraints on the root are stripped.
	if c := db.Closure([]string{"zlib>=1.3"}); !c["zlib"] {
		t.Errorf("constrained closure wrong: %v", c)
	}
}

func TestDepName(t *testing.T) {
	for in, want := range map[string]string{
		"glibc":          "glibc",
		"glibc>=2.38":    "glibc",
		"libz.so=1-64":   "libz.so",
		"foo<2":          "foo",
		" spaced ":       "spaced",
		"libfoo.so>=1.2": "libfoo.so",
	} {
		if got := DepName(in); got != want {
			t.Errorf("DepName(%q) = %q, want %q", in, got, want)
		}
	}
}
