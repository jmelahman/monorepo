// Tests live in package greeter_test so they exercise the package exactly
// the way a caller does. Use an internal test (package greeter) only for
// things the public API genuinely cannot reach.
package greeter_test

import (
	"errors"
	"testing"

	"github.com/jmelahman/golang-template/greeter"
)

func TestGreet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
		opts    []greeter.Option
	}{
		{
			name:  "default greeting",
			input: "world",
			want:  "Hello, world!",
		},
		{
			name:  "custom greeting",
			input: "world",
			want:  "Howdy, world!",
			opts:  []greeter.Option{greeter.WithGreeting("Howdy")},
		},
		{
			name:  "shouted",
			input: "world",
			want:  "HELLO, WORLD!",
			opts:  []greeter.Option{greeter.WithShout()},
		},
		{
			name:  "surrounding whitespace trimmed",
			input: "  world\n",
			want:  "Hello, world!",
		},
		{
			name:    "blank name",
			input:   "   ",
			wantErr: greeter.ErrEmptyName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := greeter.New(tt.opts...).Greet(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Greet(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Greet(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
