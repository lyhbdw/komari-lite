package tasks

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
	"gorm.io/gorm"
)

func TestPingTasksCacheDoesNotRepublishInvalidatedLoad(t *testing.T) {
	db := dbcore.OpenTestDB(t)
	if err := db.AutoMigrate(&models.PingTask{}); err != nil {
		t.Fatal(err)
	}
	dbcore.SwapInstance(t, db)
	InvalidatePingTasksCache()
	t.Cleanup(InvalidatePingTasksCache)
	task := models.PingTask{Name: "before", Target: "localhost"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	loaded, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	resume := func() { releaseOnce.Do(func() { close(release) }) }
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
	go func() { _, err := GetAllPingTasks(); done <- err }()
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
			t.Error("cache load did not finish")
		}
	}()
	select {
	case <-loaded:
	case <-time.After(5 * time.Second):
		t.Fatal("cache query did not reach pause")
	}
	if err := db.Model(&task).Update("name", "after").Error; err != nil {
		t.Fatal(err)
	}
	InvalidatePingTasksCache()
	// A post-write reader may refresh before the older query returns.
	if _, err := GetAllPingTasks(); err != nil {
		t.Fatal(err)
	}
	// Release and join here so that the final read observes any stale republish.
	resume()
	if err := <-done; err != nil {
		joined = true
		t.Fatal(err)
	}
	joined = true
	got, err := GetAllPingTasks()
	if err != nil || len(got) != 1 || got[0].Name != "after" {
		t.Fatalf("invalidated load republished stale tasks: %+v, %v", got, err)
	}
}

func TestPingTasksCacheInvalidatesPartialWrites(t *testing.T) {
	for _, operation := range []string{"edit", "default"} {
		t.Run(operation, func(t *testing.T) {
			db := dbcore.OpenTestDB(t)
			if err := db.AutoMigrate(&models.PingTask{}); err != nil {
				t.Fatal(err)
			}
			dbcore.SwapInstance(t, db)
			InvalidatePingTasksCache()
			t.Cleanup(InvalidatePingTasksCache)
			first := models.PingTask{Name: "first", Target: "localhost", DefaultOn: true, Clients: models.StringArray{}}
			second := models.PingTask{Name: "second", Target: "localhost", DefaultOn: true, Clients: models.StringArray{}}
			if err := db.Create(&first).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&second).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := GetAllPingTasks(); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "edit":
				first.Name = "updated"
				err := EditPingTask([]*models.PingTask{&first, {Id: second.Id + 100, Name: "missing"}})
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					t.Fatalf("edit error = %v", err)
				}
			case "default":
				var updates int
				if err := db.Callback().Update().Before("gorm:update").Register(t.Name(), func(tx *gorm.DB) {
					updates++
					if updates == 2 {
						tx.AddError(errors.New("second write failed"))
					}
				}); err != nil {
					t.Fatal(err)
				}
				if err := AddDefaultOnClientUUID("new-node"); err == nil {
					t.Fatal("expected second write failure")
				}
			}
			got, err := GetAllPingTasks()
			if err != nil || len(got) != 2 {
				t.Fatalf("tasks = %+v, %v", got, err)
			}
			if operation == "edit" && got[0].Name != "updated" || operation == "default" && !got[0].AppliesToClient("new-node") {
				t.Fatalf("partial write missing from cache: %+v", got)
			}
		})
	}
}

func pingCacheFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db := dbcore.OpenTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.PingTask{}); err != nil {
		t.Fatal(err)
	}
	dbcore.SwapInstance(t, db)
	InvalidatePingTasksCache()
	t.Cleanup(InvalidatePingTasksCache)
	return db
}

func TestPingTasksCacheDatabaseIdentityAndEmptyHits(t *testing.T) {
	firstDB := pingCacheFixture(t)
	if err := firstDB.Create(&models.PingTask{Name: "first", Target: "localhost", Clients: models.StringArray{}}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := GetAllPingTasks(); err != nil {
		t.Fatal(err)
	}
	secondDB := dbcore.OpenTestDB(t)
	if err := secondDB.AutoMigrate(&models.PingTask{}); err != nil {
		t.Fatal(err)
	}
	if err := secondDB.Create(&models.PingTask{Name: "second", Target: "localhost"}).Error; err != nil {
		t.Fatal(err)
	}
	t.Run("different DB", func(t *testing.T) {
		dbcore.SwapInstance(t, secondDB)
		got, err := GetAllPingTasks()
		if err != nil || len(got) != 1 || got[0].Name != "second" {
			t.Fatalf("wrong DB tasks: %+v, %v", got, err)
		}
	})
	for i := 0; i < 2; i++ {
		got, err := GetAllPingTasks()
		if err != nil || len(got) != 1 || got[0].Name != "first" || got[0].Clients == nil {
			t.Fatalf("restored DB tasks/empty shape: %+v, %v", got, err)
		}
	}
	if err := firstDB.Where("1 = 1").Delete(&models.PingTask{}).Error; err != nil {
		t.Fatal(err)
	}
	InvalidatePingTasksCache()
	var queries atomic.Int64
	if err := firstDB.Callback().Query().Before("gorm:query").Register(t.Name(), func(*gorm.DB) { queries.Add(1) }); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		got, err := GetAllPingTasks()
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("empty tasks: %+v, %v", got, err)
		}
	}
	if got := queries.Load(); got != 1 {
		t.Fatalf("empty cache used %d queries, want 1", got)
	}
}

func TestPingTasksCacheAllWriteEntrypoints(t *testing.T) {
	db := pingCacheFixture(t)
	if metricstore.GetStore() != nil {
		t.Fatal("metric store leaked from another test")
	}
	if err := config.SetDb(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&config.ConfigItem{Key: "metric_db_dsn", Value: `"` + filepath.Join(t.TempDir(), "metrics.db") + `"`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := metricstore.InitializeStore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := metricstore.CloseStoreContext(context.Background()); err != nil {
			t.Error(err)
		}
	})
	assertTasks := func(names ...string) []models.PingTask {
		t.Helper()
		got, err := GetAllPingTasks()
		if err != nil {
			t.Fatal(err)
		}
		actual := make([]string, len(got))
		for i := range got {
			actual[i] = got[i].Name
		}
		if len(names) != len(actual) || len(names) != 0 && !reflect.DeepEqual(names, actual) {
			t.Fatalf("task order/names = %v, want %v", actual, names)
		}
		return got
	}
	assertTasks()
	first, err := AddPingTask(nil, true, "first", "localhost", "icmp", 60)
	if err != nil {
		t.Fatal(err)
	}
	assertTasks("first")
	second, err := AddPingTask([]string{"manual"}, false, "second", "localhost", "icmp", 60)
	if err != nil {
		t.Fatal(err)
	}
	assertTasks("first", "second")
	if err := UpdatePingTaskOrder(map[uint]int{first: 10, second: -1}); err != nil {
		t.Fatal(err)
	}
	got := assertTasks("second", "first")
	if got[0].Weight != -1 || got[1].Weight != 10 {
		t.Fatalf("cached order weights: %+v", got)
	}
	edit := got[1]
	edit.Name, edit.Target, edit.Interval = "edited", "changed-target", 0
	edit.Clients = nil
	if err := EditPingTask([]*models.PingTask{&edit}); err != nil {
		t.Fatal(err)
	}
	got = assertTasks("second", "edited")
	if got[1].Target != "changed-target" || got[1].Interval != 0 || got[1].Clients == nil || len(got[1].Clients) != 0 {
		t.Fatalf("cached edit: %+v", got[1])
	}
	if err := AddDefaultOnClientUUID("new-node"); err != nil {
		t.Fatal(err)
	}
	got = assertTasks("second", "edited")
	if got[0].AppliesToClient("new-node") || !got[1].AppliesToClient("new-node") {
		t.Fatalf("cached default assignment: %+v", got)
	}
	if err := AddDefaultOnClientUUID("new-node"); err != nil {
		t.Fatal(err)
	}
	got = assertTasks("second", "edited")
	if len(got[1].Clients) != 1 {
		t.Fatalf("duplicate default assignment: %+v", got[1])
	}
	if err := DeletePingTask([]uint{first}); err != nil {
		t.Fatal(err)
	}
	assertTasks("second")
	if err := DeletePingTask([]uint{second}); err != nil {
		t.Fatal(err)
	}
	assertTasks()
}

func TestPingTasksCacheReadFailureIsNotCached(t *testing.T) {
	db := pingCacheFixture(t)
	wantErr := errors.New("read unavailable")
	if err := db.Callback().Query().Before("gorm:query").Register(t.Name(), func(tx *gorm.DB) { tx.AddError(wantErr) }); err != nil {
		t.Fatal(err)
	}
	if _, err := GetAllPingTasks(); !errors.Is(err, wantErr) {
		t.Fatalf("read error = %v, want %v", err, wantErr)
	}
	if err := db.Callback().Query().Remove(t.Name()); err != nil {
		t.Fatal(err)
	}
	if got, err := GetAllPingTasks(); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("read recovery: %+v, %v", got, err)
	}
}

func TestPingTasksCacheEditPreservesWriteError(t *testing.T) {
	db := pingCacheFixture(t)
	task := models.PingTask{Name: "original", Target: "localhost"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := GetAllPingTasks(); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("update unavailable")
	if err := db.Callback().Update().Before("gorm:update").Register(t.Name(), func(tx *gorm.DB) { tx.AddError(wantErr) }); err != nil {
		t.Fatal(err)
	}
	task.Name = "rejected"
	if err := EditPingTask([]*models.PingTask{&task}); !errors.Is(err, wantErr) {
		t.Fatalf("write error = %v, want %v", err, wantErr)
	}
	got, err := GetAllPingTasks()
	if err != nil || len(got) != 1 || got[0].Name != "original" {
		t.Fatalf("failed edit changed cached task: %+v, %v", got, err)
	}
}

func TestClonePingTasksPreservesClientsShape(t *testing.T) {
	for _, clients := range []models.StringArray{nil, {}, {"node-a"}} {
		source := []models.PingTask{{Name: "original", Clients: clients}}
		cloned := clonePingTasks(source)
		if (cloned[0].Clients == nil) != (clients == nil) || len(cloned[0].Clients) != len(clients) {
			t.Fatalf("clone changed clients shape: source=%#v clone=%#v", clients, cloned[0].Clients)
		}
		cloned[0].Name = "changed"
		if len(clients) != 0 {
			cloned[0].Clients[0] = "changed"
			if clients[0] != "node-a" {
				t.Fatal("clone shares clients backing array")
			}
		}
		if source[0].Name != "original" {
			t.Fatal("clone shares task backing array")
		}
	}
}

func TestGetAllPingTasksCacheAndInvalidation(t *testing.T) {
	db := dbcore.OpenTestDB(t)
	if err := db.AutoMigrate(&models.PingTask{}); err != nil {
		t.Fatal(err)
	}
	dbcore.SwapInstance(t, db)
	InvalidatePingTasksCache()
	t.Cleanup(InvalidatePingTasksCache)

	var queries int
	if err := db.Callback().Query().Before("gorm:query").Register(t.Name(), func(*gorm.DB) {
		queries++
	}); err != nil {
		t.Fatal(err)
	}

	taskID, err := AddPingTask([]string{"node-a"}, true, "CT", "1.1.1.1", "icmp", 60)
	if err != nil {
		t.Fatal(err)
	}
	afterAddQueries := queries

	// ReloadPingSchedule inside AddPingTask already populated the cache.
	list, err := GetAllPingTasks()
	if err != nil || len(list) != 1 || list[0].Id != taskID {
		t.Fatalf("GetAllPingTasks() = %+v, %v", list, err)
	}
	if queries != afterAddQueries {
		t.Fatalf("expected cached GetAllPingTasks to perform 0 queries, got %d", queries-afterAddQueries)
	}

	// Mutating returned slice must not corrupt cached state.
	list[0].Name = "mutated"
	list[0].Clients[0] = "mutated-client"
	again, err := GetAllPingTasks()
	if err != nil || again[0].Name != "CT" || again[0].Clients[0] != "node-a" {
		t.Fatalf("cache was mutated by caller: %+v", again)
	}

	// AddDefaultOnClientUUID invalidates and refreshes cache.
	if err := AddDefaultOnClientUUID("node-b"); err != nil {
		t.Fatal(err)
	}
	updated, err := GetAllPingTasks()
	if err != nil || len(updated[0].Clients) != 2 || !updated[0].AppliesToClient("node-b") {
		t.Fatalf("default-on client was not reflected in cache: %+v", updated)
	}
}
