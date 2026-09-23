package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB210: a package named on a package manager's command line.
//
// `npm install axios` in a PKGBUILD or an install scriptlet is not a build
// step, it is a download-and-execute with a friendlier name: the registry
// serves whatever version it has at that moment, nothing in the PKGBUILD
// verifies what arrives, makepkg's checksums never see it, and most package
// managers run the package's own install hooks the moment it lands. The
// malicious AUR pushes of 2025 used exactly this shape — a `post_install`
// or `prepare()` gaining an `npm install` of a handful of registry packages
// whose postinstall scripts carried the payload.
//
// The rule covers every package manager pkglint knows that will fetch a
// package by name, and reads the specs through argWords, so a package list
// the PKGBUILD keeps in a variable is still read. A spec pinned to an exact
// version or a full commit is reported at Warn rather than Error: what arrives
// is then at least reproducible, though still unverified.
//
// Two ecosystems are deliberately left to other rules. `go install pkg@vN` is
// verified by the module checksum database and runs nothing on install, so it
// stays with PB204 (which reports the mutable @latest form). And a bare
// `npm install`/`pip install -r`/`bundle install` installs the project's own
// manifest, which is what this rule asks for; whether that manifest is
// locked is PB202/PB206–PB209's question.

// registrySpec is one package an invocation fetches by name.
type registrySpec struct {
	Spec string
	// Pinned means an exact version or a full commit hash: the fetch is
	// reproducible, if still unverified.
	Pinned bool
	// Ref marks a URL or VCS shorthand rather than a registry name, so the
	// message does not claim a registry served it.
	Ref bool
	// Unread marks a word that was still an expansion after rendering and
	// is not plainly a path: pkglint cannot say what it names, only that
	// the installer will look it up.
	Unread bool
}

// adhocInstaller describes one package manager for PB210: how to read the
// specs off its command line, where they come from, and what to do instead.
type adhocInstaller struct {
	// registry names where the packages come from ("the npm registry").
	registry string
	// runs says what the package manager executes out of the fetched
	// package on arrival, or "" when nothing runs until the build uses it.
	// `{it}` and `{its}` are replaced by the pronoun for the spec count.
	runs string
	// advice is the remedy, phrased for the ecosystem.
	advice string
	// valueFlags are the options that take their value as the next word.
	valueFlags map[string]bool
	// specs returns the registry packages the invocation names; nil when it
	// installs from the project's manifest, from local files, or is a
	// subcommand this rule does not judge.
	specs func(s scannedArgs) []registrySpec
}

// scannedArgs is a command line split into flags and positionals with the
// installer's value-taking flags resolved.
type scannedArgs struct {
	flags  []string            // as written; `--flag=value` kept whole
	values map[string][]string // value flag → the values it was given, opaque ones included
	pos    []string
	// shown maps a positional pkglint could not resolve to the text it was
	// written as, for the message: `"${_p:-axios}"` renders to a marker,
	// which is nothing a reader can act on.
	shown map[string]string
	// unit is the file the command came from, for the exemptions that
	// depend on what else the file does.
	unit *pkgbuild.Unit
}

// has reports whether any of flags was passed, in either spelling.
func (s scannedArgs) has(flags ...string) bool {
	for _, f := range s.flags {
		name, _, _ := strings.Cut(f, "=")
		for _, want := range flags {
			if name == want {
				return true
			}
		}
	}
	return false
}

// value returns every value given to any of flags.
func (s scannedArgs) value(flags ...string) []string {
	var out []string
	for _, f := range flags {
		out = append(out, s.values[f]...)
	}
	return out
}

// scanArgs reads a command's words. Values of the installer's value-taking
// flags are set aside (`--prefix /usr` names no package), a `--flag=value`
// word records its value under the flag, and everything after `--` is a
// positional. Words come through argWords, so an array the PKGBUILD keeps its
// package list in is expanded; a word that stays opaque is kept as it
// rendered, for the classifiers to tell a path (the guidelines' own
// `"$srcdir/….tgz"`) from a package pkglint cannot name (`$_deps`), with the
// text it was written as set aside for the message.
func scanArgs(c Command, valueFlags map[string]bool) scannedArgs {
	s := scannedArgs{values: map[string][]string{}, shown: map[string]string{}, unit: c.Unit}
	var words []string
	for i := range c.Args {
		ws := argWords(c, i)
		if len(ws) == 1 && hasVarRef(ws[0]) {
			if src := wordSource(c, i); src != "" {
				s.shown[ws[0]] = src
			}
		}
		words = append(words, ws...)
	}
	rest := false
	for i := 0; i < len(words); i++ {
		w := words[i]
		if rest || !strings.HasPrefix(w, "-") || w == "-" {
			s.pos = append(s.pos, w)
			continue
		}
		if w == "--" {
			rest = true
			continue
		}
		if name, val, ok := strings.Cut(w, "="); ok && valueFlags[name] {
			s.flags = append(s.flags, w)
			s.values[name] = append(s.values[name], val)
			continue
		}
		s.flags = append(s.flags, w)
		if valueFlags[w] && i+1 < len(words) {
			i++
			s.values[w] = append(s.values[w], words[i])
		}
	}
	return s
}

// wordSource returns the command's i'th argument as the file spells it, or
// "" when the command was not read from a file.
func wordSource(c Command, i int) string {
	if c.Unit == nil || i >= len(c.ArgWord) || c.ArgWord[i] == nil {
		return ""
	}
	w := c.ArgWord[i]
	start, end := off(w.Pos()), off(w.End())
	if start < 0 || end > len(c.Unit.Raw) || start >= end {
		return ""
	}
	return string(c.Unit.Raw[start:end])
}

// shownAs returns a positional as the PKGBUILD wrote it when pkglint could
// not resolve it, and the word itself otherwise.
func (s scannedArgs) shownAs(w string) string {
	if src, ok := s.shown[w]; ok {
		return src
	}
	return strings.ReplaceAll(w, "\x00", "…")
}

// sub returns the first positional — the subcommand — and the positionals
// after it.
func (s scannedArgs) sub() (string, []string) {
	if len(s.pos) == 0 {
		return "", nil
	}
	return s.pos[0], s.pos[1:]
}

// --- spec classification ------------------------------------------------------

var (
	// semverExact is a full three-part version, the only npm-style spec that
	// names exactly one release: `1`, `1.2`, `1.x` and `^1.2.3` are ranges.
	semverExact = regexp.MustCompile(`^=?v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.+-]*)?$`)
	// exactVersion is a plain dotted version with no range operator, for the
	// ecosystems where `==1.2` or `foo.1.2` is exact.
	exactVersion = regexp.MustCompile(`^v?[0-9]+(?:\.[0-9]+)*(?:[-+.~][0-9A-Za-z.+-]*)?$`)
	// commitHash is a full git object id.
	commitHash = regexp.MustCompile(`(?:^|[#@=/:])([0-9a-f]{40}|[0-9a-f]{64})(?:$|[/?&])`)
)

// localArchiveSuffixes are files a package manager installs from disk rather
// than looking up in a registry.
var localArchiveSuffixes = []string{
	".tgz", ".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst", ".tar", ".zip",
	".whl", ".gem", ".rock", ".rockspec", ".nupkg", ".crate", ".tar.lz",
}

// isLocalSpec reports whether a word names something on disk — a path, an
// archive, a `file:` URL — rather than a package to look up.
func isLocalSpec(w string) bool {
	if w == "." || w == ".." || hasPrefixAny(w, "./", "../", "/", "~", "file:") {
		return true
	}
	if strings.Contains(w, "://") {
		return false // a remote archive is fetched, however it is named
	}
	lower := strings.ToLower(w)
	for _, suf := range localArchiveSuffixes {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	return false
}

// unreadSpec classifies a word that is still an expansion after rendering.
// One that is plainly on disk — under $srcdir, $pkgdir or $startdir, holding
// a directory separator, or an archive by suffix — installs from a file and
// is no spec. Anything else may name a package, and pkglint can say no more
// about it than that; the rule reports it rather than guessing either way.
func unreadSpec(w string) (registrySpec, bool) {
	if isLocalSpec(w) || strings.Contains(w, "/") || hasPrefixAny(w, "$srcdir", "$pkgdir", "$startdir") {
		return registrySpec{}, false
	}
	return registrySpec{Spec: w, Unread: true}, true
}

// isRefSpec reports whether a word is a URL or VCS locator.
func isRefSpec(w string) bool {
	return strings.Contains(w, "://") ||
		hasPrefixAny(w, "git@", "git+", "github:", "gitlab:", "bitbucket:", "gist:", "hg+", "svn+")
}

// refSpec classifies a URL or VCS shorthand: pinned only by a full commit.
func refSpec(w string) registrySpec {
	return registrySpec{Spec: w, Ref: true, Pinned: commitHash.MatchString(w)}
}

var npmName = regexp.MustCompile(`^(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+$`)

// npmSpec classifies an npm-style package argument: a registry name with an
// optional `@version`, an alias (`npm:name@version`), a GitHub `user/repo`
// shorthand, or a URL. A local path or tarball is not a spec.
func npmSpec(w string) (registrySpec, bool) {
	if hasVarRef(w) {
		return unreadSpec(w)
	}
	if isLocalSpec(w) {
		return registrySpec{}, false
	}
	w = strings.TrimPrefix(w, "npm:")
	if i := strings.Index(w, "@npm:"); i > 0 {
		w = w[i+len("@npm:"):] // an alias: `foo@npm:bar@1.0.0` installs bar
	}
	if isRefSpec(w) {
		return refSpec(w), true
	}
	name, version := w, ""
	if i := strings.LastIndexByte(w, '@'); i > 0 {
		name, version = w[:i], w[i+1:]
	}
	if !strings.HasPrefix(name, "@") && strings.Contains(name, "/") {
		// `user/repo[#ref]` is GitHub shorthand to npm.
		return refSpec(w), true
	}
	if !npmName.MatchString(strings.TrimPrefix(name, "@")) && !npmName.MatchString(name) {
		return registrySpec{}, false
	}
	return registrySpec{Spec: w, Pinned: semverExact.MatchString(version)}, true
}

// npmSpecs classifies every positional.
func npmSpecs(pos []string) []registrySpec {
	var out []registrySpec
	for _, w := range pos {
		if spec, ok := npmSpec(w); ok {
			out = append(out, spec)
		}
	}
	return out
}

var pep508Name = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?(?:\[[^\]]*\])?`)

// pipSpec classifies a pip-style requirement: `name`, `name[extra]==1.2`,
// a VCS URL, or a direct URL. Only `==`/`===` to a version without a wildcard
// is exact; every other operator is a range the index resolves.
func pipSpec(w string) (registrySpec, bool) {
	if hasVarRef(w) {
		return unreadSpec(w)
	}
	if isLocalSpec(w) {
		return registrySpec{}, false
	}
	if isRefSpec(w) {
		return refSpec(w), true
	}
	loc := pep508Name.FindStringIndex(w)
	if loc == nil {
		return registrySpec{}, false
	}
	rest := strings.TrimSpace(w[loc[1]:])
	if rest != "" && !hasPrefixAny(rest, "=", "<", ">", "~", "!", "@", ";") {
		return registrySpec{}, false
	}
	pinned := false
	if v, ok := strings.CutPrefix(rest, "==="); ok {
		pinned = exactVersion.MatchString(strings.TrimSpace(v))
	} else if v, ok := strings.CutPrefix(rest, "=="); ok {
		v = strings.TrimSpace(v)
		pinned = !strings.Contains(v, "*") && !strings.Contains(v, ",") && exactVersion.MatchString(v)
	} else if v, ok := strings.CutPrefix(rest, "@"); ok {
		// PEP 508 direct reference: `name @ url`.
		return refSpec(strings.TrimSpace(v)), true
	}
	return registrySpec{Spec: w, Pinned: pinned}, true
}

func pipSpecs(pos []string) []registrySpec {
	var out []registrySpec
	for _, w := range pos {
		if spec, ok := pipSpec(w); ok {
			out = append(out, spec)
		}
	}
	return out
}

// namedSpec classifies a plain registry name with an optional exact-version
// marker: `sep` is what separates name from version (`@`, `:`, `.`, `==`),
// and exact decides whether the version names one release. A name is
// anything word-like; paths, archives and URLs are handed to the ref/local
// classifiers first.
func namedSpec(w, sep string, exact *regexp.Regexp) (registrySpec, bool) {
	if hasVarRef(w) {
		return unreadSpec(w)
	}
	if isLocalSpec(w) {
		return registrySpec{}, false
	}
	if isRefSpec(w) {
		return refSpec(w), true
	}
	name, version := w, ""
	if sep != "" {
		if i := strings.LastIndex(w, sep); i > 0 {
			name, version = w[:i], w[i+len(sep):]
		}
	}
	if name == "" || strings.ContainsAny(name, " \t=<>!^~*|,") {
		return registrySpec{}, false
	}
	return registrySpec{Spec: w, Pinned: version != "" && exact.MatchString(version)}, true
}

func namedSpecs(pos []string, sep string, exact *regexp.Regexp) []registrySpec {
	var out []registrySpec
	for _, w := range pos {
		if spec, ok := namedSpec(w, sep, exact); ok {
			out = append(out, spec)
		}
	}
	return out
}

// pinAll marks every spec pinned when a separate --version flag names an
// exact release, the spelling cargo, gem, dotnet and nuget use. A version
// pkglint cannot read (`-v "$_ver"`) counts as one: the flag says a version
// was chosen, and Warn still says nothing verifies what it fetches.
func pinAll(specs []registrySpec, versions []string, exact *regexp.Regexp) []registrySpec {
	if len(versions) == 0 {
		return specs
	}
	pinned := true
	for _, v := range versions {
		if !hasVarRef(v) && (strings.Contains(v, "*") || !exact.MatchString(strings.TrimPrefix(v, "="))) {
			pinned = false
		}
	}
	if !pinned {
		return specs
	}
	for i := range specs {
		if !specs[i].Ref && !specs[i].Unread {
			specs[i].Pinned = true
		}
	}
	return specs
}

// --- the installers -----------------------------------------------------------

// nodeValueFlags are the value-taking options shared across npm, yarn, pnpm
// and bun; each tool adds its own on top.
var nodeValueFlags = flagSet(
	"--cache", "--prefix", "--registry", "--userconfig", "--globalconfig", "--tag",
	"--loglevel", "--omit", "--include", "--workspace", "-w", "--script-shell",
	"--audit-level", "--install-strategy", "--before", "--otp", "--depth",
	"--fetch-retries", "--fetch-retry-mintimeout", "--fetch-retry-maxtimeout",
	"--fetch-timeout", "--maxsockets", "--proxy", "--https-proxy", "--noproxy",
	"--user", "--group", "--cafile", "--ca", "--cert", "--key", "--nodedir",
	"--node-options", "--lockfile-version", "--logs-dir", "--os", "--cpu", "--libc",
	"--scope", "--access", "--tmp", "--location", "--auth-type", "--package", "-p",
	"--call", "-c", "--cwd", "--reporter", "--filter", "-F", "--dir", "-C",
	"--store-dir", "--virtual-store-dir", "--modules-dir", "--cache-dir",
	"--lockfile-dir", "--config", "--child-concurrency", "--network-concurrency",
	"--use-node-version", "--node-version", "--cache-folder", "--modules-folder",
	"--mutex", "--network-timeout", "--global-folder", "--link-folder",
	"--preferred-cache-folder", "--use-yarnrc", "--mode", "--backend",
	"--concurrent-scripts", "--shell", "--allow-build", "--workspace-concurrency",
)

func flagSet(flags ...string) map[string]bool {
	m := make(map[string]bool, len(flags))
	for _, f := range flags {
		m[f] = true
	}
	return m
}

// npmInstallSubs are the spellings npm accepts for install, plus the
// subcommands that install named packages by other names.
var npmInstallSubs = map[string]bool{
	"install": true, "i": true, "in": true, "ins": true, "inst": true, "insta": true,
	"instal": true, "isnt": true, "isnta": true, "isntal": true, "isntall": true,
	"add": true, "install-test": true, "it": true, "install-ci-test": true, "cit": true,
	"update": true, "up": true, "upgrade": true, "udpate": true,
}

// runnerSpecs reads an npx-style "fetch a package and run its binary"
// invocation: the package is the first positional, unless --package names it
// and the positional is the command to run. A tool the project's own
// manifest provides resolves locally, so with a manifest install in the same
// file the fetch only happens when --yes or --package asks for one; a
// scriptlet never installs a manifest, so there it always fetches.
// --no-install (--no) forbids the fetch outright.
func runnerSpecs(s scannedArgs, pos []string) []registrySpec {
	if s.has("--no-install", "--no") {
		return nil
	}
	if pkgs := s.value("--package", "-p"); len(pkgs) > 0 {
		return npmSpecs(pkgs)
	}
	if len(pos) == 0 {
		return nil
	}
	if !s.has("--yes", "-y") && manifestInstallIn(s.unit) {
		return nil
	}
	return npmSpecs(pos[:1])
}

// nodeAdvice is the remedy for every npm-family install.
const nodeAdvice = "install from the project's package.json and lockfile (`npm ci`) or declare the tool in makedepends"

var adhocInstallers = map[string]adhocInstaller{
	"npm": {
		registry: "the npm registry", runs: "runs {its} install scripts", advice: nodeAdvice,
		valueFlags: nodeValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			switch {
			case npmInstallSubs[sub]:
				return npmSpecs(pos)
			case sub == "exec" || sub == "x":
				return runnerSpecs(s, pos)
			}
			return nil
		},
	},
	"npx": {
		registry: "the npm registry", runs: "runs {it}", advice: nodeAdvice,
		valueFlags: nodeValueFlags,
		specs:      func(s scannedArgs) []registrySpec { return runnerSpecs(s, s.pos) },
	},
	"yarn": {
		registry: "the npm registry", runs: "runs {its} install scripts", advice: nodeAdvice,
		valueFlags: nodeValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			switch sub {
			case "add", "up", "upgrade":
				return npmSpecs(pos)
			case "global":
				if len(pos) > 0 && pos[0] == "add" {
					return npmSpecs(pos[1:])
				}
			case "dlx":
				if pkgs := s.value("--package", "-p"); len(pkgs) > 0 {
					return npmSpecs(pkgs)
				}
				if len(pos) > 0 {
					return npmSpecs(pos[:1])
				}
			}
			return nil
		},
	},
	"pnpm": {
		registry: "the npm registry", advice: nodeAdvice,
		valueFlags: nodeValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			switch sub {
			case "add", "install", "i", "update", "up", "upgrade":
				return npmSpecs(pos)
			case "dlx":
				if pkgs := s.value("--package", "-p"); len(pkgs) > 0 {
					return npmSpecs(pkgs)
				}
				if len(pos) > 0 {
					return npmSpecs(pos[:1])
				}
			}
			return nil
		},
	},
	"pnpx": {
		registry: "the npm registry", runs: "runs {it}", advice: nodeAdvice,
		valueFlags: nodeValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			if len(s.pos) > 0 {
				return npmSpecs(s.pos[:1])
			}
			return nil
		},
	},
	"bun": {
		registry: "the npm registry", advice: nodeAdvice,
		valueFlags: nodeValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			switch sub {
			case "add", "install", "i", "update":
				return npmSpecs(pos)
			case "x":
				return runnerSpecs(s, pos)
			}
			return nil
		},
	},
	"bunx": {
		registry: "the npm registry", runs: "runs {it}", advice: nodeAdvice,
		valueFlags: nodeValueFlags,
		specs:      func(s scannedArgs) []registrySpec { return runnerSpecs(s, s.pos) },
	},
	"pip": {
		registry: "PyPI", runs: "runs {its} build backend", advice: pipAdvice,
		valueFlags: pipValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			if sub, pos := s.sub(); sub == "install" && !s.has("--no-index") {
				return pipSpecs(pos)
			}
			return nil
		},
	},
	"pipx": {
		registry: "PyPI", runs: "runs {its} build backend", advice: "declare the tool in makedepends",
		valueFlags: flagSet("--python", "--pip-args", "--index-url", "-i", "--spec", "--suffix", "--preinstall", "--fetch-missing-python"),
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			switch sub {
			case "install":
				return pipSpecs(pos)
			case "run":
				if spec := s.value("--spec"); len(spec) > 0 {
					return pipSpecs(spec)
				}
				if len(pos) > 0 {
					return pipSpecs(pos[:1])
				}
			case "inject":
				if len(pos) > 1 {
					return pipSpecs(pos[1:])
				}
			}
			return nil
		},
	},
	"uv": {
		registry: "PyPI", runs: "runs {its} build backend", advice: pipAdvice,
		valueFlags: uvValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			if s.has("--offline", "--no-index") {
				return nil
			}
			sub, pos := s.sub()
			with := pipSpecs(s.value("--with"))
			switch sub {
			case "add":
				return append(pipSpecs(pos), with...)
			case "pip":
				if len(pos) > 0 && pos[0] == "install" {
					return pipSpecs(pos[1:])
				}
			case "tool":
				if len(pos) == 0 {
					return nil
				}
				switch pos[0] {
				case "install", "upgrade":
					return append(pipSpecs(pos[1:]), with...)
				case "run", "uvx":
					return append(uvxSpecs(s, pos[1:]), with...)
				}
			case "run":
				return with
			}
			return nil
		},
	},
	"uvx": {
		registry: "PyPI", runs: "runs {it}", advice: "declare the tool in makedepends",
		valueFlags: uvValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			if s.has("--offline", "--no-index") {
				return nil
			}
			return append(uvxSpecs(s, s.pos), pipSpecs(s.value("--with"))...)
		},
	},
	"poetry": {
		registry: "PyPI", runs: "runs {its} build backend", advice: pipAdvice,
		valueFlags: flagSet("--group", "-G", "--extras", "-E", "--source", "--python", "--platform", "--optional", "-C", "--directory", "--project", "-P", "--markers"),
		specs: func(s scannedArgs) []registrySpec {
			if sub, pos := s.sub(); sub == "add" {
				return pipSpecs(pos)
			}
			return nil
		},
	},
	"gem": {
		registry: "RubyGems", runs: "builds {its} native extensions",
		advice:     "declare the gem as a ruby-* makedepends entry, or install local .gem files (--local) or bundler against a committed Gemfile.lock",
		valueFlags: flagSet("-v", "--version", "--platform", "--install-dir", "-i", "--bindir", "-n", "--source", "-s", "--build-root", "-B", "--config-file", "--trust-policy", "-P", "--file", "--without", "--http-proxy", "--bulk-threshold", "--document"),
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			if sub != "install" || s.has("--local", "-l") {
				return nil
			}
			return pinAll(namedSpecs(pos, ":", exactVersion), s.value("-v", "--version"), exactVersion)
		},
	},
	"cargo": {
		registry: "crates.io", runs: "runs {its} build scripts",
		advice:     "build the crate from a checksummed source= tarball, or declare the tool in makedepends",
		valueFlags: cargoValueFlags,
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			if sub != "install" || s.has("--path") {
				return nil
			}
			if git := s.value("--git"); len(git) > 0 {
				pinned := false
				for _, rev := range s.value("--rev") {
					pinned = hasVarRef(rev) || commitHash.MatchString("@"+rev)
				}
				var out []registrySpec
				for _, g := range git {
					out = append(out, registrySpec{Spec: g, Ref: true, Pinned: pinned})
				}
				return out
			}
			return pinAll(namedSpecs(pos, "@", semverExact), s.value("--version", "--vers"), semverExact)
		},
	},
	"composer": {
		registry: "Packagist", runs: "runs {its} plugins and scripts",
		advice:     "install from the project's composer.lock (`composer install --no-scripts`) or declare the library in makedepends",
		valueFlags: flagSet("--working-dir", "-d", "--prefer-install", "--with", "--audit-format"),
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			if sub == "global" && len(pos) > 0 {
				sub, pos = pos[0], pos[1:]
			}
			if sub != "require" {
				return nil
			}
			var out []registrySpec
			for _, w := range pos {
				if spec, ok := composerSpec(w); ok {
					out = append(out, spec)
				}
			}
			return out
		},
	},
	"cpanm": {
		registry: "CPAN", runs: "runs {its} Makefile.PL", advice: "declare the module as a perl-* makedepends entry",
		valueFlags: cpanValueFlags,
		specs:      func(s scannedArgs) []registrySpec { return namedSpecs(s.pos, "@", exactVersion) },
	},
	"cpan": {
		registry: "CPAN", runs: "runs {its} Makefile.PL", advice: "declare the module as a perl-* makedepends entry",
		valueFlags: cpanValueFlags,
		specs:      func(s scannedArgs) []registrySpec { return namedSpecs(s.pos, "", exactVersion) },
	},
	"luarocks": {
		registry: "LuaRocks", runs: "runs {its} build", advice: "declare the rock as a lua-* makedepends entry",
		valueFlags: flagSet("--server", "--only-server", "--tree", "--lua-version", "--lua-dir", "--deps-mode", "--from", "--only-from"),
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			if (sub != "install" && sub != "build") || len(pos) == 0 {
				return nil
			}
			// Restricted to a server that is a directory, luarocks fetches
			// nothing; the rocks are already in $srcdir.
			for _, server := range s.value("--only-server", "--only-from") {
				if !strings.Contains(server, "://") {
					return nil
				}
			}
			specs := namedSpecs(pos[:1], "", exactVersion)
			if len(pos) > 1 {
				specs = pinAll(specs, pos[1:2], exactVersion)
			}
			return specs
		},
	},
	"cabal": {
		registry: "Hackage", runs: "runs {its} Setup script", advice: "declare the package as a haskell-* makedepends entry",
		valueFlags: flagSet("--installdir", "--install-method", "--package-env", "--builddir", "--with-compiler", "-w", "--constraint", "--project-file", "--index-state", "--prefix", "--store-dir", "--overwrite-policy", "--jobs", "-j", "--flags", "-f"),
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			sub = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(sub, "v2-"), "v1-"), "new-")
			if sub != "install" {
				return nil
			}
			var out []registrySpec
			for _, w := range pos {
				if spec, ok := cabalSpec(w); ok {
					out = append(out, spec)
				}
			}
			return out
		},
	},
	"dotnet": {
		registry: "NuGet", runs: "restores {it} into the build",
		advice:     "restore from a committed packages.lock.json (`dotnet restore --locked-mode`) or declare the tool in makedepends",
		valueFlags: flagSet("--tool-path", "--version", "-v", "--configfile", "--add-source", "--source", "-s", "--framework", "-f", "--verbosity", "--tool-manifest", "--arch", "-a", "--package-directory", "-n", "--project"),
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			switch sub {
			case "tool":
				if len(pos) > 1 && (pos[0] == "install" || pos[0] == "update") {
					return pinAll(namedSpecs(pos[1:], "", exactVersion), s.value("--version", "-v"), exactVersion)
				}
			case "add":
				for i, w := range pos {
					if w == "package" && i+1 < len(pos) {
						return pinAll(namedSpecs(pos[i+1:], "", exactVersion), s.value("--version", "-v"), exactVersion)
					}
				}
			}
			return nil
		},
	},
	"nuget": {
		registry: "NuGet", runs: "restores {it} into the build",
		advice:     "restore from a committed packages.lock.json or declare the package in makedepends",
		valueFlags: flagSet("-Version", "-Source", "-OutputDirectory", "-Framework", "-ConfigFile", "-FallbackSource", "-PackageSaveMode", "-Verbosity", "-DependencyVersion", "-SolutionDirectory"),
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			if sub != "install" && sub != "update" {
				return nil
			}
			return pinAll(namedSpecs(pos, "", exactVersion), s.value("-Version"), exactVersion)
		},
	},
	"deno": {
		registry: "its registry", runs: "runs {it}",
		advice:     "vendor the module or pin an exact version and lock it with deno.lock",
		valueFlags: flagSet("--config", "-c", "--import-map", "--lock", "--cert", "--v8-flags", "--location", "--seed", "--ext", "--name", "-n", "--root", "--entrypoint", "--env-file"),
		specs: func(s scannedArgs) []registrySpec {
			sub, pos := s.sub()
			switch sub {
			case "install", "add", "cache":
				return denoSpecs(pos)
			case "run":
				if len(pos) > 0 {
					return denoSpecs(pos[:1])
				}
			}
			return nil
		},
	},
	"opam": {
		registry: "the opam repository", runs: "runs {its} build", advice: "declare the package as an ocaml-* makedepends entry",
		valueFlags: flagSet("--switch", "--root", "-j", "--jobs", "--criteria", "--solver", "--destdir"),
		specs: func(s scannedArgs) []registrySpec {
			if sub, pos := s.sub(); sub == "install" {
				return namedSpecs(pos, ".", exactVersion)
			}
			return nil
		},
	},
	"nimble": {
		registry: "the Nimble directory", runs: "runs {its} build", advice: "declare the package as a nim-* makedepends entry",
		valueFlags: flagSet("--nimbleDir", "--nim", "--parser", "-d"),
		specs: func(s scannedArgs) []registrySpec {
			if sub, pos := s.sub(); sub == "install" {
				return namedSpecs(pos, "@", nimbleExact)
			}
			return nil
		},
	},
}

const pipAdvice = "declare the module as a python-* makedepends entry"

var pipValueFlags = flagSet(
	"-r", "--requirement", "-c", "--constraint", "-e", "--editable", "-i", "--index-url",
	"--extra-index-url", "-f", "--find-links", "-t", "--target", "--prefix", "--root",
	"--src", "-b", "--build", "--platform", "--python-version", "--implementation",
	"--abi", "--progress-bar", "--proxy", "--retries", "--timeout", "--exists-action",
	"--trusted-host", "--cert", "--client-cert", "--cache-dir", "--log", "--config-settings",
	"-C", "--report", "--python", "--use-feature", "--use-deprecated", "--global-option",
	"--install-option", "--root-user-action", "--upgrade-strategy", "--no-binary", "--only-binary",
)

var uvValueFlags = flagSet(
	"--python", "-p", "--index", "--index-url", "-i", "--extra-index-url", "--find-links", "-f",
	"--from", "--with", "--with-requirements", "--with-editable", "--constraints", "-c",
	"-r", "--requirement", "--requirements", "--constraint", "-e", "--editable", "--override",
	"--overrides", "--build-constraints", "--build-constraint", "-b", "--cache-dir", "--project", "--directory",
	"--only-binary", "--no-binary", "--reinstall-package", "--refresh-package", "--upgrade-package", "-P",
	"--python-version", "--python-platform", "--with-executables-from",
	"--config-file", "--python-preference", "--link-mode", "--resolution", "--exclude-newer",
	"--color", "--index-strategy", "--keyring-provider", "--group", "--only-group", "--extra",
	"--optional", "--package", "--script", "--rev", "--tag", "--branch", "--env-file",
	"--exclude-package", "--prerelease", "--fork-strategy", "--target", "--prefix",
	"--config-setting", "-C", "--no-build-package", "--no-binary-package", "--bounds",
	"--default-index", "--isolated",
)

var cpanValueFlags = flagSet(
	"-l", "--local-lib", "-L", "--local-lib-contained", "--mirror", "--from", "-M",
	"--cpanfile", "--with-feature", "--without-feature", "--format", "--save-dists",
	"--configure-timeout", "--build-timeout", "--test-timeout", "--auto-cleanup", "-j", "-I", "-O",
)

// uvxSpecs reads `uvx [--from SPEC] COMMAND`: the command is the package
// unless --from names one.
func uvxSpecs(s scannedArgs, pos []string) []registrySpec {
	if from := s.value("--from"); len(from) > 0 {
		return pipSpecs(from)
	}
	if len(pos) > 0 {
		return pipSpecs(pos[:1])
	}
	return nil
}

var composerRange = regexp.MustCompile(`[\^~*<>|, ]|^dev-|-dev$|@`)

// composerSpec classifies `vendor/package[:constraint]`; `=` also separates
// the constraint. Only a constraint that is one version is exact.
func composerSpec(w string) (registrySpec, bool) {
	if hasVarRef(w) {
		return unreadSpec(w)
	}
	if isLocalSpec(w) || isRefSpec(w) {
		return registrySpec{}, false
	}
	name, constraint := w, ""
	if i := strings.IndexAny(w, ":="); i > 0 {
		name, constraint = w[:i], w[i+1:]
	}
	if !strings.Contains(name, "/") {
		return registrySpec{}, false
	}
	pinned := constraint != "" && !composerRange.MatchString(constraint) && exactVersion.MatchString(constraint)
	return registrySpec{Spec: w, Pinned: pinned}, true
}

var cabalVersionSuffix = regexp.MustCompile(`-([0-9]+(?:\.[0-9]+)*)$`)

// cabalSpec classifies `pkg`, `pkg-1.2.3` or `pkg ==1.2.3` (one word, quoted).
func cabalSpec(w string) (registrySpec, bool) {
	if hasVarRef(w) {
		return unreadSpec(w)
	}
	if isLocalSpec(w) || isRefSpec(w) {
		return registrySpec{}, false
	}
	name, rest, hasOp := strings.Cut(w, " ")
	if hasOp {
		v, ok := strings.CutPrefix(strings.TrimSpace(rest), "==")
		if !ok {
			return registrySpec{Spec: w}, true
		}
		return registrySpec{Spec: w, Pinned: exactVersion.MatchString(strings.TrimSpace(v))}, true
	}
	if strings.ContainsAny(name, "=<>^") {
		return registrySpec{}, false
	}
	return registrySpec{Spec: w, Pinned: cabalVersionSuffix.MatchString(name)}, true
}

// denoSpecs classifies deno module specifiers: `npm:`/`jsr:` registry
// packages and remote URLs. A bare name or path is a local file or import-map
// entry, not a fetch.
func denoSpecs(pos []string) []registrySpec {
	var out []registrySpec
	for _, w := range pos {
		switch {
		case hasVarRef(w):
		case strings.HasPrefix(w, "npm:"), strings.HasPrefix(w, "jsr:"):
			if spec, ok := npmSpec(strings.TrimPrefix(w, "jsr:")); ok {
				spec.Spec = w
				out = append(out, spec)
			}
		case strings.Contains(w, "://"):
			spec := refSpec(w)
			// deno.land/x and esm.sh URLs carry the version in the path;
			// one spelled out in full is immutable there.
			if m := denoURLVersion.FindStringSubmatch(w); m != nil && semverExact.MatchString(m[1]) {
				spec.Pinned = true
			}
			out = append(out, spec)
		}
	}
	return out
}

var (
	denoURLVersion = regexp.MustCompile(`@(v?[0-9]+\.[0-9]+\.[0-9]+[^/]*)/`)
	nimbleExact    = regexp.MustCompile(`^(?:v?[0-9]+\.[0-9]+\.[0-9]+|#[0-9a-f]{40})$`)
)

// manifestInstallIn reports whether the unit installs a project's own
// dependency manifest somewhere — `npm ci`, `npm install` with no package,
// `yarn`, `pnpm install`, `bun install` — which is what lets an npx-style
// runner find the tool locally instead of fetching it.
func manifestInstallIn(u *pkgbuild.Unit) bool {
	if u == nil || u.Scriptlet || u.File == nil {
		return false
	}
	found := false
	syntax.Walk(u.File, func(n syntax.Node) bool {
		call, ok := n.(*syntax.CallExpr)
		if !ok || found || len(call.Args) == 0 {
			return !found
		}
		name, _ := pkgbuild.RenderWord(call.Args[0], nil)
		name = basename(name)
		var sub string
		var pkgs int
		skip := false
		for _, w := range call.Args[1:] {
			s, dyn := pkgbuild.RenderWord(w, nil)
			if skip {
				skip = false
				continue
			}
			if strings.HasPrefix(s, "-") {
				skip = nodeValueFlags[s]
				continue
			}
			if sub == "" {
				sub = s
				continue
			}
			if !dyn && !hasVarRef(s) && !isLocalSpec(s) {
				pkgs++
			}
		}
		switch name {
		case "npm":
			found = sub == "ci" || (npmInstallSubs[sub] && pkgs == 0)
		case "yarn":
			found = sub == "" || sub == "install"
		case "pnpm", "bun":
			found = (sub == "install" || sub == "i") && pkgs == 0
		}
		return !found
	})
	return found
}

// pythonModule unwraps `python -m pip …` (or pipx, poetry) into the command
// it stands for, so the installer tables see it under its own name.
func pythonModule(c Command) (Command, bool) {
	switch c.Name {
	case "python", "python3", "python2":
	default:
		return c, false
	}
	if len(c.Args) < 2 || c.Args[0] != "-m" {
		return c, false
	}
	switch c.Args[1] {
	case "pip", "pipx", "poetry":
	default:
		return c, false
	}
	m := c
	m.Name = c.Args[1]
	m.Args = c.Args[2:]
	if len(c.ArgDyn) >= 2 {
		m.ArgDyn = c.ArgDyn[2:]
	}
	if len(c.ArgWord) >= 2 {
		m.ArgWord = c.ArgWord[2:]
	}
	return m, true
}

// adhocInvocation is one command PB210 reports: the installer, what it was
// asked to fetch, and the scan the specs were read from.
type adhocInvocation struct {
	inst  adhocInstaller
	specs []registrySpec
	args  scannedArgs
	name  string
}

// adhocInstall returns the registry packages a command fetches by name and
// the installer that describes it, or ok=false when PB210 has nothing to say
// about the command.
func adhocInstall(c Command) (adhocInvocation, bool) {
	if m, ok := pythonModule(c); ok {
		c = m
	}
	inst, ok := adhocInstallers[c.Name]
	if !ok {
		return adhocInvocation{}, false
	}
	args := scanArgs(c, inst.valueFlags)
	specs := inst.specs(args)
	if len(specs) == 0 {
		return adhocInvocation{}, false
	}
	return adhocInvocation{inst: inst, specs: specs, args: args, name: c.Name}, true
}

// invocation names the command as the reader wrote it — `npm install`,
// `uv pip install`, `dotnet tool install`, `npx` — without the packages.
func (a adhocInvocation) invocation() string {
	switch a.name {
	case "npx", "pnpx", "bunx", "uvx", "cpan", "cpanm":
		return a.name
	}
	words := []string{a.name}
	for _, w := range a.args.pos {
		if len(words) == 1 || adhocSubwords[w] {
			words = append(words, w)
			if len(words) == 3 {
				break
			}
			continue
		}
		break
	}
	return strings.Join(words, " ")
}

// adhocSubwords are the second-level subcommands worth naming.
var adhocSubwords = map[string]bool{
	"install": true, "add": true, "run": true, "require": true, "package": true,
	"uvx": true, "upgrade": true, "update": true, "x": true, "i": true,
}

// namesPackages reports whether an npm-family install names anything at all
// after its subcommand — a registry package, a tarball, a path, or a word
// pkglint cannot read. Such a command installs no manifest, so a rule whose
// remedy is "install the lockfile instead" (`npm ci`, --frozen-lockfile) has
// nothing to say to it.
func namesPackages(c Command) bool {
	s := scanArgs(c, nodeValueFlags)
	_, rest := s.sub()
	return len(rest) > 0
}

// adhocClaims reports whether PB210 reports the command, for the lockfile
// rules that would otherwise give a manifest-shaped remedy (`npm ci`,
// --require-hashes) to a command that installs no manifest.
func adhocClaims(c Command) bool {
	_, ok := adhocInstall(c)
	return ok
}

func checkAdhocInstall(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		a, ok := adhocInstall(c)
		if !ok {
			continue
		}
		sev := Warn
		var names, unread []string
		refs, unpinned := 0, 0
		for _, s := range a.specs {
			if s.Unread {
				// As written, quotes and all: the reader is being pointed at
				// a word in the file, not told a package name.
				unread = append(unread, a.args.shownAs(s.Spec))
				continue
			}
			names = append(names, fmt.Sprintf("%q", s.Spec))
			if !s.Pinned {
				sev = Error
				unpinned++
			}
			if s.Ref {
				refs++
			}
		}
		plural, possessive := "it", "its"
		if len(names) > 1 {
			plural, possessive = "them", "their"
		}
		when := "build"
		if c.Unit.Scriptlet {
			when = "install"
		}
		var how string
		switch {
		case len(names) == 0:
			how = "at the version " + a.inst.registry + " serves at " + when + " time"
		case unpinned > 0 && refs == len(names):
			how = "from a ref nothing pins"
		case unpinned > 0:
			how = "at whatever version " + a.inst.registry + " serves at " + when + " time"
		default:
			how = "from " + a.inst.registry + " with nothing in the PKGBUILD verifying what arrives"
		}
		verb := ""
		if a.inst.runs != "" && !a.args.has("--ignore-scripts", "--no-scripts") {
			r := strings.NewReplacer("{it}", plural, "{its}", possessive)
			verb = " and " + r.Replace(a.inst.runs)
			if c.Unit.Scriptlet {
				verb += " as root"
			}
		}
		advice := a.inst.advice
		if c.Unit.Scriptlet {
			// The remedies above are for a build; a scriptlet has no manifest
			// and no makedepends, and no business fetching anything.
			advice = "files a package needs belong in package(), fetched by makepkg from a checksummed source= entry, not installed by a scriptlet"
		}
		var msg string
		switch {
		case len(names) == 0:
			// Every package word is one pkglint could not resolve. It is
			// still an install by name — an unreadable name, which is the
			// reason for Warn rather than Error: nothing says the version
			// is not pinned, and nothing says it is.
			msg = fmt.Sprintf("%s fetches %s, which pkglint cannot read; whatever registry package it names arrives %s%s; %s",
				a.invocation(), strings.Join(unread, ", "), how, verb, advice)
		case len(unread) > 0:
			isAre := "is a word"
			if len(unread) > 1 {
				isAre = "are words"
			}
			msg = fmt.Sprintf("%s fetches %s %s%s; %s %s pkglint cannot read; %s",
				a.invocation(), strings.Join(names, ", "), how, verb, strings.Join(unread, ", "), isAre, advice)
		default:
			msg = fmt.Sprintf("%s fetches %s %s%s; %s",
				a.invocation(), strings.Join(names, ", "), how, verb, advice)
		}
		out = append(out, c.finding("PB210", sev, "%s", msg))
	}
	return out
}
