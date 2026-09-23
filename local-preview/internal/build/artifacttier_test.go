package build

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// tarDirRaw tars a directory's contents (relative paths, no compression) — the
// minimum the fake tier needs so Open can reproduce the tree via extractTar.
// Returns a non-nil error if the source path doesn't exist.
func tarDirRaw(srcDir string) ([]byte, error) {
	root, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == "." {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if d.IsDir() {
			return tw.WriteHeader(&tar.Header{Name: name + "/", Mode: 0o755, Typeflag: tar.TypeDir})
		}
		if !info.Mode().IsRegular() {
			return nil
		}
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
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fakeTier is an in-memory ArtifactTier. Save tars the source dir so Open can
// reproduce it, and records every key it was asked to persist.
type fakeTier struct {
	mu    sync.Mutex
	blobs map[string][]byte
	saved []string
	// saveDelay stalls every Save, widening the window in which an async
	// persist has not yet finished — so a test can prove a guarantee comes
	// from a synchronous wait, not from the pool happening to win the race.
	saveDelay time.Duration
}

func fkey(repo, side, hash string) string { return repo + "/" + side + "/" + hash }

func (f *fakeTier) Save(_ context.Context, repo, side, hash, srcDir string) error {
	if f.saveDelay > 0 {
		time.Sleep(f.saveDelay)
	}
	blob, err := tarDirRaw(srcDir)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, fkey(repo, side, hash))
	if err == nil {
		if f.blobs == nil {
			f.blobs = map[string][]byte{}
		}
		f.blobs[fkey(repo, side, hash)] = blob
	}
	return err
}

func (f *fakeTier) Open(_ context.Context, repo, side, hash string) (io.ReadCloser, int64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.blobs[fkey(repo, side, hash)]
	if !ok {
		return nil, 0, false, nil
	}
	return io.NopCloser(bytes.NewReader(b)), 0, true, nil
}

func (f *fakeTier) has(repo, side, hash string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.blobs[fkey(repo, side, hash)]
	return ok
}

func (f *fakeTier) savedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.saved...)
}

// countingRunner counts build-step invocations, delegating the actual run.
type countingRunner struct {
	delegate Runner
	mu       sync.Mutex
	n        int
}

func (c *countingRunner) Run(ctx context.Context, spec RunSpec, out io.Writer) error {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return c.delegate.Run(ctx, spec, out)
}

func (c *countingRunner) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestHydrateSkipsRebuild is the core of the durable tier: after an evicted
// deploy's local artifacts are gone, a re-request hydrates them from the tier
// and runs no build steps.
func TestHydrateSkipsRebuild(t *testing.T) {
	src := newFixtureRepo(t)
	tier := &fakeTier{}
	runner := &countingRunner{delegate: &autoRunner{}}
	e := newEnv(t, src, func(q *Queue) {
		q.runner = runner
		q.SetAutoStart(false) // keep the test about build/hydrate, not processes
		q.SetArtifactTier(tier)
	})

	row := e.deployAndWait(t, "main")
	if runner.count() == 0 {
		t.Fatal("expected build steps on the first deploy")
	}

	// The fe and be sides persist to the tier asynchronously after publish.
	waitUntil(t, "both sides persisted", func() bool {
		return tier.has("demo", "fe", row.FeHash) && tier.has("demo", "be", row.BeHash)
	})

	// Evict the deploy the way retention does: mark the row, delete local files.
	if err := e.db.SetDeployEvicted(row.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.files.RemoveFrontend("demo", row.FeHash); err != nil {
		t.Fatal(err)
	}
	if err := e.files.RemoveBackend("demo", row.BeHash); err != nil {
		t.Fatal(err)
	}
	if e.files.HasFrontend("demo", row.FeHash) || e.files.HasBackend("demo", row.BeHash) {
		t.Fatal("local artifacts should be gone after eviction")
	}

	before := runner.count()
	again := e.deployAndWait(t, row.SHA)
	if again.FeHash != row.FeHash || again.BeHash != row.BeHash {
		t.Fatalf("re-deploy changed hashes: fe %s→%s be %s→%s",
			row.FeHash, again.FeHash, row.BeHash, again.BeHash)
	}
	if built := runner.count() - before; built != 0 {
		t.Fatalf("expected no build steps after hydrate, ran %d", built)
	}
	if !e.files.HasFrontend("demo", row.FeHash) || !e.files.HasBackend("demo", row.BeHash) {
		t.Fatal("artifacts were not hydrated back into the local store")
	}
}

// TestPersistTargetsContentAddresses checks a build persists each side under
// the deploy's own content-address.
func TestPersistTargetsContentAddresses(t *testing.T) {
	src := newFixtureRepo(t)
	tier := &fakeTier{}
	e := newEnv(t, src, func(q *Queue) {
		q.SetAutoStart(false)
		q.SetArtifactTier(tier)
	})
	row := e.deployAndWait(t, "main")
	waitUntil(t, "both sides persisted", func() bool {
		return tier.has("demo", "fe", row.FeHash) && tier.has("demo", "be", row.BeHash)
	})
	for _, want := range []string{fkey("demo", "fe", row.FeHash), fkey("demo", "be", row.BeHash)} {
		found := false
		for _, k := range tier.savedKeys() {
			if k == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected persist of %q; saved=%v", want, tier.savedKeys())
		}
	}
}

// TestReadyImpliesDurable locks in the invariant that a deploy does not reach
// "ready" until its frontend and backend are in the durable tier. A worker
// serves by hydrating from the tier, so auto-start or a first visit would
// otherwise race the async persist and 502 with "not present in durable tier".
// The tier's Save is delayed so the async persist pool cannot win the race on
// its own — the guarantee has to come from the synchronous gate before ready.
func TestReadyImpliesDurable(t *testing.T) {
	src := newFixtureRepo(t)
	tier := &fakeTier{saveDelay: 300 * time.Millisecond}
	e := newEnv(t, src, func(q *Queue) {
		q.SetAutoStart(false)
		q.SetArtifactTier(tier)
	})

	// deployAndWait returns the instant the row flips to ready; both sides must
	// already be durable at that moment, with no waitUntil masking a race.
	row := e.deployAndWait(t, "main")
	if !tier.has("demo", "fe", row.FeHash) {
		t.Error("frontend not durable at ready")
	}
	if !tier.has("demo", "be", row.BeHash) {
		t.Error("backend not durable at ready")
	}
}

// TestPersistPoolDrainsOnStop verifies a buffered upload is not dropped when
// the queue stops — Stop drains in-flight and queued persist jobs.
func TestPersistPoolDrainsOnStop(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	root := t.TempDir()
	files := store.New(
		filepath.Join(root, "artifacts"),
		filepath.Join(root, "state"),
		filepath.Join(root, "tmp"),
	)
	super := supervise.New(database, files, filepath.Join(root, "logs"))
	t.Cleanup(super.StopAll)
	tier := &fakeTier{}
	q := NewQueue(database, gitrepo.NewManager(filepath.Join(root, "repos")), files, super, filepath.Join(root, "logs"), nil)
	q.SetArtifactTier(tier)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	q.Start(ctx, 0) // persist workers only, no build workers

	// A real published dir so the upload has something to tar.
	dir := files.FrontendDir("demo", "abcd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	q.enqueuePersist("demo", "fe", "abcd")
	q.Stop()

	if keys := tier.savedKeys(); len(keys) != 1 || keys[0] != fkey("demo", "fe", "abcd") {
		t.Fatalf("expected drained upload of demo/fe/abcd, got %v", keys)
	}
}

// TestUploadPersistsToTier: a CI upload lands in the durable tier just like a
// build's publish — a worker serving the freshly uploaded deploy hydrates
// immediately, not after the next reconcile pass.
func TestUploadPersistsToTier(t *testing.T) {
	src := newFixtureRepo(t)
	tier := &fakeTier{}
	e := newEnv(t, src, func(q *Queue) {
		q.SetAutoStart(false)
		q.SetArtifactTier(tier)
	})
	body := tarGzBytes(t, map[string]string{"marker": "UPLOADED"})
	res, err := upload(t, e, "main", SideBackend, "", body, false)
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "uploaded backend persisted", func() bool {
		return tier.has("demo", "be", res.Hash)
	})
}
