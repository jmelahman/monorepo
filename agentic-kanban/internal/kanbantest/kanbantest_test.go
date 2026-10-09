package kanbantest

import (
	"net/http"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	IsolateEnv()
	os.Exit(m.Run())
}

func TestNewServesTheAPI(t *testing.T) {
	env := New(t)
	resp, err := http.Get(env.Server.URL + "/api/boards")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/boards = %d, want 200", resp.StatusCode)
	}
}
