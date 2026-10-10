package notifier

import (
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/config"
)

func TestUpdateOnlineStateTracksConnectionBeforeNotificationsAreEnabled(t *testing.T) {
	clientID := "notification-disabled-online-client"
	clientStates.Delete(clientID)
	t.Cleanup(func() { clientStates.Delete(clientID) })

	if shouldNotify := updateOnlineState(clientID, 42); shouldNotify {
		t.Fatal("first connection should not send an online notification")
	}

	state := getOrInitState(clientID)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.connectionID != 42 {
		t.Fatalf("connectionID = %d, want 42", state.connectionID)
	}
	if state.isFirstConnection {
		t.Fatal("first connection should be recorded")
	}
	if !state.isConnExist {
		t.Fatal("connection should be marked as present")
	}
}

func TestOnlineNotificationConsumesNotifiedOfflineState(t *testing.T) {
	db := dbcore.OpenTestDB(t)
	dbcore.SwapInstance(t, db)
	if err := db.AutoMigrate(&models.Client{}, &models.OfflineNotification{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := config.SetDb(db); err != nil {
		t.Fatalf("set config db: %v", err)
	}
	if err := config.Set(config.NotificationEnabledKey, true); err != nil {
		t.Fatalf("set notification_enabled: %v", err)
	}

	confVal, confErr := config.GetAs[bool](config.NotificationEnabledKey, false)
	t.Logf("NotificationEnabledKey in test: %v, err: %v", confVal, confErr)

	clientID := "test-offline-recovery-node"
	clientStates.Delete(clientID)
	t.Cleanup(func() { clientStates.Delete(clientID) })

	client := models.Client{UUID: clientID, Name: "Test Node"}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	// 1. Initial state: NotifiedOffline = false
	noti := models.OfflineNotification{
		Client:          clientID,
		Enable:          true,
		NotifiedOffline: false,
	}
	if err := db.Create(&noti).Error; err != nil {
		t.Fatalf("create notification: %v", err)
	}

	// First connection when server restarts or starts: should NOT trigger online notification
	OnlineNotification(clientID, 100)

	var check models.OfflineNotification
	if err := db.Where("client = ?", clientID).First(&check).Error; err != nil {
		t.Fatalf("query noti: %v", err)
	}
	if check.NotifiedOffline {
		t.Fatal("notified_offline should remain false")
	}

	// 2. Simulate offline alert fired: NotifiedOffline = true
	if err := db.Model(&models.OfflineNotification{}).Where("client = ?", clientID).Update("notified_offline", true).Error; err != nil {
		t.Fatalf("update to offline: %v", err)
	}

	// Reset in-memory state to simulate server restart during offline period
	clientStates.Delete(clientID)

	// Now node reconnects after server restart: should consume NotifiedOffline (set to false)
	OnlineNotification(clientID, 200)

	// Wait briefly for the async notification goroutine to complete before cleanup
	time.Sleep(50 * time.Millisecond)

	if err := db.Where("client = ?", clientID).First(&check).Error; err != nil {
		t.Fatalf("query noti: %v", err)
	}
	if check.NotifiedOffline {
		t.Fatal("notified_offline should be consumed (set to false) upon recovery")
	}
}
