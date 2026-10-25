package pkgfile

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/pkglint/internal/pkgfile/pkgtest"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func read(t *testing.T, data []byte) *Package {
	t.Helper()
	pkg, err := Read(bytes.NewReader(data), "test.pkg.tar")
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

func TestReadParsesPkgInfo(t *testing.T) {
	pkg := read(t, pkgtest.Tar(pkgtest.Info("demo", "x86_64",
		"depend = zlib", "depend = glibc>=2.38", "optdepend = gimp: image editing",
		"provides = libdemo.so=1-64", "backup = etc/demo.conf", "xdata = pkgtype=pkg")))
	if pkg.Info.Name != "demo" || pkg.Info.Arch != "x86_64" || pkg.Info.Version != "1.0-1" {
		t.Errorf("basic fields wrong: %+v", pkg.Info)
	}
	if len(pkg.Info.Depends) != 2 || pkg.Info.Depends[0] != "zlib" {
		t.Errorf("depends wrong: %v", pkg.Info.Depends)
	}
	if len(pkg.Info.Provides) != 1 || len(pkg.Info.Backup) != 1 || len(pkg.Info.OptDepends) != 1 {
		t.Errorf("lists wrong: %+v", pkg.Info)
	}
	if pkg.Info.IsDebug() {
		t.Error("regular package classified as debug")
	}
}

// TestReadZstdArchive pins the format pacman actually ships: the archive is
// compressed with zstd, read through the concurrency-1 decoder, and parsed
// like a plain tar. The fuzz seeds cover this path too, but a named test is
// what a reviewer looks for.
func TestReadZstdArchive(t *testing.T) {
	plain := pkgtest.Tar(pkgtest.Info("demo", "x86_64", "depend = zlib"))
	var zst bytes.Buffer
	zw, err := zstd.NewWriter(&zst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	pkg := read(t, zst.Bytes())
	if pkg.Info.Name != "demo" || len(pkg.Info.Depends) != 1 {
		t.Errorf("zstd archive parsed wrong: %+v", pkg.Info)
	}
}

// The legacy formats still sit in pacman caches and on mirrors' archives.
// stdlib has no bzip2 writer, so that one is a checked-in fixture.
func TestReadLegacyCompressions(t *testing.T) {
	var xzBuf bytes.Buffer
	xw, err := xz.NewWriter(&xzBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xw.Write(pkgtest.Tar(pkgtest.Info("demo", "x86_64", "depend = zlib"))); err != nil {
		t.Fatal(err)
	}
	if err := xw.Close(); err != nil {
		t.Fatal(err)
	}
	bz, err := os.ReadFile("testdata/demo-1.0-1-x86_64.pkg.tar.bz2")
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"xz": xzBuf.Bytes(), "bzip2": bz} {
		pkg := read(t, data)
		if pkg.Info.Name != "demo" || len(pkg.Info.Depends) != 1 || pkg.Info.Depends[0] != "zlib" {
			t.Errorf("%s archive parsed wrong: %+v", name, pkg.Info)
		}
	}
}

func TestReadRejectsNonPackages(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{Name: "hello.txt", Size: 2, Mode: 0o644})
	_, _ = tw.Write([]byte("hi"))
	_ = tw.Close()
	if _, err := Read(bytes.NewReader(buf.Bytes()), "x"); err == nil {
		t.Fatal("expected an error for a tarball without .PKGINFO")
	}
}

func TestClassifyELF(t *testing.T) {
	obj := pkgtest.ELF(pkgtest.ELFOpts{
		Type: elf.ET_DYN, Soname: "libdemo.so.1",
		Needed:  []string{"libc.so.6", "libz.so.1"},
		Rpath:   "/usr/lib:/opt/demo/lib",
		Runpath: "$ORIGIN",
		PIE:     true, TextRel: true, Relro: true, BindNow: true,
		ExecStack: true, Symtab: true, DTDebug: true, Interp: true,
		Undefined: []string{"gzopen"}, Defined: []string{"demo_init"},
	})
	pkg := read(t, pkgtest.Tar(pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/lib/libdemo.so.1", Data: obj, Mode: 0o755}))
	e := pkg.Entry("usr/lib/libdemo.so.1")
	if e == nil || !e.IsELF || e.ELF == nil {
		t.Fatalf("ELF member not classified: %+v", e)
	}
	i := e.ELF
	if i.Soname != "libdemo.so.1" {
		t.Errorf("soname = %q", i.Soname)
	}
	if len(i.Needed) != 2 || i.Needed[1] != "libz.so.1" {
		t.Errorf("needed = %v", i.Needed)
	}
	if len(i.Rpath) != 2 || i.Rpath[1] != "/opt/demo/lib" {
		t.Errorf("rpath = %v", i.Rpath)
	}
	if len(i.Runpath) != 1 || i.Runpath[0] != "$ORIGIN" {
		t.Errorf("runpath = %v", i.Runpath)
	}
	if !i.PIE || !i.TextRel || !i.HasRelro || !i.BindNow || !i.GnuStackExec || !i.GnuStackPresent {
		t.Errorf("flags wrong: %+v", i)
	}
	if !i.Unstripped || !i.HasDTDebug || !i.HasInterp || !i.HasDynamic {
		t.Errorf("structure flags wrong: %+v", i)
	}
	if len(i.UndefinedSyms) != 1 || i.UndefinedSyms[0] != "gzopen" {
		t.Errorf("undefined = %v", i.UndefinedSyms)
	}
	if !i.Exported["demo_init"] {
		t.Errorf("exported = %v", i.Exported)
	}
}

// A large ELF member is buffered once, at its declared size. io.ReadAll's
// doubling plus the append that re-attached the magic bytes cost about three
// times the member, which a run of zeros makes cheap to send.
func TestClassifyELFAllocatesOnce(t *testing.T) {
	obj := pkgtest.ELF(pkgtest.ELFOpts{})
	obj = append(obj, make([]byte, 16<<20-len(obj))...)
	tarBytes := pkgtest.Tar(pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: obj, Mode: 0o755})

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	pkg, err := Read(bytes.NewReader(tarBytes), "demo-1-1-x86_64.pkg.tar")
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if delta := after.TotalAlloc - before.TotalAlloc; delta >= uint64(2*len(obj)) {
		t.Errorf("Read allocated %d bytes for a %d-byte ELF member, want < %d", delta, len(obj), 2*len(obj))
	}
	if e := pkg.Entry("usr/bin/demo"); e == nil || !e.IsELF || e.ELF == nil {
		t.Errorf("padded ELF member not classified: %+v", e)
	}
}

// A member cut short by the end of the archive is an error from the tar
// reader, never a panic in the exact-size read.
func TestClassifyTruncatedELF(t *testing.T) {
	obj := pkgtest.ELF(pkgtest.ELFOpts{})
	obj = append(obj, make([]byte, 16<<20-len(obj))...)
	tarBytes := pkgtest.Tar(pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: obj, Mode: 0o755})
	if _, err := Read(bytes.NewReader(tarBytes[:len(tarBytes)/2]), "demo-1-1-x86_64.pkg.tar"); err == nil {
		t.Error("Read of a truncated archive succeeded, want an error")
	}
}

// A .PKGINFO past its bound refuses the archive: grading on a truncated
// prefix would drop every depend = line after the cut without a word.
func TestReadRefusesOversizedPkgInfo(t *testing.T) {
	data := pkgtest.Tar(pkgtest.Info("demo", "x86_64", "pkgdesc = "+strings.Repeat("x", 1<<20)))
	if _, err := Read(bytes.NewReader(data), "demo-1-1-x86_64.pkg.tar"); err == nil ||
		!strings.Contains(err.Error(), ".PKGINFO larger than") {
		t.Errorf("Read error = %v, want a .PKGINFO size refusal", err)
	}
}

// The zstd window bound is pinned, not tightened: this passes against the
// library default too, and exists so a dependency bump that moves the default
// shows up here. Each frame is a bare header (single-segment off, so the
// window descriptor byte applies) and an empty last raw block.
func TestDecompressZstdWindowBound(t *testing.T) {
	frame := func(windowDesc byte) []byte {
		return []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, windowDesc, 0x01, 0x00, 0x00}
	}
	drain := func(b []byte) error {
		r, err := decompress(bufio.NewReader(bytes.NewReader(b)))
		if err != nil {
			return err
		}
		_, err = io.ReadAll(r)
		return err
	}
	if err := drain(frame(0x98)); err != nil { // 512 MiB
		t.Errorf("512 MiB window: %v, want accepted", err)
	}
	if err := drain(frame(0xA0)); err == nil || !strings.Contains(err.Error(), "window size exceeded") { // 1 GiB
		t.Errorf("1 GiB window: %v, want a window size refusal", err)
	}
}

func TestClassifyMinimalELF(t *testing.T) {
	obj := pkgtest.ELF(pkgtest.ELFOpts{Type: elf.ET_EXEC, NoStack: true})
	pkg := read(t, pkgtest.Tar(pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: obj, Mode: 0o755}))
	i := pkg.Entry("usr/bin/demo").ELF
	if i == nil {
		t.Fatal("minimal ELF did not parse")
	}
	if i.Type != elf.ET_EXEC || i.PIE || i.GnuStackPresent || i.HasRelro || i.Unstripped {
		t.Errorf("unexpected flags: %+v", i)
	}
}

func TestClassifyScriptsAndArchives(t *testing.T) {
	pkg := read(t, pkgtest.Tar(pkgtest.Info("demo", "any"),
		pkgtest.Member{Name: "usr/bin/tool", Data: []byte("#!/usr/bin/python3\nprint('hi')\n"), Mode: 0o755},
		pkgtest.Member{Name: "usr/bin/tool2", Data: []byte("#!/usr/bin/env -S bash -e\necho hi\n"), Mode: 0o755},
		pkgtest.Member{Name: "usr/lib/libdemo.a", Data: []byte("!<arch>\nrest"), Mode: 0o644},
		// No newline at all: the interpreter line runs to end of file.
		pkgtest.Member{Name: "usr/bin/tool3", Data: []byte("#!/usr/bin/perl -w"), Mode: 0o755},
		// A line longer than the first read still yields its interpreter.
		pkgtest.Member{Name: "usr/bin/tool4", Data: []byte("#!/usr/bin/env " + strings.Repeat("A=1 ", 100) + "ruby\nputs 1\n"), Mode: 0o755},
		// The scriptlet is retained whole; its shebang is read from that copy.
		pkgtest.Member{Name: ".INSTALL", Data: []byte("#!/bin/bash\npost_install() { :; }\n"), Mode: 0o644},
	))
	if got := pkg.Entry("usr/bin/tool3").Interpreter(); got != "perl" {
		t.Errorf("unterminated shebang interpreter = %q, want perl", got)
	}
	if got := pkg.Entry("usr/bin/tool4").Interpreter(); got != "ruby" {
		t.Errorf("long shebang interpreter = %q, want ruby", got)
	}
	if e := pkg.Entry(".INSTALL"); !e.IsScript || e.Interpreter() != "bash" || len(e.Data) == 0 {
		t.Errorf(".INSTALL: IsScript %v, interpreter %q, %d bytes retained", e.IsScript, e.Interpreter(), len(e.Data))
	}
	if got := pkg.Entry("usr/bin/tool").Interpreter(); got != "python3" {
		t.Errorf("interpreter = %q, want python3", got)
	}
	if got := pkg.Entry("usr/bin/tool2").Interpreter(); got != "bash" {
		t.Errorf("env interpreter = %q, want bash", got)
	}
	if !pkg.Entry("usr/lib/libdemo.a").IsAr {
		t.Error("static archive not classified")
	}
}

func TestModeBits(t *testing.T) {
	pkg := read(t, pkgtest.Tar(pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/suid", Data: []byte("x"), Mode: 0o4755},
		pkgtest.Member{Name: "usr/bin/sgid", Data: []byte("x"), Mode: 0o2755},
		pkgtest.Member{Name: "var/spool/demo/", Type: tar.TypeDir, Mode: 0o1777},
		pkgtest.Member{Name: "usr/share/", Type: tar.TypeDir, Mode: 0o777},
	))
	for name, want := range map[string]fs.FileMode{
		"usr/bin/suid":   fs.ModeSetuid | 0o755,
		"usr/bin/sgid":   fs.ModeSetgid | 0o755,
		"var/spool/demo": fs.ModeSticky | 0o777,
	} {
		if e := pkg.Entry(name); e == nil {
			t.Errorf("%s: missing", name)
		} else if e.Mode != want {
			t.Errorf("%s: mode %v, want %v", name, e.Mode, want)
		}
	}
	if e := pkg.Entry("usr/share"); e == nil || !e.IsDir() {
		t.Error("directory entry not normalized (trailing slash)")
	}
}

func TestMTreeParsing(t *testing.T) {
	mtree := "#mtree\n/set type=file time=1700000000.0\n./usr/lib/mod.py time=1700000123.0\n./usr/lib/mod.pyc\n./usr/lib/with\\040space time=1700000100.0\n"
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	info := pkgtest.Info("demo", "any")
	_ = tw.WriteHeader(&tar.Header{Name: ".PKGINFO", Size: int64(len(info)), Mode: 0o644, ModTime: time.Unix(1, 0)})
	_, _ = tw.Write([]byte(info))
	_ = tw.WriteHeader(&tar.Header{Name: ".MTREE", Size: int64(len(mtree)), Mode: 0o644, ModTime: time.Unix(1, 0)})
	_, _ = tw.Write([]byte(mtree))
	_ = tw.Close()
	pkg := read(t, buf.Bytes())
	if got := pkg.MTree["usr/lib/mod.py"]; got.Unix() != 1700000123 {
		t.Errorf("mod.py time = %v", got)
	}
	if got := pkg.MTree["usr/lib/mod.pyc"]; got.Unix() != 1700000000 {
		t.Errorf("mod.pyc default time = %v", got)
	}
	if got := pkg.MTree["usr/lib/with space"]; got.Unix() != 1700000100 {
		t.Errorf("escaped-name time = %v", got)
	}
}

// gzipMTree compresses text as a .MTREE member payload.
func gzipMTree(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// mtreeBomb builds a gzip stream that expands past maxMTreeBytes while staying
// tiny on disk: the chunk is written repeatedly, so nothing near the ceiling is
// ever held in memory here. Only the parser has to refuse it.
func mtreeBomb(t testing.TB) []byte {
	t.Helper()
	chunk := bytes.Repeat([]byte("./usr/lib/padpadpadpad time=1700000000.0\n"), 1<<10)
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	for n := 0; n <= maxMTreeBytes; n += len(chunk) {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// tarWithMTree wraps a .MTREE payload beside a minimal .PKGINFO.
func tarWithMTree(t *testing.T, mtree []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	info := pkgtest.Info("demo", "any")
	_ = tw.WriteHeader(&tar.Header{Name: ".PKGINFO", Size: int64(len(info)), Mode: 0o644, ModTime: time.Unix(1, 0)})
	_, _ = tw.Write([]byte(info))
	_ = tw.WriteHeader(&tar.Header{Name: ".MTREE", Size: int64(len(mtree)), Mode: 0o644, ModTime: time.Unix(1, 0)})
	_, _ = tw.Write(mtree)
	_ = tw.Close()
	return buf.Bytes()
}

func TestMTreeBombIsDropped(t *testing.T) {
	t.Run("over ceiling", func(t *testing.T) {
		bomb := mtreeBomb(t)
		if len(bomb) > 1<<20 {
			t.Fatalf("bomb seed is %d bytes; it should stay small", len(bomb))
		}
		pkg := read(t, tarWithMTree(t, bomb))
		if pkg.MTree != nil {
			t.Errorf("over-limit .MTREE produced a map of %d entries; want nil", len(pkg.MTree))
		}
		if pkg.Info.Name != "demo" {
			t.Errorf("archive should still load: name = %q", pkg.Info.Name)
		}
	})
	t.Run("under ceiling", func(t *testing.T) {
		mtree := "#mtree\n/set type=file time=1700000000.0\n./usr/lib/mod.py time=1700000123.0\n"
		pkg := read(t, tarWithMTree(t, gzipMTree(t, mtree)))
		if got := pkg.MTree["usr/lib/mod.py"]; got.Unix() != 1700000123 {
			t.Errorf("gzip .MTREE under the ceiling should still parse: mod.py time = %v", got)
		}
	})
}

func TestIsPackagePath(t *testing.T) {
	for path, want := range map[string]bool{
		"foo-1.0-1-x86_64.pkg.tar.zst": true,
		"foo-1.0-1-any.pkg.tar.xz":     true,
		"foo-1.0-1-any.pkg.tar":        true,
		"dir/foo.PKG.TAR.ZST":          true,
		"PKGBUILD":                     false,
		"foo.tar.gz":                   false,
	} {
		if got := IsPackagePath(path); got != want {
			t.Errorf("IsPackagePath(%q) = %v, want %v", path, got, want)
		}
	}
}
