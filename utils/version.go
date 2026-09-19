package utils

var (
	// Keep the source fallback aligned with the upstream base release. Release
	// builds may override both values with -ldflags.
	CurrentVersion = "1.5.0-fix1"
	VersionHash    = "lite"
)
