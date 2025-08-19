package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charlievieth/fastwalk"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
)

var (
	includeHidden bool
	noIgnore      bool
	quiet         bool
	debug         bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "check-symlinks [paths...]",
		Short:   "Check for broken symbolic links",
		Run:     runCheckSymlinks,
		Version: fmt.Sprintf("%s\ncommit %s", version, commit),
	}

	rootCmd.Flags().BoolVar(&includeHidden, "hidden", false, "include hidden files and directories in the check")
	rootCmd.Flags().BoolVar(&noIgnore, "no-ignore", false, "don't use ignore files")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "run in debug mode")
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

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	topLevel, err := getTopLevel(".")
	if err != nil {
		log.Errorf("Failed to find toplevel directory: %v", err)
	}

	// Load ignore patterns
	var ignorePatterns []string
	if !noIgnore {
		ignorePatterns = loadIgnorePatterns(topLevel)
	}

	var wg sync.WaitGroup
	var filesChecked int64
	rc := 0
	paths := make(chan string, 100)
	done := make(chan struct{})

	// Worker pool
	for range int64(runtime.NumCPU() - 1) {
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
					if shouldIgnorePath(topLevel, path, ignorePatterns) {
						log.Debug("Skipping: ", path)
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
func loadIgnorePatterns(rootDir string) []string {
	var patterns []string

	// Check for .symlinkignore in current directory
	configPath := filepath.Join(rootDir, ".symlinkignore")
	if patterns = loadPatternsFromFile(configPath); len(patterns) > 0 {
		log.Debug("Found ignore file: ", configPath)
		return patterns
	}

	// Check for .config/symlinkignore
	configPath = filepath.Join(rootDir, ".config", "symlinkignore")
	if patterns = loadPatternsFromFile(configPath); len(patterns) > 0 {
		log.Debug("Found ignore file: ", configPath)
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
func shouldIgnorePath(topLevel, path string, patterns []string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	rel, err := filepath.Rel(topLevel, absPath)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)

	for _, p := range patterns {
		p = strings.TrimSuffix(filepath.ToSlash(p), "/")
		// Match prefix
		if strings.HasPrefix(rel, p) {
			return true
		}
		// Match directory
		if strings.HasPrefix(rel, p+"/") {
			return true
		}
		// Match filename
		if base == p {
			return true
		}
	}
	return false
}

func getTopLevel(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil && info.IsDir() {
			return dir, nil
		}
		// If we reach the root, stop
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository")
		}
		dir = parent
	}
}
