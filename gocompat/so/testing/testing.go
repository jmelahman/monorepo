// Package testing mirrors the subset of solod.dev/so/testing needed to run
// the generated test harness (cli/test/main.go) under a regular Go build:
//
//	go run ./cli/test
//
// This exercises the exact same tests against the gocompat implementations.
package testing

import (
	"fmt"
	"os"

	"solod.dev/so/mem"
)

type T struct {
	name   string
	failed bool
}

func (t *T) Name() string             { return t.name }
func (t *T) Allocator() mem.Allocator { return mem.System }
func (t *T) Fail()                    { t.failed = true }
func (t *T) Failed() bool             { return t.failed }
func (t *T) Log(msg string)           { fmt.Printf("    %s\n", msg) }

func (t *T) Error(msg string) {
	fmt.Printf("    %s\n", msg)
	t.failed = true
}

// Fatal marks the test as failed; by solod convention the test function
// must return immediately after calling it.
func (t *T) Fatal(msg string) {
	fmt.Printf("    %s\n", msg)
	t.failed = true
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
		fmt.Printf("=== RUN   %s\n", tests[i].Name)
		tests[i].F(t)
		if t.failed {
			failed++
			fmt.Printf("--- FAIL: %s\n", tests[i].Name)
		} else {
			fmt.Printf("--- PASS: %s\n", tests[i].Name)
		}
	}
	if failed > 0 {
		fmt.Printf("FAIL\t%s\t%d of %d failed\n", pkg, failed, len(tests))
		os.Exit(1)
	}
	fmt.Printf("ok\t%s\t%d tests\n", pkg, len(tests))
}
