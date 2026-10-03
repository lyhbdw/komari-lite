// Package ttlcache 提供一个极简的带 TTL 并发安全内存缓存，
// 替代已归档的 github.com/patrickmn/go-cache。
//
// 语义差异说明（相对 go-cache）：
//   - 仅支持整体默认 TTL（New 时指定），不支持按条目覆盖 TTL；
//     现有调用点（geoip / traffic / pingStats）都只用默认 TTL，语义等价。
//   - 惰性过期：Get 时检查过期，不启动后台清理 goroutine。
//     条目量级为「节点数 × 常数」（几十到几百），即使全部过期
//     驻留内存也不足 1MB，无需主动清理。
package ttlcache

import (
	"sync"
	"time"
)

type entry struct {
	value     any
	expiresAt time.Time
}

// Cache 是并发安全的 TTL 内存缓存。零值不可用，必须通过 New 创建。
type Cache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]entry
}

// New 创建缓存，所有条目在 ttl 后过期。
func New(ttl time.Duration) *Cache {
	return &Cache{
		ttl:     ttl,
		entries: make(map[string]entry),
	}
}

// Get 返回 key 对应的值；不存在或已过期时 found 为 false。
func (c *Cache) Get(key string) (value any, found bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		// 惰性删除：过期条目在这里顺带清掉。
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}
	return e.value, true
}

// Set 写入 key，使用 New 时指定的 TTL。
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	c.entries[key] = entry{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// Flush 清空全部条目。
func (c *Cache) Flush() {
	c.mu.Lock()
	c.entries = make(map[string]entry)
	c.mu.Unlock()
}
