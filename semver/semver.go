package semver

import (
	"fmt"
	"regexp"
	"strconv"
)

func CalculateNextVersion(tag string, incMajor, incMinor, incPatch bool) (string, error) {
	// Regex to match semver with optional pre-release
	re := regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z]+)(?:\.(\d*)))?`)
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return "", fmt.Errorf("invalid semver tag: %s", tag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	preRelease := matches[4]
	preReleaseNum := matches[5]

	if incMajor {
		major++
		minor = 0
		patch = 0
		preReleaseNum = ""
	} else if incMinor {
		minor++
		patch = 0
		preReleaseNum = ""
	} else if incPatch || preRelease == "" {
		patch++
		preReleaseNum = ""
	} else {
		// If pre-release exists but has no number, increment patch
		if preReleaseNum == "" {
			patch++
		} else {
			// Try to increment pre-release number
			num, err := strconv.Atoi(preReleaseNum)
			if err != nil {
				// If not a valid number, increment patch
				patch++
			} else {
				preReleaseNum = strconv.Itoa(num + 1)
			}
		}
	}

	// Construct the version string
	if preRelease != "" && preReleaseNum != "" {
		return fmt.Sprintf("v%d.%d.%d-%s.%s", major, minor, patch, preRelease, preReleaseNum), nil
	} else if preRelease != "" {
		return fmt.Sprintf("v%d.%d.%d-%s", major, minor, patch, preRelease), nil
	}
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}
