package store

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tarEntry struct {
	name string
	body string
	link string // non-empty → a symlink with this target
}

func rawTar(entries []tarEntry) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0o755}
		if e.link != "" {
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = e.link
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(e.body))
		}
		tw.WriteHeader(hdr)
		if e.link == "" {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	return buf.Bytes()
}

func TestExtractTarRejectsUnsafeArchives(t *testing.T) {
	cases := []struct {
		name    string
		entries []tarEntry
		wantErr bool
	}{
		{"normal", []tarEntry{{name: "a/b.txt", body: "hi"}}, false},
		{"parent escape", []tarEntry{{name: "../evil", body: "x"}}, true},
		{"absolute path", []tarEntry{{name: "/etc/evil", body: "x"}}, true},
		{"nested escape", []tarEntry{{name: "ok/../../evil", body: "x"}}, true},
		{"escaping symlink", []tarEntry{{name: "link", link: "../../etc/passwd"}}, true},
		{"absolute symlink", []tarEntry{{name: "link", link: "/etc/passwd"}}, true},
		{"empty archive", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			_, err := ExtractTar(bytes.NewReader(rawTar(tc.entries)), dst, 0)
			if tc.wantErr != (err != nil) {
				t.Fatalf("ExtractTar err = %v, wantErr = %v", err, tc.wantErr)
			}
			// Nothing may ever land outside the destination.
			if _, statErr := os.Stat(filepath.Join(filepath.Dir(dst), "evil")); statErr == nil {
				t.Fatal("an entry escaped the extraction root")
			}
		})
	}
}

// The payload policy (backend artifacts): symlinks extract verbatim wherever
// they point — a venv's absolute link into its run container is legitimate —
// while entry names and hardlinks keep the strict rules.
func TestExtractTarPayloadSymlinkPolicy(t *testing.T) {
	dst := t.TempDir()
	entries := []tarEntry{
		{name: ".venv/bin/python3", body: "elf"},
		{name: ".venv/bin/python", link: "/opt/uv/python/bin/python3"},
		{name: "rel-escape", link: "../outside"},
	}
	if _, err := ExtractTarPayload(bytes.NewReader(rawTar(entries)), dst, 0); err != nil {
		t.Fatalf("payload extract: %v", err)
	}
	got, err := os.Readlink(filepath.Join(dst, ".venv/bin/python"))
	if err != nil || got != "/opt/uv/python/bin/python3" {
		t.Fatalf("symlink = %q, %v", got, err)
	}

	// Entry-name escapes stay rejected even under the payload policy.
	if _, err := ExtractTarPayload(bytes.NewReader(rawTar([]tarEntry{{name: "../evil", body: "x"}})), t.TempDir(), 0); err == nil {
		t.Fatal("payload extract accepted an escaping entry name")
	}

	// Hardlinks stay strict: os.Link resolves host-side at extract time.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	tw.WriteHeader(&tar.Header{Name: "hard", Typeflag: tar.TypeLink, Linkname: "../../etc/passwd", Mode: 0o644})
	tw.Close()
	if _, err := ExtractTarPayload(bytes.NewReader(buf.Bytes()), t.TempDir(), 0); err == nil {
		t.Fatal("payload extract accepted an escaping hardlink")
	}

	// And the strict extractor still refuses what the payload one allows.
	if _, err := ExtractTar(bytes.NewReader(rawTar(entries)), t.TempDir(), 0); err == nil {
		t.Fatal("strict extract accepted an absolute symlink")
	}
}

// A tar whose cumulative decompressed size exceeds maxBytes aborts with
// ErrArchiveTooLarge and leaves no over-cap file behind — the gzip-bomb guard.
func TestExtractTarEnforcesDecompressionCap(t *testing.T) {
	// Two 400-byte files: under an 800-byte cap both fit; under 700 the second
	// trips it, and under 300 the first does.
	entries := []tarEntry{
		{name: "a.bin", body: strings.Repeat("x", 400)},
		{name: "b.bin", body: strings.Repeat("y", 400)},
	}
	cases := []struct {
		name    string
		max     int64
		wantErr bool
	}{
		{"unlimited", 0, false},
		{"exact fit", 800, false},
		{"second file over", 700, true},
		{"first file over", 300, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			_, err := ExtractTar(bytes.NewReader(rawTar(entries)), dst, tc.max)
			if tc.wantErr {
				if !errors.Is(err, ErrArchiveTooLarge) {
					t.Fatalf("ExtractTar err = %v, want ErrArchiveTooLarge", err)
				}
				// The over-cap entry must not be left on disk.
				if _, statErr := os.Stat(filepath.Join(dst, "b.bin")); statErr == nil {
					t.Fatal("partial over-cap file was not removed")
				}
			} else if err != nil {
				t.Fatalf("ExtractTar err = %v, want nil", err)
			}
		})
	}
}

// A gzip bomb (a tiny compressed body expanding far past the cap) aborts before
// its decompressed bytes land on disk.
func TestExtractTarStopsGzipBomb(t *testing.T) {
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	tw := tar.NewWriter(gw)
	const size = 50 << 20 // 50 MiB of zeros — highly compressible
	tw.WriteHeader(&tar.Header{Name: "bomb", Mode: 0o644, Typeflag: tar.TypeReg, Size: size})
	io.Copy(tw, io.LimitReader(zeroReader{}, size))
	tw.Close()
	gw.Close()
	if gzBuf.Len() > 1<<20 {
		t.Fatalf("compressed bomb is %d bytes; expected it to compress well under the cap", gzBuf.Len())
	}
	dst := t.TempDir()
	if _, err := ExtractTar(bytes.NewReader(gzBuf.Bytes()), dst, 1<<20); !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("ExtractTar err = %v, want ErrArchiveTooLarge", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// ExtractTar accepts raw (uncompressed) tar as well as gzip.
func TestExtractTarAcceptsRawTar(t *testing.T) {
	dst := t.TempDir()
	if _, err := ExtractTar(bytes.NewReader(rawTar([]tarEntry{{name: "f.txt", body: "raw"}})), dst, 0); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "raw" {
		t.Fatalf("extracted %q", got)
	}
}
