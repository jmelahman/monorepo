// Package config loads a repository's .config/lockstep/config.toml: the pins
// lockstep keeps in step, where each one's version comes from, and where its
// copies are written.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// File is the path of the config, relative to the repository root.
const File = ".config/lockstep/config.toml"

// The places a pin's version can come from.
const (
	// SourceUVLock is the version uv.lock, at the repository root, locks
	// the pin's package at.
	SourceUVLock = "uv.lock"
	// SourcePreCommit is the tag in the `# frozen:` comment (or else the
	// rev) of the pin's Repo in .pre-commit-config.yaml, at the repository
	// root.
	SourcePreCommit = "pre-commit"
)

var (
	// ErrNoConfig is returned by Load for a repository without a config.
	ErrNoConfig = errors.New("no " + File)
	// ErrBadConfig is returned by Load for a config that does not describe
	// valid pins.
	ErrBadConfig = errors.New("invalid " + File)
)

// Pin is one version kept in step across the repository.
type Pin struct {
	// Name is the pin's key in the config.
	Name string `toml:"-"`
	// Source is where the version is defined, SourceUVLock or
	// SourcePreCommit. It is unset when Version is.
	Source string `toml:"source"`
	// Version is the version itself, for a pin nothing else defines.
	Version string `toml:"version"`
	// Package is the name the pin goes by in uv.lock and in hooks'
	// additional_dependencies; Name when unset.
	Package string `toml:"package"`
	// Repo is a pre-commit hook repository whose rev follows the pin.
	Repo string `toml:"repo"`
	// Action is a GitHub Action, as owner/name, whose Input follows the pin
	// wherever a step uses it.
	Action string `toml:"action"`
	// Input is the input of Action that holds the version; "version" when
	// unset.
	Input string `toml:"input"`
}

type file struct {
	Pins map[string]Pin `toml:"pins"`
}

// Load reads and validates the config of the repository at root, and
// returns its pins sorted by name, with Package and Input defaulted.
func Load(root string) ([]Pin, error) {
	data, err := os.ReadFile(filepath.Join(root, File))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoConfig
	}
	if err != nil {
		return nil, err
	}
	var f file
	md, err := toml.Decode(string(data), &f)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadConfig, err)
	}
	// A misspelled key would otherwise leave a copy silently unmanaged.
	if keys := md.Undecoded(); len(keys) > 0 {
		return nil, fmt.Errorf("%w: unknown key %s", ErrBadConfig, keys[0])
	}

	pins := make([]Pin, 0, len(f.Pins))
	for name, p := range f.Pins {
		p.Name = name
		if p.Package == "" {
			p.Package = name
		}
		if p.Action != "" && p.Input == "" {
			p.Input = "version"
		}
		if err := p.validate(); err != nil {
			return nil, fmt.Errorf("%w: pins.%s: %w", ErrBadConfig, name, err)
		}
		pins = append(pins, p)
	}
	slices.SortFunc(pins, func(a, b Pin) int { return strings.Compare(a.Name, b.Name) })
	return pins, nil
}

func (p Pin) validate() error {
	switch {
	case p.Source == "" && p.Version == "":
		return errors.New("needs a source or a version")
	case p.Source != "" && p.Version != "":
		return errors.New("has both a source and a version")
	case p.Source != "" && p.Source != SourceUVLock && p.Source != SourcePreCommit:
		return fmt.Errorf("source %q is not %q or %q", p.Source, SourceUVLock, SourcePreCommit)
	case p.Source == SourcePreCommit && p.Repo == "":
		return errors.New(`source "pre-commit" needs a repo`)
	case p.Input != "" && p.Action == "":
		return errors.New("input needs an action")
	case p.Action != "" && strings.Count(p.Action, "/") < 1:
		return fmt.Errorf("action %q is not owner/name", p.Action)
	}
	return nil
}
