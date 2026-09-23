package rules

import (
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
)

// --- PB820: FHS layout --------------------------------------------------------

// fhsForbiddenDirs sit on tmpfs or are runtime state; packaged files there
// vanish at boot or fight the service manager.
var fhsForbiddenDirs = []string{"tmp/", "var/tmp/", "run/", "var/run/", "var/lock/"}

// fhsValidDirs is where packages may install (namcap's valid_paths). The
// package-metadata dotfiles are members of the archive too and always fine.
var fhsValidDirs = []string{
	"etc/", "opt/", "lib/modules/", "usr/bin/", "usr/include/", "usr/lib/",
	"usr/lib32/", "usr/sbin/", "usr/share/", "usr/src/", "var/cache/",
	"var/lib/", "var/log/", "var/opt/", "var/spool/", "var/state/",
	".PKGINFO", ".INSTALL", ".CHANGELOG", ".MTREE", ".BUILDINFO",
}

// mingwValidDirs are additionally allowed for mingw-* cross packages.
var mingwValidDirs = []string{
	"usr/x86_64-w64-mingw32/", "usr/i686-w64-mingw32/",
}

func checkFilesystemLayout(ctx *Context) []Finding {
	valid := fhsValidDirs
	if strings.HasPrefix(ctx.File.Info.Name, "mingw-") {
		valid = append(append([]string(nil), valid...), mingwValidDirs...)
	}
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		name := e.Name
		if e.IsDir() {
			name += "/"
		}
		// The forbidden directory itself is PB823's (empty-directory) concern;
		// anything inside it is an error.
		if e.IsDir() && sliceContains(fhsForbiddenDirs, name) {
			continue
		}
		if hasPrefixAny(name, fhsForbiddenDirs...) {
			out = append(out, pkgFinding("PB820", Error, e.Name,
				"file in a temporary/runtime directory; it disappears at boot and pacman cannot manage it"))
			continue
		}
		// The retired pre-FHS trees get their specific message rather than the
		// generic nonstandard-directory warning.
		if e.IsFile() && strings.HasPrefix(e.Name, "usr/man/") {
			out = append(out, pkgFinding("PB820", Error, e.Name,
				"man pages belong in usr/share/man, not the pre-FHS usr/man"))
			continue
		}
		if e.IsFile() && strings.HasPrefix(e.Name, "usr/info/") {
			out = append(out, pkgFinding("PB820", Error, e.Name,
				"info pages belong in usr/share/info, not the pre-FHS usr/info"))
			continue
		}
		ok := false
		for _, d := range valid {
			if strings.HasPrefix(name, d) || strings.HasPrefix(d, name) {
				ok = true
				break
			}
		}
		if !ok {
			out = append(out, pkgFinding("PB820", Warn, e.Name,
				"file outside the standard FHS directories pacman manages"))
			continue
		}
		if !e.IsFile() {
			continue
		}
		// The stray-page heuristic skips usr/lib and usr/share: application
		// trees legitimately carry man/ directories there (node_modules,
		// bundled docs), and usr/share/man is the right place anyway.
		heuristicTree := !strings.HasPrefix(e.Name, "usr/lib/") && !strings.HasPrefix(e.Name, "usr/share/")
		switch {
		case strings.HasPrefix(e.Name, "usr/lib/ruby/site_ruby/"):
			out = append(out, pkgFinding("PB820", Warn, e.Name,
				"site_ruby is for locally-built ruby code; packaged gems belong in the versioned vendor path"))
		case heuristicTree && pathComponent(e.Name, "man"):
			out = append(out, pkgFinding("PB820", Info, e.Name,
				"path contains a 'man' component outside usr/share/man; if this is a man page it is mis-installed"))
		case heuristicTree && pathComponent(e.Name, "info"):
			out = append(out, pkgFinding("PB820", Info, e.Name,
				"path contains an 'info' component outside usr/share/info; if this is an info page it is mis-installed"))
		}
	}
	return out
}

func pathComponent(name, comp string) bool {
	for _, part := range strings.Split(name, "/") {
		if part == comp {
			return true
		}
	}
	return false
}

func sliceContains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// --- PB821: permissions -------------------------------------------------------

func checkFilePermissions(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsSymlink() || e.IsHardlink() {
			continue
		}
		perm := e.Mode.Perm()
		if perm&0o002 != 0 {
			out = append(out, pkgFinding("PB821", Error, e.Name,
				"world-writable (%o); any local user can replace its contents", perm))
		}
		if e.Mode&(fs.ModeSetuid|fs.ModeSetgid) != 0 {
			out = append(out, pkgFinding("PB821", Warn, e.Name,
				"setuid/setgid file; every such bit deserves review"))
		}
		if perm&0o004 == 0 {
			out = append(out, pkgFinding("PB821", Warn, e.Name,
				"not world-readable (%o); non-root users cannot use it", perm))
		}
		if e.IsDir() && perm&0o001 == 0 {
			out = append(out, pkgFinding("PB821", Warn, e.Name,
				"directory is not world-executable (%o); non-root users cannot enter it", perm))
		}
		if e.IsFile() && strings.HasSuffix(e.Name, ".a") && perm != 0o644 && perm != 0o444 {
			out = append(out, pkgFinding("PB821", Warn, e.Name,
				"static library with mode %o; should be 644", perm))
		}
	}
	return out
}

// --- PB822: ownership ---------------------------------------------------------

func checkFileOwnership(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		rootUser := e.Uname == "root" || (e.Uname == "" && e.UID == 0)
		rootGroup := e.Gname == "root" || (e.Gname == "" && e.GID == 0)
		if rootUser && rootGroup {
			continue
		}
		u, g := e.Uname, e.Gname
		if u == "" {
			u = itoa(e.UID)
		}
		if g == "" {
			g = itoa(e.GID)
		}
		out = append(out, pkgFinding("PB822", Error, e.Name,
			"owned by %s:%s; makepkg packages everything as root:root", u, g))
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [24]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// --- PB823: empty directories -------------------------------------------------

func checkEmptyDirectories(ctx *Context) []Finding {
	nonEmpty := map[string]bool{}
	for i := range ctx.File.Entries {
		nonEmpty[path.Dir(ctx.File.Entries[i].Name)] = true
	}
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsDir() && !nonEmpty[e.Name] {
			out = append(out, pkgFinding("PB823", Info, e.Name,
				"empty directory; if intentional keep it with options=(emptydirs), otherwise drop it"))
		}
	}
	return out
}

// --- PB824: filenames ---------------------------------------------------------

func validFilenameChar(r rune) bool {
	return r >= 0x20 && r < 0x7f
}

func checkFilenames(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		for _, r := range e.Name {
			if !validFilenameChar(r) {
				out = append(out, pkgFinding("PB824", Warn, e.Name,
					"filename contains non-ASCII or control characters"))
				break
			}
		}
	}
	return out
}

// --- PB825/826: hardlinks and symlinks ----------------------------------------

func checkHardlinks(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsHardlink() && path.Dir(e.Name) != path.Dir(e.Linkname) {
			out = append(out, pkgFinding("PB825", Error, e.Name,
				"hard link across directories (to %s); use a symlink", e.Linkname))
		}
	}
	return out
}

func checkSymlinks(ctx *Context) []Finding {
	if ctx.DB == nil {
		return nil // external targets cannot be resolved without the database
	}
	// With a declared dependency missing from this host, its file list is
	// invisible and any verdict would be guesswork.
	if !ctx.facts().closureComplete {
		return nil
	}
	// Everything this package ships…
	have := map[string]bool{}
	for i := range ctx.File.Entries {
		have[strings.TrimSuffix(ctx.File.Entries[i].Name, "/")] = true
	}
	// …plus everything reachable from depends. Namcap uses direct depends
	// only; the closure avoids flagging targets shipped by a transitive
	// dependency.
	closure := ctx.facts().depClosure
	// A -debug package's links point into its base package.
	if ctx.File.Info.IsDebug() {
		base := strings.TrimSuffix(ctx.File.Info.Name, "-debug")
		for p := range ctx.DB.Closure([]string{base}) {
			closure[p] = true
		}
	}
	inDeps := func(target string) bool {
		for p := range closure {
			pkg := ctx.DB.Get(p)
			if pkg == nil {
				continue
			}
			for _, f := range pkg.Files() {
				if strings.TrimSuffix(f, "/") == target {
					return true
				}
			}
		}
		return false
	}
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !e.IsSymlink() {
			continue
		}
		target := e.Linkname
		if !strings.HasPrefix(target, "/") {
			target = path.Join(path.Dir(e.Name), target)
		}
		target = strings.TrimSuffix(strings.TrimPrefix(path.Clean(target), "/"), "/")
		if target == "" || strings.HasPrefix(target, "..") {
			continue
		}
		// Paths under etc and var are routinely generated at install/run time
		// (hooks, sysadmin state), so their absence from any package's file
		// list proves nothing.
		if hasPrefixAny(target, "etc/", "var/", "run/") {
			continue
		}
		if have[target] || inDeps(target) {
			continue
		}
		out = append(out, pkgFinding("PB826", Error, e.Name,
			"symlink points to %s, which neither this package nor its dependencies ship", e.Linkname))
	}
	return out
}

// --- PB827/828/829: single-file landmines ---------------------------------------

func checkLibtoolFiles(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsFile() && strings.HasSuffix(e.Name, ".la") {
			out = append(out, pkgFinding("PB827", Warn, e.Name,
				"libtool archive; delete .la files in package()"))
		}
	}
	return out
}

func checkPerllocal(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if strings.HasSuffix(e.Name, "perllocal.pod") {
			out = append(out, pkgFinding("PB828", Error, e.Name,
				"perllocal.pod is perl's shared install log; every perl package shipping it conflicts with the others"))
		}
	}
	return out
}

func checkInfoDirFile(ctx *Context) []Finding {
	if ctx.File.Has("usr/share/info/dir") {
		return []Finding{pkgFinding("PB829", Error, "usr/share/info/dir",
			"the info directory index is generated by pacman's hook; shipping it conflicts with every other info-bearing package")}
	}
	return nil
}

// --- PB830: python bytecode timestamps ------------------------------------------

// pycSource maps a compiled python file to its source: "pkg/__pycache__/
// mod.cpython-312.opt-1.pyc" -> "pkg/mod.py", "mod.pyc" -> "mod.py".
func pycSource(name string) string {
	if !strings.HasSuffix(name, ".pyc") && !strings.HasSuffix(name, ".pyo") {
		return ""
	}
	dir, base := path.Split(name)
	base = base[:len(base)-len(path.Ext(base))] // drop .pyc/.pyo
	if strings.HasSuffix(strings.TrimSuffix(dir, "/"), "__pycache__") {
		dir = path.Dir(strings.TrimSuffix(dir, "/"))
		if dot := strings.IndexByte(base, '.'); dot >= 0 {
			base = base[:dot] // mod.cpython-312.opt-1 -> mod
		}
		return path.Join(dir, base+".py")
	}
	return dir + base + ".py"
}

func checkPycMtimes(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !e.IsFile() {
			continue
		}
		src := pycSource(e.Name)
		if src == "" {
			continue
		}
		srcEntry := ctx.File.Entry(src)
		if srcEntry == nil {
			continue
		}
		if srcEntry.ModTime.After(e.ModTime) {
			out = append(out, pkgFinding("PB830", Error, e.Name,
				"stale bytecode: %s is newer than its compiled file; recompile after the last touch of the sources", src))
			continue
		}
		// The tar timestamps agree, but pacman installs the mtree ones.
		mt := ctx.File.MTree
		if mt != nil {
			st, sok := mt[src]
			ct, cok := mt[e.Name]
			if sok && cok && st.After(ct) {
				out = append(out, pkgFinding("PB830", Warn, e.Name,
					"stale bytecode by .MTREE timestamps: %s is newer than its compiled file", src))
			}
		}
	}
	return out
}

// --- PB831..PB839: assorted landmines -------------------------------------------

func checkPyTestsDir(ctx *Context) []Finding {
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if strings.HasPrefix(e.Name, "usr/lib/python") && strings.HasSuffix(e.Name, "site-packages/tests") {
			return []Finding{pkgFinding("PB831", Error, e.Name,
				"a generic 'tests' directory directly under site-packages collides with every other package that makes the same mistake")}
		}
	}
	return nil
}

func checkSystemdLocation(ctx *Context) []Finding {
	info := ctx.File.Info
	if info.Name == "systemd" {
		return nil
	}
	for _, p := range info.Provides {
		if pkgDepName(p) == "systemd" {
			return nil
		}
	}
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name, "etc/systemd/system/") {
			out = append(out, pkgFinding("PB832", Warn, e.Name,
				"packaged systemd units belong in usr/lib/systemd/system; etc is the administrator's override directory"))
		}
	}
	return out
}

func checkDbusLocation(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name, "etc/dbus-1/system.d/") {
			out = append(out, pkgFinding("PB833", Warn, e.Name,
				"packaged D-Bus policies belong in usr/share/dbus-1/system.d; etc is for the administrator"))
		}
	}
	return out
}

// --- PB834: license files -------------------------------------------------------

// spdxCommonDir is where Arch's `licenses` package ships the license texts
// that are shared system-wide; identifiers found there need no per-package
// copy. commonDir is its pre-SPDX predecessor, still checked so packages
// built under the old license spellings (GPL2, LGPL2.1, …) resolve.
// Overridable for tests.
var (
	spdxCommonDir = "/usr/share/licenses/spdx"
	commonDir     = "/usr/share/licenses/common"
)

// licenseExprSep splits an SPDX expression into identifier tokens.
var licenseExprSep = regexp.MustCompile(`[\s()]+`)

// licenseTokens extracts the identifier tokens of an SPDX license expression,
// dropping operators.
func licenseTokens(expr string) []string {
	var out []string
	for _, tok := range licenseExprSep.Split(expr, -1) {
		switch strings.ToUpper(tok) {
		case "", "AND", "OR", "WITH":
			continue
		}
		out = append(out, strings.TrimSuffix(tok, "+"))
	}
	return out
}

// isCommonLicense reports whether the identifier's text ships system-wide, so
// the package needs no copy of its own. Without the host's spdx directory only
// the always-custom spellings are treated as uncommon.
func isCommonLicense(tok string) bool {
	lower := strings.ToLower(tok)
	if strings.HasPrefix(lower, "custom") || strings.HasPrefix(tok, "LicenseRef-") {
		return false
	}
	entries, err := os.ReadDir(spdxCommonDir)
	if err != nil {
		return true // no host data: assume common, stay quiet
	}
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSuffix(e.Name(), ".txt"), tok) {
			return true
		}
	}
	if old, err := os.ReadDir(commonDir); err == nil {
		for _, e := range old {
			if strings.EqualFold(e.Name(), tok) {
				return true
			}
		}
	}
	return false
}

func checkLicenseFiles(ctx *Context) []Finding {
	info := ctx.File.Info
	if info.IsDebug() {
		return nil
	}
	if len(info.License) == 0 {
		return []Finding{pkgFinding("PB834", Error, ".PKGINFO",
			"package declares no license; users cannot tell what terms the software is under")}
	}
	var uncommon []string
	for _, l := range info.License {
		for _, tok := range licenseTokens(l) {
			if !isCommonLicense(tok) {
				uncommon = append(uncommon, tok)
			}
		}
	}
	if len(uncommon) == 0 {
		return nil
	}
	prefix := "usr/share/licenses/" + info.Name
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.Name == prefix && e.IsSymlink() {
			return nil // license dir symlinked into another package's; namcap warns, we accept
		}
		if strings.HasPrefix(e.Name, prefix+"/") && (e.IsFile() || e.IsSymlink()) {
			return nil
		}
	}
	return []Finding{pkgFinding("PB834", Error, ".PKGINFO",
		"license %q is not distributed system-wide, but the package installs nothing under %s/",
		strings.Join(uncommon, ", "), prefix)}
}

// --- PB835: backup entries -------------------------------------------------------

func checkBackupFiles(ctx *Context) []Finding {
	var out []Finding
	for _, b := range ctx.File.Info.Backup {
		if !ctx.File.Has(b) {
			out = append(out, pkgFinding("PB835", Error, ".PKGINFO",
				"backup entry %q names a file the package does not ship", b))
		}
	}
	return out
}

// --- PB836: documentation weight -------------------------------------------------

func checkDocsHeavy(ctx *Context) []Finding {
	name := ctx.File.Info.Name
	if strings.HasSuffix(name, "-doc") || strings.HasSuffix(name, "-docs") {
		return nil
	}
	var total, docs int64
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !e.IsFile() {
			continue
		}
		total += e.Size
		if strings.HasPrefix(e.Name, "usr/share/doc") {
			docs += e.Size
		}
	}
	if total > 0 && float64(docs)/float64(total) > 0.5 {
		return []Finding{pkgFinding("PB836", Info, "usr/share/doc",
			"%.0f%% of the package is documentation; consider a -docs split package", 100*float64(docs)/float64(total))}
	}
	return nil
}

// --- PB837/838/839: build residue ------------------------------------------------

func checkSphinxCache(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if e.IsFile() && strings.HasSuffix(e.Name, "environment.pickle") {
			out = append(out, pkgFinding("PB837", Warn, e.Name,
				"sphinx-build cache file; exclude the .doctrees directory from the installed docs"))
		}
	}
	return out
}

// mimeCacheFiles are outputs of update-mime-database/update-desktop-database,
// generated on the user's system by pacman hooks.
var mimeCacheFiles = map[string]bool{
	"usr/share/applications/mimeinfo.cache": true,
	"usr/share/mime/XMLnamespaces":          true,
	"usr/share/mime/aliases":                true,
	"usr/share/mime/globs":                  true,
	"usr/share/mime/magic":                  true,
	"usr/share/mime/subclasses":             true,
}

func checkMimeCache(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if mimeCacheFiles[e.Name] {
			out = append(out, pkgFinding("PB838", Error, e.Name,
				"generated MIME database file; pacman's hook rebuilds it, and shipping a copy conflicts with other packages"))
		}
	}
	return out
}

var scrollkeeperRe = regexp.MustCompile(`var.*/scrollkeeper/?$`)

func checkScrollkeeper(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if scrollkeeperRe.MatchString(e.Name) {
			out = append(out, pkgFinding("PB839", Warn, e.Name,
				"scrollkeeper catalog directory; the documentation system it served was retired in 2008"))
		}
	}
	return out
}

// --- PB842: jar placement -------------------------------------------------------

// PB842 is the only built-package rule the guideline sweep added: the other
// candidates were already PB820's. Files under /usr/libexec draw its
// outside-the-standard-directories warning (usr/libexec is not a valid-path
// prefix), /run is in its forbidden set, and /opt stays valid on purpose —
// namcap's valid_paths and the guidelines both allow self-contained trees
// there.

func checkJarLocation(ctx *Context) []Finding {
	var out []Finding
	for i := range ctx.File.Entries {
		e := &ctx.File.Entries[i]
		if !e.IsFile() || !strings.HasSuffix(e.Name, ".jar") {
			continue
		}
		// usr/lib/jvm is the java-runtime-common home for whole JVM
		// distributions; the JDK's internal jars are part of the JVM, not
		// libraries other packages load by convention.
		if strings.HasPrefix(e.Name, "usr/share/java/") || strings.HasPrefix(e.Name, "usr/lib/jvm/") {
			continue
		}
		out = append(out, pkgFinding("PB842", Info, e.Name,
			"jar outside usr/share/java; the Java package guidelines put jars there (symlink into the app's own tree when it insists on a private layout)"))
	}
	return out
}
