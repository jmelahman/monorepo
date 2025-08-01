package semver

import (
	"fmt"
	"regexp"
	"strconv"
)

type Version struct {
	Prefix        string
	Major         int
	Minor         int
	Patch         int
	PreRelease    string
	PreReleaseNum int
}

func (v *Version) String() string {
	versionStr := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)

	if v.Prefix != "" {
		versionStr = v.Prefix + versionStr
	}

	if v.PreRelease != "" {
		versionStr = fmt.Sprintf("%s-%s", versionStr, v.PreRelease)
		if v.PreReleaseNum > 0 {
			versionStr = fmt.Sprintf("%s.%d", versionStr, v.PreReleaseNum)
		}
	}
	return versionStr
}

func ParseSemver(tag string) (*Version, error) {
	re := regexp.MustCompile(`(?:(.+/))?v(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z]+)(?:\.(\d+))?)?`)
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return nil, fmt.Errorf("invalid semver tag: %s", tag)
	}

	prefix := matches[1]
	major, _ := strconv.Atoi(matches[2])
	minor, _ := strconv.Atoi(matches[3])
	patch, _ := strconv.Atoi(matches[4])
	preRelease := matches[5]
	preReleaseNum := 0
	if len(matches) > 6 && matches[6] != "" {
		preReleaseNum, _ = strconv.Atoi(matches[6])
	}

	return &Version{
		Prefix:        prefix,
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
	} else if incPatch {
		version.Patch++
		version.PreReleaseNum = 0
	} else if suffix == "" {
		version.Patch++
		version.PreReleaseNum = 0
	} else if version.PreRelease != suffix {
		// TODO: This should probably only consider tags with HEAD as an ancestor.
		// Find the largest PreReleaseNum for the given suffix
		largestPreReleaseNum := 0
		for _, existingTag := range allTags {
			existingVersion, err := ParseSemver(existingTag)
			if err == nil && existingVersion.PreRelease == suffix {
				if existingVersion.PreReleaseNum > largestPreReleaseNum {
					largestPreReleaseNum = existingVersion.PreReleaseNum
				}
			}
		}
		if largestPreReleaseNum > 0 {
			version.PreReleaseNum = largestPreReleaseNum + 1
		}
	} else {
		version.PreReleaseNum++
	}

	version.PreRelease = suffix

	return version.String(), nil
}
