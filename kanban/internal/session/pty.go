package session

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/jmelahman/kanban/internal/db"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type ptyControl struct {
	Type string `json:"type"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
	Data string `json:"data,omitempty"`
}

// AttachClaude opens a PTY exec for the `claude` CLI inside the session container,
// and pipes IO to/from a websocket.
func (m *Manager) AttachClaude(ctx context.Context, sess *db.Session, w http.ResponseWriter, r *http.Request, command []string, workDir string) error {
	if sess.ContainerID == nil || *sess.ContainerID == "" {
		http.Error(w, "session not running", http.StatusBadRequest)
		return errors.New("not running")
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	if len(command) == 0 {
		command = []string{"claude"}
	}

	att, err := m.docker.ExecAttachTTY(ctx, *sess.ContainerID, command, workDir, nil)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+err.Error()))
		return err
	}
	defer att.Conn.Close()

	// Container → websocket.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := att.Conn.Reader.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					log.Printf("pty read: %v", err)
				}
				_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[session ended]\r\n"))
				return
			}
		}
	}()

	// Websocket → container (handles control frames for resize).
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return nil
		}
		switch msgType {
		case websocket.TextMessage:
			var ctl ptyControl
			if err := json.Unmarshal(data, &ctl); err == nil && ctl.Type == "resize" {
				_ = m.docker.ResizeExec(ctx, att.ID, uint(ctl.Cols), uint(ctl.Rows))
				continue
			}
			if _, err := att.Conn.Conn.Write(data); err != nil {
				return nil
			}
		case websocket.BinaryMessage:
			if _, err := att.Conn.Conn.Write(data); err != nil {
				return nil
			}
		}
	}
}
