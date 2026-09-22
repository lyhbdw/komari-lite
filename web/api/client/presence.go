package client

import (
	"sync"
	"time"

	"github.com/komari-monitor/komari/utils/notifier"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

const (
	// 如果超过这个时间没有收到任何消息，则认为连接已死
	readWait        = 11 * time.Second
	postPresenceTTL = 35 * time.Second
)

type postPresenceEntry struct {
	connID     int64
	timer      *time.Timer
	generation uint64
}

var (
	postPresenceMu     sync.Mutex
	postPresenceStates = make(map[string]*postPresenceEntry)
)

// refreshPostPresence 管理 HTTP POST 上报者的在线/离线状态。
func refreshPostPresence(uuid string) {
	postPresenceMu.Lock()
	defer postPresenceMu.Unlock()

	if entry, exists := postPresenceStates[uuid]; exists {
		entry.generation++
		entry.timer.Stop()
		gen := entry.generation
		entry.timer = time.AfterFunc(postPresenceTTL, func() {
			postPresenceExpired(uuid, entry.connID, gen)
		})
		agent_runtime.KeepAlivePresence(uuid, entry.connID, postPresenceTTL)
		return
	}

	connID := time.Now().UnixNano()
	agent_runtime.KeepAlivePresence(uuid, connID, postPresenceTTL)
	agent_runtime.MarkV2Client(uuid)
	go safeOnlineNotification(uuid, connID)

	defaultGeneration := uint64(0)
	entry := &postPresenceEntry{connID: connID, generation: defaultGeneration}
	entry.timer = time.AfterFunc(postPresenceTTL, func() {
		postPresenceExpired(uuid, connID, defaultGeneration)
	})
	postPresenceStates[uuid] = entry
}

func postPresenceExpired(uuid string, connID int64, gen uint64) {
	postPresenceMu.Lock()
	e, ok := postPresenceStates[uuid]
	if !ok || e.connID != connID || e.generation != gen {
		postPresenceMu.Unlock()
		return
	}
	delete(postPresenceStates, uuid)
	postPresenceMu.Unlock()

	// time.AfterFunc 回调里同步做运行时清理和通知，任何一处 panic
	// 都会击穿 timer goroutine 导致进程崩溃，必须 recover。
	defer func() { _ = recover() }()
	agent_runtime.SetPresence(uuid, connID, false)
	agent_runtime.ClearV2ClientIfOffline(uuid)
	safeOfflineNotification(uuid, connID)
}

// safeOnlineNotification 在独立 goroutine 中做上线通知，panic 不外泄。
func safeOnlineNotification(uuid string, connID int64) {
	defer func() { _ = recover() }()
	notifier.OnlineNotification(uuid, connID)
}

// safeOfflineNotification 做下线通知，panic 不外泄。
func safeOfflineNotification(uuid string, connID int64) {
	defer func() { _ = recover() }()
	notifier.OfflineNotification(uuid, connID)
}
