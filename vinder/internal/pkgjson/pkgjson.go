// Package pkgjson edits package.json files the way a person would: keys
// keep their order and the file keeps its indentation, so the diff shows
// only what changed.
package pkgjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrNotObject is returned when a path runs through a value that is not an
// object.
var ErrNotObject = errors.New("not an object")

// File is a package.json held as an ordered object.
type File struct {
	root   *object
	path   string
	indent string
}

// object is a JSON object that remembers its key order. Values are raw
// JSON, or *object for the ones an edit has descended into.
type object struct {
	vals map[string]any
	keys []string
}

// Read reads and parses file.
func Read(file string) (*File, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	root, err := parseObject(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &File{root: root, path: file, indent: detectIndent(data)}, nil
}

// String returns the string at path, and whether there is one.
func (f *File) String(path ...string) (string, bool) {
	raw, ok := f.get(path)
	if !ok {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// Strings returns the string values of the object at path, skipping any
// that are not strings.
func (f *File) Strings(path ...string) map[string]string {
	raw, ok := f.get(path)
	if !ok {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range m {
		var s string
		if json.Unmarshal(v, &s) == nil {
			out[k] = s
		}
	}
	return out
}

// SetString sets the string at path, creating the objects along it as
// needed. A new key goes at the end of its object.
func (f *File) SetString(value string, path ...string) error {
	raw, err := marshalString(value)
	if err != nil {
		return err
	}
	obj, err := f.descend(path[:len(path)-1], true)
	if err != nil {
		return err
	}
	obj.set(path[len(path)-1], json.RawMessage(raw))
	return nil
}

// Delete removes the key at path, and then any object along it the removal
// left empty.
func (f *File) Delete(path ...string) error {
	objs := []*object{f.root}
	for _, key := range path[:len(path)-1] {
		next, err := objs[len(objs)-1].child(key, false)
		if err != nil || next == nil {
			return err
		}
		objs = append(objs, next)
	}
	objs[len(objs)-1].remove(path[len(path)-1])
	for i := len(objs) - 1; i > 0 && len(objs[i].keys) == 0; i-- {
		objs[i-1].remove(path[i-1])
	}
	return nil
}

// Write writes the file back, in its own indentation, with a trailing
// newline.
func (f *File) Write() error {
	data, err := f.root.marshal()
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, data, "", f.indent); err != nil {
		return err
	}
	out.WriteByte('\n')
	return os.WriteFile(f.path, out.Bytes(), 0o644)
}

func (f *File) get(path []string) (json.RawMessage, bool) {
	obj, err := f.descend(path[:len(path)-1], false)
	if err != nil || obj == nil {
		return nil, false
	}
	switch v := obj.vals[path[len(path)-1]].(type) {
	case json.RawMessage:
		return v, true
	case *object:
		data, err := v.marshal()
		return data, err == nil
	}
	return nil, false
}

func (f *File) descend(path []string, create bool) (*object, error) {
	obj := f.root
	for _, key := range path {
		next, err := obj.child(key, create)
		if err != nil || next == nil {
			return nil, err
		}
		obj = next
	}
	return obj, nil
}

// child returns the object under key, parsing it on first use. Without
// create a missing key gives nil.
func (o *object) child(key string, create bool) (*object, error) {
	switch v := o.vals[key].(type) {
	case *object:
		return v, nil
	case json.RawMessage:
		child, err := parseObject(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, ErrNotObject)
		}
		o.vals[key] = child
		return child, nil
	}
	if !create {
		return nil, nil
	}
	child := &object{vals: map[string]any{}}
	o.set(key, child)
	return child, nil
}

func (o *object) set(key string, v any) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = v
}

func (o *object) remove(key string) {
	if _, ok := o.vals[key]; !ok {
		return
	}
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

func (o *object) marshal() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := marshalString(k)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		switch v := o.vals[k].(type) {
		case json.RawMessage:
			b.Write(v)
		case *object:
			data, err := v.marshal()
			if err != nil {
				return nil, err
			}
			b.Write(data)
		}
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshalString encodes s as a JSON string as a person would write it:
// json.Marshal escapes <, > and &, which version ranges are full of.
func marshalString(s string) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func parseObject(data []byte) (*object, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, ErrNotObject
	}
	obj := &object{vals: map[string]any{}}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		obj.set(key, v)
	}
	return obj, nil
}

// detectIndent returns the whitespace before the first indented key, two
// spaces if there is none.
func detectIndent(data []byte) string {
	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != line && strings.HasPrefix(trimmed, `"`) {
			return line[:len(line)-len(trimmed)]
		}
	}
	return "  "
}
