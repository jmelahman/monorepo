package server

import "testing"

func TestParseReservedUpstreams(t *testing.T) {
	t.Run("valid, trimmed, case-folded", func(t *testing.T) {
		got, err := parseReservedUpstreams([]string{" App = 127.0.0.1:3100 ", "docs=10.0.0.5:80", ""})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got["app"] != "127.0.0.1:3100" || got["docs"] != "10.0.0.5:80" {
			t.Fatalf("got %#v", got)
		}
	})

	for _, tc := range []struct{ name, in string }{
		{"no separator", "apponly"},
		{"empty label", "=127.0.0.1:80"},
		{"empty addr", "app="},
		{"dotted label", "app.preview=127.0.0.1:80"},
	} {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if _, err := parseReservedUpstreams([]string{tc.in}); err == nil {
				t.Fatalf("expected error for %q", tc.in)
			}
		})
	}

	t.Run("rejects duplicate label", func(t *testing.T) {
		if _, err := parseReservedUpstreams([]string{"app=127.0.0.1:1", "app=127.0.0.1:2"}); err == nil {
			t.Fatal("expected error on duplicate label")
		}
	})

	t.Run("empty input yields empty map", func(t *testing.T) {
		got, err := parseReservedUpstreams(nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("got %#v, %v", got, err)
		}
	})
}
