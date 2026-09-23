package devcontainer

import (
	"reflect"
	"testing"
)

func parseT(t *testing.T, src string, env map[string]string) Config {
	t.Helper()
	cfg, err := parse([]byte(src), func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cfg
}

func TestParseFullConfig(t *testing.T) {
	cfg := parseT(t, `{
	  // the sandbox image
	  "name": "Sandbox",
	  "image": "example/dev@sha256:abc",
	  /* personal bind mounts are dropped,
	     named volumes are kept */
	  "mounts": [
	    "source=${localEnv:HOME}/.claude,target=/home/dev/.claude,type=bind",
	    "source=devc-go-mod,target=${localEnv:REMOTE_HOME:/home/dev}/go/pkg/mod,type=volume",
	    {"source": "devc-npm", "target": "/home/dev/.npm", "type": "volume"},
	    {"source": "devc-ws", "target": "${localWorkspaceFolder}/x", "type": "volume"},
	  ],
	  "remoteUser": "${localEnv:REMOTE_USER:dev}",
	  "runArgs": ["--cap-add=NET_ADMIN"],
	}`, map[string]string{"HOME": "/home/me"})

	want := Config{
		Image: "example/dev@sha256:abc",
		Mounts: []Mount{
			{Source: "devc-go-mod", Target: "/home/dev/go/pkg/mod"},
			{Source: "devc-npm", Target: "/home/dev/.npm"},
		},
		Home: "/home/dev",
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("cfg = %+v, want %+v", cfg, want)
	}
}

func TestParseLocalEnvPrecedence(t *testing.T) {
	cfg := parseT(t, `{"image": "img", "remoteUser": "${localEnv:U:dev}"}`,
		map[string]string{"U": "alice"})
	if cfg.Home != "/home/alice" {
		t.Fatalf("home = %q, want env value over default", cfg.Home)
	}
}

func TestParseRootUser(t *testing.T) {
	if got := parseT(t, `{"image": "img", "remoteUser": "root"}`, nil).Home; got != "/root" {
		t.Fatalf("home = %q", got)
	}
}

func TestParseContainerUserFallback(t *testing.T) {
	if got := parseT(t, `{"image": "img", "containerUser": "node"}`, nil).Home; got != "/home/node" {
		t.Fatalf("home = %q", got)
	}
}

// A Dockerfile-based devcontainer has no image to run steps in — the caller
// falls back to the host, so this is a zero Config, not an error.
func TestParseDockerfileBuildIsUnusable(t *testing.T) {
	cfg := parseT(t, `{"build": {"dockerfile": "Dockerfile"}, "remoteUser": "dev"}`, nil)
	if cfg.Image != "" || cfg.Home != "" {
		t.Fatalf("dockerfile config should be unusable, got %+v", cfg)
	}
}

func TestParseUnresolvedImageIsUnusable(t *testing.T) {
	cfg := parseT(t, `{"image": "${localEnv:MISSING_IMG}"}`, nil)
	if cfg.Image != "" {
		t.Fatalf("unresolved image should be unusable, got %q", cfg.Image)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	if _, err := parse([]byte(`{"image": `), func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("want parse error")
	}
}

func TestParseTrailingCommaBeforeCommentedCloser(t *testing.T) {
	cfg := parseT(t, "{\n\"image\": \"img\", // note\n}", nil)
	if cfg.Image != "img" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestParseCommentInsideString(t *testing.T) {
	cfg := parseT(t, `{"image": "reg//img", "mounts": ["source=v1,target=/a/*b,type=volume"]}`, nil)
	if cfg.Image != "reg//img" {
		t.Fatalf("image = %q — comment stripping ate a string", cfg.Image)
	}
	if len(cfg.Mounts) != 1 || cfg.Mounts[0].Target != "/a/*b" {
		t.Fatalf("mounts = %+v", cfg.Mounts)
	}
}

func TestVolumeMountValidation(t *testing.T) {
	cfg := parseT(t, `{"image": "img", "mounts": [
	  "source=ok.vol-1,target=/x,type=volume",
	  "source = sp.vol , target = /sp , type = volume",
	  "source=bad name,target=/x,type=volume",
	  "source=rel,target=notabs,type=volume",
	  "source=tmpfsy,target=/y,type=tmpfs",
	  42
	]}`, nil)
	want := []Mount{{Source: "ok.vol-1", Target: "/x"}, {Source: "sp.vol", Target: "/sp"}}
	if !reflect.DeepEqual(cfg.Mounts, want) {
		t.Fatalf("mounts = %+v, want %+v", cfg.Mounts, want)
	}
}
