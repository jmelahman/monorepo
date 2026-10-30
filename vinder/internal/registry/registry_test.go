package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestLatestBefore(t *testing.T) {
	t.Parallel()

	day := func(d int) time.Time { return time.Date(2026, time.January, d, 0, 0, 0, 0, time.UTC) }
	srv := registrytest.New(t,
		registrytest.Package{Name: "plain", Version: "1.0.0", Published: day(1)},
		registrytest.Package{Name: "plain", Version: "1.10.0", Published: day(3)},
		registrytest.Package{Name: "plain", Version: "1.9.0", Published: day(2)},
		registrytest.Package{Name: "plain", Version: "2.0.0-rc.1", Published: day(2), Tags: []string{"next"}},
		registrytest.Package{Name: "plain", Version: "2.0.0", Published: day(5)},
		// A release above latest is not one to move to, however old.
		registrytest.Package{Name: "held", Version: "1.0.0", Published: day(4), Tags: []string{"latest"}},
		registrytest.Package{Name: "held", Version: "1.1.0", Published: day(1), Tags: []string{"broken"}},
		registrytest.Package{Name: "held", Version: "0.9.0", Published: day(2)},
	)

	tests := []struct {
		cutoff  time.Time
		name    string
		pkg     string
		want    string
		wantErr bool
	}{
		{name: "latest old enough", pkg: "plain", cutoff: day(5), want: "2.0.0"},
		{name: "highest older than latest", pkg: "plain", cutoff: day(4), want: "1.10.0"},
		{name: "older still", pkg: "plain", cutoff: day(2), want: "1.9.0"},
		{name: "nothing old enough", pkg: "plain", cutoff: day(0), want: ""},
		{name: "not above latest", pkg: "held", cutoff: day(3), want: "0.9.0"},
		{name: "missing", pkg: "nope", cutoff: day(5), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := (&registry.Client{URL: srv.URL}).LatestBefore(context.Background(), tt.pkg, tt.cutoff)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LatestBefore(%q) error = %v, wantErr %v", tt.pkg, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("LatestBefore(%q) = %q, want %q", tt.pkg, got, tt.want)
			}
		})
	}
}

// TestLatestBeforeNoTimes refuses a registry that strips release times, which
// could otherwise pass off anything as old enough.
func TestLatestBeforeNoTimes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"dist-tags": {"latest": "1.0.0"}, "versions": {"1.0.0": {}}}`))
	}))
	t.Cleanup(srv.Close)

	if got, err := (&registry.Client{URL: srv.URL}).LatestBefore(context.Background(), "plain", time.Now()); err == nil {
		t.Errorf("LatestBefore() = %q, want an error", got)
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
