package rules

import (
	"regexp"
	"strings"
	"testing"
	"testing/quick"
)

// hostArchRe is the regexp hostArchIn replaced. It stays here as the
// specification: the scanner is an optimization, and the only thing that makes
// it safe is that it decides exactly what this pattern decides. It is built
// from hostArchAlt, the same alternation foreignTokenRe embeds.
var hostArchRe = regexp.MustCompile(`\b` + hostArchAlt + `\b`)

// TestHostArchRegexpCoversNames guards the two lists against drifting apart:
// the scanner reads hostArchNames, hostArchAlt spells the alternation out.
func TestHostArchRegexpCoversNames(t *testing.T) {
	for _, a := range hostArchNames {
		if got := hostArchRe.FindString(a); got != a {
			t.Errorf("hostArchRe does not match %q (got %q)", a, got)
		}
	}
	// And nothing the alternation matches is missing from the set.
	alts := strings.TrimSuffix(strings.TrimPrefix(hostArchAlt, "(?:"), ")")
	if alts == hostArchAlt {
		t.Fatalf("hostArchAlt %q is not the expected (?:…) group", hostArchAlt)
	}
	for alt := range strings.SplitSeq(alts, "|") {
		if !hostArchSet[alt] {
			t.Errorf("hostArchAlt alternative %q is not in hostArchNames", alt)
		}
	}
}

func TestHostArchInMatchesRegexp(t *testing.T) {
	cases := []string{
		"", "x86_64", "x86_64-linux-gnu", "foo_x86_64", "lib32-x86_64",
		"arm", "arm64", "armv7h", "armv6h-gnueabihf", "aarch64", "aarch",
		"/usr/lib/x86_64-linux-gnu/", "-DCMAKE_SYSTEM_PROCESSOR=aarch64",
		"i686 x86_64", "x86_64 i686", "no architecture here at all",
		"ppc", "ppc64le", "riscv64", "loong64", "loong",
		"_x86_64_", "x86_64_", "$CARCH", "${CARCH}",
		"日本語x86_64日本語", "café-aarch64", "x86_64\narm",
		"ARM", "X86_64", "arm-none-eabi-gcc",
	}
	for _, s := range cases {
		if got, want := hostArchIn(s), hostArchRe.FindString(s); got != want {
			t.Errorf("hostArchIn(%q) = %q, hostArchRe = %q", s, got, want)
		}
	}
}

// TestHostArchInMatchesRegexpQuick pushes the equivalence past the cases
// anybody thought to write down, over strings assembled from the bytes that
// decide it: word characters, separators, and an architecture name.
func TestHostArchInMatchesRegexpQuick(t *testing.T) {
	alphabet := []string{
		"x86_64", "arm", "armv7h", "aarch64", "i686", "ppc", "loong64", "riscv64",
		"_", "-", "/", " ", ".", "=", "a", "6", "4", "64", "v", "é", "\n",
	}
	f := func(idx []uint8) bool {
		var b strings.Builder
		for _, i := range idx {
			b.WriteString(alphabet[int(i)%len(alphabet)])
		}
		s := b.String()
		return hostArchIn(s) == hostArchRe.FindString(s)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 20000}); err != nil {
		t.Error(err)
	}
}
