package semver

import (
	"fmt"
	"regexp"
	"strconv"
)

type Version struct {
	Major         int
	Minor         int
	Patch         int
	PreRelease    string
	PreReleaseNum int
}

func ParseSemver(tag string) (*Version, error) {
	re := regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z]+)(?:\.(\d*)))?`)
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return nil, fmt.Errorf("invalid semver tag: %s", tag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	preRelease := matches[4]
	preReleaseNum := 0
	if matches[5] != "" {
		preReleaseNum, _ = strconv.Atoi(matches[5])
	}

	return &Version{
		Major:         major,
		Minor:         minor,
		Patch:         patch,
		PreRelease:    preRelease,
		PreReleaseNum: preReleaseNum,
	}, nil
}

func CompareSemver(v1, v2 *Version) bool {
	if v1.Major > v2.Major {
		return true
	}
	if v1.Major < v2.Major {
		return false
	}
	if v1.Minor > v2.Minor {
		return true
	}
	if v1.Minor < v2.Minor {
		return false
	}
	if v1.Patch > v2.Patch {
		return true
	}
	if v1.Patch < v2.Patch {
		return false
	}

	// Handle pre-release versions
	if v1.PreRelease == "" && v2.PreRelease != "" {
		return true
	}
	if v1.PreRelease != "" && v2.PreRelease == "" {
		return false
	}
	if v1.PreRelease == v2.PreRelease {
		return v1.PreReleaseNum > v2.PreReleaseNum
	}
	return false
}

func CalculateNextVersion(tag string, allTags []string, incMajor, incMinor, incPatch bool, suffix string) (string, error) {
	// Parse the current version
	version, err := ParseSemver(tag)
	if err != nil {
		return "", err
	}

	// Find highest patch version for current major.minor
	highestPatch := -1
	for _, existingTag := range allTags {
		existingVersion, err := ParseSemver(existingTag)
		if err != nil {
			continue
		}
		if existingVersion.Major == version.Major &&
			existingVersion.Minor == version.Minor &&
			existingVersion.Patch > highestPatch {
			highestPatch = existingVersion.Patch
		}
	}

	// Increment version based on flags
	if incMajor {
		version.Major++
		version.Minor = 0
		version.Patch = 0
		version.PreReleaseNum = 0
	} else if incMinor {
		version.Minor++
		version.Patch = 0
		version.PreReleaseNum = 0
	} else if incPatch || version.PreRelease == "" {
		version.Patch++
		version.PreReleaseNum = 0
	} else {
		// If there's a higher patch version, use that instead of incrementing pre-release
		if highestPatch > version.Patch {
			version.Patch = highestPatch
			version.PreReleaseNum = 1
		} else if version.PreReleaseNum == 0 {
			version.Patch++
			version.PreRelease = ""
		} else {
			version.PreReleaseNum++
		}
	}

	// Apply suffix if provided
	if suffix != "" {
		// When adding a new suffix, preserve the original patch version
		if version.PreRelease != suffix {
			version.PreRelease = suffix
			version.PreReleaseNum = 1
			if version.PreRelease == "" {
				version.PreReleaseNum = 0
			}
			// Reset patch only if there was no pre-release before
			if version.PreRelease == "" {
				version.Patch = version.Patch
			}
		}
	}

	// Construct the version string
	if version.PreRelease != "" {
		if version.PreReleaseNum > 0 {
			return fmt.Sprintf("v%d.%d.%d-%s.%d", version.Major, version.Minor, version.Patch, version.PreRelease, version.PreReleaseNum), nil
		}
		return fmt.Sprintf("v%d.%d.%d-%s", version.Major, version.Minor, version.Patch, version.PreRelease), nil
	}
	return fmt.Sprintf("v%d.%d.%d", version.Major, version.Minor, version.Patch), nil
}
