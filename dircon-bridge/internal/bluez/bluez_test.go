package bluez_test

import (
	"errors"
	"testing"

	"github.com/jmelahman/dircon-bridge/internal/bluez"
)

func TestParseAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "canonical", in: "D6:F8:D9:7F:99:4B", want: "D6:F8:D9:7F:99:4B", ok: true},
		{name: "lower case", in: "d6:f8:d9:7f:99:4b", want: "D6:F8:D9:7F:99:4B", ok: true},
		{name: "dashes", in: "D6-F8-D9-7F-99-4B", want: "D6:F8:D9:7F:99:4B", ok: true},
		{name: "too short", in: "D6:F8:D9:7F:99"},
		{name: "too long", in: "D6:F8:D9:7F:99:4B:00"},
		{name: "not hex", in: "G6:F8:D9:7F:99:4B"},
		{name: "no separators", in: "D6F8D97F994B"},
		{name: "path injection", in: "D6:F8:D9:7F:99:4B/x"},
		{name: "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := bluez.ParseAddress(tt.in)
			if !tt.ok {
				if !errors.Is(err, bluez.ErrBadAddress) {
					t.Fatalf("ParseAddress(%q) error = %v, want ErrBadAddress", tt.in, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ParseAddress(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
}
