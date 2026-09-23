package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmelahman/fullstack-template/internal/db"
)

func newTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return NewMux(Deps{Store: store, Build: BuildInfo{Version: "test"}})
}

func doJSON(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	mux := newTestMux(t)
	rec := doJSON(t, mux, "GET", "/api/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" || resp["version"] != "test" {
		t.Fatalf("unexpected body: %v", resp)
	}
}

func TestItemLifecycle(t *testing.T) {
	mux := newTestMux(t)

	rec := doJSON(t, mux, "POST", "/api/items", `{"title":"first"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", rec.Code, rec.Body)
	}
	var created db.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Title != "first" {
		t.Fatalf("unexpected item: %+v", created)
	}

	rec = doJSON(t, mux, "GET", "/api/items", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}
	var items []db.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	rec = doJSON(t, mux, "DELETE", "/api/items/1", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	rec = doJSON(t, mux, "DELETE", "/api/items/1", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404", rec.Code)
	}
}

func TestCreateItemValidation(t *testing.T) {
	mux := newTestMux(t)

	for name, body := range map[string]string{
		"empty title":      `{"title":"  "}`,
		"missing title":    `{}`,
		"malformed":        `{`,
		"wrong value type": `{"title":42}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, mux, "POST", "/api/items", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestUnknownAPIPathIs404(t *testing.T) {
	mux := newTestMux(t)
	rec := doJSON(t, mux, "GET", "/api/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
