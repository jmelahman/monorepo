package qwen35

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"
)

// parallel runs f over [0,n) in blocks of at most grain, on up to workers
// goroutines. Blocks are claimed as workers free up, since they cost unevenly.
func parallel(workers, n, grain int, f func(lo, hi int)) {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if n <= grain || workers == 1 {
		for lo := 0; lo < n; lo += grain {
			f(lo, min(lo+grain, n))
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(workers, (n+grain-1)/grain) {
		wg.Go(func() {
			for {
				lo := int(next.Add(int64(grain))) - grain
				if lo >= n {
					return
				}
				f(lo, min(lo+grain, n))
			}
		})
	}
	wg.Wait()
}

func dotGeneric(a, b []float32) float32 {
	b = b[:len(a)]
	var s0, s1, s2, s3 float32
	i := 0
	for ; i+4 <= len(a); i += 4 {
		s0 += a[i] * b[i]
		s1 += a[i+1] * b[i+1]
		s2 += a[i+2] * b[i+2]
		s3 += a[i+3] * b[i+3]
	}
	for ; i < len(a); i++ {
		s0 += a[i] * b[i]
	}
	return s0 + s1 + s2 + s3
}

func axpyGeneric(alpha float32, x, y []float32) {
	y = y[:len(x)]
	for i, v := range x {
		y[i] += alpha * v
	}
}

// matrixBlock is the side of the blocks linear splits its work into, so a block
// of tokens and a block of weight rows both stay in a core's cache.
const matrixBlock = 32

// linear sets dst[t,o] = x[t,:]·w[o,:] for tokens t and outputs o: x has rows of
// width in, w is out×in, dst has rows of width out.
func linear(workers int, dst, x, w []float32, in, out int) {
	tokens := len(x) / in
	parallel(workers, out, matrixBlock, func(lo, hi int) {
		for t0 := 0; t0 < tokens; t0 += matrixBlock {
			t1 := min(t0+matrixBlock, tokens)
			for o := lo; o < hi; o++ {
				row := w[o*in : (o+1)*in]
				for t := t0; t < t1; t++ {
					dst[t*out+o] = dot(x[t*in:(t+1)*in], row)
				}
			}
		}
	})
}

func sigmoid(x float32) float32 {
	return float32(1 / (1 + math.Exp(-float64(x))))
}

func silu(x float32) float32 { return x * sigmoid(x) }

// rmsNorm sets dst to src's rows of width len(w), each divided by its root mean
// square and scaled by 1+w: the zero-centred form the checkpoints store.
func rmsNorm(workers int, dst, src, w []float32, eps float32) {
	width := len(w)
	parallel(workers, len(src)/width, 16, func(lo, hi int) {
		for t := lo; t < hi; t++ {
			in, out := src[t*width:(t+1)*width], dst[t*width:(t+1)*width]
			scale := invRMS(in, eps)
			for i, v := range in {
				out[i] = v * scale * (1 + w[i])
			}
		}
	})
}

func invRMS(x []float32, eps float32) float32 {
	return float32(1 / math.Sqrt(float64(dot(x, x)/float32(len(x))+eps)))
}
