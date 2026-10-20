package utils

import "os"

// CurrentVersion and VersionHash accept release metadata from go build -X.
// Keep constant initializers: function-call initializers overwrite linker values.
// Explicit nonempty environment overrides remain supported for existing
// deployments and database upgrade/rollback identities.
var (
	CurrentVersion = "1.0.0"
	VersionHash    = "lite"
)

func init() {
	CurrentVersion = envOr("KOMARI_VERSION", CurrentVersion)
	VersionHash = envOr("KOMARI_VERSION_HASH", VersionHash)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
