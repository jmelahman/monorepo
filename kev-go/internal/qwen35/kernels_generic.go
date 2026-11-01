//go:build !amd64

package qwen35

func dot(a, b []float32) float32 { return dotGeneric(a, b) }

func axpy(alpha float32, x, y []float32) { axpyGeneric(alpha, x, y) }
