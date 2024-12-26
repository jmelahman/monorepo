package semver

import (
	"fmt"
	"regexp"
	"strconv"
)

func CalculateNextVersion(tag string, incMajor, incMinor bool) (string, error) {
	// Regex to match semver with optional pre-release
	re := regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z]+)(\d*))?`)
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
		preRelease = ""
		preReleaseNum = ""
	} else if incMinor {
		minor++
		patch = 0
		preRelease = ""
		preReleaseNum = ""
	} else {
		if preRelease != "" {
			if preReleaseNum == "" {
				preReleaseNum = "1"
			} else {
				num, _ := strconv.Atoi(preReleaseNum)
				preReleaseNum = strconv.Itoa(num + 1)
			}
		} else {
			patch++
		}
	}

	// Construct the version string
	if preRelease != "" && preReleaseNum != "" {
		return fmt.Sprintf("v%d.%d.%d-%s%s", major, minor, patch, preRelease, preReleaseNum), nil
	}
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}
