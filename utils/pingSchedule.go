package utils

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/scheduler"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

// PingTaskManager 管理定时器和任务
type PingTaskManager struct {
	mu    sync.Mutex
	tasks map[int][]models.PingTask
	// gates 按 interval 防止同一调度组重叠执行：
	// 每 tick 无上限 go executePingTask 会在任务慢于 interval 时堆积。
	gates map[int]*atomic.Bool
}

var manager = &PingTaskManager{
	tasks: make(map[int][]models.PingTask),
	gates: make(map[int]*atomic.Bool),
}

// Reload 重载时间表
func (m *PingTaskManager) Reload(pingTasks []models.PingTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scheduler.RemovePrefix("ping:")
	m.tasks = make(map[int][]models.PingTask)
	m.gates = make(map[int]*atomic.Bool)

	// 按Interval分组任务
	taskGroups := make(map[int][]models.PingTask)
	for _, task := range pingTasks {
		if task.Interval <= 0 {
			continue
		}
		taskGroups[task.Interval] = append(taskGroups[task.Interval], task)
	}

	// 为每个唯一的Interval创建协程
	for interval, tasks := range taskGroups {
		interval := interval
		tasks := append([]models.PingTask(nil), tasks...)
		m.tasks[interval] = tasks
		gate := &atomic.Bool{}
		m.gates[interval] = gate
		if err := scheduler.AddContextFunc(fmt.Sprintf("ping:%d", interval), scheduler.Every(time.Duration(interval)*time.Second), false, func(ctx context.Context) {
			// 上一轮还没跑完时跳过本轮 tick，避免任务堆积。
			if !gate.CompareAndSwap(false, true) {
				return
			}
			defer gate.Store(false)
			for _, task := range tasks {
				go executePingTask(ctx, task)
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

// executePingTask 执行单个PingTask
func executePingTask(ctx context.Context, task models.PingTask) {
	for _, clientUUID := range targetPingClientUUIDs(task) {
		select {
		case <-ctx.Done():
			// Context was canceled, stop sending pings.
			return
		default:
			// Context is still active, continue.
		}

		agent_runtime.DispatchPing(clientUUID, v2.PingParams{TaskID: task.Id, Type: task.Type, Target: task.Target})
	}
}

// targetPingClientUUIDs 根据任务配置计算本次调度需要下发的在线服务器列表。
func targetPingClientUUIDs(task models.PingTask) []string {
	return task.Clients
}

// ReloadPingSchedule 加载或重载时间表
func ReloadPingSchedule(pingTasks []models.PingTask) error {
	return manager.Reload(pingTasks)
}
