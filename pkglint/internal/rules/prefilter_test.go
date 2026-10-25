package rules

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// The hand-written prefilters stand in for regexps; each must agree with the
// regexp it replaced on every input.

var hexOnlyRe = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func prefilterInputs(alphabet string, n, maxLen int, seed int64) []string {
	r := rand.New(rand.NewSource(seed))
	out := make([]string, 0, n)
	for range n {
		b := make([]byte, r.Intn(maxLen))
		for i := range b {
			b[i] = alphabet[r.Intn(len(alphabet))]
		}
		out = append(out, string(b))
	}
	return out
}

func TestHasOnionHostMatchesRegexp(t *testing.T) {
	cases := []string{
		"", ".", ".onion", "a.onion", "abcdefghijklmnop.onion", "ABCDEFGHIJKLMNOP.ONION",
		"http://abcdefghijklmnop.onion/x", "x.y.abcdefghijklmnop.onion", "abcdefghijklmnop.onions",
		strings.Repeat("a", 56) + ".onion", strings.Repeat("a", 56) + ".Onion.", "a.b.c.d", "....onio", ".onio",
	}
	cases = append(cases, prefilterInputs("ab27.onion/ ON", 20000, 40, 1)...)
	cases = append(cases, prefilterInputs("abcdefghijklmnop.onion", 20000, 40, 2)...)
	for _, s := range cases {
		if got, want := hasOnionHost(s), onionHost.MatchString(s); got != want {
			t.Fatalf("hasOnionHost(%q) = %v, regexp says %v", s, got, want)
		}
	}
}

func TestBase64BlobMatchesRegexp(t *testing.T) {
	run := func(n int) string { return strings.Repeat("Ab+/", n/4+1)[:n] }
	cases := []string{
		"", run(119), run(120), run(121), run(120) + "=", run(120) + "==", run(120) + "===",
		run(60) + " " + run(60), run(100) + " " + run(130) + "==", "x=" + run(125), run(119) + "-" + run(120),
		"  " + run(300) + "== trailing", run(240) + "=\n",
	}
	cases = append(cases, prefilterInputs("Ab+/= -", 3000, 400, 3)...)
	cases = append(cases, prefilterInputs("Ab+/", 3000, 400, 4)...)
	for _, s := range cases {
		if got, want := base64Blob(s), base64BlobRe.FindString(s); got != want {
			t.Fatalf("base64Blob(%q) = %q, regexp says %q", s, got, want)
		}
	}
}

func TestIsHexMatchesRegexp(t *testing.T) {
	cases := []string{"", "0", "deadBEEF", "g", "dead beef", "dead\n", "\ndead", "0x1f"}
	cases = append(cases, prefilterInputs("0123456789abcdefABCDEFgG \n", 20000, 12, 5)...)
	for _, s := range cases {
		if got, want := isHex(s), hexOnlyRe.MatchString(s); got != want {
			t.Fatalf("isHex(%q) = %v, regexp says %v", s, got, want)
		}
	}
}
