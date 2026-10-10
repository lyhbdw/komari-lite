package server

import (
	"context"
	"errors"
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

func TestHandshakeDeadlineStaysUnderInitialRTO(t *testing.T) {
	// 握手超时必须严格小于 1s 内核初始 SYN RTO，防止首包重传被算作延迟
	if HandshakeDeadline >= 1*time.Second {
		t.Fatalf("HandshakeDeadline (%v) must stay strictly under Linux initial SYN RTO (1s)", HandshakeDeadline)
	}
}

func TestTCPPingMeasuresSuccessAndReportsFailure(t *testing.T) {
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

	// 成功连接应返回非负毫秒延迟
	latency, err := tcpPing(addr, HandshakeDeadline)
	if err != nil {
		t.Fatalf("expected tcpPing success, got: %v", err)
	}
	if latency < 0 {
		t.Fatalf("expected non-negative latency, got %d", latency)
	}

	// 无监听端口应返回 -1 丢包
	deadLat, deadErr := tcpPing("127.0.0.1:1", 100*time.Millisecond)
	if deadErr == nil || deadLat != -1 {
		t.Fatalf("expected -1 on dead port, got %d, %v", deadLat, deadErr)
	}

	// 不可解析的主机应返回 -1 丢包
	unresLat, unresErr := tcpPing("127.0.0.1:99999", 100*time.Millisecond)
	if unresErr == nil || unresLat != -1 {
		t.Fatalf("expected -1 on unresolvable target, got %d, %v", unresLat, unresErr)
	}
}

func TestTCPHandshakeMultiAddressFallback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on 127.0.0.1: %v", err)
	}
	defer ln.Close()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// 首个地址超时/不可达时，能够顺利降级到后续有效地址
	deadIP := "192.0.2.1" // RFC 5737 TEST-NET-1 (unroutable)
	liveIP := "127.0.0.1"

	latency, err := tcpHandshake([]string{deadIP, liveIP}, port, 50*time.Millisecond)
	if err != nil || latency < 0 {
		t.Fatalf("expected success on second address, got %d, err: %v", latency, err)
	}

	// 所有地址均失败时返回 -1
	allDeadLat, allDeadErr := tcpHandshake([]string{deadIP, deadIP}, port, 50*time.Millisecond)
	if allDeadErr == nil || allDeadLat != -1 {
		t.Fatalf("expected -1 when all addresses fail, got %d, err: %v", allDeadLat, allDeadErr)
	}

	// 超过 MaxPingAddrs (3) 的第 4 个地址不应被尝试
	cappedLat, cappedErr := tcpHandshake([]string{deadIP, deadIP, deadIP, liveIP}, port, 50*time.Millisecond)
	if cappedErr == nil || cappedLat != -1 {
		t.Fatalf("expected 4th address not to be tried, got %d, %v", cappedLat, cappedErr)
	}
}

func TestDNSOverrunSentinel(t *testing.T) {
	// 测试超时被正确识别为 ErrDNSOverrun
	_, err := resolveCandidateIPs("example.com", 1*time.Nanosecond)
	if err == nil {
		t.Skip("lookup completed faster than 1ns, skipping deadline test")
	}
	if !errors.Is(err, ErrDNSOverrun) {
		t.Fatalf("expected ErrDNSOverrun on expired deadline, got: %v", err)
	}
}
