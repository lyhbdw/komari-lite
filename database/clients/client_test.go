package clients

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lyhbdw/komari-lite/cmd/flags"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
)

func TestCreateClientEntrypoints(t *testing.T) {
	oldFile, oldType := flags.DatabaseFile, flags.DatabaseType
	flags.DatabaseFile = filepath.Join(t.TempDir(), "clients.db")
	flags.DatabaseType = "sqlite"
	t.Cleanup(func() {
		InvalidateClientsCache()
		_ = dbcore.Close()
		flags.DatabaseFile, flags.DatabaseType = oldFile, oldType
	})
	if err := dbcore.Initialize(); err != nil {
		t.Fatal(err)
	}
	db := dbcore.GetDBInstance()
	on := models.PingTask{Name: "default-on", DefaultOn: true, Clients: models.StringArray{"existing"}, Target: "localhost"}
	off := models.PingTask{Name: "default-off", Clients: models.StringArray{"existing"}, Target: "localhost"}
	for _, task := range []*models.PingTask{&on, &off} {
		if err := db.Create(task).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name     string
		create   func() (string, string, error)
		wantName string
	}{
		{"default", CreateClient, ""},
		{"empty", func() (string, string, error) { return CreateClientWithName("") }, ""},
		{"named", func() (string, string, error) { return CreateClientWithName("  named node  ") }, "  named node  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			beforeClients, err := GetAllClientBasicInfo()
			if err != nil {
				t.Fatal(err)
			}
			before := time.Now().UTC()
			id, token, err := tc.create()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := uuid.Parse(id); err != nil {
				t.Fatalf("invalid UUID: %v", err)
			}
			client, err := GetClientByUUID(id)
			if err != nil {
				t.Fatal(err)
			}
			wantName := tc.wantName
			if wantName == "" {
				wantName = "client_" + id[:8]
			}
			if client.Name != wantName || token == "" || client.Token != token {
				t.Fatalf("creation fields mismatch: name=%q token match=%v", client.Name, token != "" && client.Token == token)
			}
			for _, stamp := range []time.Time{client.CreatedAt, client.UpdatedAt} {
				_, offset := stamp.Zone()
				if stamp.Before(before) || stamp.After(time.Now().UTC()) || offset != 0 {
					t.Fatalf("invalid UTC timestamp: %v", stamp)
				}
			}
			afterClients, err := GetAllClientBasicInfo()
			if err != nil {
				t.Fatal(err)
			}
			if len(afterClients) != len(beforeClients)+1 {
				t.Fatal("creation did not invalidate client cache")
			}
			for _, task := range []*models.PingTask{&on, &off} {
				if err := db.First(task, task.Id).Error; err != nil {
					t.Fatal(err)
				}
				if task.AppliesToClient(id) != task.DefaultOn {
					t.Fatalf("incorrect default-on assignment: %s", task.Name)
				}
				if !task.AppliesToClient("existing") {
					t.Fatal("existing assignment lost")
				}
			}
		})
	}
}
