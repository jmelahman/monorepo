package utils

import (
	"testing"
)

func TestFormatTwoColumns(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]string
		separator string
		expected  string
	}{
		{
			name:      "empty data",
			data:      map[string]string{},
			separator: ": ",
			expected:  "",
		},
		{
			name: "single item",
			data: map[string]string{
				"key": "value",
			},
			separator: ": ",
			expected:  "key: value",
		},
		{
			name: "multiple items with alignment",
			data: map[string]string{
				"short":     "value1",
				"very_long": "value2",
				"medium":    "value3",
			},
			separator: ": ",
			expected:  "short:     value1\nvery_long: value2\nmedium:    value3",
		},
		{
			name: "different separator",
			data: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			separator: " = ",
			expected:  "key1 = val1\nkey2 = val2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTwoColumns(tt.data, tt.separator)

			// For maps, we need to check that all expected lines are present
			// since map iteration order is not guaranteed
			if tt.name == "empty data" || tt.name == "single item" {
				if result != tt.expected {
					t.Errorf("FormatTwoColumns() = %q, want %q", result, tt.expected)
				}
			} else {
				// Check that result contains all expected lines
				expectedLines := map[string]bool{
					"short:     value1": true,
					"very_long: value2": true,
					"medium:    value3": true,
				}

				if tt.name == "different separator" {
					expectedLines = map[string]bool{
						"key1 = val1": true,
						"key2 = val2": true,
					}
				}

				// Split result into lines and verify each line is expected
				lines := splitLines(result)
				if len(lines) != len(expectedLines) {
					t.Errorf("FormatTwoColumns() returned %d lines, want %d", len(lines), len(expectedLines))
				}

				for _, line := range lines {
					if !expectedLines[line] {
						t.Errorf("FormatTwoColumns() returned unexpected line: %q", line)
					}
				}
			}
		})
	}
}

func TestFormatTwoColumnsOrdered(t *testing.T) {
	tests := []struct {
		name      string
		keys      []string
		data      map[string]string
		separator string
		expected  string
	}{
		{
			name:      "empty data",
			keys:      []string{},
			data:      map[string]string{},
			separator: ": ",
			expected:  "",
		},
		{
			name: "single item",
			keys: []string{"key"},
			data: map[string]string{
				"key": "value",
			},
			separator: ": ",
			expected:  "key: value",
		},
		{
			name: "ordered items with alignment",
			keys: []string{"very_long", "short", "medium"},
			data: map[string]string{
				"short":     "value1",
				"very_long": "value2",
				"medium":    "value3",
			},
			separator: ": ",
			expected:  "very_long: value2\nshort:     value1\nmedium:    value3",
		},
		{
			name: "missing keys in data",
			keys: []string{"key1", "missing", "key2"},
			data: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			separator: ": ",
			expected:  "key1:    value1\nkey2:    value2",
		},
		{
			name: "extra keys in data",
			keys: []string{"key1", "key2"},
			data: map[string]string{
				"key1":  "value1",
				"key2":  "value2",
				"extra": "ignored",
			},
			separator: ": ",
			expected:  "key1: value1\nkey2: value2",
		},
		{
			name: "different separator",
			keys: []string{"alpha", "beta"},
			data: map[string]string{
				"alpha": "first",
				"beta":  "second",
			},
			separator: " -> ",
			expected:  "alpha -> first\nbeta ->  second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTwoColumnsOrdered(tt.keys, tt.data, tt.separator)
			if result != tt.expected {
				t.Errorf("FormatTwoColumnsOrdered() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatTwoColumnsAlignment(t *testing.T) {
	data := map[string]string{
		"a":     "1",
		"bb":    "22",
		"ccc":   "333",
		"dddd":  "4444",
		"eeeee": "55555",
	}

	result := FormatTwoColumnsOrdered([]string{"a", "bb", "ccc", "dddd", "eeeee"}, data, ": ")
	expected := "a:     1\nbb:    22\nccc:   333\ndddd:  4444\neeeee: 55555"

	if result != expected {
		t.Errorf("FormatTwoColumnsOrdered() alignment test failed\nGot:\n%q\nWant:\n%q", result, expected)
	}
}

// Helper function to split string by newlines
func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	lines := []string{}
	current := ""
	for _, char := range s {
		if char == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
