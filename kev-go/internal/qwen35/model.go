package qwen35

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"slices"

	"github.com/jmelahman/kev-go/internal/safetensors"
)

// maxPass bounds the tokens of one pass, and so the memory of a long state: its
// activations are per token, and its caches outlive the pass only as keys,
// values and recurrent state.
const maxPass = 1024

// A Model is a loaded backbone. It's safe for concurrent use.
type Model struct {
	cfg     Config
	workers int
	layers  []*layer
	norm    []float32
	invFreq []float32

	// The embedding table stays on disk: it's a gigabyte, and most of it is
	// tokens a request never uses.
	file     *weights
	embed    *safetensors.File
	embedKey string
	tokens   map[int32][]float32
}

type layer struct {
	full      bool
	inputNorm []float32
	postNorm  []float32
	gate, up  []float32
	down      []float32

	// Gated DeltaNet.
	qkv, z, a, b, out []float32
	conv              []float32
	aLog, dtBias      []float32
	gateNorm          []float32

	// Full attention.
	q, k, v, o   []float32
	qNorm, kNorm []float32
}

// Open loads the backbone in dir, merging adapter into it if there is one.
// workers ≤ 0 uses every CPU.
func Open(dir string, adapter *Adapter, workers int) (*Model, error) {
	cfg, err := ReadConfig(filepath.Join(dir, "config.json"))
	if err != nil {
		return nil, err
	}
	w, err := openWeights(dir)
	if err != nil {
		return nil, err
	}
	m := &Model{cfg: cfg, workers: workers, file: w}
	if err := m.load(w, adapter); err != nil {
		w.close()
		return nil, err
	}
	return m, nil
}

// Close releases the checkpoint's files.
func (m *Model) Close() { m.file.close() }

// HiddenSize is the width of the states Forward returns.
func (m *Model) HiddenSize() int { return m.cfg.HiddenSize }

func (m *Model) load(w *weights, adapter *Adapter) error {
	c := m.cfg
	f, key, info, ok := w.find("embed_tokens.weight")
	if !ok {
		return fmt.Errorf("checkpoint has no embed_tokens.weight")
	}
	if !slices.Equal(info.Shape, []int{c.VocabSize, c.HiddenSize}) {
		return fmt.Errorf("embed_tokens.weight has shape %v", info.Shape)
	}
	m.embed, m.embedKey = f, key
	var err error
	if m.tokens, err = adapter.tokenDeltas(c.HiddenSize); err != nil {
		return err
	}
	if m.norm, err = w.read("norm.weight", c.HiddenSize); err != nil {
		return err
	}
	rotary := int(float64(c.HeadDim) * c.RopeParams.PartialFactor)
	for i := 0; i < rotary; i += 2 {
		exp := float32(i) / float32(rotary)
		m.invFreq = append(m.invFreq, float32(1/math.Pow(c.RopeParams.Theta, float64(exp))))
	}
	for i, kind := range c.LayerTypes {
		l := &layer{full: kind == fullAttention}
		if err := m.loadLayer(w, adapter, fmt.Sprintf("layers.%d.", i), l); err != nil {
			return err
		}
		m.layers = append(m.layers, l)
	}
	if adapter != nil && len(adapter.left) > 0 {
		for name := range adapter.left {
			return fmt.Errorf("adapter targets %s, which the backbone doesn't have", name)
		}
	}
	return nil
}

func (m *Model) loadLayer(w *weights, adapter *Adapter, p string, l *layer) error {
	c := m.cfg
	var err error
	matrix := func(dst *[]float32, name string, out, in int) {
		if err != nil {
			return
		}
		if *dst, err = w.read(p+name+".weight", out, in); err != nil {
			return
		}
		if merr := adapter.merge(m.workers, p+name, *dst, out, in); merr != nil {
			err = merr
		}
	}
	vector := func(dst *[]float32, name string, n int) {
		if err == nil {
			*dst, err = w.read(p+name, n)
		}
	}
	vector(&l.inputNorm, "input_layernorm.weight", c.HiddenSize)
	vector(&l.postNorm, "post_attention_layernorm.weight", c.HiddenSize)
	matrix(&l.gate, "mlp.gate_proj", c.IntermediateSize, c.HiddenSize)
	matrix(&l.up, "mlp.up_proj", c.IntermediateSize, c.HiddenSize)
	matrix(&l.down, "mlp.down_proj", c.HiddenSize, c.IntermediateSize)
	if l.full {
		matrix(&l.q, "self_attn.q_proj", c.NumHeads*c.HeadDim*2, c.HiddenSize)
		matrix(&l.k, "self_attn.k_proj", c.NumKVHeads*c.HeadDim, c.HiddenSize)
		matrix(&l.v, "self_attn.v_proj", c.NumKVHeads*c.HeadDim, c.HiddenSize)
		matrix(&l.o, "self_attn.o_proj", c.HiddenSize, c.NumHeads*c.HeadDim)
		vector(&l.qNorm, "self_attn.q_norm.weight", c.HeadDim)
		vector(&l.kNorm, "self_attn.k_norm.weight", c.HeadDim)
		return err
	}
	keyDim, valueDim := c.KHeadDim*c.NumKHeads, c.VHeadDim*c.NumVHeads
	convDim := keyDim*2 + valueDim
	matrix(&l.qkv, "linear_attn.in_proj_qkv", convDim, c.HiddenSize)
	matrix(&l.z, "linear_attn.in_proj_z", valueDim, c.HiddenSize)
	matrix(&l.a, "linear_attn.in_proj_a", c.NumVHeads, c.HiddenSize)
	matrix(&l.b, "linear_attn.in_proj_b", c.NumVHeads, c.HiddenSize)
	matrix(&l.out, "linear_attn.out_proj", c.HiddenSize, valueDim)
	vector(&l.aLog, "linear_attn.A_log", c.NumVHeads)
	vector(&l.dtBias, "linear_attn.dt_bias", c.NumVHeads)
	vector(&l.gateNorm, "linear_attn.norm.weight", c.VHeadDim)
	if err == nil {
		l.conv, err = w.read(p+"linear_attn.conv1d.weight", convDim, 1, c.ConvKernel)
	}
	return err
}

// A Cache is the state of a row after some tokens. Clone it to continue one
// prefix in several directions.
type Cache struct {
	n      int
	layers []layerCache
}

type layerCache struct {
	// keys and values are the past of an attention layer, as immutable segments
	// of tokens×kvHeads×headDim, so a clone shares them.
	keys, values [][]float32
	conv         []float32
	state        []float32
}

// NewCache returns the state of an empty row.
func (m *Model) NewCache() *Cache {
	c := m.cfg
	out := &Cache{layers: make([]layerCache, len(m.layers))}
	for i, l := range m.layers {
		if l.full {
			continue
		}
		convDim := c.KHeadDim*c.NumKHeads*2 + c.VHeadDim*c.NumVHeads
		out.layers[i].conv = make([]float32, (c.ConvKernel-1)*convDim)
		out.layers[i].state = make([]float32, c.NumVHeads*c.KHeadDim*c.VHeadDim)
	}
	return out
}

// Len is the number of tokens the cache has seen.
func (c *Cache) Len() int { return c.n }

// Clone returns an independent copy of c.
func (c *Cache) Clone() *Cache {
	out := &Cache{n: c.n, layers: make([]layerCache, len(c.layers))}
	for i, l := range c.layers {
		out.layers[i] = layerCache{
			keys:   slices.Clone(l.keys),
			values: slices.Clone(l.values),
			conv:   slices.Clone(l.conv),
			state:  slices.Clone(l.state),
		}
	}
	return out
}

// Embed returns the embeddings of ids, tokens×hidden.
func (m *Model) Embed(ids []int32) ([]float32, error) {
	d := m.cfg.HiddenSize
	out := make([]float32, len(ids)*d)
	var buf []byte
	var err error
	for i, id := range ids {
		row := out[i*d : (i+1)*d]
		if delta, ok := m.tokens[id]; ok {
			copy(row, delta)
			continue
		}
		if buf, err = m.embed.ReadRow(m.embedKey, int(id), row, buf); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Forward runs ids on from c, updating it, and returns the states of ids,
// tokens×hidden. A long row runs in several passes; every token's result is the
// same as in one.
func (m *Model) Forward(ctx context.Context, ids []int32, c *Cache) ([]float32, error) {
	return m.forward(ctx, ids, c, nil)
}

// forward is Forward with an observer of the states after the embedding (layer
// -1) and each layer, for parity tests.
func (m *Model) forward(ctx context.Context, ids []int32, c *Cache, observe func(layer int, x []float32)) ([]float32, error) {
	d := m.cfg.HiddenSize
	out := make([]float32, 0, len(ids)*d)
	for start := 0; start < len(ids); start += maxPass {
		x, err := m.Embed(ids[start:min(start+maxPass, len(ids))])
		if err != nil {
			return nil, err
		}
		if observe != nil {
			observe(-1, x)
		}
		tmp := make([]float32, len(x))
		for i, l := range m.layers {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			rmsNorm(m.workers, tmp, x, l.inputNorm, m.cfg.RMSNormEps)
			var mixed []float32
			if l.full {
				mixed = m.attention(l, tmp, &c.layers[i], c.n)
			} else {
				mixed = m.deltaNet(l, tmp, &c.layers[i])
			}
			addTo(x, mixed)
			rmsNorm(m.workers, tmp, x, l.postNorm, m.cfg.RMSNormEps)
			addTo(x, m.mlp(l, tmp))
			if observe != nil {
				observe(i, x)
			}
		}
		rmsNorm(m.workers, tmp, x, m.norm, m.cfg.RMSNormEps)
		out = append(out, tmp...)
		c.n += len(x) / d
	}
	return out, nil
}

func addTo(x, y []float32) {
	for i, v := range y {
		x[i] += v
	}
}

func (m *Model) mlp(l *layer, x []float32) []float32 {
	c := m.cfg
	tokens := len(x) / c.HiddenSize
	gate := make([]float32, tokens*c.IntermediateSize)
	up := make([]float32, tokens*c.IntermediateSize)
	linear(m.workers, gate, x, l.gate, c.HiddenSize, c.IntermediateSize)
	linear(m.workers, up, x, l.up, c.HiddenSize, c.IntermediateSize)
	parallel(m.workers, len(gate), 4096, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			gate[i] = silu(gate[i]) * up[i]
		}
	})
	out := make([]float32, len(x))
	linear(m.workers, out, gate, l.down, c.IntermediateSize, c.HiddenSize)
	return out
}

func (m *Model) deltaNet(l *layer, x []float32, cache *layerCache) []float32 {
	c := m.cfg
	d, tokens := c.HiddenSize, len(x)/c.HiddenSize
	keyDim, valueDim := c.KHeadDim*c.NumKHeads, c.VHeadDim*c.NumVHeads
	convDim := keyDim*2 + valueDim
	past := c.ConvKernel - 1

	seq := make([]float32, (past+tokens)*convDim)
	copy(seq, cache.conv)
	linear(m.workers, seq[past*convDim:], x, l.qkv, d, convDim)
	copy(cache.conv, seq[tokens*convDim:])

	mixed := make([]float32, tokens*convDim)
	parallel(m.workers, tokens, 4, func(lo, hi int) {
		for t := lo; t < hi; t++ {
			out := mixed[t*convDim : (t+1)*convDim]
			for j := range c.ConvKernel {
				in := seq[(t+j)*convDim : (t+j+1)*convDim]
				for ch, v := range in {
					out[ch] += l.conv[ch*c.ConvKernel+j] * v
				}
			}
			for ch, v := range out {
				out[ch] = silu(v)
			}
		}
	})

	z := make([]float32, tokens*valueDim)
	a := make([]float32, tokens*c.NumVHeads)
	b := make([]float32, tokens*c.NumVHeads)
	linear(m.workers, z, x, l.z, d, valueDim)
	linear(m.workers, a, x, l.a, d, c.NumVHeads)
	linear(m.workers, b, x, l.b, d, c.NumVHeads)

	out := make([]float32, tokens*valueDim)
	repeat := c.NumVHeads / c.NumKHeads
	kd, vd := c.KHeadDim, c.VHeadDim
	scaleQ := float32(1 / math.Sqrt(float64(kd)))
	parallel(m.workers, c.NumVHeads, 1, func(lo, hi int) {
		q, k := make([]float32, kd), make([]float32, kd)
		mem := make([]float32, vd)
		for h := lo; h < hi; h++ {
			s := cache.state[h*kd*vd : (h+1)*kd*vd]
			keyHead := h / repeat
			scale := float32(math.Exp(float64(l.aLog[h])))
			for t := range tokens {
				row := mixed[t*convDim : (t+1)*convDim]
				l2norm(q, row[keyHead*kd:(keyHead+1)*kd])
				for i := range q {
					q[i] *= scaleQ
				}
				l2norm(k, row[keyDim+keyHead*kd:keyDim+(keyHead+1)*kd])
				v := row[2*keyDim+h*vd : 2*keyDim+(h+1)*vd]

				decay := float32(math.Exp(-float64(scale) * softplus(float64(a[t*c.NumVHeads+h]+l.dtBias[h]))))
				beta := sigmoid(b[t*c.NumVHeads+h])
				for i := range s {
					s[i] *= decay
				}
				clear(mem)
				for i, ki := range k {
					axpy(ki, s[i*vd:(i+1)*vd], mem)
				}
				for i := range mem {
					mem[i] = (v[i] - mem[i]) * beta
				}
				for i, ki := range k {
					axpy(ki, mem, s[i*vd:(i+1)*vd])
				}
				res := out[t*valueDim+h*vd : t*valueDim+(h+1)*vd]
				for i, qi := range q {
					axpy(qi, s[i*vd:(i+1)*vd], res)
				}
				// RMSNormGated, on the head's output.
				scaleOut := invRMS(res, c.RMSNormEps)
				gate := z[t*valueDim+h*vd : t*valueDim+(h+1)*vd]
				for i := range res {
					res[i] = res[i] * scaleOut * l.gateNorm[i] * silu(gate[i])
				}
			}
		}
	})
	result := make([]float32, len(x))
	linear(m.workers, result, out, l.out, valueDim, d)
	return result
}

func softplus(x float64) float64 {
	return math.Log1p(math.Exp(x))
}

func l2norm(dst, src []float32) {
	inv := float32(1 / math.Sqrt(float64(dot(src, src))+1e-6))
	for i, v := range src {
		dst[i] = v * inv
	}
}

func (m *Model) attention(l *layer, x []float32, cache *layerCache, start int) []float32 {
	c := m.cfg
	d, tokens := c.HiddenSize, len(x)/c.HiddenSize
	hd := c.HeadDim
	qw := c.NumHeads * hd * 2
	kw := c.NumKVHeads * hd

	qg := make([]float32, tokens*qw)
	keys := make([]float32, tokens*kw)
	values := make([]float32, tokens*kw)
	linear(m.workers, qg, x, l.q, d, qw)
	linear(m.workers, keys, x, l.k, d, kw)
	linear(m.workers, values, x, l.v, d, kw)

	rotary := 2 * len(m.invFreq)
	half := rotary / 2
	parallel(m.workers, tokens, 8, func(lo, hi int) {
		cos, sin := make([]float32, half), make([]float32, half)
		tmp := make([]float32, rotary)
		rotate := func(v []float32) {
			copy(tmp, v[:rotary])
			for i := range half {
				v[i] = tmp[i]*cos[i] - tmp[half+i]*sin[i]
				v[half+i] = tmp[half+i]*cos[i] + tmp[i]*sin[i]
			}
		}
		for t := lo; t < hi; t++ {
			pos := float32(start + t)
			for i, f := range m.invFreq {
				s, co := math.Sincos(float64(pos * f))
				cos[i], sin[i] = float32(co), float32(s)
			}
			for h := range c.NumHeads {
				q := qg[t*qw+h*2*hd : t*qw+h*2*hd+hd]
				scale := invRMS(q, c.RMSNormEps)
				for i, v := range q {
					q[i] = v * scale * (1 + l.qNorm[i])
				}
				rotate(q)
			}
			for h := range c.NumKVHeads {
				k := keys[t*kw+h*hd : t*kw+(h+1)*hd]
				scale := invRMS(k, c.RMSNormEps)
				for i, v := range k {
					k[i] = v * scale * (1 + l.kNorm[i])
				}
				rotate(k)
			}
		}
	})

	cache.keys = append(cache.keys, keys)
	cache.values = append(cache.values, values)
	past := start
	group := c.NumHeads / c.NumKVHeads
	scaling := float32(1 / math.Sqrt(float64(hd)))

	gated := make([]float32, tokens*c.NumHeads*hd)
	parallel(m.workers, tokens*c.NumHeads, 1, func(lo, hi int) {
		weights := make([]float32, past+tokens)
		for job := lo; job < hi; job++ {
			t, h := job/c.NumHeads, job%c.NumHeads
			kv := h / group
			q := qg[t*qw+h*2*hd : t*qw+h*2*hd+hd]
			visible := past + t + 1
			w := weights[:visible]
			at := 0
			for _, seg := range cache.keys {
				n := min(len(seg)/kw, visible-at)
				for p := range n {
					w[at+p] = dot(q, seg[p*kw+kv*hd:p*kw+(kv+1)*hd]) * scaling
				}
				at += n
			}
			softmax(w)
			res := gated[t*c.NumHeads*hd+h*hd : t*c.NumHeads*hd+(h+1)*hd]
			at = 0
			for _, seg := range cache.values {
				n := min(len(seg)/kw, visible-at)
				for p := range n {
					axpy(w[at+p], seg[p*kw+kv*hd:p*kw+(kv+1)*hd], res)
				}
				at += n
			}
			gate := qg[t*qw+h*2*hd+hd : t*qw+(h+1)*2*hd]
			for i := range res {
				res[i] *= sigmoid(gate[i])
			}
		}
	})
	out := make([]float32, len(x))
	linear(m.workers, out, gated, l.o, c.NumHeads*hd, d)
	return out
}

func softmax(w []float32) {
	best := w[0]
	for _, v := range w {
		best = max(best, v)
	}
	var sum float64
	for i, v := range w {
		e := math.Exp(float64(v - best))
		w[i] = float32(e)
		sum += e
	}
	inv := float32(1 / sum)
	for i := range w {
		w[i] *= inv
	}
}
