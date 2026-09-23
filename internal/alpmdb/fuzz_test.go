package alpmdb

import "testing"

// FuzzParseDesc covers the pacman local-database desc parser. The database is
// host state, not something pkglint controls, so parsing must tolerate
// arbitrary bytes without panicking.
func FuzzParseDesc(f *testing.F) {
	f.Add([]byte("%NAME%\nglibc\n\n%VERSION%\n2.39-1\n\n%DEPENDS%\nlinux-api-headers\n\n%PROVIDES%\nlibc.so=6-64\n"))
	f.Add([]byte("%NAME%\n\n%PROVIDES%\nlibfoo.so>=1\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := parseDesc(data)
		for _, prov := range p.Provides {
			_ = DepName(prov)
		}
	})
}
