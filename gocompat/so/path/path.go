// Package path mirrors the subset of solod.dev/so/path used by undot.
package path

import "path"

// Match reports whether name matches the shell pattern.
func Match(pattern, name string) (bool, error) {
	return path.Match(pattern, name)
}
