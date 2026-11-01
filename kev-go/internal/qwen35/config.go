// Package qwen35 runs the text backbone of Qwen3.5: layers of Gated DeltaNet
// and full attention, each followed by a gated MLP. It's a port of transformers'
// modeling_qwen3_5.py, and takes every size from the checkpoint's config.
package qwen35

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is the part of a checkpoint's config.json the text backbone uses.
type Config struct {
	HiddenSize       int      `json:"hidden_size"`
	IntermediateSize int      `json:"intermediate_size"`
	NumLayers        int      `json:"num_hidden_layers"`
	LayerTypes       []string `json:"layer_types"`
	HiddenAct        string   `json:"hidden_act"`
	RMSNormEps       float32  `json:"rms_norm_eps"`
	VocabSize        int      `json:"vocab_size"`

	NumHeads   int  `json:"num_attention_heads"`
	NumKVHeads int  `json:"num_key_value_heads"`
	HeadDim    int  `json:"head_dim"`
	AttnBias   bool `json:"attention_bias"`
	ConvKernel int  `json:"linear_conv_kernel_dim"`
	NumKHeads  int  `json:"linear_num_key_heads"`
	KHeadDim   int  `json:"linear_key_head_dim"`
	NumVHeads  int  `json:"linear_num_value_heads"`
	VHeadDim   int  `json:"linear_value_head_dim"`
	RopeParams struct {
		Type          string  `json:"rope_type"`
		Theta         float64 `json:"rope_theta"`
		PartialFactor float64 `json:"partial_rotary_factor"`
	} `json:"rope_parameters"`
}

const (
	linearAttention = "linear_attention"
	fullAttention   = "full_attention"
)

// ReadConfig reads the text config from a config.json, which nests it under
// text_config in multimodal checkpoints.
func ReadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var outer struct {
		Text *Config `json:"text_config"`
	}
	if err := json.Unmarshal(data, &outer); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	cfg := outer.Text
	if cfg == nil {
		cfg = new(Config)
		if err := json.Unmarshal(data, cfg); err != nil {
			return Config{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	if err := cfg.check(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return *cfg, nil
}

func (c *Config) check() error {
	if c.RopeParams.PartialFactor == 0 {
		c.RopeParams.PartialFactor = 1
	}
	switch {
	case c.HiddenSize <= 0 || c.IntermediateSize <= 0 || c.VocabSize <= 0:
		return fmt.Errorf("missing sizes")
	case len(c.LayerTypes) != c.NumLayers || c.NumLayers == 0:
		return fmt.Errorf("%d layer types for %d layers", len(c.LayerTypes), c.NumLayers)
	case c.HiddenAct != "silu":
		return fmt.Errorf("unsupported activation %q", c.HiddenAct)
	case c.AttnBias:
		return fmt.Errorf("attention_bias isn't supported")
	case c.RopeParams.Type != "default":
		return fmt.Errorf("unsupported rope type %q", c.RopeParams.Type)
	case c.NumKVHeads == 0 || c.NumHeads%c.NumKVHeads != 0:
		return fmt.Errorf("%d heads don't divide into %d key/value heads", c.NumHeads, c.NumKVHeads)
	case c.NumKHeads == 0 || c.NumVHeads%c.NumKHeads != 0:
		return fmt.Errorf("%d value heads don't divide into %d key heads", c.NumVHeads, c.NumKHeads)
	}
	for _, t := range c.LayerTypes {
		if t != linearAttention && t != fullAttention {
			return fmt.Errorf("unsupported layer type %q", t)
		}
	}
	return nil
}
