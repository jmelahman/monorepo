package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charlievieth/fastwalk"
	"github.com/spf13/cobra"
)

var (
	includeHidden bool
	quiet         bool
	rootPath      string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "check-symlinks [paths...]",
		Short: "Check for broken symbolic links in a directory tree",
		Run:   runCheckSymlinks,
	}

	rootCmd.Flags().BoolVar(&includeHidden, "hidden", false, "include hidden files and directories in the check")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "run in quiet mode")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
}

func runCheckSymlinks(cmd *cobra.Command, args []string) {
	// Default to current directory if no args provided
	if len(args) == 0 {
		args = []string{"."}
	}

	var wg sync.WaitGroup
	var filesChecked int64
	rc := 0
	paths := make(chan string, 100)
	done := make(chan struct{})

	// Worker pool
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range paths {
				atomic.AddInt64(&filesChecked, 1)
				fi, err := os.Lstat(path)
				if err != nil || fi.Mode()&os.ModeSymlink == 0 {
					continue
				}
				_, err = os.Stat(path)
				if os.IsNotExist(err) {
					if !quiet {
						fmt.Println("Broken symlink:", path)
					}
					rc = 1
				}
			}
		}()
	}

	// Process each path
	go func() {
		defer close(paths)
		for _, rootPath := range args {
			if info, err := os.Stat(rootPath); os.IsNotExist(err) || !info.IsDir() {
				paths <- rootPath
			} else {
				err := fastwalk.Walk(nil, rootPath, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}

					// Skip hidden files and directories unless --hidden flag is set
					if !includeHidden && isHidden(path) {
						if d.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}

					paths <- path
					return nil
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error walking directory %s: %v\n", rootPath, err)
					rc = 127
				}
			}
		}
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	<-done
	if !quiet {
		fmt.Printf("Total files checked: %d\n", atomic.LoadInt64(&filesChecked))
	}
	os.Exit(rc)
}

// isHidden checks if a file or directory is hidden (starts with '.')
// It checks only the base name, not the full path
func isHidden(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".") && base != "." && base != ".."
}
