// Package errors mirrors solod.dev/so/errors.
package errors

import "errors"

// New returns a distinct error value for the given message. As in solod,
// errors created at package level act as sentinels comparable with ==.
func New(text string) error {
	return errors.New(text)
}
