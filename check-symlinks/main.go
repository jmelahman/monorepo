package main

import (
	"bufio"
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
	version = "dev"
	commit  = "none"
)

var (
	includeHidden bool
	quiet         bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "check-symlinks [paths...]",
		Short:   "Check for broken symbolic links in a directory tree",
		Run:     runCheckSymlinks,
		Version: fmt.Sprintf("%s\ncommit %s", version, commit),
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

	// Load ignore patterns
	ignorePatterns := loadIgnorePatterns()

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

					// Check if path should be ignored
					if shouldIgnorePath(path, ignorePatterns) {
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

// loadIgnorePatterns loads ignore patterns from .symlinkignore and .config/symlinkignore files
func loadIgnorePatterns() []string {
	var patterns []string
	
	// Check for .symlinkignore in current directory
	if patterns = loadPatternsFromFile(".symlinkignore"); len(patterns) > 0 {
		return patterns
	}
	
	// Check for .config/symlinkignore
	configPath := filepath.Join(".config", "symlinkignore")
	if patterns = loadPatternsFromFile(configPath); len(patterns) > 0 {
		return patterns
	}
	
	return patterns
}

// loadPatternsFromFile reads patterns from a file
func loadPatternsFromFile(filename string) []string {
	var patterns []string
	
	file, err := os.Open(filename)
	if err != nil {
		return patterns
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	
	return patterns
}

// shouldIgnorePath checks if a path matches any ignore pattern
func shouldIgnorePath(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			// If pattern is invalid, skip it
			continue
		}
		if matched {
			return true
		}
		
		// Also check if pattern matches the full path
		matched, err = filepath.Match(pattern, path)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}
