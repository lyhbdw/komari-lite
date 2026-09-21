package version

import "testing"

func TestCurrentReleaseVersion(t *testing.T) {
	if Current != "1.0.5" {
		t.Fatalf("Current = %q, want 1.0.5", Current)
	}
}
