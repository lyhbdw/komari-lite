package dbcore

import (
	"gorm.io/gorm"
	"gorm.io/driver/sqlite"
	"testing"
)

// OpenTestDB opens an isolated in-memory SQLite database for tests.
func OpenTestDB(t *testing.T) *gorm.DB {
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
func SwapInstance(t *testing.T, db *gorm.DB) {
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
