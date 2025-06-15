package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	var wg sync.WaitGroup
	rc := 0
	paths := make(chan string, 100)
	done := make(chan struct{})

	// Worker pool
	numWorkers := 8
	for i := 0; i < numWorkers; i++ {
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

	// Walk directory
	go func() {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err == nil {
				paths <- path
			}
			return nil
		})
		close(paths)
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	<-done
	os.Exit(rc)
}
