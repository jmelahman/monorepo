// Package bufio mirrors the subset of solod.dev/so/bufio that
// check-symlinks uses. Solod's Scanner owns an explicit buffer, hence the
// allocator and Free; under Go both are ignored.
package bufio

import (
	gobufio "bufio"
	"io"

	"solod.dev/so/mem"
)

const DefaultBufSize = 4096
const MaxScanTokenSize = 64 * 1024

// Scanner is returned by value, as it is in solod.
type Scanner struct {
	sc *gobufio.Scanner
}

func NewScanner(a mem.Allocator, r io.Reader) Scanner {
	_ = a
	return Scanner{sc: gobufio.NewScanner(r)}
}

func (s *Scanner) Scan() bool    { return s.sc.Scan() }
func (s *Scanner) Text() string  { return s.sc.Text() }
func (s *Scanner) Bytes() []byte { return s.sc.Bytes() }
func (s *Scanner) Err() error    { return s.sc.Err() }
func (s *Scanner) Free()         {}
