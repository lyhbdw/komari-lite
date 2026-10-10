package public

import "testing"

func TestAgentAssetVersionMatchesLiteRelease(t *testing.T) {
	if AgentAssetVersion != "v1.1.3" {
		t.Fatalf("AgentAssetVersion = %q, want v1.1.3", AgentAssetVersion)
	}
}
