package rules

import (
	"path"
	"strings"
)

// PB930–PB934 lint the Arch Python package guidelines
// (https://wiki.archlinux.org/title/Python_package_guidelines): no tox, no
// lint/coverage plugins gating the test suite, no pre-built wheels as
// sources, and the python-build + python-installer flow with its tools
// declared in makedepends. The hermeticity side of Python packaging — pip
// downloads without hashes — is PB201/PB202's.

// --- PB930: tox runs the test suite ------------------------------------------

// toxPackages are the package spellings that pull in tox.
var toxPackages = map[string]bool{"tox": true, "python-tox": true}

func checkPythonTox(ctx *Context) []Finding {
	var out []Finding
	for _, d := range depEntries(ctx, "checkdepends") {
		if !toxPackages[d.Name] {
			continue
		}
		out = append(out, findingAt("PB930", Warn, ctx.Pkg.PKGBUILD.Path, d.Pos,
			"checkdepends pulls in %s; the Python package guidelines forbid tox — run pytest (or the project's test runner) against the installed package instead", d.Name))
	}
	for _, c := range ctx.CommandsNamed("tox") {
		if c.Unit.Scriptlet || c.Fn != "check" {
			continue
		}
		out = append(out, c.finding("PB930", Warn,
			"tox rebuilds isolated environments and re-resolves dependencies inside check(); the Python package guidelines forbid it — invoke the test runner directly"))
	}
	return out
}

// --- PB931: lint plugins in checkdepends -------------------------------------

// pytestLintPlugins are pytest plugins that run linters, type checkers or
// coverage rather than tests; the guidelines say a package build is not the
// place to enforce upstream's code style.
var pytestLintPlugins = map[string]bool{
	"python-pytest-cov": true, "python-pytest-black": true,
	"python-pytest-flake8": true, "python-pytest-isort": true,
	"python-pytest-mypy": true, "python-pytest-pylint": true,
	"python-pytest-ruff": true, "python-pytest-runner": true,
}

// lintCheckdepends returns the lint/coverage plugins in checkdepends: what
// PB931 reports, and what its fix deletes.
func lintCheckdepends(ctx *Context) []depEntry {
	var out []depEntry
	for _, d := range depEntries(ctx, "checkdepends") {
		if pytestLintPlugins[d.Name] {
			out = append(out, d)
		}
	}
	return out
}

func checkPythonLintCheckdepends(ctx *Context) []Finding {
	var out []Finding
	for _, d := range lintCheckdepends(ctx) {
		out = append(out, findingAt("PB931", Info, ctx.Pkg.PKGBUILD.Path, d.Pos,
			"checkdepends names %s, a lint/coverage plugin; check() verifies the package works, not that upstream's style rules pass", d.Name))
	}
	return out
}

// --- PB932: pre-built wheel as a source --------------------------------------

func checkPythonWheelSource(ctx *Context) []Finding {
	var out []Finding
	for _, e := range ctx.Pkg.Sources() {
		name := e.Filename
		if name == "" {
			name = path.Base(e.URL)
		}
		if i := strings.IndexAny(name, "#?"); i >= 0 {
			name = name[:i]
		}
		if !strings.HasSuffix(strings.ToLower(name), ".whl") {
			continue
		}
		out = append(out, findingAt("PB932", Warn, ctx.Pkg.PKGBUILD.Path, e.Pos,
			"source %q is a pre-built wheel; the Python package guidelines require building from the sdist so the code compiled is the code reviewed", name))
	}
	return out
}

// --- PB933/PB934: the build/installer flow -----------------------------------

// pythonModuleArg returns the module a `python -m <module>` invocation runs,
// or "".
func pythonModuleArg(c Command) string {
	for i, a := range c.Args {
		if a == "-m" && i+1 < len(c.Args) {
			return c.Args[i+1]
		}
	}
	return ""
}

// buildBackendPackages maps a `python -m` module to the Arch package shipping
// it.
var buildBackendPackages = map[string]string{
	"build":     "python-build",
	"installer": "python-installer",
}

// pythonBackendGap is one build-backend package the build runs through
// `python -m` and no dependency array declares.
type pythonBackendGap struct {
	Cmd    Command
	Module string
	Pkg    string
}

// pythonBackendGaps returns those packages in source order, once each: the
// entries PB933 reports, and the ones its fix writes into makedepends.
func pythonBackendGaps(ctx *Context) []pythonBackendGap {
	var out []pythonBackendGap
	reported := map[string]bool{}
	for _, c := range ctx.CommandsNamed("python", "python3") {
		if c.Unit.Scriptlet || c.Fn == "" {
			continue
		}
		module := pythonModuleArg(c)
		pkg, ok := buildBackendPackages[module]
		if !ok || reported[pkg] {
			continue
		}
		if hasDep(ctx, "makedepends", pkg) || hasDep(ctx, "depends", pkg) {
			continue
		}
		reported[pkg] = true
		out = append(out, pythonBackendGap{Cmd: c, Module: module, Pkg: pkg})
	}
	return out
}

func checkPythonBuildBackend(ctx *Context) []Finding {
	var out []Finding
	for _, g := range pythonBackendGaps(ctx) {
		out = append(out, g.Cmd.finding("PB933", Warn,
			"python -m %s needs %q in makedepends; a clean build environment does not have it", g.Module, g.Pkg))
	}
	return out
}

// setupPyVerbs are the setup.py subcommands the deprecated distutils flow
// runs; their python-build/python-installer replacements come from the
// guidelines' standard build() and package() bodies.
var setupPyVerbs = map[string]bool{
	"build": true, "install": true, "test": true, "develop": true,
	"sdist": true, "bdist": true, "bdist_wheel": true, "bdist_egg": true,
}

func checkPythonSetupPy(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.CommandsNamed("python", "python3", "python2") {
		if c.Unit.Scriptlet || c.Fn == "" {
			continue
		}
		sawSetupPy := false
		for _, a := range c.Args {
			if a == "setup.py" || strings.HasSuffix(a, "/setup.py") {
				sawSetupPy = true
				continue
			}
			if sawSetupPy && setupPyVerbs[a] {
				out = append(out, c.finding("PB934", Warn,
					"python setup.py %s uses the removed distutils flow; the Python package guidelines build with `python -m build` and install with `python -m installer`", a))
				break
			}
		}
	}
	return out
}
