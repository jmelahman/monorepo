package pkgjson_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/vinder/internal/pkgjson"
)

func TestEdit(t *testing.T) {
	t.Parallel()

	const in = `{
    "name": "app",
    "dependencies": {"b": "^1.0.0", "a": "2.0.0"},
    "overrides": {"x": "1.0.0"},
    "private": true
}
`
	tests := []struct {
		edit func(f *pkgjson.File) error
		name string
		want string
	}{
		{
			name: "unchanged keeps order and indent",
			edit: func(*pkgjson.File) error { return nil },
			want: `{
    "name": "app",
    "dependencies": {
        "b": "^1.0.0",
        "a": "2.0.0"
    },
    "overrides": {
        "x": "1.0.0"
    },
    "private": true
}
`,
		},
		{
			name: "set existing in place",
			edit: func(f *pkgjson.File) error { return f.SetString("^1.1.0", "dependencies", "b") },
			want: `{
    "name": "app",
    "dependencies": {
        "b": "^1.1.0",
        "a": "2.0.0"
    },
    "overrides": {
        "x": "1.0.0"
    },
    "private": true
}
`,
		},
		{
			name: "set creates objects at the end",
			edit: func(f *pkgjson.File) error { return f.SetString("3.0.0", "pnpm", "overrides", "y") },
			want: `{
    "name": "app",
    "dependencies": {
        "b": "^1.0.0",
        "a": "2.0.0"
    },
    "overrides": {
        "x": "1.0.0"
    },
    "private": true,
    "pnpm": {
        "overrides": {
            "y": "3.0.0"
        }
    }
}
`,
		},
		{
			name: "delete prunes emptied objects",
			edit: func(f *pkgjson.File) error { return f.Delete("overrides", "x") },
			want: `{
    "name": "app",
    "dependencies": {
        "b": "^1.0.0",
        "a": "2.0.0"
    },
    "private": true
}
`,
		},
		{
			name: "delete missing is a no-op",
			edit: func(f *pkgjson.File) error { return f.Delete("resolutions", "x") },
			want: `{
    "name": "app",
    "dependencies": {
        "b": "^1.0.0",
        "a": "2.0.0"
    },
    "overrides": {
        "x": "1.0.0"
    },
    "private": true
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			file := filepath.Join(t.TempDir(), "package.json")
			if err := os.WriteFile(file, []byte(in), 0o644); err != nil {
				t.Fatal(err)
			}
			f, err := pkgjson.Read(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := tt.edit(f); err != nil {
				t.Fatal(err)
			}
			if err := f.Write(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("after edit:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestRead(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "package.json")
	if err := os.WriteFile(file, []byte(`{"private": true, "dependencies": {"a": "1.0.0", "b": {"bad": 1}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := pkgjson.Read(file)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := f.String("dependencies", "a"); !ok || got != "1.0.0" {
		t.Errorf(`String("dependencies", "a") = %q, %v; want "1.0.0", true`, got, ok)
	}
	if got := f.Strings("dependencies"); len(got) != 1 || got["a"] != "1.0.0" {
		t.Errorf(`Strings("dependencies") = %v, want only a`, got)
	}
	if _, ok := f.String("private"); ok {
		t.Error(`String("private") found a string in a bool`)
	}
	if err := f.SetString("x", "private", "y"); !errors.Is(err, pkgjson.ErrNotObject) {
		t.Errorf("SetString through a bool: error = %v, want %v", err, pkgjson.ErrNotObject)
	}
}
