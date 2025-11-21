package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jmelahman/tag/semver"
	"github.com/sirupsen/logrus"
)

func genTagPattern(prefix, suffix string) string {
	tagPattern := "v[0-9]*.[0-9]*.[0-9]*"
	if suffix != "" {
		tagPattern = fmt.Sprintf("%s-%s*", tagPattern, suffix)
	}
	if prefix != "" {
		tagPattern = fmt.Sprintf("%s/%s", prefix, tagPattern)
	}
	return tagPattern
}

func GetLatestSemverTag(prefix, suffix string, logger *logrus.Logger) (string, error) {
	tagPattern := genTagPattern(prefix, suffix)
	logger.WithFields(logrus.Fields{
		"pattern": tagPattern,
		"prefix":  prefix,
		"suffix":  suffix,
	}).Debug("GetLatestSemverTag")
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", tagPattern)
	output, err := cmd.Output()
	if err != nil {
		logger.Debug("GetLatestSemverTag: no tags found matching pattern, returning v0.0.0")
		if prefix == "" {
			return "v0.0.0", nil
		} else {
			return fmt.Sprintf("%s/v0.0.0", prefix), nil
		}
	}

	matchedTag := strings.TrimSpace(string(output))
	logger.WithField("matchedTag", matchedTag).Debug("GetLatestSemverTag: git describe matched tag")

	tagsAt, err := ListTagsAt(matchedTag)
	if err != nil {
		logger.WithError(err).WithField("matchedTag", matchedTag).Debug("GetLatestSemverTag: error listing tags at")
		if prefix == "" {
			return "v0.0.0", nil
		} else {
			return fmt.Sprintf("%s/v0.0.0", prefix), nil
		}
	}

	logger.WithField("count", len(tagsAt)).WithField("matchedTag", matchedTag).Debug("GetLatestSemverTag: found tags at")

	var largestTag string
	var largestVersion *semver.Version

	for _, tag := range tagsAt {
		if tag == "" {
			continue
		}

		version, err := semver.ParseSemver(tag)
		if err != nil {
			logger.WithError(err).WithField("tag", tag).Debug("GetLatestSemverTag: failed to parse tag")
			continue
		}

		// Check prefix: if prefix is empty, version.Prefix should be empty; if prefix is set, version.Prefix should be "prefix/"
		expectedPrefix := ""
		if prefix != "" {
			expectedPrefix = prefix + "/"
		}
		if version.Prefix != expectedPrefix {
			logger.WithFields(logrus.Fields{
				"tag":            tag,
				"expectedPrefix": expectedPrefix,
				"actualPrefix":   version.Prefix,
			}).Debug("GetLatestSemverTag: tag prefix mismatch")
			continue
		}

		if suffix != "" && version.PreRelease != suffix {
			logger.WithFields(logrus.Fields{
				"tag":          tag,
				"expectedSuffix": suffix,
				"actualSuffix": version.PreRelease,
			}).Debug("GetLatestSemverTag: tag suffix mismatch")
			continue
		}

		if largestVersion == nil {
			largestTag = tag
			largestVersion = version
			logger.WithField("tag", tag).Debug("GetLatestSemverTag: initializing largest tag")
			continue
		}

		if semver.CompareSemver(version, largestVersion) {
			logger.WithFields(logrus.Fields{
				"newerTag": tag,
				"olderTag": largestTag,
			}).Debug("GetLatestSemverTag: found newer tag")
			largestTag = tag
			largestVersion = version
		}
	}

	logger.WithField("latestTag", largestTag).Debug("GetLatestSemverTag: returning latest tag")

	return largestTag, nil
}

// List all git tags
func ListTags(prefix, suffix string, logger *logrus.Logger) ([]string, error) {
	tagPattern := genTagPattern(prefix, suffix)
	logger.WithFields(logrus.Fields{
		"pattern": tagPattern,
		"prefix":  prefix,
		"suffix":  suffix,
	}).Debug("ListTags")
	cmd := exec.Command("git", "tag", "-l", tagPattern)
	cmd.Stderr = os.Stderr
	tagsOutput, err := cmd.Output()
	if err != nil {
		logger.WithError(err).Debug("ListTags: error running git tag -l")
		return nil, err
	}

	tagList := strings.Split(strings.TrimSpace(string(tagsOutput)), "\n")
	logger.WithField("count", len(tagList)).Debug("ListTags: git returned tags (before filtering)")

	// Filter by suffix if specified, since git pattern matching might not be precise enough
	if suffix != "" {
		var filteredList []string
		for _, tag := range tagList {
			if tag == "" {
				continue
			}
			version, err := semver.ParseSemver(tag)
			if err != nil {
				logger.WithError(err).WithField("tag", tag).Debug("ListTags: failed to parse tag")
				continue
			}
			if version.PreRelease == suffix {
				filteredList = append(filteredList, tag)
				logger.WithField("tag", tag).WithField("suffix", suffix).Debug("ListTags: tag matches suffix")
			} else {
				logger.WithFields(logrus.Fields{
					"tag":          tag,
					"expectedSuffix": suffix,
					"actualSuffix": version.PreRelease,
				}).Debug("ListTags: tag suffix mismatch")
			}
		}
		logger.WithFields(logrus.Fields{
			"count": len(filteredList),
			"suffix": suffix,
		}).Debug("ListTags: after filtering, found tags matching suffix")
		return filteredList, nil
	}

	logger.WithField("count", len(tagList)).Debug("ListTags: returning tags")

	return tagList, nil
}

func ListTagsAt(ref string) ([]string, error) {
	cmd := exec.Command("git", "tag", "--points-at", ref)
	cmd.Stderr = os.Stderr
	tagsOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	tagList := strings.Split(strings.TrimSpace(string(tagsOutput)), "\n")

	return tagList, nil
}

func TagExists(tag string, logger *logrus.Logger) (bool, error) {
	tagRef := fmt.Sprintf("refs/tags/%s", tag)
	logger.WithField("tag", tag).Debug("TagExists: checking if tag exists")
	cmd := exec.Command("git", "show-ref", "--tags", "--quiet", tagRef)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			logger.WithField("tag", tag).Debug("TagExists: tag does not exist")
			return false, nil
		}
		logger.WithError(err).WithField("tag", tag).Debug("TagExists: error checking tag")
		return false, err
	}
	logger.WithField("tag", tag).Debug("TagExists: tag exists")
	return true, nil
}

func CreateAndPushTag(tag string, remote string, logger *logrus.Logger) error {
	logger.WithField("tag", tag).Debug("CreateAndPushTag: creating tag")
	cmd := exec.Command("git", "tag", tag)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.WithError(err).WithField("tag", tag).Debug("CreateAndPushTag: error creating tag")
		return fmt.Errorf("failed to create tag: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"tag":    tag,
		"remote": remote,
	}).Debug("CreateAndPushTag: pushing tag to remote")
	cmd = exec.Command("git", "push", "--quiet", remote, tag)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.WithError(err).WithFields(logrus.Fields{
			"tag":    tag,
			"remote": remote,
		}).Debug("CreateAndPushTag: error pushing tag")
		return fmt.Errorf("failed to push tag to %s: %w", remote, err)
	}

	logger.WithField("tag", tag).Debug("CreateAndPushTag: successfully created and pushed tag")
	return nil
}

func FetchSemverTags(remote string, prefix, suffix string, logger *logrus.Logger) error {
	// When suffix is specified, greedily fetch all matching tags (git refspecs don't support
	// wildcards in the middle like v*-suffix*). We'll filter by suffix in the code.
	var tagPattern string
	if prefix != "" {
		tagPattern = fmt.Sprintf("refs/tags/%s/v*:refs/tags/%s/v*", prefix, prefix)
	} else {
		tagPattern = "refs/tags/v*:refs/tags/v*"
	}
	logger.WithFields(logrus.Fields{
		"remote":     remote,
		"refspec":    tagPattern,
		"suffix":     suffix,
	}).Debug("FetchSemverTags: fetching from remote (suffix will be filtered later)")
	cmd := exec.Command("git", "fetch", "--quiet", "--prune", remote, tagPattern)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.WithError(err).Debug("FetchSemverTags: error fetching tags")
		return fmt.Errorf("failed to fetch tags from %s: %w", remote, err)
	}
	logger.Debug("FetchSemverTags: successfully fetched tags")
	return nil
}

func IsHEADAlreadyTagged(prefix, suffix string, logger *logrus.Logger) (bool, error) {
	tagPattern := genTagPattern(prefix, suffix)
	logger.WithFields(logrus.Fields{
		"pattern": tagPattern,
		"prefix":  prefix,
		"suffix":  suffix,
	}).Debug("IsHEADAlreadyTagged")
	cmd := exec.Command("git", "tag", "--points-at", "HEAD", "--list", tagPattern)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		logger.WithError(err).Debug("IsHEADAlreadyTagged: error checking tags")
		return false, fmt.Errorf("failed to check tags for HEAD: %w", err)
	}
	tags := strings.TrimSpace(string(output))
	if tags == "" {
		logger.Debug("IsHEADAlreadyTagged: no tags found at HEAD")
		return false, nil
	}
	logger.WithField("tags", tags).Debug("IsHEADAlreadyTagged: found tags at HEAD")
	// If suffix is specified, verify that at least one tag matches the suffix
	if suffix != "" {
		tagList := strings.Split(tags, "\n")
		for _, tag := range tagList {
			if tag == "" {
				continue
			}
			version, err := semver.ParseSemver(tag)
			if err != nil {
				logger.WithError(err).WithField("tag", tag).Debug("IsHEADAlreadyTagged: failed to parse tag")
				continue
			}
			if version.PreRelease == suffix {
				logger.WithFields(logrus.Fields{
					"tag":    tag,
					"suffix": suffix,
				}).Debug("IsHEADAlreadyTagged: tag matches suffix")
				return true, nil
			}
			logger.WithFields(logrus.Fields{
				"tag":          tag,
				"expectedSuffix": suffix,
				"actualSuffix": version.PreRelease,
			}).Debug("IsHEADAlreadyTagged: tag suffix mismatch")
		}
		logger.WithField("suffix", suffix).Debug("IsHEADAlreadyTagged: no tags at HEAD match suffix")
		return false, nil
	}
	logger.Debug("IsHEADAlreadyTagged: HEAD is tagged (suffix not specified)")
	return true, nil
}
