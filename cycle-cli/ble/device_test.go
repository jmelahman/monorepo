package ble

import "testing"

func TestIsTrainerName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Zwift match", "Zwift Trainer", true},
		{"KICKR match", "KICKR BIKE", true},
		{"Tacx match", "Tacx NEO", true},
		{"Wahoo match", "Wahoo KICKR", true},
		{"Elite match", "Elite Suito", true},
		{"Partial match", "My Zwift Device", true},
		{"Lowercase match", "tacx neo", true},
		{"No match", "Some Random Device", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTrainerName(tt.input)
			if got != tt.expected {
				t.Errorf("isTrainerName(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
