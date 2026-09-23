package proxy

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// memTier is a minimal in-memory store.ArtifactTier for the proxy hydrate test.
type memTier struct {
	blobs map[string][]byte
}

func mkey(repo, side, hash string) string { return repo + "/" + side + "/" + hash }

func (m *memTier) putDir(t *testing.T, repo, side, hash, dir string) {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		if d.IsDir() {
			return tw.WriteHeader(&tar.Header{Name: name + "/", Mode: 0o755, Typeflag: tar.TypeDir})
		}
		info, _ := d.Info()
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: info.Size(), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	tw.Close()
	if m.blobs == nil {
		m.blobs = map[string][]byte{}
	}
	m.blobs[mkey(repo, side, hash)] = buf.Bytes()
}

func (m *memTier) Save(_ context.Context, repo, side, hash, srcDir string) error { return nil }

func (m *memTier) Open(_ context.Context, repo, side, hash string) (io.ReadCloser, int64, bool, error) {
	b, ok := m.blobs[mkey(repo, side, hash)]
	if !ok {
		return nil, 0, false, nil
	}
	return io.NopCloser(bytes.NewReader(b)), 0, true, nil
}

// A static frontend whose local files were swept by the cache sweeper (but whose
// deploy is still ready) must hydrate from the tier and serve — not 404.
func TestServeStaticHydratesEvictedFrontend(t *testing.T) {
	e := newTestEnv(t)
	tier := &memTier{}
	e.files.SetArtifactTier(tier)

	sha := "abcdef0123456789"
	d := e.readyDeploy(t, sha)
	feHash := "fe" + sha[:8]
	feDir := e.files.FrontendDir("demo", feHash)

	// Persist to the tier, then simulate a cache sweep removing the local copy.
	tier.putDir(t, "demo", "fe", feHash, feDir)
	if err := os.RemoveAll(feDir); err != nil {
		t.Fatal(err)
	}
	if e.files.HasFrontend("demo", feHash) {
		t.Fatal("precondition: local frontend should be gone")
	}

	host := d.ShortSHA + "-demo.preview.localhost:8080"
	code, body, _ := doReq(t, e.router, host, "/", false)
	if code != 200 {
		t.Fatalf("serve after eviction: status %d, want 200 (should hydrate, not 404)", code)
	}
	if !strings.Contains(body, "preview home") {
		t.Fatalf("served body = %q, want the hydrated index.html", body)
	}
	if !e.files.HasFrontend("demo", feHash) {
		t.Fatal("frontend was not hydrated back onto local disk")
	}
}
