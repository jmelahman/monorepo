package server

import "testing"

func TestWarmFromMemory(t *testing.T) {
	const giB = int64(1) << 30
	tests := []struct {
		name      string
		memGiB    float64
		perGB     float64
		reserveGB float64
		want      int
	}{
		{"16GiB at 1/GiB reserve 1", 16, 1, 1, 15},
		{"16GiB at 0.75/GiB reserve 1", 16, 0.75, 1, 11},
		{"8GiB at 1/GiB reserve 1", 8, 1, 1, 7},
		{"32GiB at 1/GiB reserve 2", 32, 1, 2, 30},
		{"tiny node floors at 1", 2, 0.1, 1, 1},
		{"reserve exceeds RAM floors at 1", 1, 1, 2, 1},
		{"exactly reserve floors at 1", 4, 1, 4, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := warmFromMemory(int64(tc.memGiB*float64(giB)), tc.perGB, tc.reserveGB)
			if got != tc.want {
				t.Errorf("warmFromMemory(%.0f GiB, %.2f/GiB, reserve %.0f) = %d, want %d",
					tc.memGiB, tc.perGB, tc.reserveGB, got, tc.want)
			}
		})
	}
}

func TestWarmFromMemoryNeverZero(t *testing.T) {
	// A 0 cap reads as "unlimited" to the fleet — the opposite of a small
	// node's intent — so calibration must never produce it.
	for memGiB := 1; memGiB <= 64; memGiB++ {
		for _, perGB := range []float64{0.01, 0.5, 1, 2} {
			got := warmFromMemory(int64(memGiB)*(1<<30), perGB, 1)
			if got < 1 {
				t.Fatalf("warmFromMemory(%d GiB, %.2f/GiB, 1) = %d, want >= 1", memGiB, perGB, got)
			}
		}
	}
}
