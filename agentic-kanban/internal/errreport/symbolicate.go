package errreport

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// frameLocRE matches the "<script url>:<line>:<col>" tail of a browser stack
// frame. It covers both the V8 form ("at fn (http://h/assets/x.js:1:2)") and
// the SpiderMonkey/JavaScriptCore form ("fn@http://h/assets/x.js:1:2").
var frameLocRE = regexp.MustCompile(`(https?://[^\s()@]+?):(\d+):(\d+)`)

// Bounds on the work one report can ask for: the stack arrives from the
// browser, and decoding a bundle's map means parsing several MB of JSON.
const (
	symbolicateStackMax = 64 << 10
	symbolicateMapsMax  = 8
	symbolicateCacheMax = 4096
)

// frameKey is one reported bundle position: the fs path of the bundle's map
// plus the frame's 1-based line and column.
type frameKey struct {
	mapPath   string
	line, col int
}

// symbolicator rewrites the minified bundle locations in browser stack
// traces to original source locations ("src/components/Foo.tsx:12:3") using
// the source maps Vite emits next to each bundle in fsys.
//
// See REGRESSIONS.md: "Error tickets are only symbolicated while the build embeds source maps".
type symbolicator struct {
	fsys fs.FS

	mu sync.Mutex
	// cache remembers resolved positions ("" = unresolvable) so a recurring
	// error doesn't re-decode the maps. fsys is immutable, so entries never
	// go stale.
	cache map[frameKey]string
}

func newSymbolicator(fsys fs.FS) *symbolicator {
	if fsys == nil {
		return nil
	}
	return &symbolicator{fsys: fsys, cache: map[frameKey]string{}}
}

// symbolicate returns stack with every resolvable frame location rewritten.
//
// Best-effort by design: a frame is left exactly as reported when its bundle
// has no map in fsys (a tab still running a previous build, a third-party
// script), the map doesn't parse, or the position isn't mapped. Function names
// are left alone — a source map names identifiers at a position, not the
// function enclosing it, so guessing would mislabel frames. Non-browser
// stacks (Go panics) contain no script URLs and pass through untouched.
func (s *symbolicator) symbolicate(stack string) (out string) {
	if s == nil || stack == "" || len(stack) > symbolicateStackMax {
		return stack
	}
	defer func() {
		if rec := recover(); rec != nil {
			out = stack
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Collect the uncached generated lines wanted per map first, so each map
	// is decoded once and only the lines a frame points at are kept.
	wanted := map[string]map[int]bool{}
	var missing []frameKey
	for _, m := range frameLocRE.FindAllStringSubmatch(stack, -1) {
		k, ok := parseFrameLoc(m)
		if !ok {
			continue
		}
		if _, hit := s.cache[k]; hit {
			continue
		}
		if wanted[k.mapPath] == nil {
			if len(wanted) >= symbolicateMapsMax {
				continue
			}
			wanted[k.mapPath] = map[int]bool{}
		}
		wanted[k.mapPath][k.line] = true
		missing = append(missing, k)
	}
	if len(missing) > 0 {
		maps := make(map[string]*sourceMap, len(wanted))
		for mapPath, lines := range wanted {
			maps[mapPath] = loadSourceMap(s.fsys, mapPath, lines)
		}
		if len(s.cache)+len(missing) > symbolicateCacheMax {
			clear(s.cache)
		}
		for _, k := range missing {
			loc := ""
			if sm := maps[k.mapPath]; sm != nil {
				if src, line, col, ok := sm.lookup(k.line, k.col); ok {
					loc = fmt.Sprintf("%s:%d:%d", src, line, col)
				}
			}
			s.cache[k] = loc
		}
	}
	return frameLocRE.ReplaceAllStringFunc(stack, func(loc string) string {
		k, ok := parseFrameLoc(frameLocRE.FindStringSubmatch(loc))
		if !ok {
			return loc
		}
		if resolved := s.cache[k]; resolved != "" {
			return resolved
		}
		return loc
	})
}

// parseFrameLoc turns a frameLocRE match into the position it reports.
func parseFrameLoc(m []string) (frameKey, bool) {
	if len(m) != 4 {
		return frameKey{}, false
	}
	u, err := url.Parse(m[1])
	if err != nil {
		return frameKey{}, false
	}
	mapPath := strings.TrimPrefix(u.Path, "/") + ".map"
	if !fs.ValidPath(mapPath) {
		return frameKey{}, false
	}
	line, err1 := strconv.Atoi(m[2])
	col, err2 := strconv.Atoi(m[3])
	if err1 != nil || err2 != nil || line < 1 || col < 1 {
		return frameKey{}, false
	}
	return frameKey{mapPath, line, col}, true
}

// segment is one decoded source-map mapping: generated column → original
// position. source is -1 for a segment that maps to nothing.
type segment struct {
	genCol, source, origLine, origCol int
}

// sourceMap holds the decoded segments for just the generated lines a stack
// trace referenced (0-based), each sorted by generated column.
type sourceMap struct {
	sources []string
	lines   map[int][]segment
}

// loadSourceMap reads and decodes the v3 source map at mapPath, keeping only
// the generated lines in wanted (1-based, as stack frames report them).
// Returns nil if the map is missing or malformed.
func loadSourceMap(fsys fs.FS, mapPath string, wanted map[int]bool) *sourceMap {
	raw, err := fs.ReadFile(fsys, mapPath)
	if err != nil {
		return nil
	}
	var doc struct {
		SourceRoot string   `json:"sourceRoot"`
		Sources    []string `json:"sources"`
		Mappings   string   `json:"mappings"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	sm := &sourceMap{
		sources: make([]string, len(doc.Sources)),
		lines:   make(map[int][]segment, len(wanted)),
	}
	// Sources are relative to the map file; resolve them and drop the
	// leading "../" run so "dist/assets/../../src/x.tsx" reads "src/x.tsx".
	base := path.Join(path.Dir(mapPath), doc.SourceRoot)
	for i, s := range doc.Sources {
		// Virtual modules ("\0vite/preload-helper"), URLs and blanks aren't
		// files in the repo; leave their frames as reported (see lookup).
		if s == "" || strings.HasPrefix(s, "\x00") || strings.Contains(s, "://") {
			continue
		}
		p := path.Join(base, s)
		for strings.HasPrefix(p, "../") {
			p = p[3:]
		}
		sm.sources[i] = p
	}

	// Every field but the generated column is a delta that carries across
	// lines, so the whole string has to be walked even for a single line.
	var genLine, genCol, source, origLine, origCol int
	keep := wanted[1]
	var fields [5]int
	nfields := 0
	flush := func() {
		if nfields == 0 {
			return
		}
		genCol += fields[0]
		seg := segment{genCol: genCol, source: -1}
		if nfields >= 4 {
			source += fields[1]
			origLine += fields[2]
			origCol += fields[3]
			seg.source, seg.origLine, seg.origCol = source, origLine, origCol
		}
		if keep {
			sm.lines[genLine] = append(sm.lines[genLine], seg)
		}
		nfields = 0
	}
	value, shift := 0, uint(0)
	for i := 0; i < len(doc.Mappings); i++ {
		c := doc.Mappings[i]
		switch c {
		case ',':
			flush()
		case ';':
			flush()
			genLine++
			genCol = 0
			keep = wanted[genLine+1]
		default:
			digit := base64Value(c)
			if digit < 0 || shift > 30 {
				return nil
			}
			value |= (digit & 31) << shift
			if digit&32 != 0 {
				shift += 5
				continue
			}
			if nfields < len(fields) {
				// Low bit is the sign.
				if value&1 != 0 {
					fields[nfields] = -(value >> 1)
				} else {
					fields[nfields] = value >> 1
				}
				nfields++
			}
			value, shift = 0, 0
		}
	}
	flush()
	return sm
}

// lookup maps a 1-based generated line/column to the original source path
// and 1-based line/column.
func (sm *sourceMap) lookup(line, col int) (src string, origLine, origCol int, ok bool) {
	segs := sm.lines[line-1]
	// Last segment starting at or before the column.
	i := sort.Search(len(segs), func(i int) bool { return segs[i].genCol > col-1 }) - 1
	if i < 0 {
		return "", 0, 0, false
	}
	seg := segs[i]
	if seg.source < 0 || seg.source >= len(sm.sources) || sm.sources[seg.source] == "" {
		return "", 0, 0, false
	}
	return sm.sources[seg.source], seg.origLine + 1, seg.origCol + 1, true
}

func base64Value(c byte) int {
	switch {
	case c >= 'A' && c <= 'Z':
		return int(c - 'A')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 26
	case c >= '0' && c <= '9':
		return int(c-'0') + 52
	case c == '+':
		return 62
	case c == '/':
		return 63
	}
	return -1
}
