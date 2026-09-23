// Package testing mirrors the subset of solod.dev/so/testing needed to run
// the generated test harness (so/test/main.go) under a regular Go build:
//
//	go -C so run -modfile=gotest.mod ./test
//
// This runs the exact same tests against the gocompat implementations.
package testing

import (
	gofmt "fmt"
	"os"

	"solod.dev/so/mem"
)

type T struct {
	name    string
	failed  bool
	skipped bool
}

func (t *T) Name() string             { return t.name }
func (t *T) Allocator() mem.Allocator { return mem.System }
func (t *T) Fail()                    { t.failed = true }
func (t *T) Failed() bool             { return t.failed }
func (t *T) Log(msg string)           { gofmt.Printf("    %s\n", msg) }

func (t *T) Error(msg string) {
	gofmt.Printf("    %s\n", msg)
	t.failed = true
}

func (t *T) Errorf(format string, args ...any) {
	gofmt.Printf("    "+format+"\n", args...)
	t.failed = true
}

// Fatal marks the test as failed; by solod convention the test function must
// return immediately after calling it.
func (t *T) Fatal(msg string) {
	gofmt.Printf("    %s\n", msg)
	t.failed = true
}

func (t *T) Fatalf(format string, args ...any) {
	gofmt.Printf("    "+format+"\n", args...)
	t.failed = true
}

func (t *T) Skip(msg string) {
	gofmt.Printf("    %s\n", msg)
	t.skipped = true
}

type Test struct {
	Name string
	F    func(*T)
}

func RunTests(pkg string, args []string, tests []Test) {
	_ = args
	failed := 0
	for i := range tests {
		t := &T{name: tests[i].Name}
		gofmt.Printf("=== RUN   %s\n", tests[i].Name)
		tests[i].F(t)
		if t.failed {
			failed++
			gofmt.Printf("--- FAIL: %s\n", tests[i].Name)
		} else {
			gofmt.Printf("--- PASS: %s\n", tests[i].Name)
		}
	}
	if failed > 0 {
		gofmt.Printf("FAIL\t%s\t%d of %d failed\n", pkg, failed, len(tests))
		os.Exit(1)
	}
	gofmt.Printf("ok\t%s\t%d tests\n", pkg, len(tests))
}
