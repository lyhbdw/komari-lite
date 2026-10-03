package ttlcache

import (
	"sync"
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	c := New(time.Minute)
	if _, ok := c.Get("k"); ok {
		t.Fatal("empty cache should miss")
	}
	c.Set("k", 42)
	v, ok := c.Get("k")
	if !ok || v.(int) != 42 {
		t.Fatalf("got %v %v", v, ok)
	}
}

func TestExpiry(t *testing.T) {
	c := New(10 * time.Millisecond)
	c.Set("k", 1)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("should hit before expiry")
	}
	time.Sleep(20 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("should miss after expiry")
	}
}

func TestFlush(t *testing.T) {
	c := New(time.Minute)
	c.Set("k", 1)
	c.Flush()
	if _, ok := c.Get("k"); ok {
		t.Fatal("should miss after flush")
	}
}

func TestConcurrent(t *testing.T) {
	c := New(time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Set("k", j)
				c.Get("k")
			}
		}()
	}
	wg.Wait()
}
