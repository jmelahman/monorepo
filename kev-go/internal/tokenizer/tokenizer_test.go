package tokenizer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

var specialRE = regexp.MustCompile(`<\|([A-Za-z0-9_]+)\|>`)

// Compares against ids from the Python tokenizer. KEV_CHECKPOINT is a Kev
// checkpoint directory; tokenizer.json is too big to keep in this repo.
func TestMatchesPython(t *testing.T) {
	dir := os.Getenv("KEV_CHECKPOINT")
	if dir == "" {
		t.Skip("KEV_CHECKPOINT isn't set")
	}
	tok, err := Open(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Special map[string]int32
		Cases   []struct {
			Text string
			IDs  []int32
		}
	}
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatal(err)
	}
	for name, want := range fx.Special {
		if got, ok := tok.ID(name); !ok || got != want {
			t.Errorf("ID(%q) = %d, %v, want %d", name, got, ok, want)
		}
	}
	for _, c := range fx.Cases {
		got := tok.Encode(specialRE.ReplaceAllString(c.Text, "<¦$1¦>"))
		if !slices.Equal(got, c.IDs) && len(got)+len(c.IDs) > 0 {
			t.Errorf("Encode(%q)\n got %v\nwant %v", c.Text, got, c.IDs)
		}
	}
}

func TestNextPiece(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"don't", []string{"don", "'t"}},
		{"(foo bar", []string{"(foo", " bar"}},
		{"a  b", []string{"a", " ", " b"}},
		{"a  ", []string{"a", "  "}},
		{"x\n\n  y", []string{"x", "\n\n", " ", " y"}},
		{"12", []string{"1", "2"}},
		{" ...\n", []string{" ...\n"}},
	} {
		var got []string
		for s := c.in; s != ""; {
			n := nextPiece(s)
			got = append(got, s[:n])
			s = s[n:]
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("pieces of %q = %q, want %q", c.in, got, c.want)
		}
	}
}
