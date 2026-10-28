// Package llm embeds llama.cpp, through yzma, for scoring text rather than
// generating it: it installs the llama.cpp shared libraries, fetches GGUF
// models pinned by digest, and exposes the logits a model assigns at every
// position of a token sequence.
//
// The libraries are loaded at run time with purego, so building needs no cgo
// and no llama.cpp checkout; [Install] downloads a prebuilt release instead.
package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/hybridgroup/yzma/pkg/download"
	"github.com/hybridgroup/yzma/pkg/llama"
)

// ErrNotInstalled is returned by [Init] when the directory holds no llama.cpp
// libraries; [Install] puts them there.
var ErrNotInstalled = errors.New("llama.cpp libraries are not installed")

// LlamaVersion is the llama.cpp release [Install] downloads: the one the
// pinned yzma release was tested with. Its digest manifest is pinned too, so
// every asset is checked against a SHA-256 that ships in this binary.
var LlamaVersion = download.DefaultVersion

// CacheDir is where models and libraries are kept by default:
// $XDG_CACHE_HOME/local-ai-detection on Linux.
func CacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "local-ai-detection"), nil
}

// LibDir is the directory under cache that holds the llama.cpp libraries for
// processor ("cpu", "cuda", "vulkan", ...). It is keyed by [LlamaVersion], so
// upgrading yzma installs alongside rather than over an older release.
func LibDir(cache, processor string) string {
	tag, _, _ := strings.Cut(LlamaVersion, "@")
	return filepath.Join(cache, "llama.cpp", tag, processor)
}

// Install downloads the llama.cpp libraries for processor into dir, unless
// they are already there. Download progress is written to progress. The
// libraries are unpacked beside dir and moved into place once complete, so an
// interrupted install leaves nothing that [Init] would mistake for one.
func Install(ctx context.Context, dir, processor string, progress io.Writer) error {
	if download.AlreadyInstalled(dir) {
		return nil
	}
	proc, err := download.ParseProcessor(processor)
	if err != nil {
		return err
	}
	arch, err := download.ParseArch(runtime.GOARCH)
	if err != nil {
		return err
	}
	goos, err := download.ParseOS(runtime.GOOS)
	if err != nil {
		return err
	}
	target := download.Target{Arch: arch, OS: goos, Processor: proc, Version: LlamaVersion}
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), ".install-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := download.Install(ctx, target, tmp, tracker{progress}, nil, download.WithVerify(download.VerifyRequired)); err != nil {
		return fmt.Errorf("install llama.cpp %s for %s: %w", target.Version, processor, err)
	}
	// Whatever is in dir is not a complete install, or AlreadyInstalled
	// would have said so.
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.Rename(tmp, dir)
}

var (
	initOnce sync.Once
	errInit  error
)

// Init loads the llama.cpp libraries in dir. llama.cpp is process-global, so
// only the first call does anything, and every later one returns its result
// whatever dir it is given.
func Init(dir string) error {
	initOnce.Do(func() {
		if !download.AlreadyInstalled(dir) {
			errInit = fmt.Errorf("%w in %s", ErrNotInstalled, dir)
			return
		}
		if errInit = llama.Load(dir); errInit != nil {
			return
		}
		llama.LogSet(llama.LogSilent())
		errInit = llama.Init()
	})
	return errInit
}
