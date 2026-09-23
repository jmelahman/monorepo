package server

import "testing"

func TestCacheCapFromDisk(t *testing.T) {
	const gb = int64(1 << 30)
	cases := []struct {
		name     string
		fsBytes  int64
		fraction float64
		want     int64
	}{
		{"60 GB worker root at the default fraction", 60 * gb, workerCacheFraction, 30 * gb},
		{"200 GB volume", 200 * gb, workerCacheFraction, 100 * gb},
		{"unknown size is uncapped", 0, workerCacheFraction, 0},
		{"negative size is uncapped", -1, workerCacheFraction, 0},
		{"zero fraction is uncapped", 60 * gb, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cacheCapFromDisk(tc.fsBytes, tc.fraction); got != tc.want {
				t.Fatalf("cacheCapFromDisk(%d, %v) = %d, want %d", tc.fsBytes, tc.fraction, got, tc.want)
			}
		})
	}
}

// TestWorkerCacheCapReadsRealFilesystem: the derivation runs against a real
// directory and lands strictly between 0 and the filesystem's size.
func TestWorkerCacheCapReadsRealFilesystem(t *testing.T) {
	capBytes, fsBytes, err := workerCacheCap(t.TempDir())
	if err != nil {
		t.Skipf("filesystem size unavailable here: %v", err)
	}
	if fsBytes <= 0 || capBytes <= 0 || capBytes >= fsBytes {
		t.Fatalf("workerCacheCap = cap %d of fs %d, want 0 < cap < fs", capBytes, fsBytes)
	}
}
