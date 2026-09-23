package slug

import (
	"strings"
	"testing"
)

func TestMakeShortInputUnchanged(t *testing.T) {
	if got, want := Make("Hello World", "x"), "hello-world"; got != want {
		t.Fatalf("Make = %q, want %q", got, want)
	}
}

func TestMakeTruncatesLongInput(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := Make(long, "x")
	if len(got) > MaxLen {
		t.Fatalf("Make(%q) len = %d, want <= %d", long, len(got), MaxLen)
	}
}

func TestMakeTrimsTrailingDashAfterTruncation(t *testing.T) {
	// 80th char would be a dash; truncation must strip it so the slug is a
	// valid git ref segment.
	in := strings.Repeat("a", MaxLen-1) + " bcd"
	got := Make(in, "x")
	if strings.HasSuffix(got, "-") {
		t.Fatalf("Make(%q) = %q, has trailing dash", in, got)
	}
	if len(got) > MaxLen {
		t.Fatalf("Make(%q) len = %d, want <= %d", in, len(got), MaxLen)
	}
}

func TestMakeCollapsesRunsOfOtherChars(t *testing.T) {
	// Runs of non-alphanumeric, non-[-_] characters collapse to a single dash.
	if got, want := Make("  Foo // Bar !! baz ", "x"), "foo-bar-baz"; got != want {
		t.Fatalf("Make = %q, want %q", got, want)
	}
}

func TestMakeDoesNotCollapseUnderscores(t *testing.T) {
	// Each '-'/'_' maps to its own dash and is NOT collapsed — preserving the
	// behavior of the original per-package slugify copies. Documented so a
	// future "tidy-up" doesn't silently change emitted slugs.
	if got, want := Make("a __ b", "x"), "a---b"; got != want {
		t.Fatalf("Make = %q, want %q", got, want)
	}
}

func TestMakeEmptyUsesFallback(t *testing.T) {
	if got, want := Make("!!!", "build-cop"), "build-cop"; got != want {
		t.Fatalf("Make = %q, want %q", got, want)
	}
}
