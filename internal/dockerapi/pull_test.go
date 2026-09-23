package dockerapi

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestStreamPullProgressLines(t *testing.T) {
	old := pullProgressInterval
	pullProgressInterval = 0
	defer func() { pullProgressInterval = old }()

	// 5 MB / 10 MB on l1, 1 MB / 10 MB on l2, then l1 finishes: the "Pull
	// complete" message carries no byte counts, so l1 must snap to its total.
	stream := `{"status":"Downloading","id":"l1","progressDetail":{"current":5242880,"total":10485760}}
{"status":"Downloading","id":"l2","progressDetail":{"current":1048576,"total":10485760}}
{"status":"Pull complete","id":"l1"}
`
	var buf bytes.Buffer
	if err := streamPull(strings.NewReader(stream), &buf); err != nil {
		t.Fatalf("streamPull: %v", err)
	}
	got := strings.Split(strings.TrimSpace(buf.String()), "\n")
	want := []string{"pulled 5/10 MB", "pulled 6/20 MB", "pulled 11/20 MB"}
	if len(got) != len(want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStreamPullInStreamError(t *testing.T) {
	stream := `{"status":"Downloading","id":"l1"}
{"error":"manifest unknown"}
`
	err := streamPull(strings.NewReader(stream), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "manifest unknown") {
		t.Fatalf("err = %v, want manifest unknown", err)
	}
}
