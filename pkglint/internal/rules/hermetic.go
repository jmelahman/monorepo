package rules

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB2xx: hermeticity — network access belongs in the fetch/prepare phase,
// pinned by hashes, so that build()/package()/check() can eventually run
// with networking disabled.
var hermeticRules = []Rule{
	{
		ID:       "PB201",
		Name:     "network-in-build",
		Severity: Warn,
		Doc: "Downloads during build(), check() or package() bypass makepkg's checksum " +
			"verification and make the build non-hermetic: what gets compiled depends on network " +
			"state at build time. Move fetching to prepare() (or better, the source array) where " +
			"artifacts are pinned by lockfiles or checksums.",
		Check: checkNetworkInBuild,
	},
	{
		ID:       "PB202",
		Name:     "pip-without-hashes",
		Severity: Warn,
		Doc: "`pip install` without --require-hashes will happily fetch whatever the index serves. " +
			"With hash-pinned requirements, pip verifies every downloaded artifact against a digest " +
			"recorded in the requirements file.",
		Check: checkPipHashes,
	},
	{
		ID:       "PB203",
		Name:     "cargo-unlocked",
		Severity: Warn,
		Doc: "cargo without --locked (or --frozen/--offline) may resolve and fetch dependency " +
			"versions that differ from the committed Cargo.lock, so the built artifact is not " +
			"reproducible and unreviewed code can enter the build. Flags kept in a variable count " +
			"as passed wherever the PKGBUILD assigns it; a command whose flags come from outside " +
			"the file is left alone, since the lockfile flag may be in there.",
		Check: checkCargoLocked,
		// --locked hard-fails when the source ships no Cargo.lock (or one out
		// of sync with Cargo.toml), so the rewrite can break a working build —
		// the same reason the other lockfile-enforcing fixes (PB206–PB209)
		// are unsafe.
		FixLevel: FixUnsafe,
		Fix:      fixCargoLocked,
	},
	{
		ID:       "PB204",
		Name:     "go-implicit-downloads",
		Severity: Warn,
		Doc: "`go build` fetches modules on demand unless they were vendored or pre-downloaded. " +
			"That download happens inside build() with no makepkg-level verification (go.sum " +
			"verifies content, but resolution still depends on network state). Vendor the modules, " +
			"or run `go mod download` in prepare(). `go install pkg@latest` is worse still: the ref " +
			"is mutable, so what gets built depends on whatever upstream published most recently.",
		Check:    checkGoDownloads,
		FixLevel: FixUnsafe,
		Fix:      fixGoDownloads,
	},
	{
		ID:       "PB205",
		Name:     "go-verification-disabled",
		Severity: Warn,
		Doc: "GOFLAGS/GONOSUMCHECK/GOSUMDB/GOINSECURE settings that disable Go's module checksum " +
			"database or allow insecure transports remove the only integrity verification Go " +
			"downloads get inside a build.",
		Check:    checkGoEnvWeakening,
		FixLevel: FixSafe,
		Fix:      fixGoEnvWeakening,
	},
	{
		ID:       "PB206",
		Name:     "npm-install-unlocked",
		Severity: Info,
		Doc: "`npm install` may rewrite the dependency tree; `npm ci` (and yarn --immutable, " +
			"pnpm/bun --frozen-lockfile) installs exactly what the committed lockfile specifies " +
			"and fails otherwise.",
		Check:    checkNpmCI,
		FixLevel: FixUnsafe,
		Fix:      fixNpmCI,
	},
	{
		ID:       "PB207",
		Name:     "composer-unlocked",
		Severity: Warn,
		Doc: "`composer update` re-resolves and fetches dependency versions, abandoning the " +
			"committed composer.lock — use `composer install`, which installs exactly what the lock " +
			"pins. And either way, composer runs hook scripts and plugins from composer.json and the " +
			"dependency tree during install; pass --no-scripts so fetching packages does not also " +
			"mean executing them, then run any needed build step explicitly.",
		Check:    checkComposer,
		FixLevel: FixUnsafe,
		Fix:      lockfileFixer("PB207"),
	},
	{
		ID:       "PB208",
		Name:     "bundler-unlocked",
		Severity: Info,
		Doc: "`bundle install` without --frozen (or deployment mode) silently rewrites Gemfile.lock " +
			"when it drifts from the Gemfile, fetching versions nobody reviewed; --frozen makes the " +
			"committed lock authoritative and fails otherwise. (A `gem install` of a named gem has " +
			"no lockfile at all; that is PB210.)",
		Check:    checkBundler,
		FixLevel: FixUnsafe,
		Fix:      lockfileFixer("PB208"),
	},
	{
		ID:       "PB209",
		Name:     "uv-poetry-unlocked",
		Severity: Warn,
		Doc: "uv and poetry re-resolve dependencies unless told the committed lockfile is " +
			"authoritative: `uv sync`/`uv run` without --frozen or --locked may rewrite uv.lock, and " +
			"`poetry update` abandons poetry.lock outright. Pass --frozen, and prefer " +
			"`poetry install` against the committed lock.",
		Check:    checkUvPoetry,
		FixLevel: FixUnsafe,
		Fix:      lockfileFixer("PB209"),
	},
	{
		ID:       "PB210",
		Name:     "adhoc-package-install",
		Severity: Warn, MaxSeverity: Error, // error unless every package is pinned to an exact version
		Doc: "A package named on a package manager's command line — `npm install axios`, " +
			"`pip install requests`, `npx some-tool`, `gem install rails`, `cargo install foo` — is " +
			"fetched from a registry at build time, or at install time from a scriptlet, with nothing " +
			"in the PKGBUILD verifying what arrives: the registry serves whatever it has at that moment, " +
			"makepkg's checksums never see it, and npm, pip, gem, composer and cargo all run code out of " +
			"the package the moment it lands. This is the shape of the 2025 AUR compromises: a " +
			"`post_install` or `prepare()` gaining an `npm install` of a few registry packages whose " +
			"postinstall scripts carried the payload. Install a project's dependencies from its own " +
			"manifest and lockfile (`npm ci`, `pip install -r` with hashes, `bundle install --frozen`), " +
			"declare a tool the build needs in makedepends, or build it from a checksummed source= entry. " +
			"A spec pinned to an exact version or a full commit is at least reproducible, and is reported " +
			"at Warn rather than Error; so is a package word pkglint cannot read (`npm install $_deps`), " +
			"which could name anything.",
		Check: checkAdhocInstall,
	},
}

// networkCommands maps command names to a function deciding whether the
// specific invocation touches the network.
var networkCommands = map[string]func(Command) bool{
	"curl": always, "wget": always, "aria2c": always, "axel": always,
	"scp": always, "sftp": always, "rsync": rsyncRemote,
	"git": func(c Command) bool {
		switch c.Subcommand() {
		case "clone", "fetch", "pull", "ls-remote", "submodule", "remote":
			return true
		}
		return false
	},
	"svn": func(c Command) bool {
		sub := c.Subcommand()
		return sub == "checkout" || sub == "co" || sub == "update" || sub == "export"
	},
	"hg": func(c Command) bool {
		sub := c.Subcommand()
		return sub == "clone" || sub == "pull"
	},
	"go": func(c Command) bool {
		sub := c.Subcommand()
		if sub == "get" || (sub == "mod" && c.HasArg("download")) {
			return true
		}
		if sub == "install" {
			for _, a := range c.Args {
				if strings.Contains(a, "@") {
					return true
				}
			}
		}
		return false
	},
	"pip": pipFetches, "pip3": pipFetches,
	"python": pythonModuleFetches, "python3": pythonModuleFetches, "python2": pythonModuleFetches,
	"pipx": pipxFetches,
	"npm": func(c Command) bool {
		sub := c.Subcommand()
		if sub == "ci" || npmInstallSubs[sub] {
			return true
		}
		// `npm exec`, like npx, fetches only what the project does not provide.
		return (sub == "exec" || sub == "x") && adhocClaims(c)
	},
	"npx": adhocClaims, "bunx": adhocClaims,
	"pnpx": func(c Command) bool { return len(c.Args) > 0 },
	"pnpm": func(c Command) bool {
		switch c.Subcommand() {
		case "install", "add", "i", "update", "up", "upgrade", "dlx", "fetch":
			return true
		}
		return false
	},
	"yarn": func(c Command) bool {
		switch c.Subcommand() {
		case "install", "add", "", "up", "upgrade", "dlx", "global":
			return true
		}
		return false
	},
	"bun": func(c Command) bool {
		switch sub := c.Subcommand(); sub {
		case "install", "add", "i", "update":
			return true
		case "x":
			return adhocClaims(c)
		}
		return false
	},
	"cargo": func(c Command) bool {
		switch c.Subcommand() {
		case "fetch", "update", "add":
			return !c.HasArg("--offline")
		case "install":
			return !c.HasArg("--offline") && !c.HasArg("--path")
		}
		return false
	},
	"composer": func(c Command) bool {
		switch c.Subcommand() {
		case "install", "update", "require", "global", "create-project":
			return true
		}
		return false
	},
	"gem": gemInstallFetches,
	"bundle": func(c Command) bool {
		sub := c.Subcommand()
		return sub == "" || sub == "install" || sub == "update" // bare `bundle` runs install
	},
	"uv": func(c Command) bool {
		if c.HasArg("--offline") {
			return false
		}
		switch c.Subcommand() {
		case "sync", "add", "lock", "run":
			return true
		case "pip":
			return uvPipFetches(c)
		case "tool":
			return c.HasArg("install") || c.HasArg("run") || c.HasArg("upgrade") || c.HasArg("uvx")
		}
		return false
	},
	"uvx":    func(c Command) bool { return !c.HasArg("--offline") },
	"poetry": poetryFetches,
	"cpan":   always, "cpanm": always,
	"luarocks": func(c Command) bool {
		switch c.Subcommand() {
		case "install", "build", "download", "search":
			return true
		}
		return false
	},
	"cabal": func(c Command) bool {
		if c.HasArg("--offline") {
			return false
		}
		sub := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(c.Subcommand(), "v2-"), "v1-"), "new-")
		switch sub {
		case "install", "update", "fetch", "get":
			return true
		}
		return false
	},
	"dotnet": func(c Command) bool {
		switch c.Subcommand() {
		case "restore":
			return true
		case "tool":
			return c.HasArg("install") || c.HasArg("update") || c.HasArg("restore")
		case "add":
			return c.HasArg("package")
		}
		return false
	},
	"nuget": func(c Command) bool {
		switch c.Subcommand() {
		case "restore", "install", "update":
			return true
		}
		return false
	},
	"deno": func(c Command) bool {
		switch c.Subcommand() {
		case "install", "add", "cache":
			return true
		case "run", "compile", "bundle":
			return !c.HasArg("--cached-only") && !c.HasArg("--no-remote")
		}
		return false
	},
	"opam": func(c Command) bool {
		switch c.Subcommand() {
		case "install", "update", "upgrade", "pin", "init":
			return true
		}
		return false
	},
	"nimble": func(c Command) bool {
		switch c.Subcommand() {
		case "install", "refresh", "develop":
			return true
		}
		return false
	},
	"flatpak": func(c Command) bool {
		switch c.Subcommand() {
		case "install", "update", "remote-add":
			return true
		}
		return false
	},
	"snap": func(c Command) bool { sub := c.Subcommand(); return sub == "install" || sub == "refresh" },
}

func always(Command) bool { return true }

// rsyncRemote reports whether an rsync reaches another host. rsync is as
// often a cp with better flags (`rsync -a "$srcdir/opt" "$pkgdir/opt"`), and
// only an operand naming a host — host:path, user@host:path, host::module,
// rsync://… — or a remote shell (-e/--rsh) leaves the machine. An operand
// pkglint cannot read counts as remote unless it is rooted in the build tree.
func rsyncRemote(c Command) bool {
	for i, a := range c.Args {
		switch {
		case a == "-e" || strings.HasPrefix(a, "--rsh"):
			return true
		case strings.HasPrefix(a, "-"):
			if !strings.HasPrefix(a, "--") && strings.Contains(a, "e") {
				return true // -avze ssh
			}
			continue
		case strings.Contains(a, "://"):
			return true
		}
		head, _, _ := strings.Cut(a, "/")
		if strings.Contains(head, ":") {
			return true
		}
		if argOpaque(c, i) && !hasPrefixAny(a, "$pkgdir", "${pkgdir", "$srcdir", "${srcdir", "$startdir", "${startdir") {
			return true
		}
	}
	return false
}

func pipxFetches(c Command) bool {
	switch c.Subcommand() {
	case "install", "install-all", "inject", "run", "runpip", "upgrade", "upgrade-all", "reinstall", "reinstall-all":
		return true
	}
	return false
}

func poetryFetches(c Command) bool {
	switch c.Subcommand() {
	case "install", "update", "add", "lock":
		return true
	}
	return false
}

// pythonModuleCommands are the `python -m` modules that are package managers
// in their own right, with the networkCommands entry each stands for. (A
// separate table: networkCommands cannot refer to itself while initializing.)
var pythonModuleCommands = map[string]func(Command) bool{
	"pip": pipFetches, "pipx": pipxFetches, "poetry": poetryFetches,
}

// pythonModuleFetches answers for `python -m pip …` as networkCommands would
// for the module's own command.
func pythonModuleFetches(c Command) bool {
	m, ok := pythonModule(c)
	if !ok {
		return false
	}
	fetches, ok := pythonModuleCommands[m.Name]
	return ok && fetches(m)
}

func pipFetches(c Command) bool {
	sub := c.Subcommand()
	if sub != "install" && sub != "download" {
		return false
	}
	if c.HasArg("--no-index") {
		return false
	}
	// `pip install .` / local paths and wheels don't hit the index when
	// combined with --no-deps, a common packaging idiom.
	if c.HasArg("--no-deps") {
		remote := false
		for _, a := range c.Args {
			if a == sub || hasPrefixAny(a, "-", ".", "/", "$") || strings.HasSuffix(a, ".whl") {
				continue
			}
			remote = true
		}
		return remote
	}
	return true
}

// uvPipFetches mirrors pipFetches for `uv pip install/download`.
func uvPipFetches(c Command) bool {
	if c.Subcommand() != "pip" || (!c.HasArg("install") && !c.HasArg("download")) {
		return false
	}
	if c.HasArg("--no-index") {
		return false
	}
	if c.HasArg("--no-deps") {
		remote := false
		for _, a := range c.Args {
			if a == "pip" || a == "install" || a == "download" ||
				hasPrefixAny(a, "-", ".", "/", "$") || strings.HasSuffix(a, ".whl") {
				continue
			}
			remote = true
		}
		return remote
	}
	return true
}

// gemInstallFetches reports whether a gem install hits RubyGems rather than
// installing local .gem files.
func gemInstallFetches(c Command) bool {
	if c.Subcommand() != "install" || c.HasArg("--local") || c.HasArg("-l") {
		return false
	}
	remote := false
	for _, a := range c.Args {
		if a == "install" || hasPrefixAny(a, "-", ".", "/", "$") || strings.HasSuffix(a, ".gem") {
			continue
		}
		remote = true
	}
	return remote
}

func checkNetworkInBuild(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		if !c.InBuildPhase() {
			continue
		}
		if c.Name == "go" { // handled with more context by PB204
			continue
		}
		fetches, ok := networkCommands[c.Name]
		if !ok || !fetches(c) {
			continue
		}
		out = append(out, c.finding("PB201", Warn,
			"%s downloads during %s(); network access belongs in prepare() with pinned hashes", c.Name, c.Fn))
	}
	return out
}

func checkPipHashes(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.CommandsNamed("pip", "pip3") {
		if c.Subcommand() != "install" || !pipFetches(c) {
			continue
		}
		// A requirement named on the command line has no digest to require;
		// PB210 gives it the remedy it can act on.
		if c.HasArg("--require-hashes") || adhocClaims(c) {
			continue
		}
		out = append(out, c.finding("PB202", Warn,
			"pip install without --require-hashes: downloads are not verified against pinned digests"))
	}
	for _, c := range ctx.CommandsNamed("uv") {
		if !c.HasArg("install") || !uvPipFetches(c) || c.HasArg("--require-hashes") || adhocClaims(c) {
			continue
		}
		out = append(out, c.finding("PB202", Warn,
			"uv pip install without --require-hashes: downloads are not verified against pinned digests"))
	}
	return out
}

// cargoUnlockedCommands returns the cargo invocations that resolve dependencies
// without being pinned to Cargo.lock: the commands PB203 reports, and the ones
// its fix appends --locked to, so rule and fix cannot drift apart.
//
// The lockfile flags are read through cargoWords, so a PKGBUILD that keeps them
// in a variable and expands it at every call site has passed them. A command
// carrying a word pkglint cannot read at all (cargoFlagsHidden) is left out
// entirely: --locked may be inside it. That is only about words that could be
// flags — the `--target "$CARCH-unknown-linux-gnu"` of the guidelines' own
// `cargo fetch` is a value, and the command stays reportable.
func cargoUnlockedCommands(ctx *Context) []Command {
	var out []Command
	for _, c := range ctx.CommandsNamed("cargo") {
		switch c.Subcommand() {
		case "build", "install", "fetch", "test", "rustc":
		default:
			continue
		}
		if cargoHasFlag(c, "--locked", "--frozen", "--offline") || cargoFlagsHidden(c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func checkCargoLocked(ctx *Context) []Finding {
	var out []Finding
	for _, c := range cargoUnlockedCommands(ctx) {
		out = append(out, c.finding("PB203", Warn,
			"cargo %s without --locked: dependency resolution may diverge from Cargo.lock", c.Subcommand()))
	}
	return out
}

// goVendored reports whether the PKGBUILD vendors Go modules or pre-fetches
// them in prepare(), making implicit build-phase downloads unnecessary.
func goVendored(ctx *Context) bool {
	for _, c := range ctx.CommandsNamed("go") {
		if c.HasArg("-mod=vendor") || (c.Subcommand() == "mod" && (c.HasArg("vendor") || c.HasArg("download")) && !c.InBuildPhase()) {
			return true
		}
	}
	for _, v := range ctx.Pkg.Vars {
		for _, val := range v.Values {
			if strings.Contains(val, "-mod=vendor") {
				return true
			}
		}
	}
	for _, e := range ctx.Pkg.Sources() {
		if isVendorArchive(e) {
			return true
		}
	}
	return false
}

// goCommandVendored reports whether this particular command reads from a
// vendor tree rather than the module cache: `-mod=vendor` reaches it through
// its arguments or a GOFLAGS in scope — the guidelines' `export GOFLAGS=…`
// inside build() is where most PKGBUILDs put it, out of goVendored's reach —
// or module mode is switched off altogether, which makes go read the GOPATH
// tree the PKGBUILD assembled and never fetch anything.
func goCommandVendored(ctx *Context, c Command) bool {
	if goFlagAddressed(goFlags(ctx, c), c, "-mod=vendor") {
		return true
	}
	modes := assignmentsTo(ctx, "GO111MODULE", c)
	return len(modes) > 0 && modes[len(modes)-1] == "off"
}

// isVendorArchive reports whether a source looks like a bundled vendored-deps
// archive (vendor.tar.gz, foo-1.0-vendor.tar.zst, a local "vendor" bundle),
// rather than any URL that merely contains the substring "vendor" (e.g. a
// GitHub org literally named "somevendor"). Only the file-name component is
// considered.
func isVendorArchive(e pkgbuild.SourceEntry) bool {
	name := e.Filename // the `name::url` local name, when present
	if name == "" {
		name = path.Base(e.URL)
	}
	name = strings.ToLower(name)
	return strings.HasPrefix(name, "vendor") ||
		strings.Contains(name, "-vendor.") ||
		strings.Contains(name, "vendor.tar")
}

// mutableGoRef reports whether a go package argument pins to a mutable
// @version suffix (latest, a branch name, ...).
func mutableGoRef(arg string) (string, bool) {
	i := strings.LastIndexByte(arg, '@')
	if i < 0 {
		return "", false
	}
	switch ref := arg[i+1:]; ref {
	case "latest", "upgrade", "patch", "master", "main", "HEAD", "tip":
		return ref, true
	}
	return "", false
}

func checkGoDownloads(ctx *Context) []Finding {
	var out []Finding
	// A mutable @ref is a problem in any phase: even fetched in prepare(),
	// what gets built depends on whatever upstream published most recently.
	mutable := map[*syntax.Stmt]bool{}
	for _, c := range ctx.CommandsNamed("go") {
		sub := c.Subcommand()
		if sub != "install" && sub != "run" && sub != "get" {
			continue
		}
		for _, a := range c.Args {
			if ref, ok := mutableGoRef(a); ok {
				out = append(out, c.finding("PB204", Warn,
					"go %s %s resolves mutable ref @%s over the network; pin an exact version or commit", sub, a, ref))
				mutable[c.Stmt] = true
				break
			}
		}
	}
	if goVendored(ctx) {
		return out
	}
	for _, c := range ctx.CommandsNamed("go") {
		if !c.InBuildPhase() || mutable[c.Stmt] || goCommandVendored(ctx, c) {
			continue
		}
		sub := c.Subcommand()
		if sub != "build" && sub != "install" && sub != "test" && sub != "run" && sub != "get" && !(sub == "mod" && c.HasArg("download")) {
			continue
		}
		out = append(out, c.finding("PB204", Warn,
			"go %s may download modules during %s(); vendor them or `go mod download` in prepare()", sub, c.Fn))
	}
	return out
}

// goEnvWeakens reports whether an environment assignment disables Go's module
// checksum database or permits insecure module transports, with a message
// describing the weakening.
func goEnvWeakens(name, value string) (msg string, bad bool) {
	switch name {
	case "GOSUMDB":
		if strings.TrimSpace(value) == "off" {
			return "GOSUMDB=off disables Go's checksum database", true
		}
	case "GONOSUMCHECK", "GONOSUMDB":
		return name + " disables Go module checksum verification", true
	case "GOINSECURE":
		return "GOINSECURE allows fetching modules over insecure transports", true
	case "GOFLAGS":
		if strings.Contains(value, "-insecure") {
			return "GOFLAGS contains -insecure", true
		}
	}
	return "", false
}

func checkGoEnvWeakening(ctx *Context) []Finding {
	var out []Finding
	bad := goEnvWeakens
	for name, v := range ctx.Pkg.Vars {
		if len(v.Values) == 1 {
			if msg, isBad := bad(name, v.Values[0]); isBad {
				out = append(out, findingAt("PB205", Warn, ctx.Pkg.PKGBUILD.Path, v.Pos, "%s", msg))
			}
		}
	}
	for _, c := range ctx.Commands() {
		for _, as := range c.Call.Assigns {
			if as.Name == nil {
				continue
			}
			val := ""
			if as.Value != nil {
				val, _ = renderPlain(as.Value)
			}
			if msg, isBad := bad(as.Name.Value, val); isBad {
				out = append(out, c.finding("PB205", Warn, "%s", msg))
			}
		}
	}
	// `export`/`declare`/`local`/`readonly` are DeclClause nodes, never
	// CallExpr, so they never reach ctx.Commands(). Walk function bodies for
	// them. Top-level ones are already handled via ctx.Pkg.Vars above.
	for _, u := range ctx.Pkg.Units() {
		for _, fn := range u.Functions {
			syntax.Walk(fn.Body, func(n syntax.Node) bool {
				decl, ok := n.(*syntax.DeclClause)
				if !ok {
					return true
				}
				for _, as := range decl.Args {
					if as.Name == nil {
						continue
					}
					val := ""
					if as.Value != nil {
						val, _ = renderPlain(as.Value)
					}
					if msg, isBad := bad(as.Name.Value, val); isBad {
						out = append(out, findingAt("PB205", Warn, u.Path, as.Pos(), "%s", msg))
					}
				}
				return true
			})
		}
	}
	return out
}

// lockfileFlag is one lockfile-enforcing flag as PB206–PB209 see it: the
// commands that take it, which subcommands install from the lockfile, the
// flags that already pin it, and the wording of the finding and the fix.
// checkLockfileFlags and fixLockfileFlags read the same entry, so the checker
// and the fixer cannot disagree about what counts as pinned.
type lockfileFlag struct {
	id       string
	severity Severity
	commands []string
	// subs are the subcommands the checker reports; "" is the bare command.
	subs []string
	// fixSubs are the subcommands the fixer appends to. nil means the same
	// as subs; a narrower list is a subcommand where a trailing flag would
	// change the command's meaning or the fix is not wanted.
	fixSubs []string
	// satisfied lists the flags any one of which already pins the lockfile.
	satisfied []string
	flag      string
	// yields, when set, hands an invocation to another rule: the flag would
	// change the command's meaning rather than pin it, so neither the finding
	// nor the fix is this rule's.
	yields func(Command) bool
	// message and desc build the finding text and the fix description from
	// the resolved command name and subcommand.
	message func(name, sub string) string
	desc    func(name, sub string) string
}

var lockfileFlags = []lockfileFlag{
	{
		id: "PB206", severity: Info,
		commands: []string{"yarn"}, subs: []string{"install", ""},
		satisfied: []string{"--immutable", "--frozen-lockfile"}, flag: "--immutable",
		message: func(string, string) string {
			return "yarn install without --immutable/--frozen-lockfile may rewrite the dependency tree"
		},
		desc: func(string, string) string { return "append --immutable to `yarn install`" },
	},
	{
		id: "PB206", severity: Info,
		commands: []string{"pnpm", "bun"}, subs: []string{"install", "i"},
		satisfied: []string{"--frozen-lockfile", "--offline"}, flag: "--frozen-lockfile",
		// With package args the command adds a dependency; freezing the
		// lockfile is not equivalent. PB210 reports what it fetches.
		yields: namesPackages,
		message: func(name, sub string) string {
			return fmt.Sprintf("%s %s without --frozen-lockfile may rewrite the dependency tree", name, sub)
		},
		desc: func(name, sub string) string {
			return fmt.Sprintf("append --frozen-lockfile to `%s %s`", name, sub)
		},
	},
	{
		id: "PB207", severity: Warn,
		commands: []string{"composer"}, subs: []string{"install"},
		satisfied: []string{"--no-scripts"}, flag: "--no-scripts",
		message: func(string, string) string {
			return "composer install without --no-scripts executes hook scripts and plugins from the dependency tree"
		},
		desc: func(string, string) string { return "append --no-scripts to `composer install`" },
	},
	{
		id: "PB208", severity: Info,
		// Bare `bundle` runs install, so it is reported; the fix leaves it
		// alone rather than turn `bundle` into `bundle --frozen`.
		commands: []string{"bundle", "bundler"}, subs: []string{"install", ""}, fixSubs: []string{"install"},
		satisfied: []string{"--frozen", "--deployment", "--local"}, flag: "--frozen",
		message: func(string, string) string {
			return "bundle install without --frozen may rewrite Gemfile.lock and fetch unreviewed versions"
		},
		desc: func(string, string) string { return "append --frozen to `bundle install`" },
	},
	{
		id: "PB209", severity: Warn,
		// Only `uv sync` is fixed: for `uv run` a trailing flag would land
		// on the command being run, not on uv.
		commands: []string{"uv"}, subs: []string{"sync", "run"}, fixSubs: []string{"sync"},
		satisfied: []string{"--frozen", "--locked", "--offline", "--no-sync"}, flag: "--frozen",
		message: func(_, sub string) string {
			return fmt.Sprintf("uv %s without --frozen/--locked may re-resolve dependencies and rewrite uv.lock", sub)
		},
		desc: func(string, string) string { return "append --frozen to `uv sync`" },
	},
}

// unpinned reports whether c is one of l's lockfile-installing subcommands
// with none of the satisfying flags, and which subcommand it is. A command
// that names packages (`pnpm install left-pad`) installs no lockfile, so
// freezing one is not its remedy; PB210 reports it instead.
func (l lockfileFlag) unpinned(c Command) (sub string, ok bool) {
	sub = c.Subcommand()
	if !slices.Contains(l.subs, sub) || adhocClaims(c) || (l.yields != nil && l.yields(c)) {
		return "", false
	}
	for _, f := range l.satisfied {
		if c.HasArg(f) {
			return "", false
		}
	}
	return sub, true
}

// checkLockfileFlags reports every unpinned invocation of the lockfileFlags
// entries registered under id.
func checkLockfileFlags(ctx *Context, id string) []Finding {
	var out []Finding
	for _, l := range lockfileFlags {
		if l.id != id {
			continue
		}
		for _, c := range ctx.CommandsNamed(l.commands...) {
			sub, ok := l.unpinned(c)
			if !ok {
				continue
			}
			out = append(out, c.finding(l.id, l.severity, "%s", l.message(c.Name, sub)))
		}
	}
	return out
}

func checkNpmCI(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.CommandsNamed("npm") {
		sub := c.Subcommand()
		// `npm install <anything>` — a registry name, a tarball, a path — is
		// not a manifest install, and `npm ci` is not its remedy; a registry
		// name is PB210's.
		if (sub == "install" || sub == "i") && !namesPackages(c) {
			out = append(out, c.finding("PB206", Info,
				"npm %s may rewrite the dependency tree; prefer `npm ci` against the committed lockfile", sub))
		}
	}
	return append(out, checkLockfileFlags(ctx, "PB206")...)
}

func checkComposer(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.CommandsNamed("composer") {
		switch sub := c.Subcommand(); sub {
		case "update", "upgrade":
			out = append(out, c.finding("PB207", Warn,
				"composer %s re-resolves dependencies away from the committed composer.lock; use `composer install`", sub))
		}
	}
	return append(out, checkLockfileFlags(ctx, "PB207")...)
}

func checkBundler(ctx *Context) []Finding {
	return checkLockfileFlags(ctx, "PB208")
}

func checkUvPoetry(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.CommandsNamed("poetry") {
		if c.Subcommand() != "update" {
			continue
		}
		out = append(out, c.finding("PB209", Warn,
			"poetry update re-resolves dependencies away from the committed poetry.lock; use `poetry install`"))
	}
	return append(out, checkLockfileFlags(ctx, "PB209")...)
}
