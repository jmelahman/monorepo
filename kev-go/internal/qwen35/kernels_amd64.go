package qwen35

import "golang.org/x/sys/cpu"

var useAVX2 = cpu.X86.HasAVX2 && cpu.X86.HasFMA

//go:noescape
func dotAVX2(a, b *float32, n int) float32

//go:noescape
func axpyAVX2(alpha float32, x, y *float32, n int)

func dot(a, b []float32) float32 {
	if useAVX2 && len(a) >= 16 {
		b = b[:len(a)]
		return dotAVX2(&a[0], &b[0], len(a))
	}
	return dotGeneric(a, b)
}

func axpy(alpha float32, x, y []float32) {
	if useAVX2 && len(x) >= 16 {
		y = y[:len(x)]
		axpyAVX2(alpha, &x[0], &y[0], len(x))
		return
	}
	axpyGeneric(alpha, x, y)
}
