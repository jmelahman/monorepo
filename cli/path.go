package cli

import (
	"solod.dev/so/os"
)

// joinPath writes dir + "/" + name into buf and returns it as a string view.
// The result is only valid until the next call with the same buffer.
func joinPath(buf []byte, dir string, name string) string {
	n := copy(buf, dir)
	if n > 0 && buf[n-1] != '/' && n < len(buf) {
		buf[n] = '/'
		n++
	}
	n += copy(buf[n:], name)
	return string(buf[:n])
}

// isHidden reports whether a path element is hidden.
func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.' && name != "." && name != ".."
}

// baseName returns the last element of a slash-separated path.
func baseName(p string) string {
	i := lastSlash(p)
	if i < 0 {
		return p
	}
	return p[i+1:]
}

// lastSlash returns the index of the last '/' in p, or -1.
func lastSlash(p string) int {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return i
		}
	}
	return -1
}

// findTopLevel walks up from start looking for a .git directory.
// The result is a view into start.
func findTopLevel(start string) string {
	var buf [os.MaxPathLen]byte
	dir := start
	for {
		p := joinPath(buf[:], dir, ".git")
		fi, err := os.Stat(p)
		if err == nil && fi.IsDir() {
			return dir
		}
		i := lastSlash(dir)
		if i <= 0 {
			return ""
		}
		dir = dir[:i]
	}
}
