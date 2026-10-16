package registry_test

import (
	"context"
	"testing"

	"github.com/jmelahman/vinder/internal/registry"
	"github.com/jmelahman/vinder/internal/registry/registrytest"
)

func TestLatest(t *testing.T) {
	t.Parallel()

	srv := registrytest.New(t,
		registrytest.Package{Name: "plain", Version: "1.0.0"},
		registrytest.Package{Name: "plain", Version: "1.10.0"},
		registrytest.Package{Name: "plain", Version: "1.9.0"},
		registrytest.Package{Name: "@scope/pkg", Version: "0.2.0"},
	)

	tests := []struct {
		name    string
		pkg     string
		want    string
		wantErr bool
	}{
		{name: "highest version", pkg: "plain", want: "1.10.0"},
		{name: "scoped", pkg: "@scope/pkg", want: "0.2.0"},
		{name: "missing", pkg: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := (&registry.Client{URL: srv.URL}).Latest(context.Background(), tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Latest(%q) error = %v, wantErr %v", tt.pkg, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Latest(%q) = %q, want %q", tt.pkg, got, tt.want)
			}
		})
	}
}
