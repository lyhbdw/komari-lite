package admin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyWhitelistedFilesSkipsLiveMetricsDatabase(t *testing.T) {
	dataDir := t.TempDir()
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "metrics.db"), []byte("live"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyWhitelistedFilesFrom(dataDir, tempDir); err != nil {
		t.Fatalf("copy whitelist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "metrics.db")); !os.IsNotExist(err) {
		t.Fatalf("metrics.db was copied by the generic whitelist")
	}
}
