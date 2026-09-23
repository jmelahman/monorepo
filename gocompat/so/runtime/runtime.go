// Package runtime mirrors the subset of solod.dev/so/runtime that
// check-symlinks uses.
package runtime

import (
	goruntime "runtime"
)

const GOOS = goruntime.GOOS
const GOARCH = goruntime.GOARCH

func NumCPU() int { return goruntime.NumCPU() }

func Version() string { return goruntime.Version() }
