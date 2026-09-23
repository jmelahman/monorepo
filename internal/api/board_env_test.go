package api_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type envResp struct {
	Keys []string `json:"keys"`
}

func TestBoardEnv_GetEmpty(t *testing.T) {
	e := newEnv(t)
	b := e.seedBoard("Env Board")

	resp := e.get(fmt.Sprintf("/api/boards/%d/env", b.ID))
	assertStatus(t, resp, 200)
	body := readBody(t, resp)
	// Contract: keys is always a JSON array, never null.
	if !strings.Contains(string(body), `"keys":[]`) {
		t.Fatalf("empty env body = %s; want keys:[]", body)
	}
}

func TestBoardEnv_SetListUnset(t *testing.T) {
	e := newEnv(t)
	b := e.seedBoard("Env Board")
	path := fmt.Sprintf("/api/boards/%d/env", b.ID)

	resp := e.patch(path, map[string]any{"set": map[string]string{"MY_API_KEY": "s3cret", "OTHER": "x"}})
	assertStatus(t, resp, 200)
	got := decodeJSON[envResp](t, resp)
	if want := []string{"MY_API_KEY", "OTHER"}; !reflect.DeepEqual(got.Keys, want) {
		t.Fatalf("keys = %v; want %v", got.Keys, want)
	}

	resp = e.get(path)
	assertStatus(t, resp, 200)
	got = decodeJSON[envResp](t, resp)
	if want := []string{"MY_API_KEY", "OTHER"}; !reflect.DeepEqual(got.Keys, want) {
		t.Fatalf("keys = %v; want %v", got.Keys, want)
	}

	// Unset one key; unsetting a missing key is idempotent and fine.
	resp = e.patch(path, map[string]any{"unset": []string{"OTHER", "NEVER_EXISTED"}})
	assertStatus(t, resp, 200)
	got = decodeJSON[envResp](t, resp)
	if want := []string{"MY_API_KEY"}; !reflect.DeepEqual(got.Keys, want) {
		t.Fatalf("keys = %v; want %v", got.Keys, want)
	}
}

// TestBoardEnv_ValuesNeverReturned is the write-only contract: no API
// response may contain a stored value, in any field.
func TestBoardEnv_ValuesNeverReturned(t *testing.T) {
	e := newEnv(t)
	b := e.seedBoard("Env Board")
	path := fmt.Sprintf("/api/boards/%d/env", b.ID)
	const secret = "hunter2-super-secret"

	resp := e.patch(path, map[string]any{"set": map[string]string{"MY_API_KEY": secret}})
	assertStatus(t, resp, 200)

	for _, probe := range []string{
		path,
		fmt.Sprintf("/api/boards/%d", b.ID),
		fmt.Sprintf("/api/boards/%d/state", b.ID),
		"/api/boards",
	} {
		resp := e.get(probe)
		body := readBody(t, resp)
		if strings.Contains(string(body), secret) {
			t.Fatalf("GET %s leaked env var value: %s", probe, body)
		}
	}
}

func TestBoardEnv_Validation(t *testing.T) {
	e := newEnv(t)
	b := e.seedBoard("Env Board")
	path := fmt.Sprintf("/api/boards/%d/env", b.ID)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"reserved prefix", map[string]any{"set": map[string]string{"KANBAN_SESSION_ID": "x"}}},
		{"reserved prefix lowercase", map[string]any{"set": map[string]string{"kanban_api_url": "x"}}},
		{"invalid key digits", map[string]any{"set": map[string]string{"123BAD": "x"}}},
		{"invalid key equals", map[string]any{"set": map[string]string{"BAD=KEY": "x"}}},
		{"invalid key empty", map[string]any{"set": map[string]string{"": "x"}}},
		{"invalid unset key", map[string]any{"unset": []string{"BAD KEY"}}},
		{"empty patch", map[string]any{}},
	}
	for _, tc := range cases {
		resp := e.patch(path, tc.body)
		if resp.StatusCode != 400 {
			body := readBody(t, resp)
			t.Fatalf("%s: status = %d; want 400. body: %s", tc.name, resp.StatusCode, body)
		}
		resp.Body.Close()
	}

	// Nothing should have been stored by the rejected patches.
	resp := e.get(path)
	got := decodeJSON[envResp](t, resp)
	if len(got.Keys) != 0 {
		t.Fatalf("keys after rejected patches = %v; want empty", got.Keys)
	}
}

func TestBoardEnv_BoardNotFound(t *testing.T) {
	e := newEnv(t)

	resp := e.get("/api/boards/9999/env")
	assertStatus(t, resp, 404)
	resp.Body.Close()

	resp = e.patch("/api/boards/9999/env", map[string]any{"set": map[string]string{"K": "v"}})
	assertStatus(t, resp, 404)
	resp.Body.Close()
}
