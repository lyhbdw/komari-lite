package dbcore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCopyUpgradeDataSkipsStaleStagingDirs guards the regression where a failed
// createUpgradeBackup run left its .upgrade-backup-* staging directory behind
// (defer os.RemoveAll never ran). The next backup's copyUpgradeData walked the
// whole dataDir and copied that residue into the new staging directory, so the
// resulting upgrade-*.zip accumulated every past failure and eventually failed
// to write.
func TestCopyUpgradeDataSkipsStaleStagingDirs(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// The prefix is spelled out literally (not via the upgradeStagingDirPrefix
	// constant) so this test also compiles against a source tree that predates
	// the fix, where that constant does not exist.
	const prefix = ".upgrade-backup-"

	mainDB := filepath.Join(srcDir, "komari.db")
	metricsDB := filepath.Join(srcDir, "metrics.db")
	backupDir := filepath.Join(srcDir, "backup")
	stagingDir := filepath.Join(srcDir, prefix+"current")

	for _, dir := range []string{backupDir, stagingDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, path := range []string{mainDB, metricsDB} {
		if err := os.WriteFile(path, []byte("db"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	// A stale staging directory left by a previous failed run.
	staleDir := filepath.Join(srcDir, prefix+"stale")
	if err := os.MkdirAll(staleDir, 0o755); err != nil {
		t.Fatalf("mkdir stale: %v", err)
	}
	staleMetrics := filepath.Join(staleDir, "metrics.db")
	if err := os.WriteFile(staleMetrics, []byte("stale metrics payload"), 0o600); err != nil {
		t.Fatalf("write stale metrics: %v", err)
	}
	// Ordinary data that must be preserved in the snapshot.
	themeDir := filepath.Join(srcDir, "theme", "Emerald")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "komari-theme.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write theme: %v", err)
	}

	if err := copyUpgradeData(srcDir, dstDir, mainDB, metricsDB, backupDir, stagingDir); err != nil {
		t.Fatalf("copyUpgradeData: %v", err)
	}

	// No staging residue may be copied into the snapshot.
	for _, dir := range []string{staleDir, stagingDir} {
		rel, _ := filepath.Rel(srcDir, dir)
		if _, err := os.Stat(filepath.Join(dstDir, rel)); !os.IsNotExist(err) {
			t.Errorf("staging dir %s was copied into snapshot (err=%v)", rel, err)
		}
	}
	// Real data must survive.
	if _, err := os.Stat(filepath.Join(dstDir, "theme", "Emerald", "komari-theme.json")); err != nil {
		t.Errorf("theme data missing from snapshot: %v", err)
	}
}
