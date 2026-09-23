package cli

import (
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
	"solod.dev/so/slices"
	"solod.dev/so/strings"
	"solod.dev/so/sync"
	"solod.dev/so/sync/atomic"
)

// state is the shared walk state.
// stack and pending are guarded by mu; the atomics are not.
type state struct {
	mu   sync.Mutex
	cond sync.Cond

	stack   []string // directories left to visit; each string is owned
	pending int      // directories checked out by a worker but not yet read

	checked atomic.Int64
	broken  atomic.Bool

	patterns []string // ignore patterns; each string is owned
	topLevel string

	includeHidden bool
	quiet         bool
	debug         bool
}

// workEntry adapts work to the conc.Go thread signature.
func workEntry(arg any) any {
	st := arg.(*state)
	work(st)
	return nil
}

// work pops directories off the shared stack and reads them until the stack
// is empty and no other worker is still reading a directory.
func work(st *state) {
	var buf [os.MaxPathLen]byte
	for {
		st.mu.Lock()
		for len(st.stack) == 0 && st.pending > 0 {
			st.cond.Wait()
		}
		if len(st.stack) == 0 {
			// Nothing queued and nothing in flight: the walk is over.
			st.mu.Unlock()
			st.cond.Broadcast()
			return
		}
		last := len(st.stack) - 1
		dir := st.stack[last]
		st.stack = st.stack[:last]
		st.pending++
		st.mu.Unlock()

		readDir(st, dir, buf[:])
		mem.FreeString(mem.System, dir)

		st.mu.Lock()
		st.pending--
		st.mu.Unlock()
		st.cond.Broadcast()
	}
}

// readDir reads one directory, queues its subdirectories, and reports any
// broken symlinks it holds. buf is scratch space for building child paths.
func readDir(st *state, dir string, buf []byte) {
	entries, err := os.ReadDir(mem.System, dir)
	if err != nil && !st.quiet {
		fmt.Fprintf(os.Stderr, "Error walking directory %s\n", dir)
	}

	var subs []string
	for i := range entries {
		e := &entries[i]
		if !st.includeHidden && isHidden(e.Name) {
			continue
		}
		p := joinPath(buf, dir, e.Name)
		if shouldIgnore(st, p) {
			if st.debug {
				fmt.Printf("Skipping: %s\n", p)
			}
			continue
		}
		st.checked.Add(1)

		if e.IsDir {
			subs = slices.Append(mem.System, subs, strings.Clone(mem.System, p))
			continue
		}
		// The dirent already carries the entry type, so a symlink is
		// identified without an Lstat per entry.
		if e.Type&os.ModeSymlink == 0 {
			continue
		}
		_, serr := os.Stat(p)
		if serr == os.ErrNotExist {
			report(st, p)
		}
	}

	if len(subs) > 0 {
		st.mu.Lock()
		st.stack = slices.Extend(mem.System, st.stack, subs)
		st.mu.Unlock()
		st.cond.Broadcast()
	}
	slices.Free(mem.System, subs)
	os.FreeDirEntry(mem.System, entries)
}

// checkOne checks a single path that was named on the command line.
func checkOne(st *state, p string) {
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return
	}
	_, serr := os.Stat(p)
	if serr == os.ErrNotExist {
		report(st, p)
	}
}

// report records and prints a broken symlink.
func report(st *state, p string) {
	st.broken.Store(true)
	if st.quiet {
		return
	}
	st.mu.Lock()
	fmt.Printf("Broken symlink: %s\n", p)
	st.mu.Unlock()
}
