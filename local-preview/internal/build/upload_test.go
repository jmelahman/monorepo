package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/local-preview/internal/db"
)

// tarGzBytes builds an in-memory gzip-compressed tar from name→content.
func tarGzBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func upload(t *testing.T, e *env, ref, side, name string, body []byte, overwrite bool) (UploadResult, error) {
	t.Helper()
	return e.q.Upload(context.Background(), "demo", ref, side, name, bytes.NewReader(body), overwrite)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func assertNoLog(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("build log %s exists — the side was built, not served from the upload", path)
	}
}

func appendArtifactSection(t *testing.T, src string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(src, "preview.toml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(fixtureArtifactSection); err != nil {
		t.Fatal(err)
	}
	f.Close()
	runTestGit(t, src, "commit", "-qam", "declare cli artifact")
}

// A prebuilt frontend uploaded for a commit lands in the exact content-address
// a build would target, so deploying the commit reuses it and never runs the
// frontend build.
func TestUploadFrontendServedWithoutBuild(t *testing.T) {
	src := newFixtureRepo(t)
	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })

	res, err := upload(t, e, "main", SideFrontend, "", tarGzBytes(t, map[string]string{"index.html": "UPLOADED"}), false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Published || res.Hash == "" || res.Side != SideFrontend {
		t.Fatalf("upload result = %+v", res)
	}
	if !e.files.HasFrontend("demo", res.Hash) {
		t.Fatal("frontend not published by upload")
	}
	if got := readFile(t, filepath.Join(e.files.FrontendDir("demo", res.Hash), "index.html")); got != "UPLOADED" {
		t.Fatalf("published frontend = %q, want the uploaded bytes", got)
	}

	a := e.deployAndWait(t, "main")
	if a.FeHash != res.Hash {
		t.Fatalf("deploy fe_hash = %s, want the uploaded hash %s", a.FeHash, res.Hash)
	}
	assertNoLog(t, a.FeBuildLogPath)
	if got := readFile(t, filepath.Join(e.files.FrontendDir("demo", res.Hash), "index.html")); got != "UPLOADED" {
		t.Fatalf("frontend content changed to %q after deploy — the build was not skipped", got)
	}
}

// A prebuilt backend tree uploaded for a commit is served without building, yet
// the deploy still provisions its state dir (build.go wires that regardless of
// whether the artifact was built or uploaded).
func TestUploadBackendServedWithoutBuild(t *testing.T) {
	src := newFixtureRepo(t)
	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })

	body := tarGzBytes(t, map[string]string{"bin/fixture-server": "#!/bin/sh\n", "marker": "BACKEND-UPLOAD"})
	res, err := upload(t, e, "main", SideBackend, "", body, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Published || !e.files.HasBackend("demo", res.Hash) {
		t.Fatalf("backend not published by upload: %+v", res)
	}

	a := e.deployAndWait(t, "main")
	if a.BeHash != res.Hash {
		t.Fatalf("deploy be_hash = %s, want the uploaded hash %s", a.BeHash, res.Hash)
	}
	assertNoLog(t, a.BeBuildLogPath)
	if !e.files.HasStateDir("demo", res.Hash) {
		t.Fatal("state dir not provisioned for the uploaded backend")
	}
}

// A prebuilt downloadable artifact uploaded for a commit is ready the moment
// the deploy knows its hash — no artifact build phase runs.
func TestUploadArtifactServedWithoutBuild(t *testing.T) {
	src := newFixtureRepo(t)
	appendArtifactSection(t, src)
	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })

	res, err := upload(t, e, "main", SideArtifact, "cli", tarGzBytes(t, map[string]string{"bin/fixture-cli": "UPLOADED-CLI"}), false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Published || len(res.Files) != 1 || res.Files[0].Name != "fixture-cli" {
		t.Fatalf("artifact upload result = %+v", res)
	}
	if got := readFile(t, filepath.Join(e.files.ArtifactDir("demo", res.Hash), "fixture-cli")); got != "UPLOADED-CLI" {
		t.Fatalf("published artifact = %q", got)
	}

	a := e.deployAndWait(t, "main")
	ref := a.Artifacts["cli"]
	if ref.Hash != res.Hash || ref.Status != db.ArtifactReady {
		t.Fatalf("uploaded artifact ref = %+v, want ready at hash %s", ref, res.Hash)
	}
	assertNoLog(t, ref.LogPath)
}

func TestUploadArtifactErrors(t *testing.T) {
	src := newFixtureRepo(t)
	appendArtifactSection(t, src)
	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })

	// A name the manifest doesn't declare as an artifact.
	if _, err := upload(t, e, "main", SideArtifact, "ghost", tarGzBytes(t, map[string]string{"x": "y"}), false); !errors.Is(err, ErrNoSuchArtifact) {
		t.Fatalf("unknown artifact err = %v, want ErrNoSuchArtifact", err)
	}
	// A tar missing the declared file.
	if _, err := upload(t, e, "main", SideArtifact, "cli", tarGzBytes(t, map[string]string{"wrong-name": "z"}), false); err == nil {
		t.Fatal("expected an error when the declared file is absent from the upload")
	}
}

// A second upload without overwrite is a no-op; overwrite replaces the bytes.
func TestUploadIdempotentAndOverwrite(t *testing.T) {
	src := newFixtureRepo(t)
	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })

	first, err := upload(t, e, "main", SideFrontend, "", tarGzBytes(t, map[string]string{"index.html": "V1"}), false)
	if err != nil || !first.Published {
		t.Fatalf("first upload = %+v, err %v", first, err)
	}
	path := filepath.Join(e.files.FrontendDir("demo", first.Hash), "index.html")

	second, err := upload(t, e, "main", SideFrontend, "", tarGzBytes(t, map[string]string{"index.html": "V2"}), false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Published || second.Hash != first.Hash {
		t.Fatalf("second upload should be a no-op reusing the hash: %+v", second)
	}
	if got := readFile(t, path); got != "V1" {
		t.Fatalf("no-op upload changed content to %q", got)
	}

	third, err := upload(t, e, "main", SideFrontend, "", tarGzBytes(t, map[string]string{"index.html": "V2"}), true)
	if err != nil || !third.Published {
		t.Fatalf("overwrite upload = %+v, err %v", third, err)
	}
	if got := readFile(t, path); got != "V2" {
		t.Fatalf("overwrite left content = %q, want V2", got)
	}
}

// An uploaded frontend is shared across every commit with the same frontend
// hash — a backend-only commit reuses it and never rebuilds the frontend.
func TestUploadFrontendSharedAcrossCommits(t *testing.T) {
	src := newFixtureRepo(t)
	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })

	res, err := upload(t, e, "main", SideFrontend, "", tarGzBytes(t, map[string]string{"index.html": "SHARED"}), false)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(src, "backend", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "backend-only change")

	b := e.deployAndWait(t, runTestGit(t, src, "rev-parse", "HEAD"))
	if b.FeHash != res.Hash {
		t.Fatalf("backend-only commit changed fe hash %s → %s; uploaded frontend not shared", res.Hash, b.FeHash)
	}
	assertNoLog(t, b.FeBuildLogPath)
}
