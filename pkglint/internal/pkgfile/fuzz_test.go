package pkgfile

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/jmelahman/pkglint/internal/pkgfile/pkgtest"
)

// FuzzRead covers the whole archive path: compression sniffing, tar walking,
// .PKGINFO and .MTREE parsing, and ELF classification. The archive is the most
// untrusted input pkglint takes — a hostile package must produce an error, not
// a panic, a hang, or memory exhaustion.
func FuzzRead(f *testing.F) {
	plain := pkgtest.Tar(pkgtest.Info("demo", "x86_64"),
		pkgtest.Member{Name: "usr/bin/demo", Data: pkgtest.ELF(pkgtest.ELFOpts{}), Mode: 0o755},
		pkgtest.Member{Name: "usr/lib/demo.so", Link: "demo", Type: tar.TypeSymlink},
		pkgtest.Member{Name: ".MTREE", Data: []byte("#mtree\n/set time=1700000000.0\n./usr/bin/demo type=file mode=755\n")},
		pkgtest.Member{Name: "usr/bin/script", Data: []byte("#!/usr/bin/python3\nprint()\n"), Mode: 0o755})
	f.Add(plain)

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(plain); err != nil {
		f.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(gz.Bytes())

	var zst bytes.Buffer
	sw, err := zstd.NewWriter(&zst)
	if err != nil {
		f.Fatal(err)
	}
	if _, err := sw.Write(plain); err != nil {
		f.Fatal(err)
	}
	if err := sw.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(zst.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		pkg, err := Read(bytes.NewReader(data), "fuzz-1.0-1-x86_64.pkg.tar")
		if err == nil && pkg == nil {
			t.Fatal("Read returned neither a package nor an error")
		}
	})
}

// FuzzParsePkgInfo covers the .PKGINFO key-value parser on its own.
func FuzzParsePkgInfo(f *testing.F) {
	f.Add([]byte(pkgtest.Info("demo", "x86_64", "depend = glibc", "backup = etc/demo.conf")))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = parsePkgInfo(data)
	})
}

// FuzzParseMTree covers the .MTREE parser, including its gzip sniffing.
func FuzzParseMTree(f *testing.F) {
	f.Add([]byte("#mtree\n/set type=file time=1700000000.0\n./usr/bin/demo mode=755\n/unset time\n"))
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write([]byte("#mtree\n./x time=1.5\n")); err != nil {
		f.Fatal(err)
	}
	if err := w.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(gz.Bytes())
	// A small gzip stream that expands past maxMTreeBytes: the ceiling must
	// keep replaying under plain `go test`, not just under -fuzz.
	f.Add(mtreeBomb(f))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = parseMTree(data)
	})
}
