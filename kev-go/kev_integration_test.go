package kev

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	typesafe "github.com/jmelahman/typesafe-sdk-go"
)

type fixture struct {
	Request json.RawMessage `json:"request"`
	Record  struct {
		State     string `json:"state"`
		Questions []struct {
			Instr   string   `json:"instr"`
			Options []string `json:"options"`
		} `json:"questions"`
	} `json:"record"`
	Rows []struct {
		IDs      []int32 `json:"ids"`
		Opts     []int   `json:"opts"`
		Decide   int     `json:"decide"`
		StateLen int     `json:"state_len"`
	} `json:"rows"`
	Probs [][]float64 `json:"probs"`
}

func readFixtures(t *testing.T) []fixture {
	t.Helper()
	data, err := os.ReadFile("testdata/requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx []fixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatal(err)
	}
	return fx
}

// Rendering needs no weights, so this runs everywhere.
func TestRenderMatchesPython(t *testing.T) {
	for i, f := range readFixtures(t) {
		req, err := parseRequest(f.Request)
		if err != nil {
			t.Fatal(err)
		}
		if req.state != f.Record.State {
			t.Errorf("request %d state\n got %q\nwant %q", i, req.state, f.Record.State)
		}
		for j, q := range req.questions {
			want := f.Record.Questions[j]
			if q.q.Instructions != want.Instr || !slices.Equal(q.q.Options, want.Options) {
				t.Errorf("request %d question %d\n got %q %q\nwant %q %q", i, j, q.q.Instructions, q.q.Options, want.Instr, want.Options)
			}
		}
	}
}

func openModel(t *testing.T) *Model {
	t.Helper()
	ckpt, base := os.Getenv("KEV_CHECKPOINT"), os.Getenv("KEV_BASE")
	if ckpt == "" || base == "" {
		t.Skip("KEV_CHECKPOINT or KEV_BASE isn't set")
	}
	m, err := Open(ckpt, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}

func TestDecideMatchesPython(t *testing.T) {
	m := openModel(t)
	for i, f := range readFixtures(t) {
		req, err := parseRequest(f.Request)
		if err != nil {
			t.Fatal(err)
		}
		questions := make([]Question, len(req.questions))
		for j, q := range req.questions {
			questions[j] = q.q
		}
		s, branches, err := m.encode(req.state, questions)
		if err != nil {
			t.Fatal(err)
		}
		for j, b := range branches {
			row := f.Rows[j]
			if !slices.Equal(append(slices.Clone(s), b.ids...), row.IDs) {
				t.Errorf("request %d row %d: ids differ", i, j)
			}
			if b.decide+len(s) != row.Decide {
				t.Errorf("request %d row %d: decide at %d, want %d", i, j, b.decide+len(s), row.Decide)
			}
		}
		start := time.Now()
		got, tokens, err := m.Decide(context.Background(), req.state, questions)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("request %d: %d tokens in %v", i, tokens, time.Since(start).Round(time.Millisecond))
		for j := range got {
			for k := range got[j] {
				if d := math.Abs(got[j][k] - f.Probs[j][k]); d > 1e-4 {
					t.Errorf("request %d question %d option %d: %v, want %v", i, j, k, got[j][k], f.Probs[j][k])
				}
			}
		}
	}
}

func TestSDKThroughTransport(t *testing.T) {
	m := openModel(t)
	client, err := typesafe.New(
		typesafe.WithAPIKey("local"),
		typesafe.WithHTTPClient(&http.Client{Transport: m.Transport()}),
	)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State: "Ticket: customer was charged twice for one order and wants a refund.",
		Questions: typesafe.Questions{
			"cat":    typesafe.Choice("Which queue?", typesafe.ChoiceCriteria{"billing": "payments and refunds", "tech": "bugs and outages"}),
			"urgent": typesafe.Noul("Is this urgent?"),
			"sev":    typesafe.Score("How severe?", typesafe.ScoreCriteria{"minor", "critical"}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c, err := res.Choice("cat"); err != nil || c.Choice != "billing" {
		t.Errorf("Choice = %+v, %v", c, err)
	}
	if n, err := res.Noul("urgent"); err != nil || n.Noul <= 0 || n.Noul >= 1 {
		t.Errorf("Noul = %+v, %v", n, err)
	}
	if s, err := res.Score("sev"); err != nil || s.Level() < 0 || s.Level() > 1 {
		t.Errorf("Score = %+v, %v", s, err)
	}
	if res.Usage.InputTokens == 0 {
		t.Errorf("Usage = %+v", res.Usage)
	}
}

func TestOverlongStateIsRefused(t *testing.T) {
	m := openModel(t)
	huge := strings.Repeat("word ", maxState)
	_, _, err := m.Decide(context.Background(), huge, []Question{{Options: []string{"a"}}})
	if _, ok := errors.AsType[*OverflowError](err); !ok {
		t.Fatalf("err = %v, want an OverflowError", err)
	}
}

// A repeated state reads from the prefix cache and must answer the same.
func TestRepeatedStateAnswersTheSame(t *testing.T) {
	m := openModel(t)
	questions := []Question{{Instructions: "Is the sky blue?", Options: []string{"no", "yes"}}}
	first, _, err := m.Decide(context.Background(), "The sky is green today.", questions)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := m.Decide(context.Background(), "The sky is green today.", questions)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.cached) != 1 || !slices.Equal(first[0], second[0]) {
		t.Errorf("cached %d, answers %v and %v", len(m.cached), first, second)
	}
}

func TestErrorsReachTheClient(t *testing.T) {
	m := openModel(t)
	for body, want := range map[string]int{
		`{"questions":{"a":{"type":"noul"}}}`:               http.StatusUnprocessableEntity,
		`{"state":"x","questions":{}}`:                      http.StatusUnprocessableEntity,
		`{"state":"x","questions":{"a":{"type":"choice"}}}`: http.StatusUnprocessableEntity,
		`{"state":"x","questions":{"a":{"type":"nope"}}}`:   http.StatusUnprocessableEntity,
		`{"state":7,"questions":{"a":{"type":"noul"}}} `:    http.StatusOK,
		`not json`: http.StatusUnprocessableEntity,
	} {
		r := httptest.NewRequest(http.MethodPost, systemOnePath, strings.NewReader(body))
		w := httptest.NewRecorder()
		m.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("%s: status %d, want %d: %s", body, w.Code, want, w.Body)
		}
	}
}

func TestCancelledContextStops(t *testing.T) {
	m := openModel(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := m.Decide(ctx, "state", []Question{{Options: []string{"a"}}})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestRenderScalars(t *testing.T) {
	for in, want := range map[string]string{"1": "1", "-0": "0", "1.50": "1.5", "1e16": "1e+16", "0.00001": "1e-05", "2.0": "2.0", "1e3": "1000.0", "0.0": "0.0"} {
		if got := pythonNumber(json.Number(in)); got != want {
			t.Errorf("pythonNumber(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestChoiceConfidence(t *testing.T) {
	if got := choiceConfidence([]float64{0.5, 0.5}); got != 0 {
		t.Errorf("uniform = %v", got)
	}
	if got := scoreConfidence([]float64{0, 1, 0}); got != 1 {
		t.Errorf("certain = %v", got)
	}
}
