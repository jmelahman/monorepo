package semver

import (
	"fmt"
	"regexp"
	"strconv"
)

func CalculateNextVersion(tag string, incMajor, incMinor bool) (string, error) {
	re := regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return "", fmt.Errorf("invalid semver tag: %s", tag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	if incMajor {
		major++
		minor = 0
		patch = 0
	} else if incMinor {
		minor++
		patch = 0
	} else {
		patch++
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}
