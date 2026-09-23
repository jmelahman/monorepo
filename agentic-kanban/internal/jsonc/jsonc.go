// Package jsonc handles "JSON with comments" — the dialect used by
// devcontainer.json and VS Code config files, which permit // line comments,
// /* */ block comments, and trailing commas that the encoding/json decoder
// rejects.
//
// It is a dependency-free leaf package so that internal/docker, internal/tasks,
// and any future caller can share one implementation instead of each keeping a
// byte-identical copy.
package jsonc

// Strip returns data with comments and trailing commas removed — the JSONC
// extensions devcontainer.json and VS Code config files permit. Comments are
// stripped first so a comment sitting between a trailing comma and its
// closing bracket doesn't hide the comma.
func Strip(data []byte) []byte {
	return stripTrailingCommas(StripComments(data))
}

// StripComments removes // line comments and /* */ block comments from data,
// returning JSON the standard library can decode. Comment markers inside
// string literals are preserved (the scanner tracks string and escape state),
// so URLs and regexes in values survive untouched.
func StripComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	escape := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(data) {
			if data[i+1] == '/' {
				for i < len(data) && data[i] != '\n' {
					i++
				}
				if i < len(data) {
					out = append(out, data[i])
				}
				continue
			}
			if data[i+1] == '*' {
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// stripTrailingCommas removes commas whose next non-whitespace byte closes an
// object or array. String and escape state are tracked so commas inside
// literals survive.
func stripTrailingCommas(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	escape := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}
