package clients_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lyhbdw/komari-lite/database/clients"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
	"github.com/lyhbdw/komari-lite/web/rpc/jsonrpc"
	"gorm.io/gorm"
)

func TestAdminOrderClientsInvalidatesCacheOnPartialFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "partial failure"
		}
		t.Run(name, func(t *testing.T) {
			db := dbcore.OpenTestDB(t)
			if err := db.AutoMigrate(&models.Client{}, &models.Log{}); err != nil {
				t.Fatal(err)
			}
			dbcore.SwapInstance(t, db)
			clients.InvalidateClientsCache()
			t.Cleanup(clients.InvalidateClientsCache)
			for _, id := range []string{"first", "second"} {
				if err := db.Create(&models.Client{UUID: id, Token: "synthetic-" + id}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if _, err := clients.GetAllClientBasicInfo(); err != nil {
				t.Fatal(err)
			}
			var writes int
			if err := db.Callback().Update().Before("gorm:update").Register(t.Name(), func(tx *gorm.DB) {
				writes++
				if fail && writes == 2 {
					tx.AddError(errors.New("later order write failed"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			resp := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Principal: rpc.NewUserPrincipal("admin")}, &rpc.JsonRpcRequest{
				Version: rpc.RPC_VERSION, ID: 1, Method: "admin:orderClients", Params: map[string]int{"first": 7, "second": 8},
			})
			if fail {
				if resp.Error == nil || !strings.HasPrefix(resp.Error.Message, "Failed to update client weight: ") {
					t.Fatalf("error changed: %+v", resp)
				}
			} else if resp.Error != nil {
				t.Fatal(resp.Error)
			}
			var persisted []models.Client
			if err := db.Find(&persisted).Error; err != nil {
				t.Fatal(err)
			}
			list, err := clients.GetAllClientBasicInfo()
			if err != nil || len(list) != 2 {
				t.Fatalf("client list: %+v, %v", list, err)
			}
			weights := make(map[string]int, len(persisted))
			for _, client := range persisted {
				weights[client.UUID] = client.Weight
			}
			for _, client := range list {
				if client.Weight != weights[client.UUID] {
					t.Fatalf("order write not invalidated: client=%q cached=%d persisted=%d", client.UUID, client.Weight, weights[client.UUID])
				}
			}
		})
	}
}
