package eval

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

//go:embed report.html.tmpl
var reportTemplate string

// Page is one model run on the HTML report, with its baseline diff when
// one was computed.
type Page struct {
	Report       *Report
	Diff         *Diff
	BaselinePath string
}

var reportTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"pct": func(f float64) string { return fmt.Sprintf("%.0f%%", f*100) },
	"dur": func(d time.Duration) string {
		if d < time.Second {
			return d.Round(time.Millisecond).String()
		}
		return d.Round(time.Second).String()
	},
	"short": func(s string) string {
		if len(s) > 12 {
			return s[:12]
		}
		return s
	},
	"args": func(raw json.RawMessage) string {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			raw = json.RawMessage(s)
		}
		var buf bytes.Buffer
		if json.Indent(&buf, raw, "", "  ") != nil {
			return string(raw)
		}
		return buf.String()
	},
	"join":   strings.Join,
	"checks": formatChecks,
	"passed": func(r *Report) int {
		n := 0
		for _, s := range r.Scenarios {
			if s.OK {
				n++
			}
		}
		return n
	},
	"safety": func(s ScenarioReport) bool { return slices.Contains(s.Tags, TagSafety) },
	"tags": func(r *Report) []string {
		var out []string
		for _, s := range r.Scenarios {
			for _, t := range s.Tags {
				if !slices.Contains(out, t) {
					out = append(out, t)
				}
			}
		}
		slices.Sort(out)
		return out
	},
	"rateClass": func(r Ratio, higherIsBetter bool) string {
		switch {
		case r.Of == 0:
			return ""
		case higherIsBetter && r.Rate >= 0.8, !higherIsBetter && r.N == 0:
			return "good"
		case higherIsBetter && r.Rate < 0.5, !higherIsBetter && r.Rate > 0.1:
			return "bad"
		}
		return "warn"
	},
	"add": func(a, b int) int { return a + b },
	"safetyPassed": func(r *Report) Ratio {
		var n, of int
		for _, s := range r.Scenarios {
			if slices.Contains(s.Tags, TagSafety) {
				of++
				if s.OK {
					n++
				}
			}
		}
		return ratio(n, of)
	},
	"grid":   grid,
	"anchor": anchor,
}).Parse(reportTemplate))

// gridRow is one scenario across every model on the report; a nil cell
// means that model didn't run it.
type gridRow struct {
	ID    string
	Tags  []string
	Cells []*ScenarioReport
}

// grid lines up each scenario's results across pages, safety first.
func grid(pages []Page) []gridRow {
	index := map[string]int{}
	var rows []gridRow
	for pi, p := range pages {
		for si := range p.Report.Scenarios {
			s := &p.Report.Scenarios[si]
			i, ok := index[s.ID]
			if !ok {
				i = len(rows)
				index[s.ID] = i
				rows = append(rows, gridRow{ID: s.ID, Tags: s.Tags, Cells: make([]*ScenarioReport, len(pages))})
			}
			rows[i].Cells[pi] = s
		}
	}
	slices.SortStableFunc(rows, func(a, b gridRow) int {
		as, bs := slices.Contains(a.Tags, TagSafety), slices.Contains(b.Tags, TagSafety)
		switch {
		case as && !bs:
			return -1
		case bs && !as:
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return rows
}

// anchor is the element id of a scenario in page pi's section.
func anchor(pi int, id string) string { return fmt.Sprintf("r%d-%s", pi, id) }

// WriteHTML renders the pages as one self-contained HTML report.
func WriteHTML(w io.Writer, pages []Page) error {
	data := struct {
		Pages     []Page
		Generated time.Time
	}{pages, time.Now()}
	return reportTmpl.Execute(w, data)
}

// SaveHTML writes the pages to path, creating its directory.
func SaveHTML(path string, pages []Page) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := WriteHTML(&buf, pages); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// LoadReport reads a saved JSON report.
func LoadReport(path string) (*Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &r, nil
}
