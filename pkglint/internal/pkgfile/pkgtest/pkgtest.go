// Package pkgtest builds synthetic package archives and minimal ELF objects
// for tests. The ELF builder emits just enough structure for debug/elf (and
// therefore pkgfile's inspector) to parse: header, program headers, and the
// .dynstr/.dynsym/.dynamic/.symtab/.shstrtab sections.
package pkgtest

import (
	"archive/tar"
	"bytes"
	"debug/elf"
	"encoding/binary"
	"time"
)

// ELFOpts selects the properties of a synthetic ELF object.
type ELFOpts struct {
	Needed    []string
	Undefined []string
	Defined   []string
	Soname    string
	Rpath     string
	Runpath   string
	Type      elf.Type    // default ET_DYN
	Machine   elf.Machine // default EM_X86_64
	NoStack   bool        // omit PT_GNU_STACK entirely
	ExecStack bool        // PT_GNU_STACK carries PF_X
	Relro     bool        // include PT_GNU_RELRO
	BindNow   bool        // DT_FLAGS: DF_BIND_NOW
	TextRel   bool        // DT_TEXTREL
	PIE       bool        // DT_FLAGS_1: DF_1_PIE
	DTDebug   bool        // include DT_DEBUG
	Symtab    bool        // include .symtab (an unstripped object)
	Interp    bool        // include PT_INTERP (/lib64/ld-linux-x86-64.so.2)
}

const (
	ehsize  = 64
	phsize  = 56
	shsize  = 64
	symsize = 24
	dynsize = 16
)

// ELF renders the object described by o as bytes.
func ELF(o ELFOpts) []byte {
	if o.Type == elf.ET_NONE {
		o.Type = elf.ET_DYN
	}
	if o.Machine == elf.EM_NONE {
		o.Machine = elf.EM_X86_64
	}
	le := binary.LittleEndian

	// .dynstr
	dynstr := []byte{0}
	strOff := func(s string) uint32 {
		off := uint32(len(dynstr))
		dynstr = append(dynstr, s...)
		dynstr = append(dynstr, 0)
		return off
	}
	neededOffs := make([]uint32, len(o.Needed))
	for i, n := range o.Needed {
		neededOffs[i] = strOff(n)
	}
	var sonameOff, rpathOff, runpathOff uint32
	if o.Soname != "" {
		sonameOff = strOff(o.Soname)
	}
	if o.Rpath != "" {
		rpathOff = strOff(o.Rpath)
	}
	if o.Runpath != "" {
		runpathOff = strOff(o.Runpath)
	}
	symOffs := make([]uint32, 0, len(o.Undefined)+len(o.Defined))
	for _, s := range append(append([]string{}, o.Undefined...), o.Defined...) {
		symOffs = append(symOffs, strOff(s))
	}

	// .dynsym: null symbol, then undefined imports, then defined exports.
	var dynsym bytes.Buffer
	dynsym.Write(make([]byte, symsize))
	writeSym := func(nameOff uint32, shndx uint16) {
		var sym [symsize]byte
		le.PutUint32(sym[0:], nameOff)
		sym[4] = byte(elf.STB_GLOBAL)<<4 | byte(elf.STT_FUNC)
		le.PutUint16(sym[6:], shndx)
		dynsym.Write(sym[:])
	}
	for i := range o.Undefined {
		writeSym(symOffs[i], uint16(elf.SHN_UNDEF))
	}
	for i := range o.Defined {
		writeSym(symOffs[len(o.Undefined)+i], uint16(elf.SHN_ABS))
	}

	// .dynamic
	var dynamic bytes.Buffer
	writeDyn := func(tag elf.DynTag, val uint64) {
		var d [dynsize]byte
		le.PutUint64(d[0:], uint64(tag))
		le.PutUint64(d[8:], val)
		dynamic.Write(d[:])
	}
	for _, off := range neededOffs {
		writeDyn(elf.DT_NEEDED, uint64(off))
	}
	if o.Soname != "" {
		writeDyn(elf.DT_SONAME, uint64(sonameOff))
	}
	if o.Rpath != "" {
		writeDyn(elf.DT_RPATH, uint64(rpathOff))
	}
	if o.Runpath != "" {
		writeDyn(elf.DT_RUNPATH, uint64(runpathOff))
	}
	var flags, flags1 uint64
	if o.BindNow {
		flags |= uint64(elf.DF_BIND_NOW)
	}
	if o.PIE {
		flags1 |= uint64(elf.DF_1_PIE)
	}
	if flags != 0 {
		writeDyn(elf.DT_FLAGS, flags)
	}
	if flags1 != 0 {
		writeDyn(elf.DT_FLAGS_1, flags1)
	}
	if o.TextRel {
		writeDyn(elf.DT_TEXTREL, 0)
	}
	if o.DTDebug {
		writeDyn(elf.DT_DEBUG, 0)
	}
	writeDyn(elf.DT_NULL, 0)

	// .symtab: a single null symbol is enough for "unstripped".
	symtab := make([]byte, symsize)

	interp := []byte("/lib64/ld-linux-x86-64.so.2\x00")

	// .shstrtab
	shstr := []byte{0}
	shName := func(s string) uint32 {
		off := uint32(len(shstr))
		shstr = append(shstr, s...)
		shstr = append(shstr, 0)
		return off
	}
	nDynstr := shName(".dynstr")
	nDynsym := shName(".dynsym")
	nDynamic := shName(".dynamic")
	nSymtab := shName(".symtab")
	nShstr := shName(".shstrtab")

	// Program headers.
	type prog struct {
		typ, flags uint32
		off, size  uint64
	}
	var progs []prog
	if !o.NoStack {
		f := uint32(elf.PF_R | elf.PF_W)
		if o.ExecStack {
			f |= uint32(elf.PF_X)
		}
		progs = append(progs, prog{uint32(elf.PT_GNU_STACK), f, 0, 0})
	}
	if o.Relro {
		progs = append(progs, prog{uint32(elf.PT_GNU_RELRO), uint32(elf.PF_R), 0, 0})
	}
	interpProgIdx := -1
	if o.Interp {
		interpProgIdx = len(progs)
		progs = append(progs, prog{uint32(elf.PT_INTERP), uint32(elf.PF_R), 0, uint64(len(interp))})
	}

	// File layout: ehdr, phdrs, blobs, shdrs.
	phoff := uint64(ehsize)
	blobOff := phoff + uint64(len(progs)*phsize)
	place := func(b []byte) (off, size uint64) {
		off, size = blobOff, uint64(len(b))
		blobOff += size
		return
	}
	dynstrOff, dynstrSize := place(dynstr)
	dynsymOff, dynsymSize := place(dynsym.Bytes())
	dynamicOff, dynamicSize := place(dynamic.Bytes())
	symtabOff, symtabSize := place(symtab)
	interpOff, _ := place(interp)
	shstrOff, shstrSize := place(shstr)
	shoff := blobOff

	if interpProgIdx >= 0 {
		progs[interpProgIdx].off = interpOff
	}

	// Section headers. Order: NULL, .dynstr, .dynsym, .dynamic, [.symtab], .shstrtab
	type sect struct {
		name, typ, link, entsize uint32
		off, size                uint64
	}
	sections := []sect{
		{},
		{nDynstr, uint32(elf.SHT_STRTAB), 0, 0, dynstrOff, dynstrSize},
		{nDynsym, uint32(elf.SHT_DYNSYM), 1, symsize, dynsymOff, dynsymSize},
		{nDynamic, uint32(elf.SHT_DYNAMIC), 1, dynsize, dynamicOff, dynamicSize},
	}
	if o.Symtab {
		sections = append(sections, sect{nSymtab, uint32(elf.SHT_SYMTAB), 1, symsize, symtabOff, symtabSize})
	}
	shstrndx := uint16(len(sections))
	sections = append(sections, sect{nShstr, uint32(elf.SHT_STRTAB), 0, 0, shstrOff, shstrSize})

	var out bytes.Buffer
	// ELF header.
	ident := [16]byte{0x7f, 'E', 'L', 'F', byte(elf.ELFCLASS64), byte(elf.ELFDATA2LSB), 1}
	out.Write(ident[:])
	hdr := make([]byte, ehsize-16)
	le.PutUint16(hdr[0:], uint16(o.Type))
	le.PutUint16(hdr[2:], uint16(o.Machine))
	le.PutUint32(hdr[4:], 1) // version
	le.PutUint64(hdr[8:], 0) // entry
	le.PutUint64(hdr[16:], phoff)
	le.PutUint64(hdr[24:], shoff)
	le.PutUint32(hdr[32:], 0) // flags
	le.PutUint16(hdr[36:], ehsize)
	le.PutUint16(hdr[38:], phsize)
	le.PutUint16(hdr[40:], uint16(len(progs)))
	le.PutUint16(hdr[42:], shsize)
	le.PutUint16(hdr[44:], uint16(len(sections)))
	le.PutUint16(hdr[46:], shstrndx)
	out.Write(hdr)
	// Program headers.
	for _, p := range progs {
		ph := make([]byte, phsize)
		le.PutUint32(ph[0:], p.typ)
		le.PutUint32(ph[4:], p.flags)
		le.PutUint64(ph[8:], p.off)
		le.PutUint64(ph[16:], 0)      // vaddr
		le.PutUint64(ph[24:], 0)      // paddr
		le.PutUint64(ph[32:], p.size) // filesz
		le.PutUint64(ph[40:], p.size) // memsz
		le.PutUint64(ph[48:], 1)      // align
		out.Write(ph)
	}
	// Blobs, in placement order.
	out.Write(dynstr)
	out.Write(dynsym.Bytes())
	out.Write(dynamic.Bytes())
	out.Write(symtab)
	out.Write(interp)
	out.Write(shstr)
	// Section headers.
	for _, s := range sections {
		sh := make([]byte, shsize)
		le.PutUint32(sh[0:], s.name)
		le.PutUint32(sh[4:], s.typ)
		le.PutUint64(sh[8:], 0) // flags
		le.PutUint64(sh[16:], 0)
		le.PutUint64(sh[24:], s.off)
		le.PutUint64(sh[32:], s.size)
		le.PutUint32(sh[40:], s.link)
		le.PutUint32(sh[44:], 0) // info
		le.PutUint64(sh[48:], 1) // addralign
		le.PutUint64(sh[56:], uint64(s.entsize))
		out.Write(sh)
	}
	return out.Bytes()
}

// Member is one file in a synthetic package archive.
type Member struct {
	Name    string
	Mode    int64 // permission bits incl. setuid/setgid; defaults to 0644 / 0755 for dirs
	Type    byte  // defaults to tar.TypeReg; tar.TypeDir infers from trailing slash too
	Data    []byte
	Link    string // symlink target / hardlink source
	Uname   string // defaults to root
	Gname   string // defaults to root
	UID     int
	GID     int
	ModTime time.Time
}

// Tar builds an uncompressed package archive: .PKGINFO first (as makepkg
// writes it), then the members in order. pkgfile.Read accepts plain tar.
func Tar(pkginfo string, members ...Member) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	write := func(m Member) {
		hdr := &tar.Header{
			Name:     m.Name,
			Mode:     m.Mode,
			Uname:    m.Uname,
			Gname:    m.Gname,
			Uid:      m.UID,
			Gid:      m.GID,
			Size:     int64(len(m.Data)),
			Typeflag: m.Type,
			Linkname: m.Link,
			ModTime:  m.ModTime,
		}
		if hdr.Uname == "" && hdr.Uid == 0 {
			hdr.Uname = "root"
		}
		if hdr.Gname == "" && hdr.Gid == 0 {
			hdr.Gname = "root"
		}
		if hdr.ModTime.IsZero() {
			hdr.ModTime = base
		}
		if hdr.Typeflag == 0 {
			if len(m.Name) > 0 && m.Name[len(m.Name)-1] == '/' {
				hdr.Typeflag = tar.TypeDir
			} else {
				hdr.Typeflag = tar.TypeReg
			}
		}
		if hdr.Mode == 0 {
			if hdr.Typeflag == tar.TypeDir {
				hdr.Mode = 0o755
			} else {
				hdr.Mode = 0o644
			}
		}
		if hdr.Typeflag != tar.TypeReg {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			panic(err)
		}
		if hdr.Typeflag == tar.TypeReg && len(m.Data) > 0 {
			if _, err := tw.Write(m.Data); err != nil {
				panic(err)
			}
		}
	}
	write(Member{Name: ".PKGINFO", Data: []byte(pkginfo)})
	for _, m := range members {
		write(m)
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// Info builds a minimal .PKGINFO body. Extra lines ("depend = zlib") are
// appended verbatim.
func Info(name, arch string, extra ...string) string {
	body := "pkgname = " + name + "\npkgbase = " + name + "\npkgver = 1.0-1\n" +
		"pkgdesc = A test package\nurl = https://example.com\narch = " + arch + "\n" +
		"license = MIT\nsize = 1000\n"
	for _, e := range extra {
		body += e + "\n"
	}
	return body
}
