//go:build embed

package api_test

import (
	"strings"
	"testing"
)

func TestStaticIndexEmbedded(t *testing.T) {
	e := newEnv(t)
	resp := e.get("/")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %q; want text/html", ct)
	}
}
