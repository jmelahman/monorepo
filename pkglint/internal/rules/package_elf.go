package rules

import (
	"debug/elf"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgfile"
)

// --- PB801: arch=('any') vs machine code ------------------------------------

func checkPackageArch(ctx *Context) []Finding {
	info := ctx.File.Info
	if strings.HasPrefix(info.Name, "mingw-") {
		return nil // cross-compiled payloads are the package's whole point
	}
	var out []Finding
	binaries := 0
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !e.IsFile() || (!e.IsELF && !e.IsAr) {
			continue
		}
		binaries++
		if info.Arch == "any" {
			kind := "an ELF binary"
			if e.IsAr {
				kind = "a static archive"
			}
			out = append(out, pkgFinding("PB801", Error, e.Name,
				"package is arch=('any') but contains %s; machine code is architecture-specific", kind))
		}
	}
	if info.Arch != "any" && info.Arch != "" && binaries == 0 && !info.IsDebug() {
		out = append(out, pkgFinding("PB801", Info, ".PKGINFO",
			"package is arch=(%q) but contains no architecture-specific files; consider arch=('any')", info.Arch))
	}
	return out
}

// --- PB802: ELF placement ----------------------------------------------------

// elfStandardDirs is where ELF objects belong (namcap's valid_dirs).
var elfStandardDirs = []string{"bin/", "sbin/", "usr/bin/", "usr/sbin/", "lib/", "usr/lib/", "usr/lib32/"}

func checkELFPlacement(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !e.IsFile() || !e.IsELF {
			continue
		}
		if hasPrefixAny(e.Name, elfStandardDirs...) {
			continue
		}
		if strings.HasPrefix(e.Name, "opt/") {
			out = append(out, pkgFinding("PB802", Info, e.Name,
				"ELF file under opt/; self-contained trees are tolerated there, but usr/bin and usr/lib are preferred"))
			continue
		}
		out = append(out, pkgFinding("PB802", Error, e.Name,
			"ELF file outside the standard binary directories (usr/bin, usr/lib, …)"))
	}
	return out
}

// --- PB803/804/805/806/807: hardening ---------------------------------------

func checkELFExecStack(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if isHostELF(e) && e.ELF.GnuStackExec {
			out = append(out, pkgFinding("PB803", Warn, e.Name,
				"ELF file requests an executable stack (PT_GNU_STACK is RWX); link with -Wl,-z,noexecstack"))
		}
	}
	return out
}

func checkELFTextRel(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if isHostELF(e) && e.ELF.TextRel {
			out = append(out, pkgFinding("PB804", Warn, e.Name,
				"ELF file contains text relocations (DT_TEXTREL); rebuild with -fPIC"))
		}
	}
	return out
}

// isDebugArtifact mirrors namcap: files with ".debug" in the name are split
// debug info, which is intentionally unstripped, non-PIE and RELRO-less.
func isDebugArtifact(name string) bool { return strings.Contains(name, ".debug") }

// hostMachines are the CPU architectures whose ELF objects actually run on
// the host and therefore deserve hardening checks. GPU code objects (AMDGPU,
// CUDA) and other accelerator payloads are ELF too, but stacks, RELRO and PIE
// mean nothing there.
var hostMachines = map[elf.Machine]bool{
	elf.EM_386: true, elf.EM_X86_64: true, elf.EM_AARCH64: true, elf.EM_ARM: true,
	elf.EM_RISCV: true, elf.EM_LOONGARCH: true, elf.EM_PPC64: true,
}

// isHostELF reports whether the entry is a parsed ELF for a host CPU.
func isHostELF(e *pkgfile.Entry) bool {
	return isPackageELF(e) && hostMachines[e.ELF.Machine]
}

func checkELFRelro(ctx *Context) []Finding {
	if ctx.File.Info.IsDebug() {
		return nil
	}
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !isHostELF(e) || isDebugArtifact(e.Name) {
			continue
		}
		// Objects without a dynamic section (fully static binaries) have no
		// GOT for RELRO to protect.
		if !e.ELF.HasDynamic {
			continue
		}
		if e.ELF.HasRelro && e.ELF.BindNow {
			continue
		}
		out = append(out, pkgFinding("PB805", Warn, e.Name,
			"ELF file lacks full RELRO (PT_GNU_RELRO + BIND_NOW); link with -Wl,-z,relro,-z,now"))
	}
	return out
}

func checkELFNoPIE(ctx *Context) []Finding {
	if ctx.File.Info.IsDebug() {
		return nil
	}
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !isHostELF(e) || isDebugArtifact(e.Name) || strings.Contains(e.Name, ".so") {
			continue
		}
		// Only linked executables can be PIE; relocatable objects (crt*.o,
		// kernel modules' build artifacts) are neither PIE nor a problem.
		if e.ELF.Type != elf.ET_EXEC && e.ELF.Type != elf.ET_DYN {
			continue
		}
		pie := e.ELF.PIE || (e.ELF.Type == elf.ET_DYN && e.ELF.HasDTDebug)
		if e.ELF.Type == elf.ET_DYN && !e.ELF.HasInterp && !e.ELF.HasDTDebug && !e.ELF.PIE {
			continue // a shared object without a .so name; not an executable
		}
		if !pie {
			out = append(out, pkgFinding("PB806", Warn, e.Name,
				"executable is not PIE, so ASLR cannot relocate it; build with default (hardened) flags"))
		}
	}
	return out
}

func checkELFUnstripped(ctx *Context) []Finding {
	if ctx.File.Info.IsDebug() {
		return nil
	}
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if isHostELF(e) && !isDebugArtifact(e.Name) && e.ELF.Unstripped {
			out = append(out, pkgFinding("PB807", Warn, e.Name,
				"ELF file is unstripped (.symtab present); let makepkg strip it or use options=(strip)"))
		}
	}
	return out
}

// --- PB808: RPATH / RUNPATH ---------------------------------------------------

// safeRunpaths are the exact entries namcap accepts; each is also accepted as
// a path prefix (e.g. /usr/lib/demo).
var safeRunpaths = []string{"/usr/lib", "/usr/lib32", "/lib", "$ORIGIN", "${ORIGIN}"}

func rpathSafe(p string) bool {
	for _, ok := range safeRunpaths {
		if p == ok || strings.HasPrefix(p, ok+"/") {
			return true
		}
	}
	return false
}

func checkRpath(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !isPackageELF(e) {
			continue
		}
		report := func(tag, p string) {
			if rpathSafe(p) {
				return
			}
			sev := Error
			if p == "/usr/local/lib" {
				sev = Warn
			}
			out = append(out, pkgFinding("PB808", sev, e.Name,
				"insecure %s entry %q; anyone who can write there can hijack this binary's library loads", tag, p))
		}
		for _, p := range e.ELF.Rpath {
			report("RPATH", p)
		}
		for _, p := range e.ELF.Runpath {
			report("RUNPATH", p)
		}
	}
	return out
}
