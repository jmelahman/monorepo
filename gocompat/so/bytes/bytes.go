// Package bytes mirrors the subset of solod.dev/so/bytes used by
// undot.
package bytes

import (
	"bytes"

	"solod.dev/so/mem"
)

type Buffer struct {
	b bytes.Buffer
}

func NewBuffer(a mem.Allocator, buf []byte) Buffer {
	_ = a
	var b Buffer
	b.b.Write(buf)
	return b
}

func (b *Buffer) Bytes() []byte                     { return b.b.Bytes() }
func (b *Buffer) Len() int                          { return b.b.Len() }
func (b *Buffer) String() string                    { return b.b.String() }
func (b *Buffer) Write(p []byte) (int, error)       { return b.b.Write(p) }
func (b *Buffer) WriteByte(c byte) error            { return b.b.WriteByte(c) }
func (b *Buffer) WriteString(s string) (int, error) { return b.b.WriteString(s) }
