// Package cli implements check-symlinks: a recursive check for broken
// symbolic links.
//
// The package is written in the Solod subset of Go, so it builds with both
// toolchains. `so build` translates it to C for the release binaries; a plain
// `go build` resolves solod.dev to the gocompat/ shims (see the root go.mod)
// and produces an equivalent binary with no C compiler in sight.
package cli

import (
	"solod.dev/so/conc"
	"solod.dev/so/flag"
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
	"solod.dev/so/runtime"
	"solod.dev/so/slices"
	"solod.dev/so/strings"
)

// Version is the release version. Solod has no ldflags-style stamping, so
// this constant is the single source of truth; scripts/check-version.sh
// guards it against the pushed tag.
const Version = "0.6.0"

// defaultRoots is used when no paths are given on the command line.
var defaultRoots = [1]string{"."}

// Run checks the given paths and returns the process exit code:
// 0 if every symlink resolves, 1 if any is broken, 2 on a usage error.
func Run(args []string) int {
	var st state
	st.mu.Init()
	st.cond.Init(&st.mu)
	defer st.mu.Free()
	defer st.cond.Free()

	var noIgnore bool
	var showVersion bool
	var threads int
	fs := flag.NewFlagSet("check-symlinks", flag.ContinueOnError)
	fs.BoolVar(&st.includeHidden, "hidden", false, "include hidden files and directories in the check")
	fs.BoolVar(&noIgnore, "no-ignore", false, "don't use ignore files")
	fs.BoolVar(&st.debug, "debug", false, "run in debug mode")
	fs.BoolVar(&st.quiet, "quiet", false, "run in quiet mode")
	fs.BoolVar(&st.quiet, "q", false, "run in quiet mode")
	fs.IntVar(&threads, "threads", 0, "number of worker threads (0 = one per CPU)")
	fs.BoolVar(&showVersion, "version", false, "print the version and exit")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if showVersion {
		fmt.Printf("check-symlinks %s\n", Version)
		return 0
	}

	// cwdBuf backs both the working directory and the top-level path sliced
	// out of it, so it has to outlive the walk.
	var cwdBuf [os.MaxPathLen]byte
	cwd, err := os.Getwd(cwdBuf[:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot get working directory\n")
		return 2
	}
	st.topLevel = findTopLevel(cwd)
	if st.topLevel == "" && !st.quiet {
		fmt.Fprintf(os.Stderr, "Failed to find toplevel directory: not a git repository\n")
	}
	if !noIgnore && st.topLevel != "" {
		loadIgnorePatterns(&st)
	}
	defer freePatterns(&st)

	roots := fs.Args()
	if len(roots) == 0 {
		roots = defaultRoots[:]
	}
	// Seed the work stack. A root that is not a directory is checked
	// directly, without the hidden and ignore filters; a directory root goes
	// through the same filters as any other entry.
	for i := range roots {
		root := roots[i]
		fi, serr := os.Stat(root)
		if serr != nil || !fi.IsDir() {
			st.checked.Add(1)
			checkOne(&st, root)
			continue
		}
		if !st.includeHidden && isHidden(baseName(root)) {
			continue
		}
		if shouldIgnore(&st, root) {
			continue
		}
		st.checked.Add(1)
		st.stack = slices.Append(mem.System, st.stack, strings.Clone(mem.System, root))
	}

	if threads <= 0 {
		threads = runtime.NumCPU()
	}
	if threads == 1 {
		// Single-threaded: run the worker loop on the main thread.
		work(&st)
	} else {
		ths := make([]conc.Thread, threads)
		for i := 0; i < threads; i++ {
			ths[i] = conc.Go(workEntry, &st)
		}
		for i := 0; i < threads; i++ {
			ths[i].Wait()
		}
	}
	slices.Free(mem.System, st.stack)

	if !st.quiet {
		fmt.Printf("Total files checked: %d\n", st.checked.Load())
	}
	if st.broken.Load() {
		return 1
	}
	return 0
}
