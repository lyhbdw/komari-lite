package server

import "testing"

func TestVersionMixedPrefixesProtectDowngrades(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		less bool
	}{
		{"1.1.4", "v1.1.3", false}, {"v1.1.3", "1.1.4", true},
		{"v1.1.4", "1.1.3", false}, {"1.1.3", "v1.1.4", true},
		{"1.1.4", "v1.1.4", false}, {"v1.1.4", "1.1.4", false},
		{"1.1.4", "Snapshot-261001", false}, {"v1.1.4", "Snapshot-261001", false},
		{"1.1.4", "snapshot-261001", false},
		{"Snapshot-261001", "1.1.4", false},
	} {
		if got := versionLess(tc.a, tc.b); got != tc.less {
			t.Errorf("versionLess(%q, %q) = %v; want %v", tc.a, tc.b, got, tc.less)
		}
	}
}
