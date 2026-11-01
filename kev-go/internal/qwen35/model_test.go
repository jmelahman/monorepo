package qwen35

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/kev-go/internal/safetensors"
)

func checkClose(t *testing.T, name string, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d values, want %d", name, len(got), len(want))
	}
	var worst, scale float64
	at := 0
	for i := range got {
		if d := math.Abs(float64(got[i] - want[i])); d > worst {
			worst, at = d, i
		}
		scale = max(scale, math.Abs(float64(want[i])))
	}
	if worst > tol*max(scale, 1) {
		t.Errorf("%s: worst difference %g at %d (got %g, want %g), scale %g", name, worst, at, got[at], want[at], scale)
	} else {
		t.Logf("%s: worst difference %g, scale %g", name, worst, scale)
	}
}

// Compares every layer's states with Python's, from KEV_LAYERS (a safetensors
// file of them), on the backbone in KEV_BASE with the adapter in KEV_CHECKPOINT.
func TestLayersMatchPython(t *testing.T) {
	base, ckpt, layers := os.Getenv("KEV_BASE"), os.Getenv("KEV_CHECKPOINT"), os.Getenv("KEV_LAYERS")
	if base == "" || ckpt == "" || layers == "" {
		t.Skip("KEV_BASE, KEV_CHECKPOINT or KEV_LAYERS isn't set")
	}
	ad, err := OpenAdapter(ckpt)
	if err != nil {
		t.Fatal(err)
	}
	defer ad.Close()
	m, err := Open(base, ad, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ref, err := safetensors.Open(layers)
	if err != nil {
		t.Fatal(err)
	}
	defer ref.Close()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "requests.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx []struct {
		Rows []struct {
			IDs []int32 `json:"ids"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatal(err)
	}
	want := func(name string) []float32 {
		v, err := ref.Float32s(name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	checkClose(t, "merged k_proj", m.layers[3].k, want("merged.layers.3.self_attn.k_proj.weight"), 1e-6)
	checkClose(t, "merged in_proj_a", m.layers[0].a, want("merged.layers.0.linear_attn.in_proj_a.weight"), 1e-6)
	ids := fx[2].Rows[0].IDs
	final, err := m.forward(context.Background(), ids, m.NewCache(), func(i int, x []float32) {
		name := "embed"
		if i >= 0 {
			name = "layer." + itoa(i)
		}
		checkClose(t, name, x, want(name), 2e-3)
	})
	if err != nil {
		t.Fatal(err)
	}
	checkClose(t, "final", final, want("final"), 2e-3)
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
