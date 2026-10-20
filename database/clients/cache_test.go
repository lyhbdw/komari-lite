package clients

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"gorm.io/gorm"
)

func clientCacheFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db := dbcore.OpenTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.Client{}); err != nil {
		t.Fatal(err)
	}
	dbcore.SwapInstance(t, db)
	InvalidateClientsCache()
	t.Cleanup(InvalidateClientsCache)
	return db
}

func TestClientCacheWriteInvalidatesAfterSQL(t *testing.T) {
	for _, operation := range []string{"save", "info", "delete", "create"} {
		t.Run(operation, func(t *testing.T) {
			db := clientCacheFixture(t)
			if err := db.AutoMigrate(&models.PingTask{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&models.Client{UUID: "node", Token: "before"}).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := GetAllClientBasicInfo(); err != nil {
				t.Fatal(err)
			}
			// Read after pre-invalidation but before SQL, without holding a transaction.
			reader := func(tx *gorm.DB) {
				if tx.Statement.Table != "clients" {
					return
				}
				if _, err := GetAllClientBasicInfo(); err != nil {
					tx.AddError(err)
				}
			}
			var err error
			switch operation {
			case "save", "info":
				err = db.Callback().Update().Before("gorm:begin_transaction").Register(t.Name(), reader)
			case "delete":
				err = db.Callback().Delete().Before("gorm:begin_transaction").Register(t.Name(), reader)
			case "create":
				err = db.Callback().Create().Before("gorm:begin_transaction").Register(t.Name(), reader)
			}
			if err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "save":
				err = SaveClient(map[string]interface{}{"uuid": "node", "token": "after"})
			case "info":
				err = SaveClientInfo(map[string]interface{}{"uuid": "node", "token": "after"})
			case "delete":
				err = DeleteClient("node")
			case "create":
				_, _, err = CreateClient()
			}
			if err != nil {
				t.Fatal(err)
			}
			if operation == "create" {
				list, err := GetAllClientBasicInfo()
				if err != nil || len(list) != 2 {
					t.Fatalf("created client missing: %+v, %v", list, err)
				}
				return
			}
			if id, err := GetClientUUIDByToken("before"); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("revoked token still authenticates: %q, %v", id, err)
			}
			if operation != "delete" {
				if id, err := GetClientUUIDByToken("after"); err != nil || id != "node" {
					t.Fatalf("new token: %q, %v", id, err)
				}
			}
		})
	}
}

func TestClientCacheDoesNotRepublishInvalidatedLoad(t *testing.T) {
	db := clientCacheFixture(t)
	if err := db.Create(&models.Client{UUID: "node", Token: "before"}).Error; err != nil {
		t.Fatal(err)
	}
	loaded, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	var paused atomic.Bool
	if err := db.Callback().Query().After("gorm:after_query").Register(t.Name(), func(*gorm.DB) {
		if paused.CompareAndSwap(false, true) {
			close(loaded)
			<-release
		}
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := GetAllClientBasicInfo(); done <- err }()
	joined := false
	defer func() {
		resume()
		if joined {
			return
		}
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("client cache load did not finish")
		}
	}()
	select {
	case <-loaded:
	case <-time.After(5 * time.Second):
		t.Fatal("client cache query did not pause")
	}
	if err := SaveClient(map[string]interface{}{"uuid": "node", "token": "after"}); err != nil {
		t.Fatal(err)
	}
	if _, err := GetAllClientBasicInfo(); err != nil {
		t.Fatal(err)
	}
	resume()
	err := <-done
	joined = true
	if err != nil {
		t.Fatal(err)
	}
	if id, err := GetClientUUIDByToken("before"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("invalidated load republished revoked token: %q, %v", id, err)
	}
	if id, err := GetClientUUIDByToken("after"); err != nil || id != "node" {
		t.Fatalf("new token: %q, %v", id, err)
	}
}

func TestClientMetadataWarmLookupsUseZeroSQL(t *testing.T) {
	db := clientCacheFixture(t)
	if err := db.Create(&models.Client{UUID: "node", Token: "valid"}).Error; err != nil {
		t.Fatal(err)
	}
	var queries atomic.Int64
	if err := db.Callback().Query().Before("gorm:query").Register(t.Name(), func(*gorm.DB) { queries.Add(1) }); err != nil {
		t.Fatal(err)
	}
	if id, err := GetClientUUIDByToken("valid"); err != nil || id != "node" {
		t.Fatalf("cold token lookup: %q, %v", id, err)
	}
	if got := queries.Load(); got != 1 {
		t.Fatalf("cold token lookup used %d queries, want 1", got)
	}
	for i := 0; i < 3; i++ {
		if id, err := GetClientUUIDByToken("valid"); err != nil || id != "node" {
			t.Fatalf("warm token lookup: %q, %v", id, err)
		}
		if token, err := GetClientTokenByUUID("node"); err != nil || token != "valid" {
			t.Fatalf("warm uuid lookup: %q, %v", token, err)
		}
		if _, err := GetClientByUUID("node"); err != nil {
			t.Fatal(err)
		}
		if _, err := GetAllClientBasicInfo(); err != nil {
			t.Fatal(err)
		}
		for _, token := range []string{"", "missing"} {
			if _, err := GetClientUUIDByToken(token); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("unknown token: %v", err)
			}
		}
		if _, err := GetClientByUUID("missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("unknown client: %v", err)
		}
	}
	if got := queries.Load(); got != 1 {
		t.Fatalf("warm metadata lookups used %d SQL queries, want 0", got-1)
	}
}

func TestClientCacheContextPropagation(t *testing.T) {
	db := clientCacheFixture(t)
	type contextKey struct{}
	wantDeadline := time.Now().Add(time.Minute)
	ctx, cancel := context.WithDeadline(context.WithValue(context.Background(), contextKey{}, "caller"), wantDeadline)
	defer cancel()
	var seen context.Context
	var queries int
	if err := db.Callback().Query().Before("gorm:query").Register(t.Name(), func(tx *gorm.DB) {
		seen = tx.Statement.Context
		queries++
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := GetAllClientBasicInfoWithContext(ctx); err != nil {
		t.Fatal(err)
	}
	deadline, ok := seen.Deadline()
	if !ok || !deadline.Equal(wantDeadline) || seen.Value(contextKey{}) != "caller" {
		t.Fatalf("caller context was lost: %v", seen)
	}
	cancel()
	// Existing semantics: a warm result does not touch SQL, even when canceled.
	if list, err := GetAllClientBasicInfoWithContext(ctx); err != nil || list == nil {
		t.Fatalf("warm canceled result: %+v, %v", list, err)
	}
	if queries != 1 {
		t.Fatalf("warm canceled call issued SQL: %d", queries)
	}
	InvalidateClientsCache()
	if _, err := GetAllClientBasicInfoWithContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cold canceled result: %v", err)
	}
	if _, err := GetAllClientBasicInfoWithContext(nil); err != nil {
		t.Fatalf("nil context: %v", err)
	}
}

func TestClientCacheDatabaseIdentityAndEmptyHits(t *testing.T) {
	firstDB := clientCacheFixture(t)
	if err := firstDB.Create(&models.Client{UUID: "same-node", Token: "first-token"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := GetAllClientBasicInfo(); err != nil {
		t.Fatal(err)
	}
	secondDB := dbcore.OpenTestDB(t)
	if err := secondDB.AutoMigrate(&models.Client{}); err != nil {
		t.Fatal(err)
	}
	if err := secondDB.Create(&models.Client{UUID: "same-node", Token: "second-token"}).Error; err != nil {
		t.Fatal(err)
	}
	t.Run("different DB", func(t *testing.T) {
		dbcore.SwapInstance(t, secondDB)
		if token, err := GetClientTokenByUUID("same-node"); err != nil || token != "second-token" {
			t.Fatalf("wrong DB token: %q, %v", token, err)
		}
		if _, err := GetClientUUIDByToken("first-token"); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("foreign DB token accepted: %v", err)
		}
	})
	if token, err := GetClientTokenByUUID("same-node"); err != nil || token != "first-token" {
		t.Fatalf("restored DB token: %q, %v", token, err)
	}
	if err := DeleteClient("same-node"); err != nil {
		t.Fatal(err)
	}
	var queries atomic.Int64
	if err := firstDB.Callback().Query().Before("gorm:query").Register(t.Name(), func(*gorm.DB) { queries.Add(1) }); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		list, err := GetAllClientBasicInfo()
		if err != nil || list == nil || len(list) != 0 {
			t.Fatalf("empty cache: %+v, %v", list, err)
		}
		if _, err := GetClientUUIDByToken("first-token"); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("deleted token accepted: %v", err)
		}
	}
	if got := queries.Load(); got != 1 {
		t.Fatalf("empty snapshot queried %d times, want 1", got)
	}
}

func TestClientCacheTTLRefresh(t *testing.T) {
	db := clientCacheFixture(t)
	if err := db.Create(&models.Client{UUID: "node", Token: "before"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := GetAllClientBasicInfo(); err != nil {
		t.Fatal(err)
	}
	// Simulate out-of-band SQL. In-process writers invalidate immediately;
	// the existing TTL bounds metadata staleness for external writers.
	if err := db.Model(&models.Client{}).Where("uuid = ?", "node").Update("token", "after").Error; err != nil {
		t.Fatal(err)
	}
	clientsCacheMu.Lock()
	clientsCachedAt = time.Now().Add(-clientsCacheTTL)
	clientsCacheMu.Unlock()
	if id, err := GetClientUUIDByToken("after"); err != nil || id != "node" {
		t.Fatalf("TTL refresh: %q, %v", id, err)
	}
	if _, err := GetClientUUIDByToken("before"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("TTL retained revoked token: %v", err)
	}
}

func TestClientCacheReadFailureIsNotCached(t *testing.T) {
	db := clientCacheFixture(t)
	if err := db.Create(&models.Client{UUID: "node", Token: "valid"}).Error; err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("read unavailable")
	if err := db.Callback().Query().Before("gorm:query").Register(t.Name(), func(tx *gorm.DB) { tx.AddError(wantErr) }); err != nil {
		t.Fatal(err)
	}
	if _, err := GetClientUUIDByToken("valid"); !errors.Is(err, wantErr) {
		t.Fatalf("token read error = %v, want %v", err, wantErr)
	}
	if _, err := GetClientByUUID("node"); !errors.Is(err, wantErr) {
		t.Fatalf("client read error = %v, want %v", err, wantErr)
	}
	if err := db.Callback().Query().Remove(t.Name()); err != nil {
		t.Fatal(err)
	}
	if id, err := GetClientUUIDByToken("valid"); err != nil || id != "node" {
		t.Fatalf("read recovery: %q, %v", id, err)
	}
}

func TestClientCacheFailedWritesPreserveStoredToken(t *testing.T) {
	for _, save := range []struct {
		name  string
		write func(map[string]interface{}) error
	}{{"save", SaveClient}, {"info", SaveClientInfo}} {
		t.Run(save.name, func(t *testing.T) {
			db := clientCacheFixture(t)
			if err := db.Create(&models.Client{UUID: "node", Token: "valid"}).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := GetAllClientBasicInfo(); err != nil {
				t.Fatal(err)
			}
			wantErr := errors.New("update unavailable")
			if err := db.Callback().Update().Before("gorm:update").Register(t.Name(), func(tx *gorm.DB) { tx.AddError(wantErr) }); err != nil {
				t.Fatal(err)
			}
			if err := save.write(map[string]interface{}{"uuid": "node", "token": "rejected"}); !errors.Is(err, wantErr) {
				t.Fatalf("write error = %v, want %v", err, wantErr)
			}
			if id, err := GetClientUUIDByToken("valid"); err != nil || id != "node" {
				t.Fatalf("failed write revoked valid token: %q, %v", id, err)
			}
			if _, err := GetClientUUIDByToken("rejected"); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("failed write accepted rejected token: %v", err)
			}
		})
	}
}

func TestClientCacheMutationSafety(t *testing.T) {
	db := clientCacheFixture(t)
	expires := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := db.Create(&models.Client{UUID: "node", Token: "token", Name: "original", ExpiredAt: &expires}).Error; err != nil {
		t.Fatal(err)
	}
	// The cold result must not share nested values with the cache.
	first, err := GetAllClientBasicInfo()
	if err != nil || len(first) != 1 {
		t.Fatalf("cold list: %+v, %v", first, err)
	}
	first[0].Name = "changed"
	*first[0].ExpiredAt = expires.AddDate(1, 0, 0)
	for i := 0; i < 2; i++ {
		got, err := GetClientByUUID("node")
		if err != nil || got.Name != "original" || got.ExpiredAt == nil || !got.ExpiredAt.Equal(expires) {
			t.Fatalf("caller mutated client cache: %+v, %v", got, err)
		}
		*got.ExpiredAt = expires.AddDate(2, 0, 0)
	}
	list, err := GetAllClientBasicInfo()
	if err != nil || len(list) != 1 {
		t.Fatalf("warm list: %+v, %v", list, err)
	}
	*list[0].ExpiredAt = expires.AddDate(3, 0, 0)
	got, err := GetClientByUUID("node")
	if err != nil || !got.ExpiredAt.Equal(expires) {
		t.Fatalf("warm list mutated cache: %+v, %v", got, err)
	}
}
