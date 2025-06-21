package utils

import (
	"fmt"
	"strings"
)

// FormatTwoColumns formats key-value pairs into aligned two columns
func FormatTwoColumns(data map[string]string, separator string) string {
	if len(data) == 0 {
		return ""
	}

	// Find the maximum width of the first column
	maxWidth := 0
	for key := range data {
		if len(key) > maxWidth {
			maxWidth = len(key)
		}
	}

	var lines []string
	for key, value := range data {
		padding := strings.Repeat(" ", maxWidth-len(key))
		lines = append(lines, fmt.Sprintf("%s%s%s%s", key, padding, separator, value))
	}

	return strings.Join(lines, "\n")
}

// FormatTwoColumnsOrdered formats key-value pairs in a specific order
func FormatTwoColumnsOrdered(keys []string, data map[string]string, separator string) string {
	if len(data) == 0 {
		return ""
	}

	// Find the maximum width of the first column
	maxWidth := 0
	for _, key := range keys {
		if len(key) > maxWidth {
			maxWidth = len(key)
		}
	}

	var lines []string
	for _, key := range keys {
		if value, exists := data[key]; exists {
			padding := strings.Repeat(" ", maxWidth-len(key))
			lines = append(lines, fmt.Sprintf("%s%s%s%s", key, padding, separator, value))
		}
	}

	return strings.Join(lines, "\n")
}
