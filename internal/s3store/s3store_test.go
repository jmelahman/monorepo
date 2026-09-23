package s3store

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// newTestTier connects to a real S3/MinIO endpoint described by env vars, or
// skips. Set PREVIEW_TEST_S3_ENDPOINT (host:port), PREVIEW_TEST_S3_BUCKET, and
// the access/secret keys; the bucket must already exist.
func newTestTier(t *testing.T) *Tier {
	t.Helper()
	endpoint := os.Getenv("PREVIEW_TEST_S3_ENDPOINT")
	bucket := os.Getenv("PREVIEW_TEST_S3_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("set PREVIEW_TEST_S3_ENDPOINT and PREVIEW_TEST_S3_BUCKET to run s3store integration tests")
	}
	tier, err := New(Config{
		Endpoint:  endpoint,
		Bucket:    bucket,
		Prefix:    "s3store-test",
		AccessKey: os.Getenv("PREVIEW_TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("PREVIEW_TEST_S3_SECRET_KEY"),
		UseSSL:    os.Getenv("PREVIEW_TEST_S3_USE_SSL") == "true",
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return tier
}

// writeTree materializes a small directory tree and returns its path.
func writeTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"index.html":    "<h1>hello</h1>",
		"sub/data.json": `{"k":"v"}`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSaveOpenRoundTrip(t *testing.T) {
	tier := newTestTier(t)
	ctx := context.Background()
	src := writeTree(t)

	if err := tier.Save(ctx, "demo", "fe", "roundtriphash", src); err != nil {
		t.Fatalf("save: %v", err)
	}

	rc, _, found, err := tier.Open(ctx, "demo", "fe", "roundtriphash")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !found {
		t.Fatal("expected the object to be found")
	}
	dst := t.TempDir()
	if err := extractTarTest(t, rc, dst); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("integrity check failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "sub", "data.json"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != `{"k":"v"}` {
		t.Fatalf("round-trip mismatch: %q", got)
	}
}

func TestOpenMissingReturnsNotFound(t *testing.T) {
	tier := newTestTier(t)
	rc, _, found, err := tier.Open(context.Background(), "demo", "fe", "definitelymissing")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if found {
		rc.Close()
		t.Fatal("expected found=false for a missing key")
	}
}

func TestSaveSourceGone(t *testing.T) {
	tier := newTestTier(t)
	err := tier.Save(context.Background(), "demo", "fe", "nosuchhash", filepath.Join(t.TempDir(), "does-not-exist"))
	if !errors.Is(err, ErrSourceGone) {
		t.Fatalf("expected ErrSourceGone, got %v", err)
	}
}

// extractTarTest untars a raw tar stream into destDir (test-local; the real
// hardened extractor lives in the build package).
func extractTarTest(t *testing.T, r io.Reader, destDir string) error {
	t.Helper()
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, filepath.FromSlash(hdr.Name))
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
}

// The deployed orchestrator authenticates as its EC2 instance role, so an
// unset keypair must resolve credentials from the environment rather than sign
// with an empty static one. The session token is the part that matters: role
// credentials are temporary, and a signature missing it is rejected.
func TestCredsForFallsBackToEnvironment(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ASIAROLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "rolesecret")
	t.Setenv("AWS_SESSION_TOKEN", "roletoken")

	got, err := credsFor(Config{}).Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AccessKeyID != "ASIAROLE" || got.SecretAccessKey != "rolesecret" {
		t.Errorf("keypair = %q/%q, want ASIAROLE/rolesecret", got.AccessKeyID, got.SecretAccessKey)
	}
	if got.SessionToken != "roletoken" {
		t.Errorf("session token = %q, want roletoken — a temporary credential signed without it is rejected", got.SessionToken)
	}
}

// An explicit keypair still wins, so a MinIO endpoint with no ambient identity
// is unaffected by the fallback above.
func TestCredsForPrefersExplicitKeypair(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ASIAROLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "rolesecret")

	got, err := credsFor(Config{AccessKey: "minio", SecretKey: "miniosecret"}).Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AccessKeyID != "minio" || got.SecretAccessKey != "miniosecret" {
		t.Errorf("keypair = %q/%q, want minio/miniosecret", got.AccessKeyID, got.SecretAccessKey)
	}
}

// Half a keypair is a typo. Falling back to the environment would authenticate
// as an identity the operator did not ask for, so it fails closed instead.
func TestNewRejectsHalfKeypair(t *testing.T) {
	for _, tc := range []struct{ name, access, secret string }{
		{"secret only", "", "s"},
		{"access only", "a", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(Config{
				Endpoint:  "s3.example.com",
				Bucket:    "b",
				AccessKey: tc.access,
				SecretKey: tc.secret,
			})
			if err == nil {
				t.Fatal("New succeeded, want error")
			}
		})
	}
}
