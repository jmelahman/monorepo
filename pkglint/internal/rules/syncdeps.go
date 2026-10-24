package rules

import (
	"fmt"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// syncDepFields are the arrays makepkg's -s hands to pacman, in the order it
// checks them: depends first, then makedepends and checkdepends together.
var syncDepFields = []string{"depends", "makedepends", "checkdepends"}

// SyncDeps returns the dependency names makepkg -s would ask pacman for when
// it builds pkg on carch — what `pkglint build` installs in a container on
// makepkg's behalf, because makepkg itself cannot get root there. It reads the
// parse, never the file run: the values are the ones every rule here sees.
//
// The fields are makepkg's own: the top-level depends, makedepends and
// checkdepends, each with its _$carch variant merged in, as merge_arch_attrs
// does whatever arch= says. A split package's depends inside package_<name>()
// are deliberately not read. makepkg -s does not install them either — they
// are metadata for the archive, read only after the build — and they routinely
// name the PKGBUILD's own sibling packages, which no repository has yet.
//
// Version constraints are cut ("foo>=1.2" -> "foo", "libfoo.so=1-64" ->
// "libfoo.so"); duplicates are dropped and first-written order kept. Names are
// returned as written and not validated: the caller is the one handing them to
// a process, so the check that matters is made there.
//
// Guessing is the one thing this must not do. A list missing an entry fails
// the build with "missing dependencies" half an hour into a compile at best,
// and an entry rendered wrongly installs something nobody asked for. So any
// field whose content the parse cannot know — assigned by top-level control
// flow, holding an array reference it cannot expand, or an element built from
// a substitution or an unknown variable — is an error naming it, not a skip.
func SyncDeps(pkg *pkgbuild.Package, carch string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, field := range syncDepFields {
		for _, name := range []string{field, field + "_" + carch} {
			if pkg.ConditionalVars[name] {
				return nil, fmt.Errorf("%s is assigned under top-level control flow (an if, case or loop), so which entries makepkg sees cannot be read without running the PKGBUILD", name)
			}
			v := pkg.Vars[name]
			if v == nil {
				continue
			}
			if v.CountUnknown {
				return nil, fmt.Errorf("%s holds an array reference or word-splitting expansion that cannot be resolved without running the PKGBUILD", name)
			}
			for _, e := range varElems(v) {
				val, ok := staticVal(pkg, e.Value)
				if !ok {
					return nil, fmt.Errorf("%s entry %q is not statically known (a substitution or an unset variable), so it cannot be resolved without running the PKGBUILD", name, printable(e.Value))
				}
				// Unquoted, bash would split this into several names; quoted,
				// it is one name no package has. Either way it is not a list
				// to hand to pacman as it stands.
				if strings.ContainsAny(val, " \t\n") {
					return nil, fmt.Errorf("%s entry %q contains whitespace, so it is not one package name", name, printable(val))
				}
				dep := val
				if i := strings.IndexAny(dep, "<>="); i >= 0 {
					dep = dep[:i]
				}
				if !seen[dep] {
					seen[dep] = true
					out = append(out, dep)
				}
			}
		}
	}
	return out, nil
}

// printable renders the NUL marker RenderWord leaves for a construct it could
// not render as something a terminal shows.
func printable(s string) string {
	return strings.ReplaceAll(s, "\x00", "…")
}
