package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateNextVersion(t *testing.T) {
	testCases := []struct {
		name        string
		currentTag  string
		incMajor    bool
		incMinor    bool
		incPatch    bool
		expectedTag string
		expectError bool
	}{
		{
			name:        "Patch increment",
			currentTag:  "v1.2.3",
			expectedTag: "v1.2.4",
		},
		{
			name:        "Patch increment with incPatch",
			currentTag:  "v1.2.3",
			incPatch:    true,
			expectedTag: "v1.2.4",
		},
		{
			name:        "Minor increment",
			currentTag:  "v1.2.3",
			incMinor:    true,
			expectedTag: "v1.3.0",
		},
		{
			name:        "Major increment",
			currentTag:  "v1.2.3",
			incMajor:    true,
			expectedTag: "v2.0.0",
		},
		{
			name:        "Pre-release increment with pre-release version",
			currentTag:  "v1.2.3-rc.1",
			expectedTag: "v1.2.3-rc.2",
		},
		{
			name:        "Pre-release increment with patch version",
			currentTag:  "v1.2.3-rc.1",
			incPatch:    true,
			expectedTag: "v1.2.4-rc",
		},
		{
			name:        "Pre-release increment with minor version",
			currentTag:  "v1.2.3-rc.1",
			incMinor:    true,
			expectedTag: "v1.3.0-rc",
		},
		{
			name:        "Pre-release increment with major version",
			currentTag:  "v1.2.3-rc.1",
			incMajor:    true,
			expectedTag: "v2.0.0-rc",
		},
		{
			name:        "Pre-release without number",
			currentTag:  "v1.2.3-rc",
			expectedTag: "v1.2.4",
		},
		{
			name:        "Invalid tag format",
			currentTag:  "invalid-tag",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nextVersion, err := CalculateNextVersion(tc.currentTag, tc.incMajor, tc.incMinor, tc.incPatch)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTag, nextVersion)
			}
		})
	}
}
