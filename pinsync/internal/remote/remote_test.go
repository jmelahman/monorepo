package remote_test

import (
	"errors"
	"testing"

	"github.com/jmelahman/pinsync/internal/remote"
)

func TestParse(t *testing.T) {
	t.Parallel()

	const ref = "refs/tags/v1.0.0"
	tests := []struct {
		wantErr error
		name    string
		out     string
		want    string
	}{
		{name: "lightweight tag", out: "aaa\t" + ref + "\n", want: "aaa"},
		{name: "annotated tag peels to its commit", out: "aaa\t" + ref + "\nbbb\t" + ref + "^{}\n", want: "bbb"},
		{name: "no tag", out: "", wantErr: remote.ErrNoTag},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := remote.Parse(tt.out, ref, "https://example.com/repo", "v1.0.0")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Parse() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Parse() = %q, want %q", got, tt.want)
			}
		})
	}
}
