package server

// workerCacheFraction is the share of the data directory's filesystem a
// worker fills with hydrated artifacts before the cache sweeper trims it,
// when --cache-max-artifact-bytes is left at 0. A worker's disk is nothing
// but caches — hydrated artifacts and the docker data-root — and the
// artifact side is the one that grows without bound: every distinct preview
// served hydrates 1–2 GB and, uncapped, nothing ever reclaims it (the first
// fleet filled a 60 GB root disk in an afternoon and every hydrate after that
// died with ENOSPC). Half leaves the rest to the OS, run images, container
// layers, and hydrate scratch space.
const workerCacheFraction = 0.5

// workerCacheCap derives a worker's default resident-artifact cap from the
// total size of the filesystem holding dataDir. The pure derivation is
// cacheCapFromDisk; the filesystem read is per-OS.
func workerCacheCap(dataDir string) (capBytes, fsBytes int64, err error) {
	fsBytes, err = filesystemBytes(dataDir)
	if err != nil {
		return 0, 0, err
	}
	return cacheCapFromDisk(fsBytes, workerCacheFraction), fsBytes, nil
}

// cacheCapFromDisk is fraction of a filesystem's total bytes, or 0 (uncapped)
// when the size is unknown.
func cacheCapFromDisk(fsBytes int64, fraction float64) int64 {
	if fsBytes <= 0 || fraction <= 0 {
		return 0
	}
	return int64(float64(fsBytes) * fraction)
}
