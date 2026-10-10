package netsample

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

func TestCountersCoalescesConcurrentConsumers(t *testing.T) {
	var c cache
	var calls atomic.Int32
	reader := func() ([]gnet.IOCountersStat, error) {
		calls.Add(1)
		return []gnet.IOCountersStat{{Name: "eth0", BytesSent: 12}}, nil
	}
	now := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.get(now, reader)
			if err != nil || got[0].BytesSent != 12 {
				t.Errorf("sample=%v,%v", got, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("counter reads=%d; want 1", calls.Load())
	}
	_, _ = c.get(now.Add(time.Second), reader)
	if calls.Load() != 2 {
		t.Fatal("expired counter sample not refreshed")
	}
}
