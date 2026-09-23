// Package mem mirrors solod.dev/so/mem. Under Go the garbage collector owns
// all memory, so allocators are accepted and ignored.
package mem

type Allocator any

var System Allocator

type Arena struct{}

func NewArena(buf []byte) Arena {
	_ = buf
	return Arena{}
}
