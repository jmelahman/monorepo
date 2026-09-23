package pkgfile

import (
	"bytes"
	"debug/elf"
	"strings"
)

// ELFInfo is everything the package rules need to know about one ELF member,
// extracted once at load time so the file bytes can be dropped afterwards.
type ELFInfo struct {
	Class   elf.Class
	Machine elf.Machine
	Type    elf.Type

	HasDynamic bool
	HasInterp  bool // PT_INTERP present (a program the kernel can exec)
	HasDTDebug bool // DT_DEBUG present (dynamic linker patches executables here)

	GnuStackPresent bool
	GnuStackExec    bool // PT_GNU_STACK with PF_X
	HasRelro        bool // PT_GNU_RELRO present
	BindNow         bool // DT_BIND_NOW, DF_BIND_NOW or DF_1_NOW
	TextRel         bool // DT_TEXTREL or DF_TEXTREL
	PIE             bool // DF_1_PIE
	Unstripped      bool // has a .symtab section

	Soname  string
	Needed  []string
	Rpath   []string // DT_RPATH entries, colon-split
	Runpath []string // DT_RUNPATH entries, colon-split

	// UndefinedSyms are the dynamic symbols this object imports; Exported are
	// the defined dynamic symbols it offers. Used to judge whether a NEEDED
	// library is actually used (the static stand-in for `ldd -r -u`, which
	// pkglint must not run: it executes the inspected binary's interpreter).
	UndefinedSyms []string
	Exported      map[string]bool
}

// inspectELF parses data as an ELF object. A nil return means the file has
// the ELF magic but could not be parsed (truncated or malformed).
func inspectELF(data []byte) *ELFInfo {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer f.Close()

	info := &ELFInfo{Class: f.Class, Machine: f.Machine, Type: f.Type}

	for _, p := range f.Progs {
		switch p.Type {
		case elf.PT_INTERP:
			info.HasInterp = true
		case elf.PT_GNU_STACK:
			info.GnuStackPresent = true
			info.GnuStackExec = p.Flags&elf.PF_X != 0
		case elf.PT_GNU_RELRO:
			info.HasRelro = true
		}
	}

	if f.SectionByType(elf.SHT_DYNAMIC) != nil {
		info.HasDynamic = true
		info.Needed, _ = f.DynString(elf.DT_NEEDED)
		if sonames, _ := f.DynString(elf.DT_SONAME); len(sonames) > 0 {
			info.Soname = sonames[0]
		}
		info.Rpath = splitPaths(firstDynString(f, elf.DT_RPATH))
		info.Runpath = splitPaths(firstDynString(f, elf.DT_RUNPATH))

		if v, _ := f.DynValue(elf.DT_FLAGS); len(v) > 0 {
			flags := elf.DynFlag(v[0])
			info.BindNow = info.BindNow || flags&elf.DF_BIND_NOW != 0
			info.TextRel = info.TextRel || flags&elf.DF_TEXTREL != 0
		}
		if v, _ := f.DynValue(elf.DT_FLAGS_1); len(v) > 0 {
			flags := elf.DynFlag1(v[0])
			info.PIE = flags&elf.DF_1_PIE != 0
			info.BindNow = info.BindNow || flags&elf.DF_1_NOW != 0
		}
		if v, _ := f.DynValue(elf.DT_TEXTREL); len(v) > 0 {
			info.TextRel = true
		}
		if v, _ := f.DynValue(elf.DT_BIND_NOW); len(v) > 0 {
			info.BindNow = true
		}
		if v, _ := f.DynValue(elf.DT_DEBUG); len(v) > 0 {
			info.HasDTDebug = true
		}
	}

	if s := f.Section(".symtab"); s != nil && s.Type == elf.SHT_SYMTAB {
		info.Unstripped = true
	}

	if syms, err := f.DynamicSymbols(); err == nil {
		for _, sym := range syms {
			if sym.Name == "" {
				continue
			}
			if sym.Section == elf.SHN_UNDEF {
				info.UndefinedSyms = append(info.UndefinedSyms, sym.Name)
				continue
			}
			bind := elf.ST_BIND(sym.Info)
			if bind == elf.STB_GLOBAL || bind == elf.STB_WEAK {
				if info.Exported == nil {
					info.Exported = map[string]bool{}
				}
				info.Exported[sym.Name] = true
			}
		}
	}
	return info
}

func firstDynString(f *elf.File, tag elf.DynTag) string {
	if vals, _ := f.DynString(tag); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func splitPaths(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ":")
}
