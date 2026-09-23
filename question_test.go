package typesafe

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuestionMarshaling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		q    Question
		want string
	}{{
		name: "noul without criteria omits it",
		q:    Noul("Is this spam?"),
		want: `{"type":"noul","instructions":"Is this spam?"}`,
	}, {
		name: "noul with criteria",
		q:    Noul("Is this spam?").WithCriteria(NoulCriteria{True: "Unsolicited bulk mail"}),
		want: `{"type":"noul","instructions":"Is this spam?","criteria":{"true":"Unsolicited bulk mail"}}`,
	}, {
		name: "nil instructions are omitted",
		q:    Noul(nil),
		want: `{"type":"noul"}`,
	}, {
		name: "Null instructions are transmitted",
		q:    Noul(Null),
		want: `{"type":"noul","instructions":null}`,
	}, {
		name: "empty instructions are kept",
		q:    Noul(""),
		want: `{"type":"noul","instructions":""}`,
	}, {
		name: "nil choice description becomes null",
		q:    Choice("Pick one", ChoiceCriteria{"excited": nil}),
		want: `{"type":"choice","instructions":"Pick one","criteria":{"excited":null}}`,
	}, {
		name: "structured criteria",
		q: Choice("Route it", ChoiceCriteria{
			"billing": map[string]any{"when": "money is involved"},
		}),
		want: `{"type":"choice","instructions":"Route it","criteria":{"billing":{"when":"money is involved"}}}`,
	}, {
		name: "score criteria are an ordered array",
		q:    Score("How urgent?", ScoreCriteria{"low", "high"}),
		want: `{"type":"score","instructions":"How urgent?","criteria":["low","high"]}`,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := json.Marshal(tc.q)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Marshal =\n\t%s\nwant\n\t%s", got, tc.want)
			}
		})
	}
}

func TestQuestionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		qs      Questions
		wantErr string
	}{{
		name:    "no questions",
		qs:      Questions{},
		wantErr: "at least one question is required",
	}, {
		name:    "nil question",
		qs:      Questions{"a": nil},
		wantErr: `question "a" is nil`,
	}, {
		name:    "choice without criteria",
		qs:      Questions{"a": Choice("Pick", nil)},
		wantErr: `choice question "a" requires criteria`,
	}, {
		name:    "score with one level",
		qs:      Questions{"a": Score("Rate", ScoreCriteria{"only"})},
		wantErr: "at least 2 score levels are required",
	}, {
		name:    "score with a null level",
		qs:      Questions{"a": Score("Rate", ScoreCriteria{"low", nil})},
		wantErr: `question "a" criteria 1 must describe a score level`,
	}, {
		name:    "numeric instructions",
		qs:      Questions{"a": Noul(42)},
		wantErr: "must be a string, object, or array",
	}, {
		name: "valid",
		qs: Questions{
			"a": Noul("Is it?"),
			"b": Choice("Which?", ChoiceCriteria{"x": nil}),
			"c": Score("How much?", ScoreCriteria{"low", "high"}),
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.qs.validate()
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("validate() = %v, want nil", err)
			case tc.wantErr == "":
				return
			case err == nil:
				t.Fatalf("validate() = nil, want error containing %q", tc.wantErr)
			case !strings.Contains(err.Error(), tc.wantErr):
				t.Errorf("validate() = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestRejectsUnusableContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		qs   Questions
	}{{
		// A score level has no name to fall back on, so an explicit null
		// describes nothing.
		name: "explicit null score level",
		qs:   Questions{"a": Score("Rate", ScoreCriteria{"low", Null})},
	}, {
		// json.Number has a string Kind but marshals as a bare number.
		name: "json.Number instructions",
		qs:   Questions{"a": Noul(json.Number("5"))},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.qs.validate(); err == nil {
				t.Errorf("validate() = nil, want an error for %s", tc.name)
			}
		})
	}
}
