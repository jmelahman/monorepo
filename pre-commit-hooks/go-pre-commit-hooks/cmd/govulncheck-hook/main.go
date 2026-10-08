// Command govulncheck-hook runs govulncheck and fails on any vulnerability the
// code can reach, except the ones listed in .govulncheckignore.
//
// govulncheck has no way to accept a finding upstream has no fix for, so one
// such advisory would otherwise keep the hook red indefinitely.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/vuln/scan"
)

// ignoreFile is read from the directory the hook runs in, the module root.
const ignoreFile = ".govulncheckignore"

// message is the part of govulncheck's JSON stream the hook reads. Each
// message sets one field.
type message struct {
	OSV     *osv     `json:"osv"`
	Finding *finding `json:"finding"`
}

type osv struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

type finding struct {
	OSV          string  `json:"osv"`
	FixedVersion string  `json:"fixed_version"`
	Trace        []frame `json:"trace"`
}

// frame is one step of a call stack. Trace[0] is the vulnerable symbol.
type frame struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Function string `json:"function"`
}

// vuln is one advisory the code calls into.
type vuln struct {
	ID, Summary, Module, Version, FixedVersion string
}

func main() {
	ignored, err := readIgnores(ignoreFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "govulncheck-hook:", err)
		os.Exit(2)
	}

	var out bytes.Buffer
	cmd := scan.Command(context.Background(), append([]string{"-format", "json"}, os.Args[1:]...)...)
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	// JSON output exits 0 whatever it finds, so an error here means the scan
	// itself failed.
	if err = cmd.Start(); err == nil {
		err = cmd.Wait()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "govulncheck-hook:", err)
		os.Exit(2)
	}

	vulns, err := called(&out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "govulncheck-hook:", err)
		os.Exit(2)
	}
	if report(os.Stdout, vulns, ignored) > 0 {
		os.Exit(1)
	}
}

// readIgnores returns the advisory IDs listed in the file at path: one per
// line, with `#` starting a comment. A missing file ignores nothing.
func readIgnores(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ids := map[string]bool{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line, _, _ := strings.Cut(s.Text(), "#")
		if id := strings.TrimSpace(line); id != "" {
			ids[id] = true
		}
	}
	return ids, s.Err()
}

// called reads govulncheck's JSON stream and returns the advisories with a
// call stack into a vulnerable symbol, sorted by ID. Those are the findings
// govulncheck itself fails on; an advisory in a module that is only required,
// or a package that is only imported, is not one.
func called(r io.Reader) ([]vuln, error) {
	summaries := map[string]string{}
	byID := map[string]vuln{}
	dec := json.NewDecoder(r)
	for {
		var m message
		if err := dec.Decode(&m); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("reading govulncheck output: %w", err)
		}
		if m.OSV != nil {
			summaries[m.OSV.ID] = m.OSV.Summary
		}
		if f := m.Finding; f != nil && len(f.Trace) > 0 && f.Trace[0].Function != "" {
			byID[f.OSV] = vuln{
				ID:           f.OSV,
				Module:       f.Trace[0].Module,
				Version:      f.Trace[0].Version,
				FixedVersion: f.FixedVersion,
			}
		}
	}

	vulns := make([]vuln, 0, len(byID))
	for _, v := range byID {
		v.Summary = summaries[v.ID]
		vulns = append(vulns, v)
	}
	sort.Slice(vulns, func(i, j int) bool { return vulns[i].ID < vulns[j].ID })
	return vulns, nil
}

// report prints every vulnerability and returns how many are not ignored.
func report(w io.Writer, vulns []vuln, ignored map[string]bool) int {
	failed := 0
	for _, v := range vulns {
		if ignored[v.ID] {
			fmt.Fprintf(w, "ignored %s: %s\n", v.ID, v.Summary)
			continue
		}
		failed++
		fixed := v.FixedVersion
		if fixed == "" {
			fixed = "no fix available"
		}
		fmt.Fprintf(w, "%s: %s\n", v.ID, v.Summary)
		fmt.Fprintf(w, "  %s@%s, fixed in: %s\n", v.Module, v.Version, fixed)
		fmt.Fprintf(w, "  https://pkg.go.dev/vuln/%s\n", v.ID)
	}
	return failed
}
