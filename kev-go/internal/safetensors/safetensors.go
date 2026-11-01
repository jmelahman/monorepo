// Package safetensors reads the safetensors format: a little-endian uint64
// header length, a JSON header naming each tensor's dtype, shape and byte
// range, then the raw data. Parsing it runs nothing from the file, unlike a
// pickled checkpoint.
package safetensors

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
)

// maxHeader bounds the JSON header, so a corrupt length can't exhaust memory.
const maxHeader = 100 << 20

// Info describes one tensor in a file.
type Info struct {
	DType   string `json:"dtype"`
	Shape   []int  `json:"shape"`
	Offsets [2]int `json:"data_offsets"`
}

// A File is an open safetensors file. Tensors are read on demand.
type File struct {
	f       *os.File
	data    int64
	tensors map[string]Info
	meta    map[string]string
}

// Open reads the header of the safetensors file at path and checks that every
// tensor's byte range fits its shape and the file.
func Open(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := parse(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return st, nil
}

func parse(f *os.File) (*File, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var size uint64
	if err := binary.Read(f, binary.LittleEndian, &size); err != nil {
		return nil, fmt.Errorf("read header length: %w", err)
	}
	if size > maxHeader || int64(size)+8 > stat.Size() {
		return nil, fmt.Errorf("header length %d doesn't fit the file", size)
	}
	header := make([]byte, size)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(header, &raw); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	st := &File{f: f, data: int64(size) + 8, tensors: make(map[string]Info, len(raw))}
	for name, value := range raw {
		if name == "__metadata__" {
			if err := json.Unmarshal(value, &st.meta); err != nil {
				return nil, fmt.Errorf("metadata: %w", err)
			}
			continue
		}
		var info Info
		if err := json.Unmarshal(value, &info); err != nil {
			return nil, fmt.Errorf("tensor %s: %w", name, err)
		}
		width, err := elementSize(info.DType)
		if err != nil {
			return nil, fmt.Errorf("tensor %s: %w", name, err)
		}
		count := 1
		for _, dim := range info.Shape {
			if dim < 0 || (dim > 0 && count > math.MaxInt/width/dim) {
				return nil, fmt.Errorf("tensor %s: bad shape %v", name, info.Shape)
			}
			count *= dim
		}
		start, end := info.Offsets[0], info.Offsets[1]
		if start < 0 || end-start != count*width || st.data+int64(end) > stat.Size() {
			return nil, fmt.Errorf("tensor %s: range %v doesn't fit shape %v", name, info.Offsets, info.Shape)
		}
		st.tensors[name] = info
	}
	return st, nil
}

func elementSize(dtype string) (int, error) {
	switch dtype {
	case "BF16":
		return 2, nil
	case "F32":
		return 4, nil
	}
	return 0, fmt.Errorf("unsupported dtype %q", dtype)
}

// Close closes the file.
func (st *File) Close() error { return st.f.Close() }

// Names returns the tensors' names, sorted.
func (st *File) Names() []string {
	names := make([]string, 0, len(st.tensors))
	for name := range st.tensors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Metadata returns the header's free-form string metadata.
func (st *File) Metadata() map[string]string { return st.meta }

// Info returns the description of the named tensor.
func (st *File) Info(name string) (Info, bool) {
	info, ok := st.tensors[name]
	return info, ok
}

// Float32s reads the named tensor as float32 values in row-major order. BF16
// widens exactly: its bits are a float32's upper half.
func (st *File) Float32s(name string) ([]float32, error) {
	info, ok := st.tensors[name]
	if !ok {
		return nil, fmt.Errorf("no tensor %s", name)
	}
	buf := make([]byte, info.Offsets[1]-info.Offsets[0])
	if _, err := st.f.ReadAt(buf, st.data+int64(info.Offsets[0])); err != nil {
		return nil, fmt.Errorf("read tensor %s: %w", name, err)
	}
	width, _ := elementSize(info.DType)
	out := make([]float32, len(buf)/width)
	decode(out, buf, width)
	return out, nil
}

// ReadRow reads row i of the named matrix into dst, whose length must be the
// row's. It lets a table too big to keep in memory, like an embedding, stay on
// disk. It's safe for concurrent use.
func (st *File) ReadRow(name string, i int, dst []float32, buf []byte) ([]byte, error) {
	info, ok := st.tensors[name]
	if !ok {
		return buf, fmt.Errorf("no tensor %s", name)
	}
	if len(info.Shape) != 2 || i < 0 || i >= info.Shape[0] || len(dst) != info.Shape[1] {
		return buf, fmt.Errorf("tensor %s: no row %d of length %d in shape %v", name, i, len(dst), info.Shape)
	}
	width, _ := elementSize(info.DType)
	size := len(dst) * width
	if cap(buf) < size {
		buf = make([]byte, size)
	}
	buf = buf[:size]
	if _, err := st.f.ReadAt(buf, st.data+int64(info.Offsets[0])+int64(i*size)); err != nil {
		return buf, fmt.Errorf("read tensor %s: %w", name, err)
	}
	decode(dst, buf, width)
	return buf, nil
}

func decode(dst []float32, buf []byte, width int) {
	if width == 2 {
		for i := range dst {
			dst[i] = math.Float32frombits(uint32(binary.LittleEndian.Uint16(buf[i*2:])) << 16)
		}
		return
	}
	for i := range dst {
		dst[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
}
