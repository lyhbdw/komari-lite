package agent

import (
	"sort"
	"sync"
	"time"

	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
	"github.com/lyhbdw/komari-lite/web/connection"
)

var (
	connectedClients = make(map[string]*connection.SafeConn)
	v2Clients        = make(map[string]struct{})
	latestReport     = make(map[string]*v2.Report)
	recentReports    = make(map[string][]v2.Report)
	// presenceOnly stores online state for non-WebSocket agents.
	// value keeps connectionID and a soft expiration to avoid flicker
	presenceOnly = make(map[string]struct {
		id     int64
		expire time.Time
	})
	mu = sync.RWMutex{}
)

const recentReportRetention = time.Minute

// GetConnectedClient reads one connection without copying the fleet map.
func GetConnectedClient(uuid string) *connection.SafeConn {
	mu.RLock()
	defer mu.RUnlock()
	return connectedClients[uuid]
}

func SetConnectedClients(uuid string, conn *connection.SafeConn) {
	mu.Lock()
	defer mu.Unlock()
	connectedClients[uuid] = conn
}

func MarkV2Client(uuid string) {
	mu.Lock()
	defer mu.Unlock()
	v2Clients[uuid] = struct{}{}
}

func IsV2Client(uuid string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := v2Clients[uuid]
	return ok
}

// ClearV2ClientIfOffline clears an HTTP v2 presence marker without removing
// the marker for an active WebSocket connection using the same UUID.
func ClearV2ClientIfOffline(uuid string) bool {
	mu.Lock()
	defer mu.Unlock()
	if _, connected := connectedClients[uuid]; connected {
		return false
	}
	if _, marked := v2Clients[uuid]; !marked {
		return false
	}
	delete(v2Clients, uuid)
	return true
}

func DeleteClientConditionally(uuid string, connToRemove *connection.SafeConn) {
	mu.Lock()
	defer mu.Unlock()

	// 检查当前 map 里的 conn 是否就是要删除的这一个
	if currentConn, exists := connectedClients[uuid]; exists && currentConn == connToRemove {
		delete(connectedClients, uuid)
		// 混合传输的 agent 可能同时有活跃的 POST presence；WS 断开时
		// 只有在没有活跃 POST 上报会话的情况下才清 v2 在线标记，
		// 否则 POST 会被误判为离线。
		if p, ok := presenceOnly[uuid]; !ok || !p.expire.After(time.Now()) {
			delete(v2Clients, uuid)
		}
	}
}

// DeleteConnectedClients 清除一个 uuid 的全部运行时在线状态
// （admin 删除客户端时调用）：WS 连接条目、v2 标记、POST presence 与事件队列。
func DeleteConnectedClients(uuid string) {
	mu.Lock()
	delete(connectedClients, uuid)
	delete(v2Clients, uuid)
	delete(presenceOnly, uuid)
	mu.Unlock()
	DeleteV2EventQueue(uuid)
}

// SetPresence sets or clears presence for non-WebSocket agents.
// When present=false, it only clears if the connectionID matches current one.
// KeepAlivePresence sets presence with TTL for non-WebSocket agents.
func KeepAlivePresence(uuid string, connectionID int64, ttl time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	presenceOnly[uuid] = struct {
		id     int64
		expire time.Time
	}{id: connectionID, expire: time.Now().Add(ttl)}
}

var defaultPresenceTTL = 20 * time.Second

// SetPresence keeps compatibility with existing callers.
func SetPresence(uuid string, connectionID int64, present bool) {
	mu.Lock()
	defer mu.Unlock()
	if present {
		presenceOnly[uuid] = struct {
			id     int64
			expire time.Time
		}{id: connectionID, expire: time.Now().Add(defaultPresenceTTL)}
		return
	}
	if cur, ok := presenceOnly[uuid]; ok && cur.id == connectionID {
		delete(presenceOnly, uuid)
	}
}

// GetAllOnlineUUIDs returns a de-duplicated list of online UUIDs from both WebSocket and non-WebSocket agents.
// Expired presence entries are garbage-collected here so the map does not grow
// unboundedly for agents that stop reporting without an explicit offline path.
func GetAllOnlineUUIDs() []string {
	mu.Lock()
	defer mu.Unlock()
	set := make(map[string]struct{})
	for k := range connectedClients {
		set[k] = struct{}{}
	}
	now := time.Now()
	for k, v := range presenceOnly {
		if v.expire.After(now) {
			set[k] = struct{}{}
		} else {
			delete(presenceOnly, k)
		}
	}
	res := make([]string, 0, len(set))
	for k := range set {
		res = append(res, k)
	}
	return res
}
func GetLatestReport() map[string]*v2.Report {
	mu.RLock()
	defer mu.RUnlock()
	reportCopy := make(map[string]*v2.Report)
	for k, v := range latestReport {
		if v == nil {
			continue
		}
		item := *v
		// GPU 是指针字段，浅拷贝会与缓存共享底层 DetailedInfo 切片，
		// 调用方修改会污染运行时状态。
		if v.GPU != nil {
			gpu := *v.GPU
			gpu.DetailedInfo = append([]v2.GPUDeviceInfo(nil), v.GPU.DetailedInfo...)
			item.GPU = &gpu
		}
		reportCopy[k] = &item
	}
	return reportCopy
}

// RecordReport updates the latest runtime state and keeps only the short raw
// window used by recent-status compatibility endpoints.
func RecordReport(report v2.Report) {
	if report.UUID == "" {
		return
	}
	if report.UpdatedAt.IsZero() {
		report.UpdatedAt = time.Now().UTC()
	} else {
		report.UpdatedAt = report.UpdatedAt.UTC()
	}
	mu.Lock()
	defer mu.Unlock()
	if latest := latestReport[report.UUID]; latest == nil || !report.UpdatedAt.Before(latest.UpdatedAt) {
		item := report
		latestReport[report.UUID] = &item
	}
	cutoff := time.Now().UTC().Add(-recentReportRetention)
	reports := reportsAfter(recentReports[report.UUID], cutoff)
	if report.UpdatedAt.Before(cutoff) {
		recentReports[report.UUID] = reports
		return
	}
	insertAt := sort.Search(len(reports), func(i int) bool {
		return reports[i].UpdatedAt.After(report.UpdatedAt)
	})
	reports = append(reports, v2.Report{})
	copy(reports[insertAt+1:], reports[insertAt:])
	reports[insertAt] = report
	recentReports[report.UUID] = reports
}

func GetRecentReports(uuid string) []v2.Report {
	mu.Lock()
	defer mu.Unlock()
	reports := reportsAfter(recentReports[uuid], time.Now().UTC().Add(-recentReportRetention))
	if len(reports) == 0 {
		delete(recentReports, uuid)
		return []v2.Report{}
	}
	recentReports[uuid] = reports
	result := make([]v2.Report, len(reports))
	copy(result, reports)
	for i := range result {
		if reports[i].GPU != nil {
			gpu := *reports[i].GPU
			gpu.DetailedInfo = append([]v2.GPUDeviceInfo(nil), gpu.DetailedInfo...)
			result[i].GPU = &gpu
		}
	}
	return result
}

// reportsAfter compacts only the owned runtime slice. Read APIs copy the result
// before returning it; clearing the retired tail releases nested report data.
func reportsAfter(reports []v2.Report, cutoff time.Time) []v2.Report {
	first := sort.Search(len(reports), func(i int) bool {
		return !reports[i].UpdatedAt.Before(cutoff)
	})
	if first == 0 {
		return reports
	}
	remaining := copy(reports, reports[first:])
	clear(reports[remaining:])
	return reports[:remaining]
}

func DeleteLatestReport(uuid string) {
	mu.Lock()
	defer mu.Unlock()
	delete(latestReport, uuid)
	delete(recentReports, uuid)
}
