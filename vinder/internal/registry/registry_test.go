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

func TestResolve(t *testing.T) {
	t.Parallel()

	srv := registrytest.New(t,
		registrytest.Package{Name: "plain", Version: "1.0.0"},
		registrytest.Package{Name: "plain", Version: "1.10.0"},
		registrytest.Package{Name: "plain", Version: "1.9.0"},
		registrytest.Package{Name: "plain", Version: "2.0.0"},
		registrytest.Package{Name: "plain", Version: "2.1.0-beta.1"},
		registrytest.Package{Name: "plain", Version: "3.0.0-rc.1", Tags: []string{"next"}},
		// npm installs latest for a range it satisfies, though 1.1.0 is higher.
		registrytest.Package{Name: "held", Version: "1.0.0", Tags: []string{"latest"}},
		registrytest.Package{Name: "held", Version: "1.1.0", Tags: []string{"broken"}},
	)

	tests := []struct {
		name    string
		pkg     string
		spec    string
		want    string
		wantErr bool
	}{
		{name: "latest", pkg: "plain", spec: "latest", want: "2.1.0-beta.1"},
		{name: "other tag", pkg: "plain", spec: "next", want: "3.0.0-rc.1"},
		{name: "major", pkg: "plain", spec: "1", want: "1.10.0"},
		{name: "minor", pkg: "plain", spec: "1.9", want: "1.9.0"},
		{name: "x range", pkg: "plain", spec: "1.x", want: "1.10.0"},
		{name: "any", pkg: "plain", spec: "*", want: "2.0.0"},
		{name: "latest wins a range", pkg: "held", spec: "1", want: "1.0.0"},
		{name: "unknown tag", pkg: "plain", spec: "nope", wantErr: true},
		{name: "no release in range", pkg: "plain", spec: "9", wantErr: true},
		{name: "missing package", pkg: "nope", spec: "latest", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := (&registry.Client{URL: srv.URL}).Resolve(context.Background(), tt.pkg, tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Resolve(%q, %q) error = %v, wantErr %v", tt.pkg, tt.spec, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q, %q) = %q, want %q", tt.pkg, tt.spec, got, tt.want)
			}
		})
	}
}
