package version

import "testing"

func TestCurrentReleaseVersion(t *testing.T) {
	if Current != "1.0.8" {
		t.Fatalf("Current = %q, want 1.0.7", Current)
	}
}
