package public

import (
	"os"
	"testing"

	"github.com/lyhbdw/komari-lite/cmd/flags"
	"github.com/lyhbdw/komari-lite/database/dbcore"
)

func TestMain(m *testing.M) {
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:web_api_public_test?mode=memory&cache=shared"

	db := dbcore.GetDBInstance()
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	os.Exit(m.Run())
}
