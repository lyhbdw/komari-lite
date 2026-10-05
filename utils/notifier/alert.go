package notifier

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lyhbdw/komari-monitor-lite/database/clients"
	"github.com/lyhbdw/komari-monitor-lite/database/models"
	messageevent "github.com/lyhbdw/komari-monitor-lite/database/models/messageEvent"
	"github.com/lyhbdw/komari-monitor-lite/internal/config"
	logger "github.com/lyhbdw/komari-monitor-lite/utils/log"
	"github.com/lyhbdw/komari-monitor-lite/utils/messageSender"
	agent_runtime "github.com/lyhbdw/komari-monitor-lite/web/agent"
)

// 阈值告警配置键（存入 config 表，管理后台可改）
const (
	AlertEnabledKey  = "alert_enabled"    // 总开关，默认 false
	AlertCpuKey      = "alert_cpu"        // CPU 使用率阈值（%），0 表示禁用，默认 90
	AlertMemoryKey   = "alert_memory"     // 内存使用率阈值（%），0 表示禁用，默认 90
	AlertDiskKey     = "alert_disk"      // 磁盘使用率阈值（%），0 表示禁用，默认 90
	AlertCooldownKey = "alert_cooldown"  // 同一客户端同一指标冷却（分钟），默认 30
)

const alertCooldownDefaultMinutes = 30

// alertCache 记录每个客户端每项指标最近一次告警时间，用于冷却去重。
type alertStateMap struct {
	mu sync.Map // key: "alert:"+clientID+":"+metric -> time.Time
}

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

var alertCache = alertStateMap{}

// CheckAlert 检查各客户端最新上报的 CPU/内存/磁盘使用率，超过阈值时发送告警。
// 由调度器每分钟调用一次。
func CheckAlert() {
	enabled, err := config.GetAs[bool](AlertEnabledKey, false)
	if err != nil {
		logger.Errorf("notifier", "alert check: failed to read alert_enabled: %v", err)
		return
	}
	if !enabled {
		return
	}

	cpuThreshold, _ := config.GetAs[float64](AlertCpuKey, 90)
	memThreshold, _ := config.GetAs[float64](AlertMemoryKey, 90)
	diskThreshold, _ := config.GetAs[float64](AlertDiskKey, 90)
	cooldownMinutes, _ := config.GetAs[int](AlertCooldownKey, alertCooldownDefaultMinutes)
	if cpuThreshold <= 0 && memThreshold <= 0 && diskThreshold <= 0 {
		return
	}
	if cooldownMinutes <= 0 {
		cooldownMinutes = alertCooldownDefaultMinutes
	}
	cooldown := time.Duration(cooldownMinutes) * time.Minute

	reports := agent_runtime.GetLatestReport()
	if len(reports) == 0 {
		return
	}
	allClients, err := clients.GetAllClientBasicInfo()
	if err != nil {
		logger.Errorf("notifier", "alert check: failed to list clients: %v", err)
		return
	}

	for _, c := range allClients {
		r, ok := reports[c.UUID]
		if !ok || r == nil {
			continue
		}

		type breach struct {
			metric string
			pct    float64
		}
		var breaches []breach

		if cpuThreshold > 0 && r.CPU.Usage >= cpuThreshold {
			breaches = append(breaches, breach{"cpu", r.CPU.Usage})
		}
		if memThreshold > 0 && r.Ram.Total > 0 {
			pct := float64(r.Ram.Used) / float64(r.Ram.Total) * 100
			if pct >= memThreshold {
				breaches = append(breaches, breach{"memory", pct})
			}
		}
		if diskThreshold > 0 && r.Disk.Total > 0 {
			pct := float64(r.Disk.Used) / float64(r.Disk.Total) * 100
			if pct >= diskThreshold {
				breaches = append(breaches, breach{"disk", pct})
			}
		}
		if len(breaches) == 0 {
			continue
		}

		// 过滤冷却期内的指标
		var fired []breach
		for _, b := range breaches {
			if alertCache.canNotify(c.UUID, b.metric, cooldown) {
				fired = append(fired, b)
			}
		}
		if len(fired) == 0 {
			continue
		}

		var parts []string
		for _, b := range fired {
			parts = append(parts, fmt.Sprintf("%s %.1f%%", b.metric, b.pct))
		}

		_ = messageSender.SendNotification(models.EventMessage{
			Event:   messageevent.Alert,
			Clients: []models.Client{c},
			Time:    time.Now().UTC(),
			Emoji:   "🚨",
			Message: fmt.Sprintf("• %s: %s exceeded threshold", c.Name, strings.Join(parts, ", ")),
		})
	}
}
