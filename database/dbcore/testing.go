package dbcore

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testContext keeps cross-package fixture helpers compatible with testing.T
// without importing the testing runtime into the production dependency graph.
type testContext interface {
	Helper()
	Fatalf(string, ...any)
	Cleanup(func())
}

// OpenTestDB opens an isolated in-memory SQLite database for tests.
func OpenTestDB(t testContext) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// SwapInstance replaces the global DB instance for the duration of the test
// and restores the previous value on cleanup.
func SwapInstance(t testContext, db *gorm.DB) {
	t.Helper()
	prev := instance
	prevErr := initErr
	once.Do(func() {})
	instance = db
	initErr = nil
	t.Cleanup(func() {
		instance = prev
		initErr = prevErr
	})
}
