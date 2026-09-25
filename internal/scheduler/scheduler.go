package scheduler

import (
	"context"
	"fmt"
	logger "github.com/Tumb1er1376/komari-monitor-lite/utils/log"
	"strings"
	"sync"
	"time"
)

// Func 是调度器执行的任务函数。
// 调度器会为每次执行传入可取消的 context，便于任务在重载或关闭时尽快退出。
type Func func(ctx context.Context)

type job struct {
	cancel context.CancelFunc
}

// stopTimeout 是 StopAll cancel 之后等待在跑任务退出的上限。
const stopTimeout = 30 * time.Second

// everySchedule 以固定间隔触发。内部所有调度点都表达为 @every，
// 不支持 5/6 字段 cron 表达式。
type everySchedule struct {
	interval time.Duration
}

func (s everySchedule) Next(t time.Time) time.Time {
	return t.Add(s.interval)
}

type Manager struct {
	mu   sync.Mutex
	jobs map[string]job
	// wg 跟踪所有正在执行的任务（safeRun），StopAll cancel 后等待其退出。
	wg sync.WaitGroup
}

var defaultManager = NewManager()

func NewManager() *Manager {
	return &Manager{jobs: make(map[string]job)}
}

// AddFunc 注册一个任务，fn 会在独立 goroutine 中执行。
// spec 只接受 "@every 1m" 这类固定间隔表达式。
func AddFunc(name string, spec string, fn func()) error {
	return AddContextFunc(name, spec, false, func(context.Context) { fn() })
}

// AddContextFunc 注册一个带 context 的任务。
// spec 只接受 "@every 1m" 这类固定间隔表达式。
func AddContextFunc(name string, spec string, runImmediately bool, fn Func) error {
	return defaultManager.AddContextFunc(name, spec, runImmediately, fn)
}

func Every(duration time.Duration) string {
	return "@every " + duration.String()
}

func RemovePrefix(prefix string) {
	defaultManager.RemovePrefix(prefix)
}

func StopAll() {
	defaultManager.StopAll()
}

func (m *Manager) AddContextFunc(name string, spec string, runImmediately bool, fn Func) error {
	if name == "" {
		return fmt.Errorf("job name is empty")
	}
	s, err := Parse(spec)
	if err != nil {
		return fmt.Errorf("job %q spec is invalid: %w", name, err)
	}
	if fn == nil {
		return fmt.Errorf("job %q func is nil", name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.replace(name, cancel)

	go m.run(ctx, name, s, runImmediately, fn)
	return nil
}

func (m *Manager) RemovePrefix(prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, old := range m.jobs {
		if strings.HasPrefix(name, prefix) {
			old.cancel()
			delete(m.jobs, name)
		}
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	for name, old := range m.jobs {
		old.cancel()
		delete(m.jobs, name)
	}
	m.mu.Unlock()

	// 只 cancel 不等待会让正在跑的任务在进程退出时被硬杀（写一半的
	// 数据丢失），这里带超时等待在跑任务退出。
	waitDone := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(stopTimeout):
		logger.Warnf("scheduler", "Timed out waiting for running jobs to stop after %s", stopTimeout)
	}
}

func (m *Manager) replace(name string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if old, ok := m.jobs[name]; ok {
		old.cancel()
	}
	m.jobs[name] = job{cancel: cancel}
}

func (m *Manager) run(ctx context.Context, name string, s *everySchedule, runImmediately bool, fn Func) {
	if runImmediately {
		m.wg.Add(1)
		go safeRun(ctx, m, name, fn)
	}

	nextTick := s.Next(time.Now())
	if nextTick.IsZero() {
		logger.Warnf("scheduler", "Job %s has no next run time", name)
		return
	}
	timer := time.NewTimer(time.Until(nextTick))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			m.wg.Add(1)
			go safeRun(ctx, m, name, fn)
			// 基于当前时间而非旧基准计算下一次触发：调度停摆（宿主机
			// 挂起、GC 长停顿）后按旧基准追赶会连发多个 tick 造成突发。
			nextTick = s.Next(time.Now())
			if nextTick.IsZero() {
				return
			}
			resetTimer(timer, time.Until(nextTick))
		}
	}
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if duration < 0 {
		duration = 0
	}
	timer.Reset(duration)
}

func safeRun(ctx context.Context, m *Manager, name string, fn Func) {
	defer m.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("scheduler", "Job %s panic: %v", name, r)
		}
	}()

	select {
	case <-ctx.Done():
		return
	default:
		fn(ctx)
	}
}

// Parse 解析调度表达式，只支持 "@every 30s" 这类固定间隔。
func Parse(spec string) (*everySchedule, error) {
	spec = strings.TrimSpace(spec)
	if !strings.HasPrefix(spec, "@every ") {
		return nil, fmt.Errorf("expected \"@every <duration>\", got %q", spec)
	}
	duration, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(spec, "@every ")))
	if err != nil {
		return nil, err
	}
	if duration <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	return &everySchedule{interval: duration}, nil
}
