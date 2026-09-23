package cli

import (
	"solod.dev/so/strings"
)

// maxEntries bounds how many root entries a single run can manage.
const maxEntries = 256

// cleanEntryArg normalizes a user-supplied entry path like "./.claude/" to ".claude".
func cleanEntryArg(arg string) string {
	s := strings.TrimPrefix(arg, "./")
	s = strings.TrimSuffix(s, "/")
	return s
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// hasExactLine reports whether any line equals s once trimmed of whitespace.
func hasExactLine(lines []string, s string) bool {
	for _, ln := range lines {
		if strings.TrimSpace(ln) == s {
			return true
		}
	}
	return false
}
