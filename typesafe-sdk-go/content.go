package typesafe

import (
	"encoding/json"
	"reflect"
)

// Content is a JSON value the API accepts wherever text, a JSON object, or a JSON
// array may appear: request state, question instructions, and criteria
// descriptions.
//
// A Content holds one of:
//
//	a string           "Is this message spam?"
//	a JSON object      map[string]any{"task": "..."}, or a struct that marshals to an object
//	a JSON array       []any{...}, []string{...}
//	nil                the field is omitted from the request; see [Null]
//	json.RawMessage    copied through verbatim
//
// The API does not accept numbers or booleans, and they are rejected before a
// request is sent.
type Content any

// Null is the [Content] that sends an explicit JSON null instead of omitting the
// field. A nil Content in a question field is omitted from the request; Null is
// transmitted.
//
// Inside [ChoiceCriteria] a plain nil is already sent as null, so Null is not
// needed there.
var Null Content = nullContent{}

type nullContent struct{}

func (nullContent) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

// validateContent reports whether c is a shape the API accepts. It rejects the
// two JSON types the API has no field for, rather than letting them reach the
// server as a 422.
func validateContent(path string, c Content) error {
	if c == nil {
		return nil
	}
	// json.Number has a string Kind but marshals as a bare number.
	if _, ok := c.(json.Number); ok {
		return errf("%s must be a string, object, or array, got %T", path, c)
	}
	switch reflect.ValueOf(c).Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return errf("%s must be a string, object, or array, got %T", path, c)
	}
	return nil
}
