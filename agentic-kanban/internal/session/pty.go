package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/jmelahman/kanban/internal/db"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: checkSameOrigin,
}

// checkSameOrigin rejects WebSocket upgrades whose Origin header is missing,
// unparseable, or points at a host other than the one serving the request.
// This blocks cross-site WebSocket hijacking (CWE-346) of the session PTY and
// shell endpoints, which would otherwise hand an attacker page an interactive
// shell inside the session container.
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

type ptyControl struct {
	Type string `json:"type"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
	Data string `json:"data,omitempty"`
}

// AttachAgent upgrades the request to a WebSocket and routes it through the
// per-session agent PTY broker, which holds the docker exec connection across
// client reconnects (e.g. page refresh). The command argument is the agent
// CLI argv for the session's harness, chosen by the caller; it must be
// non-empty. harnessID names that harness and is recorded on the broker so a
// later harness switch can tell whether the running agent needs replacing.
// Both are ignored when the agent is already running.
func (m *Manager) AttachAgent(ctx context.Context, sess *db.Session, w http.ResponseWriter, r *http.Request, harnessID string, command []string, workDir string) error {
	return m.attachKind(ctx, sess, w, r, "agent", harnessID, command, workDir)
}

// AttachShell upgrades the request to a WebSocket and runs an interactive
// shell inside the session container, brokered alongside (and independent of)
// the agent PTY. The container's user-configured login shell (as recorded in
// /etc/passwd, falling back to /bin/sh) is used so the choice tracks whatever
// the image declares — no need to hardcode bash here.
func (m *Manager) AttachShell(ctx context.Context, sess *db.Session, w http.ResponseWriter, r *http.Request, workDir string) error {
	return m.attachKind(ctx, sess, w, r, "shell", "", []string{"sh", "-c", `s=$(getent passwd "$(id -u)" 2>/dev/null | cut -d: -f7); exec "${s:-/bin/sh}"`}, workDir)
}

func (m *Manager) attachKind(ctx context.Context, sess *db.Session, w http.ResponseWriter, r *http.Request, kind, harnessID string, command []string, workDir string) error {
	if sess.ContainerID == nil || *sess.ContainerID == "" {
		http.Error(w, "session not running", http.StatusBadRequest)
		return errors.New("not running")
	}
	if len(command) == 0 {
		http.Error(w, "no command configured", http.StatusBadRequest)
		return errors.New("no command configured")
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	broker, err := m.brokers.attach(ctx, sess, kind, harnessID, command, workDir)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+err.Error()))
		return err
	}
	if err := broker.register(conn); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+err.Error()))
		return err
	}
	defer broker.unregister(ctx, conn)

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return nil
		}
		switch msgType {
		case websocket.TextMessage:
			var ctl ptyControl
			if err := json.Unmarshal(data, &ctl); err == nil && ctl.Type == "resize" {
				_ = broker.resize(ctx, conn, uint(ctl.Cols), uint(ctl.Rows))
				continue
			}
			if err := broker.write(conn, data); err != nil {
				return nil
			}
		case websocket.BinaryMessage:
			if err := broker.write(conn, data); err != nil {
				return nil
			}
		}
	}
}
