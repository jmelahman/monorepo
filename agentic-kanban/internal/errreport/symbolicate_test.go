package errreport_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jmelahman/kanban/internal/errreport"
)

// testSourceMap maps, for the bundle it sits next to:
//
//	1:1  → src/components/Foo.tsx:1:1
//	1:11 → src/components/Foo.tsx:5:3
//	2:6  → node_modules/lib/index.js:10:1
//	2:21 → (unmapped)
//	3:1  → a virtual module, which is not a file to point at
const testSourceMap = `{"version":3,` +
	`"sources":["../../src/components/Foo.tsx","../../node_modules/lib/index.js","\u0000vite/preload-helper.js"],` +
	`"mappings":"AAAA,UAIE;KCKF,e;ACAA"}`

func TestReporter_SymbolicatesBrowserStack(t *testing.T) {
	store := newStore(t)
	r := errreport.New(store, errreport.Config{Enabled: true, BoardName: "Errors"})
	r.SetSourceMaps(fstest.MapFS{
		"assets/index-abc.js.map":  {Data: []byte(testSourceMap)},
		"assets/broken-abc.js.map": {Data: []byte(`{"mappings":"!!"}`)},
	})
	stack := strings.Join([]string{
		// SpiderMonkey form, column inside the second segment.
		"refetchInterval@http://localhost:7474/assets/index-abc.js:1:15",
		// V8 form.
		"    at Uu (http://localhost:7474/assets/index-abc.js:2:6)",
		"unmapped@http://localhost:7474/assets/index-abc.js:2:30",
		"noline@http://localhost:7474/assets/index-abc.js:9:1",
		"virtual@http://localhost:7474/assets/index-abc.js:3:1",
		"stale@http://localhost:7474/assets/index-old.js:1:1",
		"broken@http://localhost:7474/assets/broken-abc.js:1:1",
	}, "\n")
	r.Report(context.Background(), "boundary", "boom", stack, nil)

	board := boardSlug(t, store, "errors")
	tickets, err := store.ListTickets(context.Background(), board.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(tickets))
	}
	for _, want := range []string{
		"refetchInterval@src/components/Foo.tsx:5:3\n",
		"    at Uu (node_modules/lib/index.js:10:1)\n",
		// Anything that can't be resolved stays exactly as reported.
		"unmapped@http://localhost:7474/assets/index-abc.js:2:30\n",
		"noline@http://localhost:7474/assets/index-abc.js:9:1\n",
		"virtual@http://localhost:7474/assets/index-abc.js:3:1\n",
		"stale@http://localhost:7474/assets/index-old.js:1:1\n",
		"broken@http://localhost:7474/assets/broken-abc.js:1:1\n",
	} {
		if !strings.Contains(tickets[0].Body, want) {
			t.Errorf("body missing %q:\n%s", want, tickets[0].Body)
		}
	}
}

// A rebuild renames every bundle (content hash), so raw frames would never
// dedup across builds. Symbolicated frames must.
func TestReporter_SymbolicatedStackDedupsAcrossBuilds(t *testing.T) {
	store := newStore(t)
	r := errreport.New(store, errreport.Config{Enabled: true, BoardName: "Errors"})
	r.SetSourceMaps(fstest.MapFS{
		"assets/index-abc.js.map": {Data: []byte(testSourceMap)},
	})
	r.Report(context.Background(), "boundary", "boom", "f@http://localhost:7474/assets/index-abc.js:1:15", nil)

	// Next build: new hash, the same source line landed elsewhere, and the
	// minifier picked another name for the function.
	r.SetSourceMaps(fstest.MapFS{
		"assets/index-xyz.js.map": {Data: []byte(strings.Replace(testSourceMap, "AAAA,UAIE", "AAAA,oBAIE", 1))},
	})
	r.Report(context.Background(), "boundary", "boom", "g@http://localhost:7474/assets/index-xyz.js:1:25", nil)

	board := boardSlug(t, store, "errors")
	tickets, err := store.ListTickets(context.Background(), board.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 1 {
		t.Fatalf("expected the second build's report to dedup, got %d tickets", len(tickets))
	}
}
