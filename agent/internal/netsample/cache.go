// Package netsample shares a short-lived cross-platform gopsutil counter sample.
package netsample

import (
	"sync"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

const sampleTTL = 500 * time.Millisecond

type cache struct {
	sync.Mutex
	at       time.Time
	counters []gnet.IOCountersStat
	err      error
}

func (c *cache) get(now time.Time, read func() ([]gnet.IOCountersStat, error)) ([]gnet.IOCountersStat, error) {
	c.Lock()
	defer c.Unlock()
	if c.at.IsZero() || now.Sub(c.at) >= sampleTTL || now.Before(c.at) {
		c.counters, c.err = read()
		c.at = now
	}
	// Consumers must not mutate the shared sample.
	return append([]gnet.IOCountersStat(nil), c.counters...), c.err
}

var shared cache

func Counters() ([]gnet.IOCountersStat, error) {
	return shared.get(time.Now(), func() ([]gnet.IOCountersStat, error) { return gnet.IOCounters(true) })
}
