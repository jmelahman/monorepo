package ble

import (
	"bytes"
	"testing"
)

// The expected bytes are the packets the app sent before command() existed.
func TestCommand(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
		want []byte
	}{
		{"power on", command(0x11, 0x01), []byte{0xA0, 0x11, 0x04, 0x01, 0xB1, 0x21}},
		{"power off", command(0x11, 0x00), []byte{0xA0, 0x11, 0x04, 0x00, 0x70, 0xE1}},
		{"brightness 0", command(0x13, brightnessLevels[0]), []byte{0xA0, 0x13, 0x04, 0x01, 0x10, 0xE1}},
		{"brightness 7", command(0x13, brightnessLevels[7]), []byte{0xA0, 0x13, 0x04, 0x29, 0x10, 0xFF}},
		{"brightness 15", command(0x13, brightnessLevels[15]), []byte{0xA0, 0x13, 0x04, 0x64, 0xD0, 0xCA}},
	}
	for _, tt := range tests {
		if !bytes.Equal(tt.got, tt.want) {
			t.Errorf("%s: got % X, want % X", tt.name, tt.got, tt.want)
		}
	}
}

func TestSetBrightnessOutOfRange(t *testing.T) {
	d := &Device{}
	for _, level := range []int{-1, MaxBrightness + 1} {
		if err := d.SetBrightness(level); err == nil {
			t.Errorf("SetBrightness(%d): expected error", level)
		}
	}
}
