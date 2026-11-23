package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// id3Tag is an ID3v2.3 tag holding the given text frames, then a byte of
// audio.
func id3Tag(frames map[string]string) []byte {
	var body bytes.Buffer
	for _, id := range []string{"TIT2", "TCON", "TYER", "APIC"} {
		text, ok := frames[id]
		if !ok {
			continue
		}
		data := append([]byte{0}, text...)
		body.WriteString(id)
		_ = binary.Write(&body, binary.BigEndian, uint32(len(data)))
		body.Write([]byte{0, 0})
		body.Write(data)
	}
	n := body.Len()
	tag := make([]byte, 0, 11+n)
	tag = append(tag, 'I', 'D', '3', 3, 0, 0, byte(n>>21&0x7f), byte(n>>14&0x7f), byte(n>>7&0x7f), byte(n&0x7f))
	return append(append(tag, body.Bytes()...), 0xff)
}

func TestFileTags(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		frames map[string]string
		genre  string
		year   int
	}{
		{map[string]string{"TIT2": "Song", "APIC": strings.Repeat("x", 4000), "TCON": "Hip Hop", "TYER": "2019"}, "Hip Hop", 2019},
		{map[string]string{"TCON": "(17)"}, "(17)", 0},
		{map[string]string{"TYER": "1998-03-01"}, "", 1998},
	} {
		f := filepath.Join(dir, "t.mp3")
		if err := os.WriteFile(f, id3Tag(tc.frames), 0o644); err != nil {
			t.Fatal(err)
		}
		if g, y := fileTags(f); g != tc.genre || y != tc.year {
			t.Errorf("%v: got %q %d, want %q %d", tc.frames["TCON"], g, y, tc.genre, tc.year)
		}
	}
}

func TestNormalizeGenre(t *testing.T) {
	for in, want := range map[string]string{
		"Hip Hop": "hip-hop", "(17)": "rock", "(17)Indie": "indie", "13": "pop", " Drum  and Bass ": "drum & bass",
		"Other": "", "Electronica": "electronic", "Phonk": "phonk",
	} {
		if got := normalizeGenre(in); got != want {
			t.Errorf("normalizeGenre(%q) = %q, want %q", in, got, want)
		}
	}
	if got := genreTitle("drum & bass"); got != "Drum & Bass" {
		t.Errorf("genreTitle = %q", got)
	}
}

func TestLabelRules(t *testing.T) {
	root := library(t, "Air/Moon Safari/01 La Femme.mp3", "Air/Talkie/02 Cherry.mp3", "Beck/Sea/01 Golden.mp3")
	if err := os.WriteFile(filepath.Join(root, "Beck/Sea/01 Golden.mp3"), id3Tag(map[string]string{"TCON": "Rock", "TYER": "2002"}), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := newShelf(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l, err := labels{Genres: []string{"Electronica", "downtempo"}, Moods: []string{"Dreamy"}, Energy: "low"}.clean()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.setLabel("Air", "", l); err != nil {
		t.Fatal(err)
	}
	if err := s.setLabel("Air", "Talkie", labels{Moods: []string{"playful"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (labels{Moods: []string{"spooky"}}).clean(); err == nil {
		t.Error("an unknown mood passed")
	}
	artists, err := walkLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	all := flatten(artists)
	s.enrich(root, all)

	for _, tc := range []struct {
		r    rule
		want []string
	}{
		{rule{"genre", "is", "Electronica"}, []string{"01 La Femme", "02 Cherry"}},
		{rule{"genre", "is", "rock"}, []string{"01 Golden"}},
		{rule{"genre", "contains", "tempo"}, []string{"01 La Femme", "02 Cherry"}},
		{rule{"mood", "is", "dreamy"}, []string{"01 La Femme"}},
		{rule{"mood", "not", "dreamy"}, []string{"02 Cherry", "01 Golden"}},
		{rule{"energy", "is", "low"}, []string{"01 La Femme", "02 Cherry"}},
		{rule{"year", ">", "2000"}, []string{"01 Golden"}},
		{rule{"year", "<", "2000"}, nil},
	} {
		p := &playlist{ID: "p", Name: "x", Smart: &smart{Rules: []rule{tc.r}}}
		if err := p.check(); err != nil {
			t.Fatal(err)
		}
		if got := names(s.resolve(p, all, time.Now())); !slices.Equal(got, tc.want) {
			t.Errorf("%v: got %q, want %q", tc.r, got, tc.want)
		}
	}

	// Labels outlive a restart, and follow an album renamed.
	again, err := newShelf(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	again.mu.Lock()
	again.movedLabelsLocked(map[string]string{"Air/Talkie/02 Cherry.mp3": "Air/Talkie (Remastered)/02 Cherry.mp3"})
	again.mu.Unlock()
	if got := again.labelOf("Air", "Talkie (Remastered)").Moods; !slices.Equal(got, []string{"playful"}) {
		t.Errorf("moved album's moods = %q", got)
	}
}

// mcpReplyOf is a JSON-RPC reply as the tests read it.
type mcpReplyOf struct {
	Result struct {
		ProtocolVersion string            `json:"protocolVersion"`
		Tools           []json.RawMessage `json:"tools"`
		IsError         bool              `json:"isError"`
		Content         []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
}

func mcpCall(t *testing.T, h http.Handler, token, method string, params any) mcpReplyOf {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", method, rec.Code, rec.Body)
	}
	var out mcpReplyOf
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// toolResultOf is what the tools in these tests answer.
type toolResultOf struct {
	Saved   int      `json:"saved"`
	Refused []string `json:"refused"`
	Tracks  int      `json:"tracks"`
	Artists []struct {
		Artist string `json:"artist"`
	} `json:"artists"`
}

// toolText is a tool call's result, decoded, and whether it failed.
func toolText(t *testing.T, reply mcpReplyOf) (toolResultOf, bool) {
	t.Helper()
	var out toolResultOf
	if len(reply.Result.Content) != 1 {
		t.Fatalf("content %+v", reply.Result.Content)
	}
	if !reply.Result.IsError {
		if err := json.Unmarshal([]byte(reply.Result.Content[0].Text), &out); err != nil {
			t.Fatal(err)
		}
	}
	return out, reply.Result.IsError
}

func TestMCP(t *testing.T) {
	root := library(t, "Air/Moon Safari/01 La Femme.mp3", "Beck/Sea/01 Golden.mp3")
	s, err := newShelf(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token, err := mcpToken("", s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := mcpToken("", s.dir); again != token {
		t.Error("the token changed between starts")
	}
	mux := http.NewServeMux()
	routeMCP(mux, root, s, token)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: %d", rec.Code)
	}

	if v := mcpCall(t, mux, token, "initialize", map[string]any{"protocolVersion": "2025-06-18"}).Result.ProtocolVersion; v != "2025-06-18" {
		t.Errorf("version %v", v)
	}
	if tools := mcpCall(t, mux, token, "tools/list", nil).Result.Tools; len(tools) != 9 {
		t.Errorf("%d tools", len(tools))
	}

	out, isErr := toolText(t, mcpCall(t, mux, token, "tools/call", map[string]any{"name": "set_labels", "arguments": map[string]any{
		"labels": []any{
			map[string]any{"artist": "Air", "genres": []string{"Downtempo"}, "moods": []string{"dreamy"}},
			map[string]any{"artist": "Nobody", "genres": []string{"rock"}},
			map[string]any{"artist": "Beck", "moods": []string{"spooky"}},
		},
	}}))
	if isErr || out.Saved != 1 || len(out.Refused) != 2 {
		t.Errorf("set_labels: %v", out)
	}
	out, _ = toolText(t, mcpCall(t, mux, token, "tools/call", map[string]any{"name": "library_artists", "arguments": map[string]any{"unlabeled_only": true}}))
	if len(out.Artists) != 1 || out.Artists[0].Artist != "Beck" {
		t.Errorf("unlabeled artists: %v", out)
	}
	out, _ = toolText(t, mcpCall(t, mux, token, "tools/call", map[string]any{"name": "save_playlist", "arguments": map[string]any{
		"name": "Dreamy", "smart": map[string]any{"rules": []any{map[string]any{"field": "mood", "op": "is", "value": "dreamy"}}},
	}}))
	if out.Tracks != 1 {
		t.Errorf("save_playlist: %v", out)
	}
	if _, isErr := toolText(t, mcpCall(t, mux, token, "tools/call", map[string]any{"name": "save_playlist", "arguments": map[string]any{
		"name": "Bad", "paths": []string{"Nope/x.mp3"},
	}})); !isErr {
		t.Error("a playlist of a track not in the library was saved")
	}
}
