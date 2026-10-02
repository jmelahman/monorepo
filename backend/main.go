// Fixture backend for integration tests: /api/health plus a counter
// persisted under --state so lineage forking is observable.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address")
	state := flag.String("state", ".", "state directory")
	flag.Parse()

	countFile := filepath.Join(*state, "count")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("/api/counter", func(w http.ResponseWriter, r *http.Request) {
		n := 0
		if b, err := os.ReadFile(countFile); err == nil {
			n, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		n++
		if err := os.WriteFile(countFile, []byte(strconv.Itoa(n)), 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%d", n)
	})
	// The startup line makes the run log observable in tests.
	fmt.Printf("fixture backend listening on %s\n", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
