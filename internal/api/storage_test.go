package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/retain"
)

func TestRetentionEndpoints(t *testing.T) {
	mux, _ := newTestMux(t)

	rec := doJSON(t, mux, "GET", "/api/retention", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get default policy: %d %s", rec.Code, rec.Body)
	}
	var p retain.Policy
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Enabled() {
		t.Fatalf("default policy should be disabled, got %s (%v)", rec.Body, err)
	}

	for _, body := range []string{`{"max_deploys_per_repo":-1}`, `{"max_age_days":-2}`, `{`} {
		if rec := doJSON(t, mux, "PUT", "/api/retention", body); rec.Code != http.StatusBadRequest {
			t.Errorf("PUT %s: %d, want 400", body, rec.Code)
		}
	}

	rec = doJSON(t, mux, "PUT", "/api/retention", `{"max_deploys_per_repo":5,"max_age_days":30}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put policy: %d %s", rec.Code, rec.Body)
	}
	rec = doJSON(t, mux, "GET", "/api/retention", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.MaxDeploysPerRepo != 5 || p.MaxAgeDays != 30 {
		t.Fatalf("policy did not round-trip: %+v", p)
	}
}

func TestStorageEndpoint(t *testing.T) {
	mux, _ := newTestMux(t)
	src := newSourceRepo(t)
	registerRepo(t, mux, "demo", src)

	rec := doJSON(t, mux, "GET", "/api/storage", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get storage: %d %s", rec.Code, rec.Body)
	}
	var s struct {
		TotalBytes  int64 `json:"total_bytes"`
		MirrorBytes int64 `json:"mirror_bytes"`
		Repos       []struct {
			Repo        string `json:"repo"`
			MirrorBytes int64  `json:"mirror_bytes"`
			TotalBytes  int64  `json:"total_bytes"`
		} `json:"repos"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Repos) != 1 || s.Repos[0].Repo != "demo" {
		t.Fatalf("repos: %s", rec.Body)
	}
	// The mirror clone holds the fixture commit, so it can't be empty.
	if s.Repos[0].MirrorBytes <= 0 || s.TotalBytes < s.Repos[0].TotalBytes {
		t.Fatalf("implausible sizes: %s", rec.Body)
	}
}

// deployRef requests a deploy of ref and waits until it is ready, returning
// its id, be_hash, and fe_hash.
func deployRef(t *testing.T, mux *http.ServeMux, ref string) (id int64, beHash, feHash string) {
	t.Helper()
	rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"`+ref+`"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create deploy: %d %s", rec.Code, rec.Body)
	}
	var d struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
		BeHash string `json:"be_hash"`
		FeHash string `json:"fe_hash"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for d.Status != "ready" {
		if d.Status == "failed" || time.Now().After(deadline) {
			logs := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d/logs", d.ID), "")
			t.Fatalf("deploy %d status = %s; logs:\n%s", d.ID, d.Status, logs.Body)
		}
		time.Sleep(50 * time.Millisecond)
		rec = doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d", d.ID), "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
	}
	return d.ID, d.BeHash, d.FeHash
}

func TestGCEndpointEvictsByPolicy(t *testing.T) {
	mux, root := newTestMux(t)
	src := newSourceRepo(t)
	registerRepo(t, mux, "demo", src)
	firstID, firstBe, _ := deployRef(t, mux, "main")

	// A second commit changing both sides produces fresh artifact hashes.
	if err := os.WriteFile(filepath.Join(src, "web", "index.html"), []byte("<html>v2</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "srv", "main.txt"), []byte("backend-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "v2")
	secondID, secondBe, _ := deployRef(t, mux, "main")
	if firstBe == secondBe {
		t.Fatalf("fixture commits share be_hash %q; the test needs distinct artifacts", firstBe)
	}

	// GC with retention disabled evicts nothing.
	rec := doJSON(t, mux, "POST", "/api/gc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("gc: %d %s", rec.Code, rec.Body)
	}
	var gc struct {
		Evicted    []retain.Evicted `json:"evicted"`
		FreedBytes int64            `json:"freed_bytes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &gc); err != nil {
		t.Fatal(err)
	}
	if len(gc.Evicted) != 0 {
		t.Fatalf("gc without a policy evicted %+v", gc.Evicted)
	}

	if rec := doJSON(t, mux, "PUT", "/api/retention", `{"max_deploys_per_repo":1}`); rec.Code != 200 {
		t.Fatalf("put policy: %d %s", rec.Code, rec.Body)
	}
	rec = doJSON(t, mux, "POST", "/api/gc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("gc: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &gc); err != nil {
		t.Fatal(err)
	}
	if len(gc.Evicted) != 1 || gc.Evicted[0].ID != firstID {
		t.Fatalf("want deploy %d evicted, got %s", firstID, rec.Body)
	}
	if gc.FreedBytes <= 0 {
		t.Errorf("eviction freed nothing: %s", rec.Body)
	}

	var d struct {
		Status string `json:"status"`
	}
	rec = doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d", firstID), "")
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Status != "evicted" {
		t.Fatalf("deploy %d status = %s, want evicted", firstID, d.Status)
	}

	// The evicted deploy's artifacts and logs are gone; the survivor's stay.
	if _, err := os.Stat(filepath.Join(root, "artifacts", "demo", "be", firstBe)); !os.IsNotExist(err) {
		t.Errorf("evicted be artifact still on disk (stat err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, "logs", "demo", "be", firstBe+".log")); !os.IsNotExist(err) {
		t.Errorf("evicted be build log still on disk (stat err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts", "demo", "be", secondBe)); err != nil {
		t.Errorf("surviving be artifact missing: %v", err)
	}
	if rec := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d", secondID), ""); rec.Code != 200 {
		t.Errorf("surviving deploy: %d", rec.Code)
	}
}
