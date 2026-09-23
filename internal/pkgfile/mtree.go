package pkgfile

import (
	"bytes"
	"compress/gzip"
	"io"
	"strconv"
	"strings"
	"time"
)

// maxMTreeBytes bounds the decompressed .MTREE text. A real .MTREE is one
// line per file — a few KiB for most packages, single-digit MiB for the
// largest — while the gzip member it arrives in is capped at 64 MiB of
// compressed bytes, which could expand a thousandfold. Past the ceiling the
// map is dropped rather than truncated: a partial map would silently exempt
// every file after the cut from the rules that read it.
const maxMTreeBytes = 64 << 20

// parseMTree extracts per-file mtimes from a .MTREE member (gzip-compressed
// BSD mtree text). Only the time attribute is kept; everything else the rules
// need is already on the tar headers.
func parseMTree(data []byte) map[string]time.Time {
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil
		}
		plain, err := io.ReadAll(io.LimitReader(gz, maxMTreeBytes+1))
		if err != nil || len(plain) > maxMTreeBytes {
			return nil
		}
		data = plain
	}
	out := map[string]time.Time{}
	var defaultTime time.Time
	haveDefault := false
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "/set" || fields[0] == "/unset" {
			haveDefault = false
			for _, f := range fields[1:] {
				if t, ok := mtreeTime(f); ok && fields[0] == "/set" {
					defaultTime, haveDefault = t, true
				}
			}
			continue
		}
		name := unescapeMTree(strings.TrimPrefix(fields[0], "./"))
		t, ok := defaultTime, haveDefault
		for _, f := range fields[1:] {
			if ft, fok := mtreeTime(f); fok {
				t, ok = ft, true
			}
		}
		if ok && name != "" {
			out[name] = t
		}
	}
	return out
}

// mtreeTime parses a "time=1699999999.0" attribute.
func mtreeTime(field string) (time.Time, bool) {
	val, ok := strings.CutPrefix(field, "time=")
	if !ok {
		return time.Time{}, false
	}
	sec, _, _ := strings.Cut(val, ".")
	n, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(n, 0), true
}

// unescapeMTree undoes mtree's \ooo octal escapes (spaces and other specials
// in member names) plus doubled backslashes.
func unescapeMTree(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		if i+3 < len(s) && isOctal(s[i+1]) && isOctal(s[i+2]) && isOctal(s[i+3]) {
			n, _ := strconv.ParseUint(s[i+1:i+4], 8, 8)
			b.WriteByte(byte(n))
			i += 3
			continue
		}
		if i+1 < len(s) {
			i++
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func isOctal(c byte) bool { return c >= '0' && c <= '7' }
