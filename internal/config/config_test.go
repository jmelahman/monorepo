package config

import "testing"

func TestResolvePreviewBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		baseURL string
		addr    string
		want    string
	}{
		{
			name: "defaults to the local preview domain and listen port",
			addr: ":8080",
			want: "http://abc1234-demo.preview.localhost:8080/",
		},
		{
			name:   "explicit domain keeps the listen port",
			domain: "preview.example.com",
			addr:   "127.0.0.1:9000",
			want:   "http://abc1234-demo.preview.example.com:9000/",
		},
		{
			name: "port 80 is implied by http",
			addr: ":80",
			want: "http://abc1234-demo.preview.localhost/",
		},
		{
			// The listen port is meaningless behind a TLS-terminating proxy:
			// the base URL, not the address, is what clients can reach.
			name:    "base URL overrides scheme, host, and port",
			baseURL: "https://preview.example.com",
			addr:    ":8080",
			want:    "https://abc1234-demo.preview.example.com/",
		},
		{
			name:    "base URL keeps a non-default port",
			baseURL: "https://preview.example.com:8443",
			addr:    ":8080",
			want:    "https://abc1234-demo.preview.example.com:8443/",
		},
		{
			name:    "base URL port 443 is implied by https",
			baseURL: "https://preview.example.com:443",
			want:    "https://abc1234-demo.preview.example.com/",
		},
		{
			name:    "a trailing path in the base URL is ignored",
			baseURL: "https://preview.example.com/",
			want:    "https://abc1234-demo.preview.example.com/",
		},
		{
			name:    "a matching domain alongside the base URL is fine",
			domain:  "preview.example.com",
			baseURL: "https://preview.example.com",
			want:    "https://abc1234-demo.preview.example.com/",
		},
		{
			name: "no addr means no port",
			want: "http://abc1234-demo.preview.localhost/",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base, err := ResolvePreviewBase(tc.domain, tc.baseURL, tc.addr)
			if err != nil {
				t.Fatal(err)
			}
			if got := base.URL("abc1234", "demo"); got != tc.want {
				t.Errorf("URL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolvePreviewBaseErrors(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		baseURL string
	}{
		{name: "no scheme", baseURL: "preview.example.com"},
		{name: "unsupported scheme", baseURL: "ftp://preview.example.com"},
		{name: "no host", baseURL: "https://"},
		{
			// Two different answers to "what host are previews on?" — better
			// to stop than to pick one silently.
			name:    "domain conflicts with base URL host",
			domain:  "preview.example.com",
			baseURL: "https://previews.other.com",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolvePreviewBase(tc.domain, tc.baseURL, ":8080"); err == nil {
				t.Error("ResolvePreviewBase succeeded, want an error")
			}
		})
	}
}

// The zero PreviewBase still produces a usable URL rather than "://host/".
func TestPreviewBaseURLZeroScheme(t *testing.T) {
	base := PreviewBase{Domain: "preview.localhost"}
	if got, want := base.URL("abc1234", "demo"), "http://abc1234-demo.preview.localhost/"; got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}

func TestLoadPreviewBaseFromEnv(t *testing.T) {
	t.Setenv("PREVIEW_DATA_DIR", t.TempDir())
	t.Setenv("PREVIEW_BASE_URL", "https://preview.example.com")
	cfg, err := Load(Options{Addr: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Preview.URL("abc1234", "demo"), "https://abc1234-demo.preview.example.com/"; got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}

// An explicit flag beats the environment, as everywhere else in Load.
func TestLoadPreviewBaseFlagOverridesEnv(t *testing.T) {
	t.Setenv("PREVIEW_DATA_DIR", t.TempDir())
	t.Setenv("PREVIEW_BASE_URL", "https://env.example.com")
	cfg, err := Load(Options{Addr: ":8080", PreviewBaseURL: "https://flag.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Preview.Domain, "flag.example.com"; got != want {
		t.Errorf("domain = %q, want %q", got, want)
	}
}
