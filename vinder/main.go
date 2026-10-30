// Command vinder generates a project's vendor directory from its npm
// dependencies, checks that the committed one is up to date, and audits what
// it contains against OSV. See README.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jmelahman/vinder/internal/osv"
	"github.com/jmelahman/vinder/internal/project"
	"github.com/jmelahman/vinder/internal/registry"
	"github.com/jmelahman/vinder/internal/version"
)

// Named after whatever the binary is actually called, so usage text and
// error messages stay right if it is renamed.
var programName = filepath.Base(os.Args[0])

var (
	// errOutOfDate is returned by check when a vendor tree differs from
	// what its lockfile generates.
	errOutOfDate = errors.New("vendor directory out of date; run vinder sync")
	// errVersionSkew is returned by check when projects vendor one package
	// at different versions.
	errVersionSkew = errors.New("projects vendor different versions of the same package")
	// errVulnerable is returned by audit when any vendored package has an
	// advisory.
	errVulnerable = errors.New("vulnerable packages found")
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, programName+":", err)
		os.Exit(1)
	}
}

// run holds everything main does apart from process plumbing, which is what
// makes it testable — see main_test.go.
func run(args []string, stdout, stderr io.Writer) error {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	// Cancelling, rather than dying on the signal, lets the commands run
	// their cleanup: upgrade restores package.json and the lockfile, and
	// sync and check remove their temporary directories.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return root.ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     programName,
		Short:   "Generate, check and audit vendored npm dependencies",
		Version: version.Version(),
		// main reports errors itself, and a failed check is not a usage
		// mistake.
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newAddCmd(), newSyncCmd(), newCheckCmd(), newAuditCmd(), newUpgradeCmd(), newGitHubAppCmd())
	return root
}

func newAddCmd() *cobra.Command {
	var dir, registryURL string
	cmd := &cobra.Command{
		Use:   "add <package[@version]>...",
		Short: "Add packages to package.json and vinder.toml, creating them if needed, and vendor them",
		Long: `Add depends on each package, gives it a vinder.toml entry that bundles it as
an ES module, and syncs. A project with neither package.json nor vinder.toml
gets both, on the first package manager on PATH. To vendor anything other
than the package's main entry, edit its entry in vinder.toml and run sync.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := &registry.Client{URL: registryURL}
			_, additions, err := project.Add(cmd.Context(), dir, args, reg.Resolve)
			if err != nil {
				return fmt.Errorf("%s: %w", display(dir), err)
			}
			w := cmd.OutOrStdout()
			for _, a := range additions {
				fmt.Fprintf(w, "added %s %s\n", a.Name, a.Version)
				for _, f := range a.Files {
					fmt.Fprintf(w, "  %s\n", f)
				}
			}
			fmt.Fprintf(w, "\nImport from those files in a <script type=\"module\">, and commit them with\npackage.json, the lockfile and %s.\n", project.ConfigFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "the project's directory")
	cmd.Flags().StringVar(&registryURL, "registry", registry.DefaultURL, "npm registry to look up versions in")
	return cmd
}

func newSyncCmd() *cobra.Command {
	var all, align bool
	cmd := &cobra.Command{
		Use:   "sync [dir...]",
		Short: "Install the dependencies and regenerate the vendor directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := loadProjects(all, args)
			if err != nil {
				return err
			}
			if align {
				alignments, err := project.Align(cmd.Context(), projects)
				if err != nil {
					return err
				}
				printAlignments(cmd.OutOrStdout(), alignments)
				return nil
			}
			for _, p := range projects {
				if err := p.Sync(cmd.Context()); err != nil {
					return fmt.Errorf("%s: %w", display(p.Dir), err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "synced %s\n", display(p.OutDir()))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "every project in the git repository")
	cmd.Flags().BoolVar(&align, "align", false, "move projects onto one version of each package they share")
	return cmd
}

func newUpgradeCmd() *cobra.Command {
	var (
		all         bool
		registryURL string
	)
	cmd := &cobra.Command{
		Use:   "upgrade [dir...]",
		Short: "Upgrade direct dependencies to their latest releases, re-resolve the lockfiles, and align the projects",
		Long: `Upgrade raises each direct dependency with a plain spec (1.2.3, ^1.2.3,
~1.2.3) to the registry's latest release, resolves each lockfile again from
nothing so transitive dependencies move to the newest releases their ranges
allow, regenerates the vendor directories, and aligns the projects on one
version of each package they share. It prints what changed as Markdown, for
a pull request.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := loadProjects(all, args)
			if err != nil {
				return err
			}
			before := make([]*project.Manifest, len(projects))
			for i, p := range projects {
				// upgrade regenerates a manifest that doesn't parse, and
				// reports everything in it as new.
				m, err := project.ReadManifest(filepath.Join(p.OutDir(), project.ManifestFile))
				if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, project.ErrBadManifest) {
					return err
				}
				before[i] = m
			}

			reg := &registry.Client{URL: registryURL}
			bumps := make([][]project.Change, len(projects))
			for i, p := range projects {
				if bumps[i], err = p.Upgrade(cmd.Context(), reg.Latest); err != nil {
					return fmt.Errorf("%s: %w", display(p.Dir), err)
				}
			}
			alignments, err := project.Align(cmd.Context(), projects)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			changed := len(alignments) > 0
			for i, p := range projects {
				after, err := project.ReadManifest(filepath.Join(p.OutDir(), project.ManifestFile))
				if err != nil {
					return err
				}
				changes := project.Changes(before[i], after)
				if len(bumps[i]) == 0 && len(changes) == 0 {
					continue
				}
				changed = true
				fmt.Fprintf(w, "### %s\n\n", display(p.Dir))
				printChanges(w, "package.json", bumps[i])
				printChanges(w, "vendored", changes)
			}
			printAlignments(w, alignments)
			if !changed {
				// Lockfiles may still have moved, on dependencies nothing
				// vendored includes.
				fmt.Fprintln(w, "Nothing vendored changed.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "every project in the git repository")
	cmd.Flags().StringVar(&registryURL, "registry", registry.DefaultURL, "npm registry to look up latest releases in")
	return cmd
}

// printChanges lists changes as Markdown under a bold title, if there are
// any.
func printChanges(w io.Writer, title string, changes []project.Change) {
	if len(changes) == 0 {
		return
	}
	fmt.Fprintf(w, "**%s**\n\n", title)
	for _, c := range changes {
		switch {
		case c.From == "":
			fmt.Fprintf(w, "- `%s` %s (new)\n", c.Name, c.To)
		case c.To == "":
			fmt.Fprintf(w, "- `%s` %s (removed)\n", c.Name, c.From)
		default:
			fmt.Fprintf(w, "- `%s` %s → %s\n", c.Name, c.From, c.To)
		}
	}
	fmt.Fprintln(w)
}

// printAlignments lists what Align did as Markdown, if anything.
func printAlignments(w io.Writer, alignments []project.Alignment) {
	if len(alignments) == 0 {
		return
	}
	fmt.Fprint(w, "### Aligned\n\n")
	for _, a := range alignments {
		if a.Version == "" {
			fmt.Fprintf(w, "- `%s`: no one version satisfies every project; left as it was\n", a.Name)
			continue
		}
		var moved []string
		for dir, from := range a.Moved {
			moved = append(moved, fmt.Sprintf("%s from %s", display(dir), from))
		}
		sort.Strings(moved)
		fmt.Fprintf(w, "- `%s` on %s: %s\n", a.Name, a.Version, strings.Join(moved, ", "))
	}
	fmt.Fprintln(w)
}

func newCheckCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "check [dir...]",
		Short: "Fail if a vendor directory differs from what its lockfile generates, or projects vendor different versions of a package",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := loadProjects(all, args)
			if err != nil {
				return err
			}
			outOfDate := false
			for _, p := range projects {
				diffs, err := p.Check(cmd.Context())
				if err != nil {
					return fmt.Errorf("%s: %w", display(p.Dir), err)
				}
				for _, d := range diffs {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", display(p.Dir), d)
				}
				outOfDate = outOfDate || len(diffs) > 0
			}
			if outOfDate {
				return errOutOfDate
			}

			skews, err := project.FindSkew(projects)
			if err != nil {
				return err
			}
			for _, s := range skews {
				var uses []string
				for _, version := range sortedVersions(s.Versions) {
					dirs := make([]string, len(s.Versions[version]))
					for i, dir := range s.Versions[version] {
						dirs[i] = display(dir)
					}
					uses = append(uses, fmt.Sprintf("%s (%s)", version, strings.Join(dirs, ", ")))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "skew: %s: %s\n", s.Name, strings.Join(uses, "; "))
			}
			if len(skews) > 0 {
				return errVersionSkew
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "every project in the git repository")
	return cmd
}

func newAuditCmd() *cobra.Command {
	var (
		all    bool
		osvURL string
	)
	cmd := &cobra.Command{
		Use:   "audit [dir...]",
		Short: "Report OSV advisories for every package in the vendor manifests",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := loadProjects(all, args)
			if err != nil {
				return err
			}
			type use struct {
				p   *project.Project
				key string
			}
			var pkgs []osv.Package
			uses := map[osv.Package][]use{}
			for _, p := range projects {
				m, err := project.ReadManifest(filepath.Join(p.OutDir(), project.ManifestFile))
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("%s: no %s; run vinder sync", display(p.Dir), project.ManifestFile)
				}
				if err != nil {
					return err
				}
				for _, key := range p.Keys() {
					for _, inc := range m.Deps[key].Includes {
						pkg := osv.Package{Name: inc.Name, Version: inc.Version}
						if _, seen := uses[pkg]; !seen {
							pkgs = append(pkgs, pkg)
						}
						uses[pkg] = append(uses[pkg], use{p, key})
					}
				}
			}
			if len(pkgs) == 0 {
				return nil
			}

			ids, err := (&osv.Client{URL: osvURL}).QueryBatch(cmd.Context(), pkgs)
			if err != nil {
				return err
			}
			vulnerable := false
			ignored := 0
			for i, pkg := range pkgs {
				for _, u := range uses[pkg] {
					var report []string
					for _, id := range ids[i] {
						if u.p.Ignored(u.key, project.Package(pkg), id) {
							ignored++
							continue
						}
						report = append(report, id)
					}
					if len(report) == 0 {
						continue
					}
					vulnerable = true
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s@%s\t%s\n", display(u.p.Dir), u.key, pkg.Name, pkg.Version, strings.Join(report, " "))
				}
			}
			if ignored > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "%d advisories ignored as configured in %s\n", ignored, project.ConfigFile)
			}
			if vulnerable {
				return errVulnerable
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "every project in the git repository")
	cmd.Flags().StringVar(&osvURL, "osv-url", osv.DefaultURL, "OSV batch query endpoint")
	return cmd
}

// loadProjects loads the projects in dirs, the current directory when there
// are none, or with all set every project in the enclosing git repository.
func loadProjects(all bool, dirs []string) ([]*project.Project, error) {
	if all {
		if len(dirs) > 0 {
			return nil, errors.New("--all takes no directories")
		}
		out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return nil, fmt.Errorf("--all: finding the git repository: %w", err)
		}
		if dirs, err = project.Discover(strings.TrimSpace(string(out))); err != nil {
			return nil, err
		}
		if len(dirs) == 0 {
			return nil, fmt.Errorf("--all: no %s in the repository", project.ConfigFile)
		}
	}
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	projects := make([]*project.Project, 0, len(dirs))
	for _, dir := range dirs {
		p, err := project.Load(dir)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func sortedVersions(m map[string][]string) []string {
	versions := make([]string, 0, len(m))
	for v := range m {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	return versions
}

// display shortens an absolute path to one relative to the working
// directory, where that is shorter.
func display(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(wd, path); err == nil && len(rel) < len(path) {
		return rel
	}
	return path
}
