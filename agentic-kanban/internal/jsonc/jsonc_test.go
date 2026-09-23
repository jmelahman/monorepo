package jsonc

import (
	"encoding/json"
	"testing"
)

func TestStripComments(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no comments", `{"a":1}`, `{"a":1}`},
		{"line comment", "{\n// comment\n\"a\":1\n}", "{\n\n\"a\":1\n}"},
		{"trailing line comment", `{"a":1} // tail`, `{"a":1} `},
		{"block comment", `{"a":1,/* b */"c":2}`, `{"a":1,"c":2}`},
		{"multiline block", "{/*\nx\ny\n*/\"a\":1}", `{"a":1}`},
		// Comment markers inside string values must survive untouched.
		{"slashes in string", `{"url":"http://x/y"}`, `{"url":"http://x/y"}`},
		{"block marker in string", `{"s":"a/* not a comment */b"}`, `{"s":"a/* not a comment */b"}`},
		{"escaped quote in string", `{"s":"he said \"// hi\""}`, `{"s":"he said \"// hi\""}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(StripComments([]byte(c.in))); got != c.want {
				t.Fatalf("StripComments(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestStrip(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"trailing comma in object", `{"a":1,}`, `{"a":1}`},
		{"trailing comma in array", `[1,2,]`, `[1,2]`},
		{"nested trailing commas", `{"a":[1,],}`, `{"a":[1]}`},
		{"comma then whitespace", "{\"a\":1,\n  }", "{\"a\":1\n  }"},
		// A comment between the comma and the closer must not hide it.
		{"comma hidden by comment", "{\"a\":1, // tail\n}", "{\"a\":1 \n}"},
		{"comma in string survives", `{"s":",}"}`, `{"s":",}"}`},
		{"non-trailing commas survive", `{"a":1,"b":2}`, `{"a":1,"b":2}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(Strip([]byte(c.in))); got != c.want {
				t.Fatalf("Strip(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestStripDecodes confirms Strip's output on a devcontainer.json-shaped file
// with comments and trailing commas is valid JSON the stdlib decoder accepts.
func TestStripDecodes(t *testing.T) {
	in := []byte("{\n  // image to use\n  \"image\": \"x\",\n  \"mounts\": [\n    \"a\", /* inline */\n    \"b\",\n  ],\n}")
	var out struct {
		Image  string   `json:"image"`
		Mounts []string `json:"mounts"`
	}
	if err := json.Unmarshal(Strip(in), &out); err != nil {
		t.Fatalf("Unmarshal after Strip: %v", err)
	}
	if out.Image != "x" || len(out.Mounts) != 2 {
		t.Fatalf("decoded %+v, want {Image:x Mounts:[a b]}", out)
	}
}

// TestStripCommentsDecodes confirms the stripped output is valid JSON the
// stdlib decoder accepts, which is the whole point of the helper.
func TestStripCommentsDecodes(t *testing.T) {
	in := []byte("{\n  // name of the thing\n  \"name\": \"x\", /* inline */ \"n\": 2\n}")
	var out struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	if err := json.Unmarshal(StripComments(in), &out); err != nil {
		t.Fatalf("Unmarshal after strip: %v", err)
	}
	if out.Name != "x" || out.N != 2 {
		t.Fatalf("decoded %+v, want {Name:x N:2}", out)
	}
}
