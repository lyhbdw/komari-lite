package public

import "testing"

func TestAgentAssetVersionMatchesLiteRelease(t *testing.T) {
	if AgentAssetVersion != "1.0.9" {
		t.Fatalf("AgentAssetVersion = %q, want 1.0.9", AgentAssetVersion)
	}
}
