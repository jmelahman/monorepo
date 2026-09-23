package rules

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB4xx: filesystem and privilege hygiene.
var fsRules = []Rule{
	{
		ID:       "PB401",
		Name:     "write-outside-builddir",
		Severity: Error,
		Doc: "A build must only write beneath $srcdir and $pkgdir. Writes to $HOME or absolute " +
			"system paths either corrupt the builder's machine or hint at persistence being " +
			"installed outside the package manager's tracking.",
		Check: checkWritesOutside,
	},
	{
		ID:       "PB402",
		Name:     "privilege-escalation",
		Severity: Error,
		Doc: "makepkg builds must never escalate: sudo/su/doas/pkexec in a PKGBUILD runs arbitrary " +
			"commands as root on the build machine. Anything needing privileges belongs in the " +
			"package's install step, executed and tracked by pacman.",
		Check: checkPrivilegeEscalation,
	},
	{
		ID:       "PB403",
		Name:     "setuid-file",
		Severity: Warn,
		Doc: "chmod u+s/g+s (or 4xxx/2xxx modes), and `install -m` with such a mode, create " +
			"setuid/setgid binaries, and `setcap` grants file capabilities (cap_setuid, cap_net_raw, " +
			"…) — the capability-based equivalent of setuid. Each is a privilege boundary that " +
			"deserves explicit review whenever a package ships one. The auto-fix drops setuid/setgid " +
			"mode bits; capability grants are never rewritten automatically.",
		Check:    checkSetuid,
		FixLevel: FixUnsafe,
		Fix:      fixSetuid,
	},
	{
		ID:       "PB404",
		Name:     "install-without-destdir",
		Severity: Error,
		Doc: "A build system's install step in package() must be redirected into $pkgdir (via " +
			"DESTDIR, --root, --prefix, or --destdir). Without it, `make install` and friends write " +
			"into the builder's live filesystem instead of the staging directory — untracked by pacman " +
			"and often run with escalated privileges.",
		Check: checkInstallDestdir,
	},
	{
		ID:       "PB405",
		Name:     "sensitive-file-write",
		Severity: Critical,
		Doc: "Writing to pacman's configuration (SigLevel/repos control package trust), the dynamic " +
			"linker's preload/search config, or sudoers — or running pacman-key — reconfigures the " +
			"system's trust and privilege boundaries. In a PKGBUILD or scriptlet this is a persistence " +
			"and privilege-escalation vector.",
		Check: checkSensitiveWrites,
	},
}

// prefixes always acceptable as write targets.
var allowedWritePrefixes = []string{
	"$pkgdir", "${pkgdir", "$srcdir", "${srcdir", "$startdir", "${startdir",
	"/dev/null", "/dev/stdout", "/dev/stderr", "/dev/fd/",
	"/dev/tty", "/dev/console", "/dev/pts/", "/dev/shm/",
	"/tmp/", "$TMPDIR", "${TMPDIR",
}

// scriptletHooks are the function names pacman invokes from the file named by
// install=. Nothing else in a scriptlet runs, and pacman never calls these from
// the PKGBUILD itself.
var scriptletHooks = map[string]bool{
	"pre_install": true, "post_install": true,
	"pre_upgrade": true, "post_upgrade": true,
	"pre_remove": true, "post_remove": true,
}

// fileWriters are commands whose arguments name files they create or modify.
var fileWriters = map[string]bool{
	"cp": true, "mv": true, "install": true, "ln": true, "tee": true,
	"mkdir": true, "touch": true, "rm": true, "rmdir": true, "dd": true,
}

// fileDeleters name paths they remove rather than create. writeDests treats
// their arguments as targets — right for PB401, where erasing a path outside
// the sandbox is still a build-time write — but backwards for PB405, which is
// about *installing* trust configuration. The scriptlets in the wild use
// `rm -f /etc/sudoers.d/<pkg>` to retract a fragment an older release shipped,
// which PB405 reported as granting the escalation it actually withdraws.
// Scriptlet removals are PB502's, and it already words them as removals.
var fileDeleters = map[string]bool{"rm": true, "rmdir": true}

// writeTargetViolation names the hazard in writing to target, or "" when the
// write is fine (or another rule's to report). deleting says the command
// erases the path rather than creating it: PB405 skips deleters — retracting
// trust configuration is not installing it — so for them the sensitive-path
// deferral below would hand the finding to a rule that stays silent, and the
// build-time write must be reported here after all.
func writeTargetViolation(target string, deleting bool) string {
	t := strings.TrimSpace(target)
	if t == "" || strings.Contains(t, "\x00") {
		return ""
	}
	if !deleting && sensitiveWritePath(t) != "" {
		return "" // reported by PB405 instead, to avoid a duplicate finding
	}
	for _, p := range allowedWritePrefixes {
		if strings.HasPrefix(t, p) {
			return ""
		}
	}
	if strings.HasPrefix(t, "$HOME") || strings.HasPrefix(t, "${HOME") || strings.HasPrefix(t, "~/") {
		return "writes into the user's home directory"
	}
	if strings.HasPrefix(t, "/") {
		return "writes to an absolute system path"
	}
	return ""
}

// buildWriteScope reports whether code at this location is part of the build,
// and so bound by the $srcdir/$pkgdir rule. Scriptlets are not: they run on the
// live system after pacman has unpacked the package, where touching absolute
// paths is the entire point. A scriptlet hook defined in the PKGBUILD instead of
// an install file is dead code pacman never calls, so it is out of scope too.
func buildWriteScope(u *pkgbuild.Unit, fn string) bool {
	return !u.Scriptlet && !scriptletHooks[fn]
}

func checkWritesOutside(ctx *Context) []Finding {
	var out []Finding
	units := ctx.Pkg.Units()
	for i := range units {
		u := &units[i]
		if u.Scriptlet {
			continue
		}
		// Redirects are walked per function rather than per file so they get
		// the same treatment commands do: a scriptlet hook defined in the
		// PKGBUILD is out of scope, and a name the function (or an earlier
		// phase) rebound renders against that function's view — not the
		// file-level value, which by then names a path the build never touches.
		walkRedirects := func(fn string, root syntax.Node) {
			if !buildWriteScope(u, fn) {
				return
			}
			vars := ctx.varsFor(u, fn)
			syntax.Walk(root, func(node syntax.Node) bool {
				r, ok := node.(*syntax.Redirect)
				if !ok || r.Word == nil {
					return true
				}
				switch r.Op {
				case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll:
				default:
					return true
				}
				target, _ := pkgbuild.RenderWord(r.Word, vars)
				if why := writeTargetViolation(target, false); why != "" {
					out = append(out, findingAt("PB401", Error, u.Path, r.Pos(),
						"redirection to %q %s", target, why))
				}
				return true
			})
		}
		for name, fd := range u.Functions {
			walkRedirects(name, fd.Body)
		}
		for _, stmt := range u.TopLevel {
			walkRedirects("", stmt)
		}
	}

	for _, c := range ctx.Commands() {
		if !fileWriters[c.Name] || !buildWriteScope(c.Unit, c.Fn) {
			continue
		}
		for _, target := range writeDests(c) {
			if why := writeTargetViolation(target, fileDeleters[c.Name]); why != "" {
				out = append(out, c.finding("PB401", Error, "%s %s (%q)", c.Name, why, target))
			}
		}
	}
	return out
}

// writeDests returns the arguments of a file-writing command that name its
// destination(s).
func writeDests(c Command) []string {
	var positional []string
	var out []string
	for i := 0; i < len(c.Args); i++ {
		a := c.Args[i]
		if a == "-t" || a == "--target-directory" {
			if i+1 < len(c.Args) {
				out = append(out, c.Args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "--target-directory=") {
			out = append(out, strings.TrimPrefix(a, "--target-directory="))
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		positional = append(positional, a)
	}
	switch c.Name {
	case "mkdir", "touch", "rm", "rmdir", "tee":
		out = append(out, positional...)
	default: // cp, mv, install, ln: last positional is the destination
		if len(out) == 0 && len(positional) > 1 {
			out = append(out, positional[len(positional)-1])
		}
	}
	return out
}

func checkPrivilegeEscalation(ctx *Context) []Finding {
	var out []Finding
	esc := map[string]bool{"sudo": true, "doas": true, "pkexec": true, "su": true}
	for _, c := range ctx.Commands() {
		// Scriptlets already run as root under pacman — the install step this
		// rule points escalating builds toward — so there is nothing to escalate
		// there. In practice most scriptlet uses are `sudo -u`/`su -l`, which
		// de-escalate to a normal user: the opposite of the hazard.
		if !buildWriteScope(c.Unit, c.Fn) {
			continue
		}
		// sudo/doas are unwrapped by newCommand, so inspect the raw first word.
		raw := basename(c.RawName)
		if esc[raw] || esc[c.Name] {
			out = append(out, c.finding("PB402", Error,
				"%s escalates privileges during a build; anything needing root belongs to pacman's install step", raw))
		}
	}
	return out
}

// setuidNumericMode reports whether a numeric chmod/install mode sets the
// setuid or setgid bit. It parses the octal value and tests the 0o6000 mask —
// the same mask clearSetuidBits removes — rather than inspecting the leading
// digit, so 04755, 2755, and 7755 are all caught. Non-octal arguments,
// including symbolic modes like u+s, return false and are handled separately.
func setuidNumericMode(mode string) bool {
	if mode == "" || mode[0] == '+' || mode[0] == '-' {
		return false // a sign is never part of a mode, but ParseInt would accept one
	}
	v, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return false
	}
	return v&0o6000 != 0
}

func checkSetuid(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.CommandsNamed("chmod") {
		for _, a := range c.Args {
			symbolic := strings.Contains(a, "+s")
			numeric := setuidNumericMode(a)
			if symbolic || numeric {
				out = append(out, c.finding("PB403", Warn,
					"chmod %s creates a setuid/setgid file", a))
				break
			}
		}
	}
	for _, c := range ctx.CommandsNamed("install") {
		if mode, _, _ := installModeArg(c); setuidNumericMode(mode) {
			out = append(out, c.finding("PB403", Warn,
				"install with mode %s creates a setuid/setgid file", mode))
		}
	}
	for _, c := range ctx.CommandsNamed("setcap") {
		if c.HasArg("-r") { // -r removes capabilities
			continue
		}
		// The first non-flag argument is the capability clause, e.g.
		// cap_net_raw+ep; a bare "-" means the clause comes from stdin.
		caps := ""
		for _, a := range c.Args {
			if a == "-" || !hasPrefixAny(a, "-") {
				caps = a
				break
			}
		}
		if caps == "" {
			continue
		}
		if strings.Contains(strings.ToLower(caps), "cap_") {
			out = append(out, c.finding("PB403", Warn,
				"setcap grants file capabilities (%s) — the capability-based equivalent of a setuid binary", caps))
		} else {
			out = append(out, c.finding("PB403", Warn,
				"setcap grants file capabilities — the capability-based equivalent of a setuid binary"))
		}
	}
	return out
}

// installModeArg finds the numeric mode an `install` command applies, returning
// the mode digits, the argument word carrying it, and the full replacement for
// that word with the setuid/setgid bits cleared.
func installModeArg(c Command) (mode string, word *syntax.Word, replacement string) {
	for i := 0; i < len(c.Args); i++ {
		a := c.Args[i]
		switch {
		case strings.HasPrefix(a, "--mode="):
			m := strings.TrimPrefix(a, "--mode=")
			return m, wordByValue(c, a), "--mode=" + clearSetuidBits(m)
		case a == "--mode" || a == "-m":
			if i+1 < len(c.Args) {
				m := c.Args[i+1]
				return m, wordByValue(c, m), clearSetuidBits(m)
			}
		default:
			if m := shortFlagMode(a); m != "" {
				return m, wordByValue(c, a), strings.TrimSuffix(a, m) + clearSetuidBits(m)
			}
		}
	}
	return "", nil, ""
}

// shortFlagMode extracts the mode digits from an install short-flag cluster
// ending in -m<mode>, e.g. "-Dm4755" -> "4755". install requires -m to be last
// in a cluster because it consumes the following value.
var shortFlagModeRe = regexp.MustCompile(`^-[A-Za-z]*m([0-7]{3,4})$`)

func shortFlagMode(a string) string {
	if m := shortFlagModeRe.FindStringSubmatch(a); m != nil {
		return m[1]
	}
	return ""
}

// --- PB404: install step not redirected into $pkgdir -----------------------

func checkInstallDestdir(ctx *Context) []Finding {
	// Functions that stand a $pkgdir-rooted virtualenv up and activate it.
	// Everything pip installs afterwards lands in the staging tree — the same
	// redirection this rule asks for, spelled through PATH instead of a flag.
	venvFns := map[string]bool{}
	for _, c := range ctx.Commands() {
		if bindsPkgdirVenv(c) {
			venvFns[c.Fn] = true
		}
	}
	leaves := map[string]bool{} // fn -> leavesBuildTree, computed once each
	var out []Finding
	for _, c := range ctx.Commands() {
		if c.Fn != "package" && !strings.HasPrefix(c.Fn, "package_") {
			continue
		}
		tool, ok := liveInstallCommand(c)
		if !ok || installBindsDestdir(c) || venvFns[c.Fn] || funcBindsDestdir(c.Unit, c.Fn) {
			continue
		}
		if installStaysInBuildTree(c) {
			left, seen := leaves[c.Fn]
			if !seen {
				left = leavesBuildTree(ctx, c.Unit, c.Fn)
				leaves[c.Fn] = left
			}
			if !left {
				continue
			}
		}
		out = append(out, c.finding("PB404", Error,
			"`%s` in package() is not redirected into $pkgdir (set DESTDIR/--root/--prefix/--destdir)", tool))
	}
	return out
}

func liveInstallCommand(c Command) (string, bool) {
	switch c.Name {
	case "make":
		for _, a := range c.Args {
			if a == "install" || strings.HasPrefix(a, "install-") {
				return "make install", true
			}
		}
	case "ninja":
		if c.HasArg("install") {
			return "ninja install", true
		}
	case "cmake":
		if c.HasArg("--install") {
			return "cmake --install", true
		}
	case "meson":
		if c.Subcommand() == "install" || c.HasArg("install") {
			return "meson install", true
		}
	case "cargo":
		if c.Subcommand() == "install" {
			return "cargo install", true
		}
	case "pip", "pip3":
		if c.Subcommand() == "install" || c.HasArg("install") {
			return "pip install", true
		}
	case "python", "python3", "python2":
		if c.HasArg("setup.py") && c.HasArg("install") {
			return "python setup.py install", true
		}
	}
	return "", false
}

// installBindsDestdir reports whether the install command itself names a staging
// target: an inline DESTDIR/prefix env prefix, or any argument referencing
// $pkgdir/$DESTDIR.
func installBindsDestdir(c Command) bool {
	for _, as := range c.Call.Assigns {
		if as.Name == nil {
			continue
		}
		switch as.Name.Value {
		case "DESTDIR", "prefix", "PREFIX":
			return true
		}
		// Build systems spell the staging root a dozen ways — INSTALL_ROOT,
		// ROOT_DIR, INSTALL_PREFIX — so match on the value instead of trying
		// to keep a list of names: pointing anything at $pkgdir is the point.
		if v, _ := pkgbuild.RenderWord(as.Value, nil); referencesStaging(v) {
			return true
		}
	}
	for _, a := range c.Args {
		if referencesStaging(a) || strings.HasPrefix(a, "DESTDIR=") {
			return true
		}
	}
	return false
}

// installDestFlags are the options that name where an install lands. With a
// value that stays inside the build tree, the install is a scratch staging
// step — aseprite's `cmake --install build --prefix=staging`, followed by a
// hand-picked copy into $pkgdir — and never reaches the live system.
//
// --target is pip's (a directory); cargo's --target is a platform triple,
// which says nothing about where the install lands.
var installDestFlags = []string{"--prefix", "--root", "--destdir", "--target"}

// installStaysInBuildTree reports whether the command names at least one
// destination and every one it names stays inside the build tree: pip's
// `--root=/ --prefix=usr` lands in the live /usr however tame --prefix looks.
func installStaysInBuildTree(c Command) bool {
	named := false
	for i, a := range c.Args {
		for _, f := range installDestFlags {
			if f == "--target" && c.Name == "cargo" {
				continue
			}
			j, v := i, ""
			switch {
			case strings.HasPrefix(a, f+"="):
				v = strings.TrimPrefix(a, f+"=")
			case a == f && i+1 < len(c.Args):
				j, v = i+1, c.Args[i+1]
			default:
				continue
			}
			if (j < len(c.ArgDyn) && c.ArgDyn[j]) || !insideBuildTree(v) {
				return false
			}
			named = true
		}
	}
	return named
}

// insideBuildTree reports whether path stays under the directory package()
// runs in: relative, or rooted at $srcdir, and never climbing out with "..".
// Anything else it expands — $HOME, $prefix — could be anywhere.
func insideBuildTree(path string) bool {
	for _, root := range []string{"$srcdir", "${srcdir}"} {
		if rest, ok := strings.CutPrefix(path, root); ok && (rest == "" || rest[0] == '/') {
			path = "." + rest
			break
		}
	}
	// \x00 is how RenderWord marks what it could not render — a command
	// substitution, an arithmetic expansion — which could be anywhere.
	if path == "" || strings.ContainsAny(path, "$`~\x00") || strings.HasPrefix(path, "/") {
		return false
	}
	for seg := range strings.SplitSeq(path, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// leavesBuildTree reports whether fn, or a PKGBUILD function it calls, changes
// directory somewhere a relative install destination would no longer be
// inside the build tree: `cd /` turns `--prefix=usr` into the live /usr.
// Position is ignored — a `cd /tmp` later undone still counts — which can only
// keep a finding, never hide one.
func leavesBuildTree(ctx *Context, u *pkgbuild.Unit, fn string) bool {
	fns := map[string]bool{}
	var reach func(name string)
	reach = func(name string) {
		if fns[name] {
			return
		}
		fns[name] = true
		if fd := u.Functions[name]; fd != nil {
			for _, called := range calledFuncs(u, fd) {
				reach(called)
			}
		}
	}
	reach(fn)
	for _, c := range ctx.Commands() {
		if !fns[c.Fn] || (c.Name != "cd" && c.Name != "pushd") {
			continue
		}
		var dir string
		dyn := false
		for i, a := range c.Args {
			if !strings.HasPrefix(a, "-") || len(a) > 1 && a[1] >= '0' && a[1] <= '9' || a == "-" {
				dir, dyn = a, i < len(c.ArgDyn) && c.ArgDyn[i]
				break
			}
		}
		// `cd -` returns to $OLDPWD and pushd's +N/-N rotate the stack:
		// neither names where it goes.
		if dir == "" || dyn || dir == "-" || dir[0] == '+' || dir[0] == '-' ||
			!(insideBuildTree(dir) || referencesStaging(dir)) {
			return true
		}
	}
	return false
}

func referencesStaging(s string) bool {
	return strings.Contains(s, "$pkgdir") || strings.Contains(s, "${pkgdir") ||
		strings.Contains(s, "$DESTDIR") || strings.Contains(s, "${DESTDIR")
}

// destBindingTokens are how build systems are told where an install should
// land. On its own each is meaningless — `--prefix=/usr` is the normal thing to
// configure — so one only counts alongside a $pkgdir reference in the same word.
var destBindingTokens = []string{"DESTDIR", "destdir", "PREFIX", "prefix", "--root", "--target"}

func bindsStaging(s string) bool {
	if !referencesStaging(s) {
		return false
	}
	for _, tok := range destBindingTokens {
		if strings.Contains(s, tok) {
			return true
		}
	}
	return false
}

// funcBindsDestdir reports whether anything makepkg has already run has tied
// the install destination to $pkgdir.
//
// The obvious case is `export DESTDIR="$pkgdir"` in the same function, but the
// common one in practice is that the *configure* step bound it and the install
// step inherits it: `cmake -DCMAKE_INSTALL_PREFIX="$pkgdir/usr"` in build()
// leaves a build tree that stages correctly no matter how bare the later
// `make install` looks. Perl's `PERL_MM_OPT="... DESTDIR='$pkgdir'"` before
// Makefile.PL is the same idea. Those are the majority of package()s that
// install without naming a destination, and none of them touch the live system.
//
// Scope is the phases that precede this one in makepkg's single shell, plus the
// installing function itself; a sibling package_* split does not count, since
// its configuration is not this one's.
// PKGBUILDs also factor that setup into a helper — perl ones call a shared
// prepare_environment that exports PERL_MM_OPT — so calls are followed one
// level into functions the PKGBUILD defines itself.
func funcBindsDestdir(u *pkgbuild.Unit, fn string) bool {
	seen := map[string]bool{}
	var scan func(name string, depth int) bool
	scan = func(name string, depth int) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		fd := u.Functions[name]
		if fd == nil {
			return false
		}
		if nodeBindsDestdir(fd) {
			return true
		}
		if depth == 0 {
			return false
		}
		for _, called := range calledFuncs(u, fd) {
			if scan(called, depth-1) {
				return true
			}
		}
		return false
	}
	for _, name := range []string{"prepare", "build", "check", fn} {
		if scan(name, 1) {
			return true
		}
	}
	for _, stmt := range u.TopLevel {
		if nodeBindsDestdir(stmt) {
			return true
		}
	}
	return false
}

// calledFuncs returns the PKGBUILD-defined functions invoked from root.
func calledFuncs(u *pkgbuild.Unit, root syntax.Node) []string {
	var out []string
	syntax.Walk(root, func(n syntax.Node) bool {
		call, ok := n.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		name, _ := pkgbuild.RenderWord(call.Args[0], nil)
		if u.Functions[name] != nil {
			out = append(out, name)
		}
		return true
	})
	return out
}

// bindsPkgdirVenv reports whether this call creates or activates a Python
// virtualenv inside $pkgdir.
func bindsPkgdirVenv(c Command) bool {
	switch c.Name {
	case "virtualenv", "pyvenv":
	case "python", "python3", "python2":
		if !c.HasArg("venv") {
			return false
		}
	case "source", ".":
		// Activating it is what actually redirects the later pip installs.
	default:
		return false
	}
	for _, a := range c.Args {
		if referencesStaging(a) {
			return true
		}
	}
	return false
}

func nodeBindsDestdir(root syntax.Node) bool {
	found := false
	syntax.Walk(root, func(n syntax.Node) bool {
		if found {
			return false
		}
		switch x := n.(type) {
		case *syntax.Assign:
			if x.Name != nil && x.Name.Value == "DESTDIR" {
				found = true
				return false
			}
			if v, _ := pkgbuild.RenderWord(x.Value, nil); bindsStaging(v) {
				found = true
				return false
			}
		case *syntax.Word:
			if v, _ := pkgbuild.RenderWord(x, nil); bindsStaging(v) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// --- PB405: writes to trust/privilege configuration ------------------------

var sensitivePaths = []struct{ prefix, why string }{
	{"/etc/pacman.conf", "pacman's configuration controls repositories and signature enforcement"},
	{"/etc/pacman.d", "pacman's configuration directory controls repositories and mirrors"},
	{"/etc/ld.so.preload", "/etc/ld.so.preload forces a library into every dynamically-linked process"},
	{"/etc/ld.so.conf", "the dynamic linker's search path affects every binary"},
	{"/etc/sudoers", "sudoers governs privilege escalation"},
}

func sensitiveWritePath(target string) string {
	t := strings.TrimSpace(target)
	for _, s := range sensitivePaths {
		if strings.HasPrefix(t, s.prefix) {
			return s.why
		}
	}
	return ""
}

func checkSensitiveWrites(ctx *Context) []Finding {
	var out []Finding
	units := ctx.Pkg.Units()
	for i := range units {
		u := &units[i]
		syntax.Walk(u.File, func(node syntax.Node) bool {
			r, ok := node.(*syntax.Redirect)
			if !ok || r.Word == nil {
				return true
			}
			switch r.Op {
			case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll:
			default:
				return true
			}
			target, _ := pkgbuild.RenderWord(r.Word, ctx.vars)
			if why := sensitiveWritePath(target); why != "" {
				out = append(out, findingAt("PB405", Critical, u.Path, r.Pos(),
					"redirection to %q: %s", target, why))
			}
			return true
		})
	}
	for _, c := range ctx.Commands() {
		if c.Name == "pacman-key" || basename(c.RawName) == "pacman-key" {
			out = append(out, c.finding("PB405", Critical,
				"pacman-key manipulates the pacman trust keyring"))
			continue
		}
		if !fileWriters[c.Name] || fileDeleters[c.Name] {
			continue
		}
		for _, target := range writeDests(c) {
			if why := sensitiveWritePath(target); why != "" {
				out = append(out, c.finding("PB405", Critical, "%s writes to %q: %s", c.Name, target, why))
			}
		}
	}
	return out
}
