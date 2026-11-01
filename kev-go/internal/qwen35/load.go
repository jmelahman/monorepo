package qwen35

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jmelahman/kev-go/internal/safetensors"
)

// weights finds a backbone's tensors across a checkpoint's safetensors files,
// whichever way the checkpoint prefixes their names.
type weights struct {
	files []*safetensors.File
}

var prefixes = []string{"model.language_model.", "language_model.", "model.", ""}

func openWeights(dir string) (*weights, error) {
	names := []string{"model.safetensors"}
	entries, err := filepath.Glob(filepath.Join(dir, "*.safetensors"))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "model.safetensors.index.json"))
	switch {
	case err == nil:
		var index struct {
			WeightMap map[string]string `json:"weight_map"`
		}
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("%s: index: %w", dir, err)
		}
		names = names[:0]
		for _, file := range index.WeightMap {
			if file != filepath.Base(file) {
				return nil, fmt.Errorf("%s: index names a file outside the checkpoint: %q", dir, file)
			}
			if !slices.Contains(names, file) {
				names = append(names, file)
			}
		}
	case len(entries) == 1:
		names = []string{filepath.Base(entries[0])}
	}
	w := &weights{}
	for _, name := range names {
		f, err := safetensors.Open(filepath.Join(dir, name))
		if err != nil {
			w.close()
			return nil, err
		}
		w.files = append(w.files, f)
	}
	return w, nil
}

func (w *weights) close() {
	for _, f := range w.files {
		f.Close()
	}
}

func (w *weights) find(name string) (*safetensors.File, string, safetensors.Info, bool) {
	for _, prefix := range prefixes {
		for _, f := range w.files {
			if info, ok := f.Info(prefix + name); ok {
				return f, prefix + name, info, true
			}
		}
	}
	return nil, "", safetensors.Info{}, false
}

// read returns the named tensor, checking its shape against the config's.
func (w *weights) read(name string, shape ...int) ([]float32, error) {
	f, full, info, ok := w.find(name)
	if !ok {
		return nil, fmt.Errorf("checkpoint has no tensor %s", name)
	}
	if !slices.Equal(info.Shape, shape) {
		return nil, fmt.Errorf("tensor %s has shape %v, want %v", name, info.Shape, shape)
	}
	return f.Float32s(full)
}

// An Adapter is a peft LoRA adapter to merge into the weights as they load.
type Adapter struct {
	file *safetensors.File
	// scale is lora_alpha / r.
	scale float32
	rank  int
	// tokens are the embedding rows the adapter replaces, in delta order.
	tokens []int32
	// left holds the layers whose LoRA pair hasn't been applied yet.
	left map[string]bool
}

const adapterPrefix = "base_model.model."

// OpenAdapter opens the adapter_config.json and adapter_model.safetensors in dir.
func OpenAdapter(dir string) (*Adapter, error) {
	data, err := os.ReadFile(filepath.Join(dir, "adapter_config.json"))
	if err != nil {
		return nil, err
	}
	var cfg struct {
		PeftType string             `json:"peft_type"`
		R        int                `json:"r"`
		Alpha    float64            `json:"lora_alpha"`
		DoRA     bool               `json:"use_dora"`
		RAlpha   bool               `json:"use_rslora"`
		Tokens   map[string][]int32 `json:"trainable_token_indices"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("adapter config: %w", err)
	}
	if cfg.PeftType != "LORA" || cfg.R <= 0 || cfg.DoRA || cfg.RAlpha {
		return nil, fmt.Errorf("unsupported adapter: %s, r=%d, dora=%v, rslora=%v", cfg.PeftType, cfg.R, cfg.DoRA, cfg.RAlpha)
	}
	f, err := safetensors.Open(filepath.Join(dir, "adapter_model.safetensors"))
	if err != nil {
		return nil, err
	}
	a := &Adapter{file: f, scale: float32(cfg.Alpha / float64(cfg.R)), rank: cfg.R, tokens: cfg.Tokens["embed_tokens"], left: map[string]bool{}}
	for _, name := range f.Names() {
		trimmed := strings.TrimPrefix(name, adapterPrefix)
		switch {
		case strings.HasSuffix(trimmed, ".lora_A.weight"):
			a.left[strings.TrimSuffix(trimmed, ".lora_A.weight")] = true
		case strings.HasSuffix(trimmed, ".lora_B.weight"):
		case strings.HasSuffix(trimmed, "trainable_tokens_delta"):
		default:
			f.Close()
			return nil, fmt.Errorf("adapter tensor %s isn't one this runtime applies", name)
		}
	}
	return a, nil
}

// Close closes the adapter's file.
func (a *Adapter) Close() error { return a.file.Close() }

// merge adds scale·B·A to the layer's weight w (out×in), if the adapter has it.
func (a *Adapter) merge(workers int, name string, w []float32, out, in int) error {
	if a == nil {
		return nil
	}
	la, lb := adapterPrefix+name+".lora_A.weight", adapterPrefix+name+".lora_B.weight"
	if _, ok := a.file.Info(la); !ok {
		return nil
	}
	A, err := a.file.Float32s(la)
	if err != nil {
		return err
	}
	B, err := a.file.Float32s(lb)
	if err != nil {
		return err
	}
	if len(A) != a.rank*in || len(B) != out*a.rank {
		return fmt.Errorf("adapter %s: sizes %d and %d don't fit %d×%d", name, len(A), len(B), out, in)
	}
	parallel(workers, out, 16, func(lo, hi int) {
		for o := lo; o < hi; o++ {
			for r := range a.rank {
				axpy(a.scale*B[o*a.rank+r], A[r*in:(r+1)*in], w[o*in:(o+1)*in])
			}
		}
	})
	delete(a.left, name)
	return nil
}

func (a *Adapter) tokenDeltas(hidden int) (map[int32][]float32, error) {
	if a == nil || len(a.tokens) == 0 {
		return nil, nil
	}
	for _, name := range a.file.Names() {
		if !strings.HasSuffix(name, "trainable_tokens_delta") || !strings.Contains(name, "embed_tokens") {
			continue
		}
		delta, err := a.file.Float32s(name)
		if err != nil {
			return nil, err
		}
		if len(delta) != len(a.tokens)*hidden {
			return nil, fmt.Errorf("adapter %s: %d values for %d tokens", name, len(delta), len(a.tokens))
		}
		out := make(map[int32][]float32, len(a.tokens))
		for i, id := range a.tokens {
			out[id] = delta[i*hidden : (i+1)*hidden]
		}
		return out, nil
	}
	return nil, fmt.Errorf("adapter names trainable tokens but has no embed_tokens delta")
}
