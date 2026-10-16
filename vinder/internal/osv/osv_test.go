package osv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
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

func TestQueryBatchSplitsAndPages(t *testing.T) {
	t.Parallel()

	// Every query's first page names its package and points at a second
	// page; the second names the package again.
	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Queries []struct {
				Package struct {
					Name string `json:"name"`
				} `json:"package"`
				PageToken string `json:"page_token"`
			} `json:"queries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		sizes = append(sizes, len(req.Queries))
		type vuln struct {
			ID string `json:"id"`
		}
		type result struct {
			NextPageToken string `json:"next_page_token,omitempty"`
			Vulns         []vuln `json:"vulns"`
		}
		var res struct {
			Results []result `json:"results"`
		}
		for _, q := range req.Queries {
			if q.PageToken == "" {
				res.Results = append(res.Results, result{NextPageToken: "p2", Vulns: []vuln{{q.Package.Name + "-1"}}})
			} else {
				res.Results = append(res.Results, result{Vulns: []vuln{{q.Package.Name + "-2"}}})
			}
		}
		if err := json.NewEncoder(w).Encode(res); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)

	pkgs := make([]osv.Package, 1500)
	for i := range pkgs {
		pkgs[i] = osv.Package{Name: strconv.Itoa(i), Version: "1.0.0"}
	}
	ids, err := (&osv.Client{URL: srv.URL}).QueryBatch(context.Background(), pkgs)
	if err != nil {
		t.Fatal(err)
	}
	// 1000 first pages, then the other 500 first pages with the first
	// 500 second pages, then the last 1000 second pages.
	if want := []int{1000, 1000, 1000}; !reflect.DeepEqual(sizes, want) {
		t.Errorf("batch sizes = %v, want %v", sizes, want)
	}
	for i, got := range ids {
		if want := []string{strconv.Itoa(i) + "-1", strconv.Itoa(i) + "-2"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ids[%d] = %q, want %q", i, got, want)
		}
	}
}
