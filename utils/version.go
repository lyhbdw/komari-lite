package utils

import "os"

// CurrentVersion is the fork's own release identifier, keeping the upstream
// 1.5.0 API baseline under its own label.
//
// It is read from KOMARI_VERSION (falling back to the constant) rather than
// being an untyped constant, because dbcore.backupOnVersionUpgrade derives
// its version identity from utils.CurrentVersion + "-" + VersionHash: a value
// that never changes makes every rebuild look like an upgrade, re-zipping the
// entire data directory on each start.
var (
	CurrentVersion = envOr("KOMARI_VERSION", "1.0.0")
	VersionHash    = envOr("KOMARI_VERSION_HASH", "lite")
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
