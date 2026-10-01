package public

import "testing"

func TestAgentAssetVersionMatchesLiteRelease(t *testing.T) {
	if AgentAssetVersion != "1.0.8" {
		t.Fatalf("AgentAssetVersion = %q, want 1.0.8", AgentAssetVersion)
	}
}
