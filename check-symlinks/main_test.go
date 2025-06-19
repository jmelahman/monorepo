package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func setUp() {
	// Create a valid absolute link
	must(os.Remove("testdata/valid_absolute_link"))
	must(os.Symlink("/etc/hosts", "testdata/valid_absolute_link"))
	// Build the check-symlinks binary
	must(exec.Command("go", "build").Run())
}

func expectOutcome(t *testing.T, files []string, expectedCode int) {
	cmd := exec.Command("./check-symlinks", files...)
	output, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != expectedCode {
			t.Errorf("Test failed with case: %v\nExpected exit code: %d\nActual exit code: %d\nOutput: %s", files, expectedCode, exitErr.ExitCode(), output)
		}
	} else if err != nil {
		t.Errorf("Test failed with case: %v\nUnexpected error: %v\nOutput: %s", files, err, output)
	} else if expectedCode != 0 {
		t.Errorf("Test failed with case: %v\nExpected exit code: %d\nActual exit code: 0\nOutput: %s", files, expectedCode, output)
	}
}

func TestMain(m *testing.M) {
	setUp()
	code := m.Run()
	_ = os.Remove("testdata/valid_absolute_link")
	os.Exit(code)
}

func TestExpectSuccess(t *testing.T) {
	tests := [][]string{
		{""},
		{"testdata/doesnt_exist"},
		{"testdata/root_owned_file"},
		{"testdata/some_file"},
		{"testdata/valid_link"},
		{"testdata/.hidden_dir"},
		{"testdata/.hidden_dir/hidden_file"},
		{"--hidden", "testdata/.hidden_dir/hidden_file"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt, " "), func(t *testing.T) {
			expectOutcome(t, tt, 0)
		})
	}
}

func TestExpectFailure(t *testing.T) {
	tests := [][]string{
		{"testdata/broken_link"},
		{"testdata/recursive_broken_link"},
		{"--hidden", "testdata/.hidden_broken_link"},
		{"--hidden", "testdata/.hidden_dir/hidden_broken_link"},
		{"testdata/broken_link", "testdata/some_file", "testdata/valid_link", "", "doesnt_exist"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt, " "), func(t *testing.T) {
			expectOutcome(t, tt, 1)
		})
	}
}

func TestExpectError(t *testing.T) {
	tests := [][]string{
		{"--foo"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt, " "), func(t *testing.T) {
			expectOutcome(t, tt, 2)
		})
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
