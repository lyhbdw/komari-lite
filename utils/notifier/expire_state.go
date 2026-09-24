package notifier

import "sync"

// expireStateMap 记录每个客户端最近一次到期提醒所处的剩余天数档位，
// 用于同一天内去重。进程内状态即可：重启后最多多发一次提醒。
type expireStateMap struct {
	mu sync.Map
}

// markNotified 尝试将客户端标记为"已在 daysLeft 档位提醒过"。
// 返回 true 表示这是该档位的首次提醒（应当发送），false 表示已提醒过（应当跳过）。
func (m *expireStateMap) markNotified(clientID string, daysLeft int) bool {
	key := "expire:" + clientID
	val, loaded := m.mu.LoadOrStore(key, daysLeft)
	if !loaded {
		return true
	}
	last, _ := val.(int)
	if last == daysLeft {
		return false
	}
	m.mu.Store(key, daysLeft)
	return true
}

// take 清除客户端的提醒档位记录（续费/重新进入提醒窗口时调用）。
func (m *expireStateMap) take(clientID string) {
	m.mu.Delete("expire:" + clientID)
}
