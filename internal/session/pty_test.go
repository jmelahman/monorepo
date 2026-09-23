package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestCheckSameOrigin(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"same host and port", "http://localhost:7474", "localhost:7474", true},
		{"https same host", "https://kanban.example.com", "kanban.example.com", true},
		{"case-insensitive host", "http://LocalHost:7474", "localhost:7474", true},
		{"cross-origin attacker", "https://evil.com", "localhost:7474", false},
		{"same hostname different port", "http://localhost:5173", "localhost:7474", false},
		{"missing origin header", "", "localhost:7474", false},
		{"malformed origin", "http://[bad", "localhost:7474", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/ws/sessions/1/shell", nil)
			r.Host = tc.host
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if got := checkSameOrigin(r); got != tc.want {
				t.Fatalf("checkSameOrigin(origin=%q host=%q) = %v, want %v", tc.origin, tc.host, got, tc.want)
			}
		})
	}
}

// TestUpgrader_RejectsCrossOrigin exercises the package-level upgrader end to
// end: a real WebSocket handshake with Origin: https://evil.com must be
// rejected with 403 before the handler can touch the session container.
func TestUpgrader_RejectsCrossOrigin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mirror the production path: rely on the package upgrader to enforce origin.
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	hdr := http.Header{}
	hdr.Set("Origin", "https://evil.com")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("expected handshake to fail with cross-origin request")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response on failed upgrade, got nil (err=%v)", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("upgrade status = %d, want 403", resp.StatusCode)
	}
}

func TestUpgrader_AllowsSameOrigin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	hdr := http.Header{}
	hdr.Set("Origin", "http"+strings.TrimPrefix(srv.URL, "http"))

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatalf("same-origin upgrade failed: err=%v resp=%v", err, resp)
	}
	_ = conn.Close()
}
