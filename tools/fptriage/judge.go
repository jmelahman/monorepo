package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/jmelahman/pkglint/internal/rules"
	"github.com/jmelahman/typesafe-sdk-go"
)

// Question names are ours; the model never sees them, so each question's
// instructions carry their full meaning.
const (
	questionFP    = "fp"
	questionCause = "cause"
)

// promptVersion names the wording of the questions below. It is part of every
// verdict's cache key, so bump it whenever the questions or criteria change:
// a verdict answers the question as it was asked.
//
// Version 1 asked whether the code has "the problem the rule documentation
// describes", and the model graded against the documentation's wording — a
// prefix confined to the build tree still "lacks DESTDIR". Version 2 asks
// about the harm the rule exists to prevent.
const promptVersion = 2

// preamble opens every question: the model sees only the state and the
// question, so each one has to say what it is looking at.
const preamble = "pkglint reported this finding on the excerpt of an Arch Linux PKGBUILD or install scriptlet (the flagged line is marked with >). "

// causes are the explanations the model picks between. The descriptions are
// written against pkglint's failure modes rather than generic ones: the report
// is only useful if "parser-limitation" and "rule-too-broad" land on different
// rules, since they call for different fixes (the parser vs. the rule).
var causes = typesafe.ChoiceCriteria{
	"correct":           "The finding is right: the harm the rule exists to prevent can really happen from the flagged code, even if it is minor.",
	"parser-limitation": "pkglint misread the shell: the flagged construct does not do what the finding's message claims it does.",
	"rule-too-broad":    "The code does what the message says, but the harm cannot happen here — for example it reaches the same safety by a route the rule does not recognize.",
	"intentional":       "The maintainer knowingly did this for a stated reason: a comment or the surrounding code explains why it is needed here.",
}

// state is what the model judges: the finding in its context, plus the rule's
// own contract so the judgment is against what pkglint promises rather than
// the model's taste in PKGBUILDs. Every field is a string — the API takes no
// bare numbers — so the line lives in the excerpt, where it is marked.
type state struct {
	Rule        string `json:"rule"`
	RuleDoc     string `json:"rule_documentation"`
	BadExample  string `json:"example_the_rule_flags,omitempty"`
	GoodExample string `json:"example_the_rule_accepts,omitempty"`
	Finding     string `json:"finding"`
	File        string `json:"file"`
	Excerpt     string `json:"excerpt"`
}

func buildRequest(rule rules.Rule, c candidate, excerpt string) typesafe.SystemOneRequest {
	return typesafe.SystemOneRequest{
		State: state{
			Rule:        rule.ID + " " + rule.Name,
			RuleDoc:     rule.Doc,
			BadExample:  rule.Bad,
			GoodExample: rule.Good,
			Finding:     c.Finding.Message,
			File:        c.File,
			Excerpt:     excerpt,
		},
		Questions: typesafe.Questions{
			questionFP: typesafe.Noul(preamble +
				"Judge it by the harm the rule exists to prevent, as its documentation explains it, not by whether the code uses the exact form the documentation recommends. " +
				"Is the finding a false positive: is that harm absent from the flagged code, for example because the code achieves the same safety another way?").
				WithCriteria(typesafe.NoulCriteria{
					True:  "The harm is absent: pkglint misread the code, or the code is safe in the way the rule cares about even though it does not use the form the documentation recommends.",
					False: "The harm is present or plausible: the flagged code can really cause what the rule exists to prevent, even if the effect is minor.",
				}),
			questionCause: typesafe.Choice(preamble+"What best explains why it was reported?", causes),
		},
	}
}

// judgeOne asks one finding's questions and reads the typed answers.
func judgeOne(ctx context.Context, client *typesafe.Client, req typesafe.SystemOneRequest) (fp float64, cause string, confidence float64, requestID string, err error) {
	res, err := client.SystemOne(ctx, req)
	if err != nil {
		return 0, "", 0, "", err
	}
	n, err := res.Noul(questionFP)
	if err != nil {
		return 0, "", 0, "", err
	}
	c, err := res.Choice(questionCause)
	if err != nil {
		return 0, "", 0, "", err
	}
	return n.Noul, c.Choice, c.Confidence, res.Meta.RequestID, nil
}

// Excerpt bounds. A line can be a base64 blob or a thousand-entry array
// written on one line; the model needs its start, not all of it.
const (
	headLines   = 60
	maxLineRune = 300
)

// excerpt renders the lines around line (1-based), numbered, with the flagged
// line marked. A file-level finding (line 0), or one whose line the file no
// longer has, gets the head of the file.
func excerpt(raw []byte, line, around int) string {
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	lo, hi := 1, min(len(lines), headLines)
	if line > 0 && line <= len(lines) {
		lo, hi = max(1, line-around), min(len(lines), line+around)
	}
	var b bytes.Buffer
	for n := lo; n <= hi; n++ {
		mark := " "
		if n == line {
			mark = ">"
		}
		text := lines[n-1]
		if r := []rune(text); len(r) > maxLineRune {
			text = string(r[:maxLineRune]) + " …"
		}
		fmt.Fprintf(&b, "%s%4d  %s\n", mark, n, text)
	}
	return b.String()
}
