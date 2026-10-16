package osv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/jmelahman/vinder/internal/osv"
)

func TestQueryBatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reply   string
		want    [][]string
		status  int
		wantErr bool
	}{
		{
			name:   "results in query order",
			status: http.StatusOK,
			reply:  `{"results": [{}, {"vulns": [{"id": "GHSA-1"}, {"id": "GHSA-2"}]}]}`,
			want:   [][]string{nil, {"GHSA-1", "GHSA-2"}},
		},
		{name: "server error", status: http.StatusInternalServerError, reply: "boom", wantErr: true},
		{name: "result count mismatch", status: http.StatusOK, reply: `{"results": [{}]}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got struct {
				Queries []struct {
					Package struct {
						Ecosystem string `json:"ecosystem"`
						Name      string `json:"name"`
					} `json:"package"`
					Version string `json:"version"`
				} `json:"queries"`
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decoding request: %v", err)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.reply))
			}))
			t.Cleanup(srv.Close)

			pkgs := []osv.Package{{Name: "lib", Version: "1.0.0"}, {Name: "@scope/dep", Version: "2.3.3"}}
			ids, err := (&osv.Client{URL: srv.URL}).QueryBatch(context.Background(), pkgs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("QueryBatch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(ids, tt.want) {
				t.Errorf("QueryBatch() = %q, want %q", ids, tt.want)
			}
			if q := got.Queries; len(q) != 2 || q[1].Package.Ecosystem != "npm" || q[1].Package.Name != "@scope/dep" || q[1].Version != "2.3.3" {
				t.Errorf("request queries = %+v", q)
			}
		})
	}
}
