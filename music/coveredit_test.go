package main

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPutCover(t *testing.T) {
	fakeCoverServices(t)
	music := t.TempDir()
	album := filepath.Join(music, "Ab-Soul", "Control System")
	if err := os.MkdirAll(album, 0o755); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	routeLibrary(mux, music, nil)
	put := func(query, contentType string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/cover?"+query, bytes.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	const q = "artist=Ab-Soul&album=Control+System"

	// An uploaded PNG is kept as a JPEG, and served from then on.
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if rec := put(q, "image/png", b.Bytes()); rec.Code != http.StatusNoContent {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	got, err := os.ReadFile(filepath.Join(album, "cover.jpg"))
	if err != nil || !bytes.HasPrefix(got, []byte{0xFF, 0xD8}) {
		t.Fatalf("cover.jpg after upload: % x, %v", got[:min(4, len(got))], err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cover?"+q, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("served %d, Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}

	// What isn't a picture is turned away, and the cover stays.
	if rec := put(q, "text/plain", []byte("hello")); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text: %d, want 415", rec.Code)
	}
	// The fake archive's "front cover" isn't a real JPEG either.
	if rec := put(q, "application/json", []byte(`{"releaseGroup":"right"}`)); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("release group: %d %s", rec.Code, rec.Body)
	}
	if rec := put(q, "application/json", []byte(`{"url":"file:///etc/passwd"}`)); rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "http") {
		t.Errorf("file link: %d %s", rec.Code, rec.Body)
	}
	if rec := put("artist=Ab-Soul&album=Nothing", "image/png", b.Bytes()); rec.Code != http.StatusNotFound {
		t.Errorf("missing album: %d", rec.Code)
	}
	if rec := put("artist=..&album=etc", "image/png", b.Bytes()); rec.Code != http.StatusNotFound {
		t.Errorf("escape: %d", rec.Code)
	}
	if after, _ := os.ReadFile(filepath.Join(album, "cover.jpg")); !bytes.Equal(after, got) {
		t.Error("a refused cover changed cover.jpg")
	}
}

func TestLoosely(t *testing.T) {
	if got := loosely("section 80 a"); got != "section 80 a section* 80*" {
		t.Errorf("loosely = %q", got)
	}
}

func TestWords(t *testing.T) {
	if got := words(`K_I_D_S CD2`); got != "k i d s" {
		t.Errorf("words = %q", got)
	}
	if got := words(`Fire & Ice (Deluxe) "x" AND:y`); got != "fire ice deluxe x and y" {
		t.Errorf("words = %q", got)
	}
}
