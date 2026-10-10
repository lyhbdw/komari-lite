package utils

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/scheduler"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
	logger "github.com/lyhbdw/komari-lite/utils/log"
	agent_runtime "github.com/lyhbdw/komari-lite/web/agent"
)

// PingTaskManager 管理定时器和任务
type PingTaskManager struct {
	mu    sync.Mutex
	tasks map[int][]models.PingTask
	gates map[int]*atomic.Bool
	// runners retain their overlap gate until the whole group completes.
	runners map[int]func(context.Context)
}

var manager = &PingTaskManager{
	tasks:   make(map[int][]models.PingTask),
	runners: make(map[int]func(context.Context)),
}

// Reload 重载时间表
func (m *PingTaskManager) Reload(pingTasks []models.PingTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scheduler.RemovePrefix("ping:")
	m.tasks = make(map[int][]models.PingTask)
	m.runners = make(map[int]func(context.Context))
	if m.gates == nil {
		m.gates = make(map[int]*atomic.Bool)
	}

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
		gate := m.gates[interval]
		if gate == nil {
			gate = &atomic.Bool{}
			m.gates[interval] = gate
		}
		runner := newPingGroupRunnerWithGate(tasks, executePingTask, gate)
		m.runners[interval] = runner
		if err := scheduler.AddContextFunc(fmt.Sprintf("ping:%d", interval), scheduler.Every(time.Duration(interval)*time.Second), false, runner); err != nil {
			return err
		}
	}
	return nil
}

func newPingGroupRunner(tasks []models.PingTask, execute func(context.Context, models.PingTask)) func(context.Context) {
	return newPingGroupRunnerWithGate(tasks, execute, &atomic.Bool{})
}

func newPingGroupRunnerWithGate(tasks []models.PingTask, execute func(context.Context, models.PingTask), gate *atomic.Bool) func(context.Context) {
	return func(ctx context.Context) {
		if ctx.Err() != nil || !gate.CompareAndSwap(false, true) {
			return
		}
		defer gate.Store(false)
		var workers sync.WaitGroup
		for _, task := range tasks {
			if ctx.Err() != nil {
				break
			}
			workers.Add(1)
			go func(task models.PingTask) {
				defer workers.Done()
				defer func() {
					if recovered := recover(); recovered != nil {
						logger.Errorf("scheduler", "Ping task %d panic: %v", task.Id, recovered)
					}
				}()
				execute(ctx, task)
			}(task)
		}
		workers.Wait()
	}
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
