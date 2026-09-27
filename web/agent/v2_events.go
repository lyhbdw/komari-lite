package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	v2 "github.com/Tumb1er1376/komari-monitor-lite/protocol/v2"
)

const (
	v2EventQueueLimit = 128
	v2PingEventTTL    = 3 * time.Second
	// 零 TTL 事件的默认过期时间，防止它们在队列中永久保留。
	v2EventDefaultTTL = 10 * time.Minute
	// 升级事件给足下载窗口：agent 需要下载二进制 + 校验 + 自替换。
	v2UpdateEventTTL = 30 * time.Minute
)

type v2EventQueue struct {
	events []v2.Event
	signal chan struct{}
}

var (
	v2EventMu     sync.Mutex
	v2EventQueues = make(map[string]*v2EventQueue)
)

func getV2EventQueueLocked(uuid string) *v2EventQueue {
	q := v2EventQueues[uuid]
	if q == nil {
		q = &v2EventQueue{signal: make(chan struct{})}
		v2EventQueues[uuid] = q
	}
	return q
}

// DeleteV2EventQueue 清理客户端的事件队列并唤醒可能正在 WaitV2Events 中
// 等待的 goroutine，防止 map 只增不减以及等待者永久阻塞。
func DeleteV2EventQueue(uuid string) {
	v2EventMu.Lock()
	defer v2EventMu.Unlock()
	q := v2EventQueues[uuid]
	if q == nil {
		return
	}
	delete(v2EventQueues, uuid)
	close(q.signal)
}

func DispatchPing(uuid string, params v2.PingParams) bool {
	if conn := GetConnectedClients()[uuid]; conn != nil {
		payload := v2.Request{JSONRPC: v2.Version, Method: v2.MethodAgentPing, Params: params}
		if err := conn.WriteJSON(payload); err == nil {
			return true
		}
		// 直写失败（半开连接、写超时）：降级入队，重连后的 agent
		// 通过 pull/report 拿到该事件，而不是静默丢失。
	}
	if !IsV2Client(uuid) {
		return false
	}
	EnqueueV2Ping(uuid, params)
	return true
}

func EnqueueV2Ping(uuid string, params v2.PingParams) v2.Event {
	now := time.Now().UTC()
	event := v2.Event{
		ID:        newV2EventID(),
		Method:    v2.MethodAgentPing,
		Params:    params,
		CreatedAt: now,
		ExpiresAt: now.Add(v2PingEventTTL),
	}

	v2EventMu.Lock()
	q := getV2EventQueueLocked(uuid)
	pruneExpiredV2EventsLocked(q)
	coalesceV2EventLocked(q, event)
	q.events = append(q.events, event)
	if len(q.events) > v2EventQueueLimit {
		q.events = q.events[len(q.events)-v2EventQueueLimit:]
	}
	close(q.signal)
	q.signal = make(chan struct{})
	v2EventMu.Unlock()

	return event
}

func newV2EventID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// EnqueueV2Update 入队一条 agent.update 升级事件。升级事件按方法整体去重
// （v2EventCoalesceKey 对 update 方法返回固定 key），重复下发只保留最新一条。
func EnqueueV2Update(uuid string, params v2.UpdateParams) v2.Event {
	now := time.Now().UTC()
	event := v2.Event{
		ID:        newV2EventID(),
		Method:    v2.MethodAgentUpdate,
		Params:    params,
		CreatedAt: now,
		ExpiresAt: now.Add(v2UpdateEventTTL),
	}

	v2EventMu.Lock()
	q := getV2EventQueueLocked(uuid)
	pruneExpiredV2EventsLocked(q)
	coalesceV2EventLocked(q, event)
	q.events = append(q.events, event)
	if len(q.events) > v2EventQueueLimit {
		q.events = q.events[len(q.events)-v2EventQueueLimit:]
	}
	close(q.signal)
	q.signal = make(chan struct{})
	v2EventMu.Unlock()

	return event
}

func coalesceV2EventLocked(q *v2EventQueue, event v2.Event) {
	key := v2EventCoalesceKey(event)
	if key == "" {
		return
	}
	filtered := q.events[:0]
	for _, existing := range q.events {
		if v2EventCoalesceKey(existing) != key {
			filtered = append(filtered, existing)
		}
	}
	q.events = filtered
}

func v2EventCoalesceKey(event v2.Event) string {
	if event.Method == v2.MethodAgentUpdate {
		// 升级事件按方法整体去重：同一节点只保留最新一条待升级事件。
		return event.Method
	}
	if event.Method != v2.MethodAgentPing {
		return ""
	}
	var params v2.PingParams
	if err := bindV2EventParams(event.Params, &params); err != nil || params.TaskID == 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", event.Method, params.TaskID)
}

func bindV2EventParams(raw any, target any) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

func ackV2EventsLocked(q *v2EventQueue, ackIDs []string) {
	if len(ackIDs) == 0 || len(q.events) == 0 {
		return
	}
	acked := make(map[string]struct{}, len(ackIDs))
	for _, id := range ackIDs {
		acked[id] = struct{}{}
	}
	filtered := q.events[:0]
	for _, event := range q.events {
		if _, ok := acked[event.ID]; !ok {
			filtered = append(filtered, event)
		}
	}
	q.events = filtered
}

func pruneExpiredV2EventsLocked(q *v2EventQueue) {
	if len(q.events) == 0 {
		return
	}
	now := time.Now().UTC()
	filtered := q.events[:0]
	for _, event := range q.events {
		if event.ExpiresAt.IsZero() {
			// 零 TTL 事件按入队时间 + 默认 TTL 过期，避免永久保留。
			event.ExpiresAt = event.CreatedAt.Add(v2EventDefaultTTL)
		}
		if event.ExpiresAt.After(now) {
			filtered = append(filtered, event)
		}
	}
	q.events = filtered
}

func TakeV2Events(uuid string, ackIDs []string, limit int) []v2.Event {
	v2EventMu.Lock()
	defer v2EventMu.Unlock()

	q := v2EventQueues[uuid]
	if q == nil {
		// 队列已被清理（客户端断开）：返回空，不重建，
		// 否则等待者被唤醒后会重新把条目塞回 map。
		return []v2.Event{}
	}
	ackV2EventsLocked(q, ackIDs)
	pruneExpiredV2EventsLocked(q)
	return takeV2EventsLocked(q, limit)
}

func AckV2Events(uuid string, ackIDs []string) {
	if len(ackIDs) == 0 {
		return
	}
	v2EventMu.Lock()
	defer v2EventMu.Unlock()

	q := v2EventQueues[uuid]
	if q == nil {
		return
	}
	ackV2EventsLocked(q, ackIDs)
}

func takeV2EventsLocked(q *v2EventQueue, limit int) []v2.Event {
	if limit <= 0 || limit > len(q.events) {
		limit = len(q.events)
	}
	events := make([]v2.Event, limit)
	copy(events, q.events[:limit])
	return events
}

func WaitV2Events(uuid string, ackIDs []string, timeout time.Duration) []v2.Event {
	v2EventMu.Lock()
	q := getV2EventQueueLocked(uuid)
	ackV2EventsLocked(q, ackIDs)
	pruneExpiredV2EventsLocked(q)
	events := takeV2EventsLocked(q, v2EventQueueLimit)
	if len(events) > 0 || timeout <= 0 {
		v2EventMu.Unlock()
		return events
	}
	signal := q.signal
	v2EventMu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-signal:
	case <-timer.C:
	}
	return TakeV2Events(uuid, nil, v2EventQueueLimit)
}
