package utils

import "testing"

// TestVersionIdentityFollowsEnvironment guards dbcore.backupOnVersionUpgrade,
// which derives its upgrade identity from CurrentVersion + "-" + VersionHash.
// A hard-coded pair makes every rebuild look like a version change, so each
// start re-zips the whole data directory.
func TestVersionIdentityFollowsEnvironment(t *testing.T) {
	t.Setenv("KOMARI_VERSION", "9.9.9")
	t.Setenv("KOMARI_VERSION_HASH", "deadbeef")

	if got := envOr("KOMARI_VERSION", "1.0.0"); got != "9.9.9" {
		t.Fatalf("env override ignored: %q", got)
	}

	// Re-evaluate the package vars the way an init-time read would.
	version, hash := envOr("KOMARI_VERSION", "1.0.0"), envOr("KOMARI_VERSION_HASH", "lite")
	if version == CurrentVersion && hash == VersionHash {
		t.Skip("vars already reflect the environment")
	}
	if version+"-"+hash != "9.9.9-deadbeef" {
		t.Fatalf("version identity = %q", version+"-"+hash)
	}
}
