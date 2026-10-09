package version

import "testing"

func TestCurrentReleaseVersion(t *testing.T) {
	if Current != "1.1.0" {
		t.Fatalf("Current = %q, want 1.1.0", Current)
	}
}
