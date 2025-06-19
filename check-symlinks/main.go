package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
	"github.com/spf13/cobra"
)

var (
	includeHidden bool
	rootPath      string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "check-symlinks [paths...]",
		Short: "Check for broken symbolic links in a directory tree",
		Run:   runCheckSymlinks,
	}

	rootCmd.Flags().BoolVar(&includeHidden, "hidden", false, "include hidden files and directories in the check")

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
	rc := 0
	paths := make(chan string, 100)
	done := make(chan struct{})

	// Worker pool
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range paths {
				fi, err := os.Lstat(path)
				if err != nil || fi.Mode()&os.ModeSymlink == 0 {
					continue
				}
				_, err = os.Stat(path)
				if os.IsNotExist(err) {
					fmt.Println("Broken symlink:", path)
					rc = 1
				}
			}
		}()
	}

	// Process each path
	go func() {
		defer close(paths)
		for _, rootPath := range args {
			err := fastwalk.Walk(rootPath, func(path string, d os.DirEntry, err error) error {
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
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	<-done
	os.Exit(rc)
}

// isHidden checks if a file or directory is hidden (starts with '.')
// It checks only the base name, not the full path
func isHidden(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".") && base != "." && base != ".."
}
