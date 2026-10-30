package project

import "time"

// Berry exposes berry to the package's tests.
func (pm PackageManager) Berry() bool { return pm.berry }

// AgeGate exposes ageGate to the package's tests.
func (pm PackageManager) AgeGate(version string, age time.Duration, now time.Time) (args, env []string, err error) {
	return pm.ageGate(version, age, now)
}
