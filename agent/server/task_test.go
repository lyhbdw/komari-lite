package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/agent/dnsresolver"
)

func TestResolveIPDirect(t *testing.T) {
	// 针对纯 IP 地址，不应产生外部 DNS 查询，直接返回原 IP
	ip, err := resolveIP("127.0.0.1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ip != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %s", ip)
	}

	ip6, err := resolveIP("::1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ip6 != "::1" {
		t.Fatalf("expected ::1, got %s", ip6)
	}
}

func TestResolveIPWithCache(t *testing.T) {
	// 测试 ResolveHostWithPreference 的缓存命中与解析逻辑
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ips, err := dnsresolver.ResolveHostWithPreference(ctx, "localhost", "4")
	if err != nil {
		t.Skipf("skipping localhost lookup if environment has no resolver: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("expected at least 1 resolved IP for localhost")
	}

	// 第二次调用应命中缓存
	start := time.Now()
	cachedIPs, err := dnsresolver.ResolveHostWithPreference(ctx, "localhost", "4")
	if err != nil {
		t.Fatalf("cached lookup failed: %v", err)
	}
	if len(cachedIPs) != len(ips) {
		t.Fatalf("cache result length mismatch: got %d, want %d", len(cachedIPs), len(ips))
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Logf("cached lookup took longer than expected: %v", time.Since(start))
	}
}

func TestTCPPingLocalhost(t *testing.T) {
	// 启动一个临时的 TCP listener 来测试 tcpPing
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on 127.0.0.1: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	latency, err := tcpPing(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("expected tcpPing success, got: %v", err)
	}
	if latency < 0 {
		t.Fatalf("expected non-negative latency, got %d", latency)
	}
}
