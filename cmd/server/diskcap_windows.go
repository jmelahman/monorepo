//go:build windows

package server

import "errors"

// filesystemBytes is unsupported on Windows (no worker builds ship for it);
// the caller logs and leaves the cache uncapped.
func filesystemBytes(string) (int64, error) {
	return 0, errors.New("filesystem size unsupported on windows")
}
