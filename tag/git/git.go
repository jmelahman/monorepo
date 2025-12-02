package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jmelahman/tag/semver"
	log "github.com/sirupsen/logrus"
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

func GetLatestSemverTag(prefix, suffix string) (string, error) {
	tagPattern := genTagPattern(prefix, suffix)
	log.WithFields(log.Fields{
		"pattern": tagPattern,
		"prefix":  prefix,
		"suffix":  suffix,
	}).Debug("GetLatestSemverTag")
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", tagPattern)
	output, err := cmd.Output()
	if err != nil {
		log.Debug("GetLatestSemverTag: no tags found matching pattern, returning v0.0.0")
		if prefix == "" {
			return "v0.0.0", nil
		} else {
			return fmt.Sprintf("%s/v0.0.0", prefix), nil
		}
	}

	matchedTag := strings.TrimSpace(string(output))
	log.WithField("matchedTag", matchedTag).Debug("GetLatestSemverTag: git describe matched tag")

	tagsAt, err := ListTagsAt(matchedTag)
	if err != nil {
		log.WithError(err).WithField("matchedTag", matchedTag).Debug("GetLatestSemverTag: error listing tags at")
		if prefix == "" {
			return "v0.0.0", nil
		} else {
			return fmt.Sprintf("%s/v0.0.0", prefix), nil
		}
	}

	log.WithField("count", len(tagsAt)).WithField("matchedTag", matchedTag).Debug("GetLatestSemverTag: found tags at")

	var largestTag string
	var largestVersion *semver.Version

	for _, tag := range tagsAt {
		if tag == "" {
			continue
		}

		version, err := semver.ParseSemver(tag)
		if err != nil {
			log.WithError(err).WithField("tag", tag).Debug("GetLatestSemverTag: failed to parse tag")
			continue
		}

		// Check prefix: if prefix is empty, version.Prefix should be empty; if prefix is set, version.Prefix should be "prefix/"
		expectedPrefix := ""
		if prefix != "" {
			expectedPrefix = prefix + "/"
		}
		if version.Prefix != expectedPrefix {
			log.WithFields(log.Fields{
				"tag":            tag,
				"expectedPrefix": expectedPrefix,
				"actualPrefix":   version.Prefix,
			}).Debug("GetLatestSemverTag: tag prefix mismatch")
			continue
		}

		if suffix != "" && version.PreRelease != suffix {
			log.WithFields(log.Fields{
				"tag":            tag,
				"expectedSuffix": suffix,
				"actualSuffix":   version.PreRelease,
			}).Debug("GetLatestSemverTag: tag suffix mismatch")
			continue
		}

		if largestVersion == nil {
			largestTag = tag
			largestVersion = version
			log.WithField("tag", tag).Debug("GetLatestSemverTag: initializing largest tag")
			continue
		}

		if semver.CompareSemver(version, largestVersion) {
			log.WithFields(log.Fields{
				"newerTag": tag,
				"olderTag": largestTag,
			}).Debug("GetLatestSemverTag: found newer tag")
			largestTag = tag
			largestVersion = version
		}
	}

	log.WithField("latestTag", largestTag).Debug("GetLatestSemverTag: returning latest tag")

	return largestTag, nil
}

// List all git tags
func ListTags(prefix, suffix string) ([]string, error) {
	tagPattern := genTagPattern(prefix, suffix)
	log.WithFields(log.Fields{
		"pattern": tagPattern,
		"prefix":  prefix,
		"suffix":  suffix,
	}).Debug("ListTags")
	cmd := exec.Command("git", "tag", "-l", tagPattern)
	cmd.Stderr = os.Stderr
	tagsOutput, err := cmd.Output()
	if err != nil {
		log.WithError(err).Debug("ListTags: error running git tag -l")
		return nil, err
	}

	tagList := strings.Split(strings.TrimSpace(string(tagsOutput)), "\n")
	log.WithField("count", len(tagList)).Debug("ListTags: git returned tags (before filtering)")

	// Filter by suffix if specified, since git pattern matching might not be precise enough
	if suffix != "" {
		var filteredList []string
		for _, tag := range tagList {
			if tag == "" {
				continue
			}
			version, err := semver.ParseSemver(tag)
			if err != nil {
				log.WithError(err).WithField("tag", tag).Debug("ListTags: failed to parse tag")
				continue
			}
			if version.PreRelease == suffix {
				filteredList = append(filteredList, tag)
				log.WithField("tag", tag).WithField("suffix", suffix).Debug("ListTags: tag matches suffix")
			} else {
				log.WithFields(log.Fields{
					"tag":            tag,
					"expectedSuffix": suffix,
					"actualSuffix":   version.PreRelease,
				}).Debug("ListTags: tag suffix mismatch")
			}
		}
		log.WithFields(log.Fields{
			"count":  len(filteredList),
			"suffix": suffix,
		}).Debug("ListTags: after filtering, found tags matching suffix")
		return filteredList, nil
	}

	log.WithField("count", len(tagList)).Debug("ListTags: returning tags")

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

func TagExists(tag string) (bool, error) {
	tagRef := fmt.Sprintf("refs/tags/%s", tag)
	log.WithField("tag", tag).Debug("TagExists: checking if tag exists")
	cmd := exec.Command("git", "show-ref", "--tags", "--quiet", tagRef)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			log.WithField("tag", tag).Debug("TagExists: tag does not exist")
			return false, nil
		}
		log.WithError(err).WithField("tag", tag).Debug("TagExists: error checking tag")
		return false, err
	}
	log.WithField("tag", tag).Debug("TagExists: tag exists")
	return true, nil
}

func CreateAndPushTag(tag string, remote string) error {
	log.WithField("tag", tag).Debug("CreateAndPushTag: creating tag")
	cmd := exec.Command("git", "tag", tag)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.WithError(err).WithField("tag", tag).Debug("CreateAndPushTag: error creating tag")
		return fmt.Errorf("failed to create tag: %w", err)
	}

	log.WithFields(log.Fields{
		"tag":    tag,
		"remote": remote,
	}).Debug("CreateAndPushTag: pushing tag to remote")
	cmd = exec.Command("git", "push", "--quiet", remote, tag)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"tag":    tag,
			"remote": remote,
		}).Debug("CreateAndPushTag: error pushing tag")
		return fmt.Errorf("failed to push tag to %s: %w", remote, err)
	}

	log.WithField("tag", tag).Debug("CreateAndPushTag: successfully created and pushed tag")
	return nil
}

func FetchSemverTags(remote string, prefix, suffix string) error {
	// When suffix is specified, greedily fetch all matching tags (git refspecs don't support
	// wildcards in the middle like v*-suffix*). We'll filter by suffix in the code.
	var tagPattern string
	if prefix != "" {
		tagPattern = fmt.Sprintf("refs/tags/%s/v*:refs/tags/%s/v*", prefix, prefix)
	} else {
		tagPattern = "refs/tags/v*:refs/tags/v*"
	}
	log.WithFields(log.Fields{
		"remote":  remote,
		"refspec": tagPattern,
		"suffix":  suffix,
	}).Debug("FetchSemverTags: fetching from remote (suffix will be filtered later)")
	cmd := exec.Command("git", "fetch", "--quiet", "--prune", remote, tagPattern)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.WithError(err).Debug("FetchSemverTags: error fetching tags")
		return fmt.Errorf("failed to fetch tags from %s: %w", remote, err)
	}
	log.Debug("FetchSemverTags: successfully fetched tags")
	return nil
}

// GetLatestStableSemverTag returns the latest stable (non-pre-release) semver tag
func GetLatestStableSemverTag(prefix string) (string, error) {
	// Get all tags matching the base pattern (without suffix)
	tagPattern := "v[0-9]*.[0-9]*.[0-9]*"
	if prefix != "" {
		tagPattern = fmt.Sprintf("%s/%s", prefix, tagPattern)
	}
	log.WithFields(log.Fields{
		"pattern": tagPattern,
		"prefix":  prefix,
	}).Debug("GetLatestStableSemverTag")

	cmd := exec.Command("git", "tag", "-l", tagPattern)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		log.WithError(err).Debug("GetLatestStableSemverTag: error listing tags")
		if prefix == "" {
			return "v0.0.0", nil
		}
		return fmt.Sprintf("%s/v0.0.0", prefix), nil
	}

	tagList := strings.Split(strings.TrimSpace(string(output)), "\n")
	log.WithField("count", len(tagList)).Debug("GetLatestStableSemverTag: found tags")

	var largestTag string
	var largestVersion *semver.Version

	expectedPrefix := ""
	if prefix != "" {
		expectedPrefix = prefix + "/"
	}

	for _, tag := range tagList {
		if tag == "" {
			continue
		}

		version, err := semver.ParseSemver(tag)
		if err != nil {
			log.WithError(err).WithField("tag", tag).Debug("GetLatestStableSemverTag: failed to parse tag")
			continue
		}

		// Skip tags with different prefix
		if version.Prefix != expectedPrefix {
			continue
		}

		// Skip pre-release versions - we only want stable tags
		if version.PreRelease != "" {
			log.WithField("tag", tag).Debug("GetLatestStableSemverTag: skipping pre-release tag")
			continue
		}

		if largestVersion == nil || semver.CompareSemver(version, largestVersion) {
			largestTag = tag
			largestVersion = version
			log.WithField("tag", tag).Debug("GetLatestStableSemverTag: found newer stable tag")
		}
	}

	if largestTag == "" {
		if prefix == "" {
			return "v0.0.0", nil
		}
		return fmt.Sprintf("%s/v0.0.0", prefix), nil
	}

	log.WithField("latestTag", largestTag).Debug("GetLatestStableSemverTag: returning latest stable tag")
	return largestTag, nil
}

// GetTagAtHEAD returns the semver tag at HEAD, or empty string if none exists
func GetTagAtHEAD(prefix, suffix string) (string, error) {
	tagPattern := genTagPattern(prefix, suffix)
	log.WithFields(log.Fields{
		"pattern": tagPattern,
		"prefix":  prefix,
		"suffix":  suffix,
	}).Debug("GetTagAtHEAD")
	cmd := exec.Command("git", "tag", "--points-at", "HEAD", "--list", tagPattern)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		log.WithError(err).Debug("GetTagAtHEAD: error checking tags")
		return "", fmt.Errorf("failed to check tags for HEAD: %w", err)
	}
	tags := strings.TrimSpace(string(output))
	if tags == "" {
		log.Debug("GetTagAtHEAD: no tags found at HEAD")
		return "", nil
	}

	// If multiple tags exist, find the largest one
	tagList := strings.Split(tags, "\n")
	var largestTag string
	var largestVersion *semver.Version

	for _, tag := range tagList {
		if tag == "" {
			continue
		}
		version, err := semver.ParseSemver(tag)
		if err != nil {
			log.WithError(err).WithField("tag", tag).Debug("GetTagAtHEAD: failed to parse tag")
			continue
		}

		// Check suffix match if specified
		if suffix != "" && version.PreRelease != suffix {
			continue
		}

		if largestVersion == nil || semver.CompareSemver(version, largestVersion) {
			largestTag = tag
			largestVersion = version
		}
	}

	log.WithField("tag", largestTag).Debug("GetTagAtHEAD: returning tag at HEAD")
	return largestTag, nil
}

// IsAncestor checks if ancestorRef is an ancestor of descendantRef
func IsAncestor(ancestorRef, descendantRef string) (bool, error) {
	log.WithFields(log.Fields{
		"ancestor":   ancestorRef,
		"descendant": descendantRef,
	}).Debug("IsAncestor: checking ancestry")
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestorRef, descendantRef)
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			log.Debug("IsAncestor: not an ancestor")
			return false, nil
		}
		log.WithError(err).Debug("IsAncestor: error checking ancestry")
		return false, err
	}
	log.Debug("IsAncestor: is an ancestor")
	return true, nil
}

func IsHEADAlreadyTagged(prefix, suffix string) (bool, error) {
	tagPattern := genTagPattern(prefix, suffix)
	log.WithFields(log.Fields{
		"pattern": tagPattern,
		"prefix":  prefix,
		"suffix":  suffix,
	}).Debug("IsHEADAlreadyTagged")
	cmd := exec.Command("git", "tag", "--points-at", "HEAD", "--list", tagPattern)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		log.WithError(err).Debug("IsHEADAlreadyTagged: error checking tags")
		return false, fmt.Errorf("failed to check tags for HEAD: %w", err)
	}
	tags := strings.TrimSpace(string(output))
	if tags == "" {
		log.Debug("IsHEADAlreadyTagged: no tags found at HEAD")
		return false, nil
	}
	log.WithField("tags", tags).Debug("IsHEADAlreadyTagged: found tags at HEAD")
	// If suffix is specified, verify that at least one tag matches the suffix
	if suffix != "" {
		tagList := strings.Split(tags, "\n")
		for _, tag := range tagList {
			if tag == "" {
				continue
			}
			version, err := semver.ParseSemver(tag)
			if err != nil {
				log.WithError(err).WithField("tag", tag).Debug("IsHEADAlreadyTagged: failed to parse tag")
				continue
			}
			if version.PreRelease == suffix {
				log.WithFields(log.Fields{
					"tag":    tag,
					"suffix": suffix,
				}).Debug("IsHEADAlreadyTagged: tag matches suffix")
				return true, nil
			}
			log.WithFields(log.Fields{
				"tag":            tag,
				"expectedSuffix": suffix,
				"actualSuffix":   version.PreRelease,
			}).Debug("IsHEADAlreadyTagged: tag suffix mismatch")
		}
		log.WithField("suffix", suffix).Debug("IsHEADAlreadyTagged: no tags at HEAD match suffix")
		return false, nil
	}
	log.Debug("IsHEADAlreadyTagged: HEAD is tagged (suffix not specified)")
	return true, nil
}
