package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateNextVersion(t *testing.T) {
	testCases := []struct {
		name         string
		currentTag   string
		incMajor     bool
		incMinor     bool
		expectedTag  string
		expectError  bool
	}{
		{
			name:         "Patch increment",
			currentTag:   "v1.2.3",
			expectedTag:  "v1.2.4",
		},
		{
			name:         "Minor increment",
			currentTag:   "v1.2.3",
			incMinor:     true,
			expectedTag:  "v1.3.0",
		},
		{
			name:         "Major increment",
			currentTag:   "v1.2.3",
			incMajor:     true,
			expectedTag:  "v2.0.0",
		},
		{
			name:         "Invalid tag format",
			currentTag:   "invalid-tag",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nextVersion, err := calculateNextVersion(tc.currentTag, tc.incMajor, tc.incMinor)
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTag, nextVersion)
			}
		})
	}
}
