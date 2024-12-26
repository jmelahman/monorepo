package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestSemverTag(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		mockCmdOutput  string
		mockCmdError   error
		expectedResult string
		expectError    bool
	}{
		{
			name:           "Successful tag retrieval",
			mockCmdOutput:  "v1.2.3\n",
			expectedResult: "v1.2.3",
		},
		{
			name:           "No tags found",
			mockCmdError:   &exec.Error{},
			expectedResult: "v0.0.0",
		},
	}

	// Original implementation of getLatestSemverTag replaced with testable version
	originalGetLatestSemverTag := getLatestSemverTag
	defer func() { getLatestSemverTag = originalGetLatestSemverTag }()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getLatestSemverTag = func() (string, error) {
				if tc.mockCmdError != nil {
					return "v0.0.0", nil
				}
				return tc.mockCmdOutput, nil
			}

			result, err := getLatestSemverTag()
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestCreateAndPushTag(t *testing.T) {
	testCases := []struct {
		name        string
		tag         string
		createError error
		pushError   error
		expectError bool
	}{
		{
			name: "Successful tag creation and push",
			tag:  "v1.2.3",
		},
		{
			name:        "Tag creation fails",
			tag:         "v1.2.3",
			createError: assert.AnError,
			expectError: true,
		},
		{
			name:       "Tag push fails",
			tag:        "v1.2.3",
			pushError:  assert.AnError,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock exec.Command
			oldExecCommand := exec.Command
			defer func() { exec.Command = oldExecCommand }()

			exec.Command = func(name string, arg ...string) *exec.Cmd {
				cmd := oldExecCommand(name, arg...)
				
				if name == "git" && arg[0] == "tag" {
					if tc.createError != nil {
						cmd.Run = func() error { return tc.createError }
					}
				}
				
				if name == "git" && arg[0] == "push" {
					if tc.pushError != nil {
						cmd.Run = func() error { return tc.pushError }
					}
				}
				
				return cmd
			}

			err := createAndPushTag(tc.tag)
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
