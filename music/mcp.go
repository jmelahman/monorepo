package main

import (
	"cmp"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The MCP: the library as tools an agent can use, over MCP's streamable
// HTTP at /mcp. It reads the library with its plays and labels, labels
// artists and albums, and makes playlists. Every answer is one JSON
// reply; the server never starts a stream of its own.
//
// It can change the library's records, so it wants a bearer token: the
// MUSIC_MCP_TOKEN variable's, or one made on first start and kept in
// the state folder as mcp-token.

// mcpVersions are the protocol versions it speaks, newest first.
var mcpVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

// mcpToken is the token the MCP wants: the given one, or the one kept
// under state, made the first time.
func mcpToken(given, state string) (string, error) {
	if given != "" {
		return given, nil
	}
	file := filepath.Join(state, "mcp-token")
	if b, err := os.ReadFile(file); err == nil && len(strings.TrimSpace(string(b))) >= 16 {
		return strings.TrimSpace(string(b)), nil
	}
	token := newID() + newID() + newID() + newID()
	if err := os.MkdirAll(state, 0o755); err != nil {
		return "", err
	}
	if err := writePrivate(file, []byte(token+"\n")); err != nil {
		return "", err
	}
	slog.Info("mcp: made a token", "file", file)
	return token, nil
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpReply struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

// mcpTool is one tool: what the agent is told of it, and what it does
// with the arguments it is called with.
type mcpTool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Annotations map[string]any  `json:"annotations,omitempty"`

	call func(args json.RawMessage) (any, error) `json:"-"`
}

func routeMCP(mux *http.ServeMux, music string, s *shelf, token string) {
	tools := mcpTools(music, s)
	authorized := func(r *http.Request) bool {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		return ok && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
	}
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="music"`)
			http.Error(w, "the MCP wants its token", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodPost:
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent) // there are no sessions to end
			return
		default:
			// No stream from the server: everything is a reply.
			w.Header().Set("Allow", "POST, DELETE")
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var req mcpRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil {
			writeJSON(w, mcpReply{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &mcpError{-32700, "not JSON-RPC"}})
			return
		}
		if len(req.ID) == 0 {
			// A notification or a response: taken, and nothing to say.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		reply := mcpReply{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &p)
			version := mcpVersions[0]
			if slices.Contains(mcpVersions, p.ProtocolVersion) {
				version = p.ProtocolVersion
			}
			reply.Result = map[string]any{
				"protocolVersion": version,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "music", "title": "Music library", "version": "1"},
				"instructions":    mcpInstructions,
			}
		case "ping":
			reply.Result = map[string]any{}
		case "tools/list":
			reply.Result = map[string]any{"tools": tools}
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil {
				reply.Error = &mcpError{-32602, "bad params"}
				break
			}
			i := slices.IndexFunc(tools, func(t mcpTool) bool { return t.Name == p.Name })
			if i < 0 {
				reply.Error = &mcpError{-32602, "no tool " + p.Name}
				break
			}
			if len(p.Arguments) == 0 || string(p.Arguments) == "null" {
				p.Arguments = json.RawMessage("{}")
			}
			out, err := tools[i].call(p.Arguments)
			reply.Result = toolResult(out, err)
		default:
			reply.Error = &mcpError{-32601, "no method " + req.Method}
		}
		writeJSON(w, reply)
	})
}

// toolResult puts what a tool gave as MCP has it: JSON as text, or the
// error as one the agent can read and act on.
func toolResult(out any, err error) map[string]any {
	if err != nil {
		return map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": err.Error()}}}
	}
	b, _ := json.Marshal(out)
	res := map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}
	if len(b) > 0 && b[0] == '{' {
		res["structuredContent"] = json.RawMessage(b)
	}
	return res
}

const mcpInstructions = `The listener's own music library: artists, albums and tracks, with how often each track was played.
Labels say what the music sounds like — genres, moods and an energy — for an artist or for one album; an album's own labels win over its artist's, field by field, and a track with no genre labelled falls back on its file's genre tag. Smart playlists and the app's mixes are made from them.
To label: page through library_artists (unlabeled_only finds what is left), look at library_albums where an artist's albums differ, and save with set_labels. Label what you know; leave out what you would be guessing at.
To make a playlist: find tracks with search_tracks, or try a smart playlist's rules with preview_smart_playlist, then save_playlist. A smart playlist stays current as the library grows, so prefer one when rules can say what is wanted.`

// trackOut is a track as the tools give it.
type trackOut struct {
	Path   string   `json:"path"`
	Title  string   `json:"title"`
	Artist string   `json:"artist"`
	Album  string   `json:"album"`
	Year   int      `json:"year,omitempty"`
	Genres []string `json:"genres,omitempty"`
	Moods  []string `json:"moods,omitempty"`
	Energy string   `json:"energy,omitempty"`
	Plays  int      `json:"plays,omitempty"`
}

func outTracks(ts []libTrack) []trackOut {
	out := make([]trackOut, len(ts))
	for i, t := range ts {
		out[i] = trackOut{t.Path, t.Name, t.Artist, t.Album, t.Year, t.Genres, t.Moods, t.Energy, t.Plays}
	}
	return out
}

// groupOut is an artist or an album as the listings give it: its size,
// how it has been played, what its files say, and its own labels.
type groupOut struct {
	Artist     string         `json:"artist"`
	Album      string         `json:"album,omitempty"`
	Albums     []string       `json:"albums,omitempty"`
	Tracks     int            `json:"tracks"`
	Plays      int            `json:"plays"`
	Years      string         `json:"years,omitempty"`
	FileGenres map[string]int `json:"file_genres,omitempty"`
	Labels     *labels        `json:"labels,omitempty"`
}

func (g *groupOut) add(t libTrack, years *[2]int) {
	g.Tracks++
	g.Plays += t.Plays
	if t.fileGenre != "" {
		if g.FileGenres == nil {
			g.FileGenres = map[string]int{}
		}
		g.FileGenres[t.fileGenre]++
	}
	if t.Year != 0 {
		if years[0] == 0 || t.Year < years[0] {
			years[0] = t.Year
		}
		years[1] = max(years[1], t.Year)
	}
}

func yearSpan(y [2]int) string {
	switch y[0] {
	case 0:
		return ""
	case y[1]:
		return strconv.Itoa(y[0])
	}
	return fmt.Sprintf("%d–%d", y[0], y[1])
}

func page[T any](all []T, offset, limit, def int) (out []T, next int) {
	if limit <= 0 {
		limit = def
	}
	limit = min(limit, 500)
	offset = max(0, min(offset, len(all)))
	end := min(len(all), offset+limit)
	if end < len(all) {
		next = end
	}
	return all[offset:end], next
}

func decodeArgs(args json.RawMessage, v any) error {
	d := json.NewDecoder(strings.NewReader(string(args)))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("the arguments could not be read: %w", err)
	}
	return nil
}

func mcpTools(music string, s *shelf) []mcpTool {
	m := mcpLib{music, s}
	schema := func(js string) json.RawMessage { return json.RawMessage(js) }
	readOnly := map[string]any{"readOnlyHint": true, "openWorldHint": false}
	writes := map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}

	return []mcpTool{
		{
			Name:  "library_artists",
			Title: "List artists",
			Description: "Lists the library's artists, by name, a page at a time: their albums, tracks, plays, years, " +
				"the genres their files are tagged with (and how many tracks have each), and the artist's own labels. " +
				"unlabeled_only leaves out artists already labelled. Page on with offset = the reply's next.",
			InputSchema: schema(`{"type":"object","properties":{
				"name":{"type":"string","description":"Only artists whose name contains this."},
				"unlabeled_only":{"type":"boolean"},
				"offset":{"type":"integer","minimum":0},
				"limit":{"type":"integer","minimum":1,"maximum":500,"description":"Default 100."}},"additionalProperties":false}`),
			Annotations: readOnly,
			call:        m.libraryArtists,
		},
		{
			Name:  "library_albums",
			Title: "List albums",
			Description: "Lists albums, a page at a time: their tracks, plays, years, file genre tags and the album's own labels " +
				"(an album with none goes by its artist's). Use it where an artist's albums sound unlike each other.",
			InputSchema: schema(`{"type":"object","properties":{
				"artist":{"type":"string","description":"Only this artist's albums, by exact name."},
				"name":{"type":"string","description":"Only albums whose name contains this."},
				"offset":{"type":"integer","minimum":0},
				"limit":{"type":"integer","minimum":1,"maximum":500,"description":"Default 100."}},"additionalProperties":false}`),
			Annotations: readOnly,
			call:        m.libraryAlbums,
		},
		{
			Name:  "search_tracks",
			Title: "Search tracks",
			Description: "Finds tracks, each with its path (what playlists are made of), year, genres, moods, energy and plays " +
				"as they fall to it. Every filter given must hold. Sorted by artist unless sort says otherwise.",
			InputSchema: schema(`{"type":"object","properties":{
				"query":{"type":"string","description":"In the title, artist or album."},
				"artist":{"type":"string","description":"Exact artist name."},
				"genre":{"type":"string"},
				"mood":{"type":"string"},
				"energy":{"type":"string","enum":["low","medium","high"]},
				"year_from":{"type":"integer"},
				"year_to":{"type":"integer"},
				"sort":{"type":"string","enum":["artist","plays","added","year"]},
				"offset":{"type":"integer","minimum":0},
				"limit":{"type":"integer","minimum":1,"maximum":500,"description":"Default 50."}},"additionalProperties":false}`),
			Annotations: readOnly,
			call:        m.searchTracks,
		},
		{
			Name:  "label_vocabulary",
			Title: "Label vocabulary",
			Description: "The moods and energies a label may give, and the genres in use now (labelled or from file tags) with how " +
				"many tracks have each. Reuse a genre already in use where one fits, so mixes gather rather than scatter.",
			InputSchema: schema(`{"type":"object","properties":{},"additionalProperties":false}`),
			Annotations: readOnly,
			call:        m.labelVocabulary,
		},
		{
			Name:  "set_labels",
			Title: "Label artists and albums",
			Description: "Says what artists and albums sound like. Each entry names an artist, or an artist and one of its albums, and " +
				"replaces what was said of it: genres (lower case, most telling first, e.g. [\"phonk\", \"hip-hop\"]), " +
				"moods (from label_vocabulary), energy (low, medium or high). A field left out says nothing, so the artist's — " +
				"or for genre the file's — is used; an entry with every field empty forgets the label. " +
				"Entries that can't be saved are listed with why; the rest are saved.",
			InputSchema: schema(`{"type":"object","properties":{"labels":{"type":"array","minItems":1,"maxItems":500,"items":{
				"type":"object","properties":{
					"artist":{"type":"string"},
					"album":{"type":"string","description":"Leave out to label the artist."},
					"genres":{"type":"array","items":{"type":"string"},"maxItems":6},
					"moods":{"type":"array","items":{"type":"string"},"maxItems":4},
					"energy":{"type":"string","enum":["low","medium","high"]}},
				"required":["artist"],"additionalProperties":false}}},"required":["labels"],"additionalProperties":false}`),
			Annotations: writes,
			call:        m.setLabels,
		},
		{
			Name:        "list_playlists",
			Title:       "List playlists",
			Description: "The listener's playlists: id, name, how many tracks, and a smart one's rules.",
			InputSchema: schema(`{"type":"object","properties":{},"additionalProperties":false}`),
			Annotations: readOnly,
			call:        m.listPlaylists,
		},
		{
			Name:        "get_playlist",
			Title:       "Read a playlist",
			Description: "A playlist's tracks, in order, and a smart one's rules.",
			InputSchema: schema(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}`),
			Annotations: readOnly,
			call:        m.getPlaylist,
		},
		{
			Name:        "preview_smart_playlist",
			Title:       "Try a smart playlist's rules",
			Description: "Shows what a smart playlist's rules would hold, without saving it: how many tracks and the first 30.\n" + smartHelp,
			InputSchema: schema(`{"type":"object","properties":{"smart":` + smartSchema + `},"required":["smart"],"additionalProperties":false}`),
			Annotations: readOnly,
			call:        m.previewSmartPlaylist,
		},
		{
			Name:  "save_playlist",
			Title: "Save a playlist",
			Description: "Makes a playlist, or with id replaces one: either paths, tracks in order (from search_tracks), or smart, " +
				"rules the playlist keeps to as the library changes.\n" + smartHelp,
			InputSchema: schema(`{"type":"object","properties":{
				"id":{"type":"string","description":"Leave out to make a new playlist."},
				"name":{"type":"string"},
				"paths":{"type":"array","items":{"type":"string"},"maxItems":5000},
				"smart":` + smartSchema + `},"required":["name"],"additionalProperties":false}`),
			Annotations: map[string]any{"readOnlyHint": false, "destructiveHint": true, "openWorldHint": false},
			call:        m.savePlaylist,
		},
	}
}

// mcpLib is what the tools work on.
type mcpLib struct {
	music string
	s     *shelf
}

// library is every track, with its plays and labels.
func (m mcpLib) library() ([]libTrack, error) {
	artists, err := walkLibrary(m.music)
	if err != nil {
		return nil, err
	}
	all := flatten(artists)
	m.s.enrich(m.music, all)
	return all, nil
}

func (m mcpLib) libraryArtists(args json.RawMessage) (any, error) {
	var in struct {
		Name          string `json:"name"`
		UnlabeledOnly bool   `json:"unlabeled_only"`
		Offset        int    `json:"offset"`
		Limit         int    `json:"limit"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	all, err := m.library()
	if err != nil {
		return nil, err
	}
	book := m.s.labelsCopy()
	var out []groupOut
	years := map[string]*[2]int{}
	for _, t := range all {
		if in.Name != "" && !strings.Contains(strings.ToLower(t.Artist), strings.ToLower(in.Name)) {
			continue
		}
		if n := len(out); n == 0 || out[n-1].Artist != t.Artist {
			if l, ok := book.Artists[t.Artist]; ok {
				if in.UnlabeledOnly {
					continue
				}
				out = append(out, groupOut{Artist: t.Artist, Labels: &l})
			} else {
				out = append(out, groupOut{Artist: t.Artist})
			}
			years[t.Artist] = &[2]int{}
		}
		g := &out[len(out)-1]
		if !slices.Contains(g.Albums, t.Album) {
			g.Albums = append(g.Albums, t.Album)
		}
		g.add(t, years[t.Artist])
	}
	for i := range out {
		out[i].Years = yearSpan(*years[out[i].Artist])
	}
	got, next := page(out, in.Offset, in.Limit, 100)
	return map[string]any{"total": len(out), "next": next, "artists": got}, nil
}

func (m mcpLib) libraryAlbums(args json.RawMessage) (any, error) {
	var in struct {
		Artist, Name  string
		Offset, Limit int
	}
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	all, err := m.library()
	if err != nil {
		return nil, err
	}
	book := m.s.labelsCopy()
	var out []groupOut
	var years []*[2]int
	for _, t := range all {
		if in.Artist != "" && t.Artist != in.Artist {
			continue
		}
		if in.Name != "" && !strings.Contains(strings.ToLower(t.Album), strings.ToLower(in.Name)) {
			continue
		}
		if n := len(out); n == 0 || out[n-1].Artist != t.Artist || out[n-1].Album != t.Album {
			g := groupOut{Artist: t.Artist, Album: t.Album}
			if l, ok := book.Albums[albumKey(t.Artist, t.Album)]; ok {
				g.Labels = &l
			}
			out = append(out, g)
			years = append(years, &[2]int{})
		}
		out[len(out)-1].add(t, years[len(years)-1])
	}
	if in.Artist != "" && len(out) == 0 {
		return nil, fmt.Errorf("no artist %q in the library", in.Artist)
	}
	for i := range out {
		out[i].Years = yearSpan(*years[i])
	}
	got, next := page(out, in.Offset, in.Limit, 100)
	return map[string]any{"total": len(out), "next": next, "albums": got}, nil
}

func (m mcpLib) searchTracks(args json.RawMessage) (any, error) {
	var in struct {
		Query, Artist, Genre, Mood, Energy, Sort string
		YearFrom                                 int `json:"year_from"`
		YearTo                                   int `json:"year_to"`
		Offset, Limit                            int
	}
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	all, err := m.library()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(in.Query)
	genre := normalizeGenre(in.Genre)
	var hits []libTrack
	for _, t := range all {
		switch {
		case q != "" && !strings.Contains(strings.ToLower(t.Name+" "+t.Artist+" "+t.Album), q),
			in.Artist != "" && t.Artist != in.Artist,
			genre != "" && !slices.Contains(t.Genres, genre),
			in.Mood != "" && !slices.Contains(t.Moods, strings.ToLower(in.Mood)),
			in.Energy != "" && t.Energy != in.Energy,
			in.YearFrom != 0 && (t.Year == 0 || t.Year < in.YearFrom),
			in.YearTo != 0 && (t.Year == 0 || t.Year > in.YearTo):
			continue
		}
		hits = append(hits, t)
	}
	switch in.Sort {
	case "plays":
		slices.SortStableFunc(hits, func(a, b libTrack) int { return cmp.Compare(b.Plays, a.Plays) })
	case "added":
		slices.SortStableFunc(hits, func(a, b libTrack) int { return cmp.Compare(b.Added, a.Added) })
	case "year":
		slices.SortStableFunc(hits, func(a, b libTrack) int { return cmp.Compare(a.Year, b.Year) })
	}
	got, next := page(hits, in.Offset, in.Limit, 50)
	return map[string]any{"total": len(hits), "next": next, "tracks": outTracks(got)}, nil
}

func (m mcpLib) labelVocabulary(json.RawMessage) (any, error) {
	all, err := m.library()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, t := range all {
		for _, g := range t.Genres {
			counts[g]++
		}
	}
	type gc struct {
		Genre  string `json:"genre"`
		Tracks int    `json:"tracks"`
	}
	genres := make([]gc, 0, len(counts))
	for g, n := range counts {
		genres = append(genres, gc{g, n})
	}
	slices.SortFunc(genres, func(a, b gc) int { return cmp.Or(cmp.Compare(b.Tracks, a.Tracks), strings.Compare(a.Genre, b.Genre)) })
	return map[string]any{"moods": moods, "energies": energies, "genres": genres}, nil
}

func (m mcpLib) setLabels(args json.RawMessage) (any, error) {
	var in struct {
		Labels []struct {
			Artist string `json:"artist"`
			Album  string `json:"album"`
			labels
		} `json:"labels"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	artists, err := walkLibrary(m.music)
	if err != nil {
		return nil, err
	}
	has := map[string]bool{}
	for _, a := range artists {
		has[a.Name] = true
		for _, al := range a.Albums {
			has[albumKey(a.Name, al.Name)] = true
		}
	}
	saved := 0
	var refused []string
	for _, e := range in.Labels {
		what := e.Artist
		if e.Album != "" {
			what = albumKey(e.Artist, e.Album)
		}
		if !has[what] {
			refused = append(refused, what+": not in the library")
			continue
		}
		l, err := e.clean()
		if err != nil {
			refused = append(refused, what+": "+err.Error())
			continue
		}
		if err := m.s.setLabel(e.Artist, e.Album, l); err != nil {
			return nil, err
		}
		saved++
	}
	return map[string]any{"saved": saved, "refused": refused}, nil
}

func (m mcpLib) listPlaylists(json.RawMessage) (any, error) {
	all, err := m.library()
	if err != nil {
		return nil, err
	}
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	type out struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Tracks int    `json:"tracks"`
		Smart  *smart `json:"smart,omitempty"`
	}
	res := []out{}
	for _, p := range m.s.playlists {
		res = append(res, out{p.ID, p.Name, len(m.s.resolve(p, all, time.Now())), p.Smart})
	}
	return map[string]any{"playlists": res}, nil
}

func (m mcpLib) getPlaylist(args json.RawMessage) (any, error) {
	var in struct{ ID string }
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	all, err := m.library()
	if err != nil {
		return nil, err
	}
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	p := m.s.find(in.ID)
	if p == nil {
		return nil, fmt.Errorf("no playlist %q", in.ID)
	}
	return map[string]any{"id": p.ID, "name": p.Name, "smart": p.Smart, "tracks": outTracks(m.s.resolve(p, all, time.Now()))}, nil
}

func (m mcpLib) previewSmartPlaylist(args json.RawMessage) (any, error) {
	var in struct{ Smart *smart }
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	p := playlist{ID: "preview", Name: "preview", Smart: in.Smart}
	if err := p.check(); err != nil {
		return nil, err
	}
	all, err := m.library()
	if err != nil {
		return nil, err
	}
	m.s.mu.Lock()
	ts := m.s.resolve(&p, all, time.Now())
	m.s.mu.Unlock()
	return map[string]any{"tracks": len(ts), "first": outTracks(ts[:min(30, len(ts))])}, nil
}

func (m mcpLib) savePlaylist(args json.RawMessage) (any, error) {
	var in playlist
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	if (in.Smart == nil) == (len(in.Paths) == 0) {
		return nil, errors.New("give either paths or smart")
	}
	if err := in.check(); err != nil {
		return nil, err
	}
	all, err := m.library()
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, t := range all {
		known[t.Path] = true
	}
	for _, p := range in.Paths {
		if !known[p] {
			return nil, fmt.Errorf("no track %q in the library", p)
		}
	}
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	now := time.Now()
	p := m.s.find(in.ID)
	switch {
	case in.ID == "":
		p = &playlist{ID: newID(), Created: now}
		m.s.playlists = append(m.s.playlists, p)
	case p == nil:
		return nil, fmt.Errorf("no playlist %q", in.ID)
	}
	p.Name, p.Paths, p.Smart, p.Updated = in.Name, in.Paths, in.Smart, now
	if err := m.s.savePlaylistsLocked(); err != nil {
		return nil, err
	}
	return map[string]any{"id": p.ID, "name": p.Name, "tracks": len(m.s.resolve(p, all, now))}, nil
}

const smartHelp = `A smart playlist: match "all" or "any" of its rules; each rule a field, an op and a value.
Fields and ops: artist, album, title — contains, is, not, starts; genre — is, not, contains; mood, energy — is, not;
plays, year — >, <, =; added, played — within, before (a number of days). Sort: artist, added, plays, played or random; limit 0 is none.`

const smartSchema = `{"type":"object","properties":{
	"match":{"type":"string","enum":["all","any"]},
	"rules":{"type":"array","items":{"type":"object","properties":{
		"field":{"type":"string","enum":["artist","album","title","genre","mood","energy","plays","year","added","played"]},
		"op":{"type":"string"},
		"value":{"type":"string"}},"required":["field","op","value"],"additionalProperties":false}},
	"sort":{"type":"string","enum":["artist","added","plays","played","random"]},
	"limit":{"type":"integer","minimum":0,"maximum":5000}},"required":["rules"],"additionalProperties":false}`
