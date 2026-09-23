package pkgbuild

import "strings"

// SrcInfo is a parsed .SRCINFO file. Unlike the PKGBUILD it is inert
// key-value text and safe to read directly. Fields from the pkgbase section
// and all pkgname sections are merged; the linter only does presence and
// equality checks.
type SrcInfo struct {
	Fields map[string][]string
}

// ParseSrcInfo parses .SRCINFO content.
func ParseSrcInfo(data []byte) *SrcInfo {
	si := &SrcInfo{Fields: map[string][]string{}}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		si.Fields[k] = append(si.Fields[k], v)
	}
	return si
}

// Get returns the first value for key.
func (s *SrcInfo) Get(key string) (string, bool) {
	if vals := s.Fields[key]; len(vals) > 0 {
		return vals[0], true
	}
	return "", false
}

// All returns every value for key.
func (s *SrcInfo) All(key string) []string {
	return s.Fields[key]
}
