//go:build !cgo

package speech

import "errors"

const native = false

func load(dir, lang string, threads int) (engine, error) {
	return nil, errors.New("built without cgo, so no voices on this computer")
}
