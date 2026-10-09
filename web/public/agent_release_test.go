package public

import "testing"

func TestAgentAssetVersionMatchesLiteRelease(t *testing.T) {
	if AgentAssetVersion != "1.1.0" {
		t.Fatalf("AgentAssetVersion = %q, want 1.1.0", AgentAssetVersion)
	}
}
