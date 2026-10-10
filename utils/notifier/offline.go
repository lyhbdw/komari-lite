package notifier

import (
	logger "github.com/lyhbdw/komari-lite/utils/log"
	"sync"
	"time"

	"github.com/lyhbdw/komari-lite/database/clients"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	messageevent "github.com/lyhbdw/komari-lite/database/models/messageEvent"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/utils/messageSender"
	"github.com/lyhbdw/komari-lite/utils/renewal"
)

// notificationState 保存单个客户端的通知状态。
// 通过在结构体中嵌入互斥锁，实现每个客户端细粒度的锁定，比全局锁更高效。
type notificationState struct {
	mu                  sync.Mutex // 互斥锁，保护该客户端状态
	pendingOfflineSince time.Time  // 客户端离线的时间。为零值表示客户端在线或已发送离线通知。
	isFirstConnection   bool       // 标记是否为首次上线连接。
	isConnExist         bool       // 标记是否存在连接
	connectionID        int64      // 连接ID，用于区分不同的连接会话，防止竞态条件
}

// clientStates 使用 sync.Map 实现对客户端状态的并发访问。
// 映射关系：clientID (string) -> *notificationState
var clientStates sync.Map

// getNotificationConfig 获取指定客户端的通知配置。
// 返回配置对象和一个布尔值，指示全局和该客户端是否启用通知。
func getNotificationConfig(clientID string) (*models.OfflineNotification, bool) {
	conf, err := config.GetAs[bool](config.NotificationEnabledKey, false)
	if err != nil || !conf {
		return nil, false
	}

	notiConf := models.OfflineNotification{Client: clientID}
	db := dbcore.GetDBInstance()
	if err := db.Model(&models.OfflineNotification{}).Where("client = ?", clientID).FirstOrCreate(&notiConf).Error; err != nil {
		logger.Errorf("notifier", "Failed to get or create offline notification config for client %s: %v", clientID, err)
		return nil, false
	}

	return &notiConf, notiConf.Enable
}

// getOrInitState 从 sync.Map 获取客户端状态，不存在则新建并存储。
func getOrInitState(clientID string) *notificationState {
	// 原子性地加载或存储该客户端的状态。
	val, _ := clientStates.LoadOrStore(clientID, &notificationState{isFirstConnection: true})
	return val.(*notificationState)
}

// OfflineNotification 在启用通知且未在宽限期内发送的情况下，发送客户端离线通知。
func OfflineNotification(clientID string, endedConnectionID int64) {
	client, err := clients.GetClientByUUID(clientID)
	if err != nil {
		return
	}

	notiConf, enabled := getNotificationConfig(clientID)
	if !enabled {
		return
	}

	gracePeriod := time.Duration(notiConf.GracePeriod) * time.Second
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Minute // 默认宽限期
	}

	now := time.Now().UTC()

	// 冷却时间：同一客户端离线通知在冷却期内不重复发送。
	cooldown := time.Duration(notiConf.Cooldown) * time.Second
	if cooldown > 0 && notiConf.LastNotified != nil && now.Sub(*notiConf.LastNotified) < cooldown {
		return
	}

	state := getOrInitState(clientID)

	state.mu.Lock()
	// 如果已处于待通知状态，则不做处理。
	// 只有当离线事件来自当前的连接会话时，我们才认为它有效。
	if !state.pendingOfflineSince.IsZero() || state.connectionID != endedConnectionID {
		state.mu.Unlock()
		return
	}
	// 标记该客户端为待离线。
	state.pendingOfflineSince = now
	state.mu.Unlock()

	// 新建协程，等待宽限期后判断是否需要发送通知。
	go func(startTime time.Time, expectedConnectionID int64) {
		time.Sleep(gracePeriod)

		state.mu.Lock()
		defer state.mu.Unlock()

		// 检查离线状态是否仍为本次协程启动时的状态。
		// 若为零值，说明客户端已重连。
		// 当前的 connectionID 是否还是我们触发离线时的那个ID。如果不是，说明客户端重连过，本次离线通知已失效。
		if state.pendingOfflineSince.IsZero() || state.connectionID != expectedConnectionID {
			logger.Infof("notifier", "%s is reconnected new connID: %d, old connID: %d", clientID, state.connectionID, expectedConnectionID)
			return
		}

		// 即将发送通知，重置待通知状态。
		// 需要多一个boolean 是因为pendingOfflineSince在offline睡眠后才修改，可能导致online判断不对
		state.pendingOfflineSince = time.Time{}
		state.isConnExist = false

		// Send notification
		go func() {
			if err := messageSender.SendNotification(models.EventMessage{
				Event:   messageevent.Offline,
				Clients: []models.Client{client},
				Time:    time.Now().UTC(),
				Emoji:   "🔴",
				Message: "节点连接已断开",
			}); err != nil {
				logger.ErrorArgs("notifier", "Failed to send offline notification:", err)
			}
		}()

		// 更新数据库中的最后通知时间与已告警离线状态
		db := dbcore.GetDBInstance()
		if err := db.Model(&models.OfflineNotification{}).Where("client = ?", clientID).Updates(map[string]any{
			"last_notified":    now.UTC(),
			"notified_offline": true,
		}).Error; err != nil {
			logger.Errorf("notifier", "Failed to update last_notified/notified_offline for client %s: %v", clientID, err)
		}
	}(now, endedConnectionID)
}

// updateOnlineState records a connection before notification configuration is checked.
// It reports whether this connection should produce an online notification.
func updateOnlineState(clientID string, connectionID int64) bool {
	state := getOrInitState(clientID)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.connectionID = connectionID

	// 规则1：首次连接不通知。
	if state.isFirstConnection {
		state.isFirstConnection = false
		// 同时清除任何待离线状态（如服务器重启时客户端本已离线）
		state.pendingOfflineSince = time.Time{}
		state.isConnExist = true
		return false
	}

	// 检查客户端是否处于待离线状态。
	wasPending := !state.pendingOfflineSince.IsZero()
	// 上线时总是清除待离线状态。
	state.pendingOfflineSince = time.Time{}

	// 规则2：宽限期内重连，不通知。
	if wasPending {
		return false
	}

	// 规则3: 没断开后重连, 不通知
	// 为了解决OfflineNotify中不是全程加锁
	if state.isConnExist {
		logger.Infof("notifier", "%s has connection exist: %d", clientID, connectionID)
		return false
	} else {
		state.isConnExist = true
	}

	return true
}

// OnlineNotification records the connection state and, when enabled, sends a client online notification.
func OnlineNotification(clientID string, connectionID int64) {
	client, err := clients.GetClientByUUID(clientID)
	if err != nil {
		return
	}
	// 上线时检测续费
	renewal.CheckAndAutoRenewal(client)
	updateOnlineState(clientID, connectionID)
	_, enabled := getNotificationConfig(clientID)
	if !enabled {
		return
	}

	// 规则4：只有此前触发并发送过离线告警的节点，恢复上线时才发送恢复通知。
	// 通过原子更新 notified_offline 字段（true -> false）：
	// 1. 服务端重启/升级后，离线节点的恢复通知依然能正确触发，不因内存状态丢失而漏发；
	// 2. 正常重启时所有在线节点不会被误触发上线通知；
	// 3. 高并发上报时同一恢复事件只触发一次通知。
	db := dbcore.GetDBInstance()
	res := db.Model(&models.OfflineNotification{}).
		Where("client = ? AND notified_offline = ?", clientID, true).
		Update("notified_offline", false)
	if res.Error != nil || res.RowsAffected == 0 {
		return
	}

	go func() {
		if err := messageSender.SendNotification(models.EventMessage{
			Event:   messageevent.Online,
			Clients: []models.Client{client},
			Time:    time.Now().UTC(),
			Emoji:   "🟢",
			Message: "节点已恢复在线",
		}); err != nil {
			logger.ErrorArgs("notifier", "Failed to send online notification:", err)
		}
	}()
}
