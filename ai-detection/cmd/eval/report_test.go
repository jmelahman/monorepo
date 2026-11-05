package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/local-ai-detection/eval"
)

func TestReportWritesMetricsAndJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scores := filepath.Join(dir, "scores.csv")
	writer, err := eval.CreateObservations(scores)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 20 {
		observations := []eval.Observation{
			{
				ID: i, Source: "wp_human", Machine: false,
				Score: 0.90 + float64(i)*0.001,
			},
			{
				ID: 100 + i, Source: "wp_machine_continuation_gpt-3.5-trubo", Machine: true,
				Score: 0.30 + float64(i)*0.001,
			},
		}
		for _, observation := range observations {
			if err := writer.Write(observation); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	jsonPath := filepath.Join(dir, "report.json")
	var out bytes.Buffer
	err = runReport([]string{
		"--scores", scores,
		"--json", jsonPath,
		"--name", "synthetic",
		"--bootstrap", "50",
		"--min-group", "1",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ROC-AUC", "cross-fitted threshold", "By machine model", "gpt-3.5-trubo"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report is missing %q:\n%s", want, out.String())
		}
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var report reportJSON
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("report JSON is invalid: %v", err)
	}
	if report.N != 40 || report.Humans != 20 || report.Machines != 20 {
		t.Errorf("report counts = %d/%d/%d, want 40/20/20", report.N, report.Humans, report.Machines)
	}
	if report.AUC < 0.99 {
		t.Errorf("AUC = %v, want near 1 for separated scores", report.AUC)
	}
}
