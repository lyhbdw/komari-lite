package scheduler

import (
	"context"
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Func 是 corn 调度器执行的任务函数。
// 调度器会为每次执行传入可取消的 context，便于任务在重载或关闭时尽快退出。
type Func func(ctx context.Context)

type job struct {
	cancel context.CancelFunc
}

// stopTimeout 是 StopAll cancel 之后等待在跑任务退出的上限。
const stopTimeout = 30 * time.Second

type schedule interface {
	Next(time.Time) time.Time
}

type everySchedule struct {
	interval time.Duration
}

func (s everySchedule) Next(t time.Time) time.Time {
	return t.Add(s.interval)
}

type cronSchedule struct {
	seconds map[int]struct{}
	minutes map[int]struct{}
	hours   map[int]struct{}
	dom     map[int]struct{}
	months  map[int]struct{}
	dow     map[int]struct{}
	// domRestricted / dowRestricted 记录字段是否被限制（非 *）。
	// 标准 cron 语义：dom 与 dow 同时受限时按 OR 匹配，否则各自为 * 时
	// 恒真（此前实现把两者做 AND，导致 "0 0 1 * 1"（每月 1 号或每个
	// 周一）这类表达式永不匹配）。
	domRestricted bool
	dowRestricted bool
}

func (s cronSchedule) Next(t time.Time) time.Time {
	next := t.UTC().Truncate(time.Second).Add(time.Second)
	limit := next.Add(366 * 24 * time.Hour)
	// 逐秒扫描对"永不匹配"的表达式要烧 ~3100 万次迭代。先逐级跳到
	// 候选时间（月→日→时→分→秒），每次失配至少前进一个单位。
	for next.Before(limit) {
		if v, ok := s.advance(next); ok {
			return v
		}
	}
	return time.Time{}
}

// advance 返回 >= t 的下一个候选时间；ok=false 表示 t 已越界。
func (s cronSchedule) advance(t time.Time) (time.Time, bool) {
	local := t.In(time.Local)
	for {
		if _, ok := s.months[int(local.Month())]; !ok {
			// 跳到下个月 1 号 00:00:00。
			local = time.Date(local.Year(), local.Month()+1, 1, 0, 0, 0, 0, time.Local)
			continue
		}
		if !s.matchDay(local) {
			local = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, 1)
			continue
		}
		if _, ok := s.hours[local.Hour()]; !ok {
			local = time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, time.Local).Add(time.Hour)
			continue
		}
		if _, ok := s.minutes[local.Minute()]; !ok {
			local = time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), 0, 0, time.Local).Add(time.Minute)
			continue
		}
		if _, ok := s.seconds[local.Second()]; !ok {
			local = local.Add(time.Second)
			continue
		}
		return local.UTC(), true
	}
}

// matchDay 按标准 cron 语义匹配日字段：dom 与 dow 均受限时取 OR，
// 否则只要受限的一方匹配即可（未受限的恒真）。
func (s cronSchedule) matchDay(t time.Time) bool {
	if _, ok := s.months[int(t.Month())]; !ok {
		return false
	}
	domOK := true
	if s.domRestricted {
		_, domOK = s.dom[t.Day()]
	}
	dowOK := true
	if s.dowRestricted {
		_, dowOK = s.dow[int(t.Weekday())]
	}
	if s.domRestricted && s.dowRestricted {
		return domOK || dowOK
	}
	return domOK && dowOK
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

// AddFunc 按 cron 表达式注册一个任务，fn 会在独立 goroutine 中执行。
// 支持 5 字段、6 字段 cron 表达式，以及 @every 1m 这类固定间隔表达式。
func AddFunc(name string, spec string, fn func()) error {
	return AddContextFunc(name, spec, false, func(context.Context) { fn() })
}

// AddContextFunc 按 cron 表达式注册一个任务，支持传递带 context 的 func。
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
		return fmt.Errorf("corn job name is empty")
	}
	s, err := Parse(spec)
	if err != nil {
		return fmt.Errorf("corn job %q spec is invalid: %w", name, err)
	}
	if fn == nil {
		return fmt.Errorf("corn job %q func is nil", name)
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

func (m *Manager) run(ctx context.Context, name string, s schedule, runImmediately bool, fn Func) {
	if runImmediately {
		m.wg.Add(1)
		go safeRun(ctx, m, name, fn)
	}

	nextTick := s.Next(time.Now())
	if nextTick.IsZero() {
		logger.Warnf("scheduler", "Corn job %s has no next run time", name)
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
			logger.Errorf("scheduler", "Corn job %s panic: %v", name, r)
		}
	}()

	select {
	case <-ctx.Done():
		return
	default:
		fn(ctx)
	}
}

// Parse 解析 corn 表达式。
// 支持：
//   - 5 字段：minute hour day-of-month month day-of-week
//   - 6 字段：second minute hour day-of-month month day-of-week
//   - @every 1m / @every 30s
//
// 字段支持 *、*/n、a-b、a-b/n、逗号列表和具体数字。
func Parse(spec string) (schedule, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty spec")
	}
	if strings.HasPrefix(spec, "@every ") {
		duration, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(spec, "@every ")))
		if err != nil {
			return nil, err
		}
		if duration <= 0 {
			return nil, fmt.Errorf("@every duration must be positive")
		}
		return everySchedule{interval: duration}, nil
	}

	fields := strings.Fields(spec)
	if len(fields) == 5 {
		fields = append([]string{"0"}, fields...)
	}
	if len(fields) != 6 {
		return nil, fmt.Errorf("expected 5 or 6 fields, got %d", len(fields))
	}

	seconds, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("second: %w", err)
	}
	minutes, err := parseField(fields[1], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	hours, err := parseField(fields[2], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	dom, err := parseField(fields[3], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month: %w", err)
	}
	months, err := parseField(fields[4], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	dow, err := parseField(fields[5], 0, 7)
	if err != nil {
		return nil, fmt.Errorf("day-of-week: %w", err)
	}
	if _, ok := dow[7]; ok {
		dow[0] = struct{}{}
		delete(dow, 7)
	}

	// 字段受限 = 表达式中不是 "*"（或等价的 "* /n" 全跨度步进）。
	// dom/dow 同时受限时按标准 cron OR 语义匹配。
	domRestricted := !isFullField(fields[3], 1, 31)
	dowRestricted := !isFullField(fields[5], 0, 7)

	return cronSchedule{
		seconds:       seconds,
		minutes:       minutes,
		hours:         hours,
		dom:           dom,
		months:        months,
		dow:           dow,
		domRestricted: domRestricted,
		dowRestricted: dowRestricted,
	}, nil
}

// isFullField 判断字段是否覆盖整个取值范围（等价于 *）。
func isFullField(field string, min, max int) bool {
	values, err := parseField(field, min, max)
	if err != nil {
		return false
	}
	return len(values) == max-min+1
}

func parseField(field string, min int, max int) (map[int]struct{}, error) {
	values := make(map[int]struct{})
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty part")
		}

		base := part
		step := 1
		if strings.Contains(part, "/") {
			parts := strings.Split(part, "/")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid step %q", part)
			}
			base = parts[0]
			parsedStep, err := strconv.Atoi(parts[1])
			if err != nil || parsedStep <= 0 {
				return nil, fmt.Errorf("invalid step %q", parts[1])
			}
			step = parsedStep
		}

		start, end, err := parseRange(base, min, max)
		if err != nil {
			return nil, err
		}
		for i := start; i <= end; i += step {
			values[i] = struct{}{}
		}
	}
	return values, nil
}

func parseRange(base string, min int, max int) (int, int, error) {
	if base == "*" || base == "" {
		return min, max, nil
	}
	if strings.Contains(base, "-") {
		parts := strings.Split(base, "-")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid range %q", base)
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid range start %q", parts[0])
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid range end %q", parts[1])
		}
		if start > end || start < min || end > max {
			return 0, 0, fmt.Errorf("range %q out of bounds %d-%d", base, min, max)
		}
		return start, end, nil
	}

	value, err := strconv.Atoi(base)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid value %q", base)
	}
	if value < min || value > max {
		return 0, 0, fmt.Errorf("value %d out of bounds %d-%d", value, min, max)
	}
	return value, value, nil
}
