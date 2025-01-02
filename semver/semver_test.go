package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSemver(t *testing.T) {
	testCases := []struct {
		name        string
		tag         string
		expectedVer *Version
		expectError bool
	}{
		{
			name: "Standard version",
			tag:  "v1.2.3",
			expectedVer: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
			},
		},
		{
			name: "Prefixed version",
			tag:  "org/v1.2.3",
			expectedVer: &Version{
				Prefix: "org/",
				Major:  1,
				Minor:  2,
				Patch:  3,
			},
		},
		{
			name: "Pre-release version",
			tag:  "v1.2.3-rc",
			expectedVer: &Version{
				Major:         1,
				Minor:         2,
				Patch:         3,
				PreRelease:    "rc",
				PreReleaseNum: 0,
			},
		},
		{
			name: "Prefixed pre-release version",
			tag:  "org/v1.2.3-rc.1",
			expectedVer: &Version{
				Prefix:        "org/",
				Major:         1,
				Minor:         2,
				Patch:         3,
				PreRelease:    "rc",
				PreReleaseNum: 1,
			},
		},
		{
			name: "Build metadata version",
			tag:  "v1.2.3+21AF26D3",
			expectedVer: &Version{
				Major:         1,
				Minor:         2,
				Patch:         3,
				PreRelease:    "",
				PreReleaseNum: 0,
			},
		},
		{
			name:        "Invalid version",
			tag:         "invalid-tag",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			version, err := ParseSemver(tc.tag)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedVer, version)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	testCases := []struct {
		name     string
		v1       *Version
		v2       *Version
		expected bool
	}{
		{
			name:     "v1 major version higher",
			v1:       &Version{Major: 2, Minor: 0, Patch: 0},
			v2:       &Version{Major: 1, Minor: 9, Patch: 9},
			expected: true,
		},
		{
			name:     "v1 minor version higher",
			v1:       &Version{Major: 1, Minor: 2, Patch: 0},
			v2:       &Version{Major: 1, Minor: 1, Patch: 9},
			expected: true,
		},
		{
			name:     "v1 patch version higher",
			v1:       &Version{Major: 1, Minor: 1, Patch: 2},
			v2:       &Version{Major: 1, Minor: 1, Patch: 1},
			expected: true,
		},
		{
			name:     "v1 pre-release version higher",
			v1:       &Version{Major: 1, Minor: 1, Patch: 1, PreRelease: "rc", PreReleaseNum: 2},
			v2:       &Version{Major: 1, Minor: 1, Patch: 1, PreRelease: "rc", PreReleaseNum: 1},
			expected: true,
		},
		{
			name:     "Identical versions",
			v1:       &Version{Major: 1, Minor: 1, Patch: 1},
			v2:       &Version{Major: 1, Minor: 1, Patch: 1},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CompareSemver(tc.v1, tc.v2)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCalculateNextVersion(t *testing.T) {
	testCases := []struct {
		name        string
		currentTag  string
		incMajor    bool
		incMinor    bool
		incPatch    bool
		suffix      string
		allTags     []string
		expectedTag string
		expectError bool
	}{
		{
			name:        "Invalid tag format",
			currentTag:  "invalid-tag",
			expectError: true,
		},
		{
			name:        "Next tag already exists error",
			currentTag:  "v1.2.3",
			allTags:     []string{"v1.2.3", "v1.3.0"},
			expectedTag: "v1.2.4",
		},
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
			name:        "Patch increment with suffix",
			currentTag:  "v1.2.3",
			incPatch:    true,
			suffix:      "alpha",
			expectedTag: "v1.2.4-alpha",
		},
		{
			name:        "Minor increment",
			currentTag:  "v1.2.3",
			incMinor:    true,
			expectedTag: "v1.3.0",
		},
		{
			name:        "Minor increment with suffix",
			currentTag:  "v1.2.3",
			incMinor:    true,
			suffix:      "alpha",
			expectedTag: "v1.3.0-alpha",
		},
		{
			name:        "Major increment",
			currentTag:  "v1.2.3",
			incMajor:    true,
			expectedTag: "v2.0.0",
		},
		{
			name:        "Major increment with suffix",
			currentTag:  "v1.2.3",
			incMajor:    true,
			suffix:      "alpha",
			expectedTag: "v2.0.0-alpha",
		},
		{
			name:        "Pre-release increment with pre-release version",
			currentTag:  "v1.2.3-rc.1",
			suffix:      "rc",
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
			expectedTag: "v1.2.3-rc.1",
		},
		{
			name:        "Add suffix to version",
			currentTag:  "v1.2.3",
			suffix:      "beta",
			expectedTag: "v1.2.3-beta",
		},
		{
			name:        "Add suffix to pre-release version",
			currentTag:  "v1.1.1-beta",
			suffix:      "beta",
			expectedTag: "v1.1.1-beta.1",
		},
		{
			name:        "Override existing pre-release with suffix",
			currentTag:  "v1.2.3-rc.1",
			suffix:      "beta",
			expectedTag: "v1.2.3-beta.1",
		},
		{
			name:        "Override existing pre-release with suffix",
			currentTag:  "v1.2.3-rc.1",
			suffix:      "beta",
			allTags:     []string{"v1.2.3-rc.1", "v1.2.3-beta.2"},
			expectedTag: "v1.2.3-beta.3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nextVersion, err := CalculateNextVersion(tc.currentTag, tc.allTags, tc.incMajor, tc.incMinor, tc.incPatch, tc.suffix)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTag, nextVersion)
			}
		})
	}
}
