package safetensors

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func write(t *testing.T, header string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.safetensors")
	out := binary.LittleEndian.AppendUint64(nil, uint64(len(header)))
	out = append(append(out, header...), data...)
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFloat32s(t *testing.T) {
	var data []byte
	data = binary.LittleEndian.AppendUint32(data, math.Float32bits(1.5))
	data = binary.LittleEndian.AppendUint32(data, math.Float32bits(-2))
	data = binary.LittleEndian.AppendUint16(data, uint16(math.Float32bits(0.5)>>16))
	path := write(t, `{"__metadata__":{"format":"pt"},"f":{"dtype":"F32","shape":[2],"data_offsets":[0,8]},"b":{"dtype":"BF16","shape":[1],"data_offsets":[8,10]}}`, data)
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if got := st.Names(); !slices.Equal(got, []string{"b", "f"}) {
		t.Errorf("names %v", got)
	}
	if got, err := st.Float32s("f"); err != nil || !slices.Equal(got, []float32{1.5, -2}) {
		t.Errorf("f = %v, %v", got, err)
	}
	if got, err := st.Float32s("b"); err != nil || !slices.Equal(got, []float32{0.5}) {
		t.Errorf("b = %v, %v", got, err)
	}
	if _, err := st.Float32s("missing"); err == nil {
		t.Error("missing tensor read")
	}
}

func TestOpenRejects(t *testing.T) {
	for name, tc := range map[string]struct {
		header string
		want   string
	}{
		"past the file": {`{"f":{"dtype":"F32","shape":[2],"data_offsets":[0,8]}}`, "doesn't fit"},
		"wrong size":    {`{"f":{"dtype":"F32","shape":[1],"data_offsets":[0,2]}}`, "doesn't fit"},
		"dtype":         {`{"f":{"dtype":"F64","shape":[1],"data_offsets":[0,8]}}`, "unsupported"},
		"overflow":      {`{"f":{"dtype":"F32","shape":[4611686018427387904,4],"data_offsets":[0,0]}}`, "bad shape"},
		"not json":      {`nope`, "parse header"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Open(write(t, tc.header, make([]byte, 4)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}
