// Package pkgfile reads built pacman packages (.pkg.tar.zst and friends) for
// static analysis: the archive is decompressed and walked exactly once, and
// everything the rules need — metadata, file listing, ELF facts, shebangs —
// is extracted on the way through. Nothing from the package is ever executed;
// ELF files are parsed with debug/elf, never loaded.
package pkgfile

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// maxELFSize caps how much of a single member is buffered for ELF analysis.
// Members over the cap keep IsELF from the magic bytes but get no ELF facts.
const maxELFSize = 512 << 20

// Entry is one member of the package archive.
type Entry struct {
	Name     string // path inside the package, as stored (no leading slash)
	Uname    string
	Gname    string
	Linkname string // symlink target or hardlink source
	ModTime  time.Time
	Size     int64
	UID, GID int
	Mode     fs.FileMode
	Type     byte // tar type flag: tar.TypeReg, TypeDir, TypeSymlink, TypeLink

	// Facts extracted from regular files during the single pass.
	IsELF    bool
	IsAr     bool     // static archive ("!<arch>" magic)
	IsScript bool     // has a #! line
	ELF      *ELFInfo // non-nil when IsELF and the file parsed
	Shebang  []string // interpreter line words, e.g. ["/usr/bin/env", "python3"]

	// Data holds the full contents of the few small members rules read
	// verbatim: .INSTALL (scriptlet analysis) and pkg-config .pc files
	// (dependency inference). nil for everything else.
	Data []byte
}

// IsDir reports whether the entry is a directory.
func (e *Entry) IsDir() bool { return e.Type == tar.TypeDir }

// IsFile reports whether the entry is a regular file.
func (e *Entry) IsFile() bool { return e.Type == tar.TypeReg }

// IsSymlink reports whether the entry is a symbolic link.
func (e *Entry) IsSymlink() bool { return e.Type == tar.TypeSymlink }

// IsHardlink reports whether the entry is a hard link.
func (e *Entry) IsHardlink() bool { return e.Type == tar.TypeLink }

// Package is a fully loaded package archive.
type Package struct {
	Path    string
	Info    PkgInfo
	Entries []Entry

	// MTree maps member path -> mtime from the .MTREE metadata, when present.
	// Arch packages carry both tar headers and mtree; the two can disagree
	// (see the stale-bytecode rule).
	MTree map[string]time.Time

	names map[string]int
}

// Has reports whether the package contains a member with the exact stored
// name (no leading slash, directories without trailing slash).
func (p *Package) Has(name string) bool {
	_, ok := p.names[name]
	return ok
}

// Entry returns the member with the given name.
func (p *Package) Entry(name string) *Entry {
	if i, ok := p.names[name]; ok {
		return &p.Entries[i]
	}
	return nil
}

// IsPackagePath reports whether path names a built pacman package archive.
func IsPackagePath(p string) bool {
	base := strings.ToLower(path.Base(p))
	return strings.HasSuffix(base, ".pkg.tar") || strings.Contains(base, ".pkg.tar.")
}

// Load reads the package archive at path.
func Load(pkgPath string) (*Package, error) {
	f, err := os.Open(pkgPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f, pkgPath)
}

// Read parses a package archive from r. name is used for error messages and
// the resulting Package's Path.
func Read(r io.Reader, name string) (*Package, error) {
	dec, err := decompress(bufio.NewReaderSize(r, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	// The zstd decoder holds its block buffers until Close; the walk below
	// is the whole life of this reader, so release them on the way out.
	// gzip's Close is a no-op on the source and harmless; the rest are not
	// closers.
	if c, ok := dec.(io.Closer); ok {
		defer c.Close()
	}

	pkg := &Package{Path: name, names: map[string]int{}}
	tr := tar.NewReader(dec)
	sawPkginfo := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: reading archive: %w", name, err)
		}
		clean := strings.TrimPrefix(strings.TrimSuffix(hdr.Name, "/"), "./")
		e := Entry{
			Name:     clean,
			Type:     hdr.Typeflag,
			Mode:     fs.FileMode(hdr.Mode).Perm() | specialModeBits(hdr.Mode),
			UID:      hdr.Uid,
			GID:      hdr.Gid,
			Uname:    hdr.Uname,
			Gname:    hdr.Gname,
			Size:     hdr.Size,
			Linkname: strings.TrimPrefix(hdr.Linkname, "./"),
			ModTime:  hdr.ModTime,
		}
		switch clean {
		case ".PKGINFO":
			data, err := io.ReadAll(io.LimitReader(tr, 1<<20))
			if err != nil {
				return nil, fmt.Errorf("%s: reading .PKGINFO: %w", name, err)
			}
			pkg.Info = parsePkgInfo(data)
			sawPkginfo = true
		case ".MTREE":
			data, err := io.ReadAll(io.LimitReader(tr, 64<<20))
			if err == nil {
				pkg.MTree = parseMTree(data)
			}
		default:
			if hdr.Typeflag == tar.TypeReg {
				classify(&e, tr)
			}
		}
		pkg.Entries = append(pkg.Entries, e)
		pkg.names[clean] = len(pkg.Entries) - 1
	}
	if !sawPkginfo {
		return nil, fmt.Errorf("%s: no .PKGINFO — not a built pacman package", name)
	}
	return pkg, nil
}

// specialModeBits maps tar's setuid/setgid/sticky mode bits onto fs.FileMode.
func specialModeBits(mode int64) fs.FileMode {
	var out fs.FileMode
	if mode&0o4000 != 0 {
		out |= fs.ModeSetuid
	}
	if mode&0o2000 != 0 {
		out |= fs.ModeSetgid
	}
	if mode&0o1000 != 0 {
		out |= fs.ModeSticky
	}
	return out
}

// retainData reports whether a member's full contents should be kept on the
// Entry for the rules to read.
func retainData(name string, size int64) bool {
	if size > 1<<20 {
		return false
	}
	if name == ".INSTALL" {
		return true
	}
	if strings.HasSuffix(name, ".pc") {
		for _, d := range []string{"usr/lib/pkgconfig/", "usr/share/pkgconfig/", "usr/lib32/pkgconfig/"} {
			if strings.HasPrefix(name, d) {
				return true
			}
		}
	}
	return false
}

// classify peeks at a regular file's contents and extracts the facts rules
// need: ELF structure, static-archive magic, shebang line, retained contents.
func classify(e *Entry, r io.Reader) {
	if retainData(e.Name, e.Size) {
		data, err := io.ReadAll(io.LimitReader(r, 1<<20))
		if err != nil {
			return
		}
		e.Data = data
		if bytes.HasPrefix(data, []byte("#!")) {
			e.IsScript = true
			line, _, _ := strings.Cut(string(data[2:]), "\n")
			e.Shebang = strings.Fields(line)
		}
		return
	}
	head := make([]byte, 4)
	n, _ := io.ReadFull(r, head)
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte("\x7fELF")):
		e.IsELF = true
		if e.Size <= maxELFSize {
			rest, err := io.ReadAll(r)
			if err != nil {
				return
			}
			data := append(head, rest...)
			e.ELF = inspectELF(data)
		}
	case bytes.HasPrefix(head, []byte("!<ar")):
		e.IsAr = true
	case bytes.HasPrefix(head, []byte("#!")):
		e.IsScript = true
		line := readShebangLine(head[2:], r)
		e.Shebang = strings.Fields(line)
	}
}

// readShebangLine reads the remainder of the interpreter line.
func readShebangLine(prefix []byte, r io.Reader) string {
	buf := append([]byte(nil), prefix...)
	if i := bytes.IndexByte(buf, '\n'); i >= 0 {
		return string(buf[:i])
	}
	chunk := make([]byte, 256)
	for len(buf) < 1024 {
		n, err := r.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if i := bytes.IndexByte(buf, '\n'); i >= 0 {
				return string(buf[:i])
			}
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}

// Interpreter returns the command a script's shebang resolves to: the
// interpreter's basename, or for /usr/bin/env the first non-flag argument.
func (e *Entry) Interpreter() string {
	if len(e.Shebang) == 0 {
		return ""
	}
	cmd := path.Base(e.Shebang[0])
	if cmd != "env" {
		return cmd
	}
	for _, a := range e.Shebang[1:] {
		if strings.HasPrefix(a, "-") || strings.Contains(a, "=") {
			continue // env flags and VAR=val assignments
		}
		return path.Base(a)
	}
	return ""
}

// decompress detects the archive's compression by magic bytes. pacman has
// shipped .zst, .xz, .gz, .bz2 and plain .tar packages over the years.
func decompress(br *bufio.Reader) (io.Reader, error) {
	head, err := br.Peek(6)
	if err != nil && len(head) < 4 {
		return nil, fmt.Errorf("archive too short")
	}
	switch {
	case bytes.HasPrefix(head, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		zr, err := zstd.NewReader(br, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, err
		}
		return zr.IOReadCloser(), nil
	case bytes.HasPrefix(head, []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}):
		return xz.NewReader(br)
	case bytes.HasPrefix(head, []byte{0x1f, 0x8b}):
		return gzip.NewReader(br)
	case bytes.HasPrefix(head, []byte("BZh")):
		return bzip2.NewReader(br), nil
	default:
		return br, nil // assume plain tar; tar.Reader errors if it is not
	}
}
