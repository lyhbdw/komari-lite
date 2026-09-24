package notifier

import (
	"sync"
	"time"
)

// alertStateMap 记录每个客户端每项指标最近一次告警时间，用于冷却去重。
type alertStateMap struct {
	mu sync.Map // key: "alert:"+clientID+":"+metric -> time.Time
}

// canNotify 检查该客户端该指标是否已过冷却期；若已过则记录本次时间并返回 true。
func (m *alertStateMap) canNotify(clientID, metric string, cooldown time.Duration) bool {
	key := "alert:" + clientID + ":" + metric
	now := time.Now()
	val, loaded := m.mu.LoadOrStore(key, now)
	if !loaded {
		return true
	}
	last, _ := val.(time.Time)
	if now.Sub(last) < cooldown {
		return false
	}
	m.mu.Store(key, now)
	return true
}
