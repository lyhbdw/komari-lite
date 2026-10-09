package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lyhbdw/komari-lite/agent/dnsresolver"
	"github.com/lyhbdw/komari-lite/agent/protocol/transport"
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
	"github.com/lyhbdw/komari-lite/agent/ws"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// resolveIP 解析域名到 IP 地址，排除 DNS 查询时间，遵循自定义 DNS 及 IPv4/IPv6 偏好配置
func resolveIP(target string) (string, error) {
	// 如果已经是 IP 地址，直接返回
	if ip := net.ParseIP(target); ip != nil {
		return target, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := dnsresolver.ResolveHostWithPreference(ctx, target, flags.PreferIPVersion)
	if err != nil || len(ips) == 0 {
		return "", errors.New("failed to resolve target")
	}
	return ips[0], nil
}

func icmpPing(target string, timeout time.Duration) (int64, error) {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		host = target
	}
	host = strings.Trim(host, "[]")

	ipStr, err := resolveIP(host)
	if err != nil {
		return -1, err
	}
	dstIP := net.ParseIP(ipStr)
	if dstIP == nil {
		return -1, errors.New("invalid IP address")
	}

	isIPv4 := dstIP.To4() != nil
	network := "udp4"
	listenAddr := "0.0.0.0"
	if !isIPv4 {
		network = "udp6"
		listenAddr = "::"
	}

	c, err := icmp.ListenPacket(network, listenAddr)
	if err != nil {
		if isIPv4 {
			c, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		} else {
			c, err = icmp.ListenPacket("ip6:ipv6-icmp", "::")
		}
		if err != nil {
			return -1, err
		}
	}
	defer c.Close()

	_ = c.SetDeadline(time.Now().Add(timeout))

	var msg icmp.Message
	if isIPv4 {
		msg = icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   os.Getpid() & 0xffff,
				Seq:  1,
				Data: []byte("KOMARI"),
			},
		}
	} else {
		msg = icmp.Message{
			Type: ipv6.ICMPTypeEchoRequest,
			Code: 0,
			Body: &icmp.Echo{
				ID:   os.Getpid() & 0xffff,
				Seq:  1,
				Data: []byte("KOMARI"),
			},
		}
	}

	wb, err := msg.Marshal(nil)
	if err != nil {
		return -1, err
	}

	var dst net.Addr
	if _, ok := c.LocalAddr().(*net.IPAddr); ok {
		dst = &net.IPAddr{IP: dstIP}
	} else {
		dst = &net.UDPAddr{IP: dstIP}
	}

	start := time.Now()
	if _, err := c.WriteTo(wb, dst); err != nil {
		return -1, err
	}

	rb := make([]byte, 1500)
	proto := 1
	if !isIPv4 {
		proto = 58
	}
	for {
		n, _, err := c.ReadFrom(rb)
		if err != nil {
			return -1, err
		}
		rtt := time.Since(start).Milliseconds()

		rm, err := icmp.ParseMessage(proto, rb[:n])
		if err != nil {
			continue
		}
		switch rm.Type {
		case ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply:
			return rtt, nil
		}
	}
}

func tcpPing(target string, timeout time.Duration) (int64, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		// No port, assume port 80
		host = target
		port = "80"
	}

	// If the host is an IPv6 literal, it might be wrapped in brackets.
	host = strings.Trim(host, "[]")

	ip, err := resolveIP(host)
	if err != nil {
		return -1, err
	}

	targetAddr := net.JoinHostPort(ip, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", targetAddr, timeout)
	if err != nil {
		return -1, err
	}
	defer conn.Close()
	return time.Since(start).Milliseconds(), nil
}

func httpPing(target string, timeout time.Duration) (int64, error) {
	// Handle raw IPv6 address for URL
	if strings.Contains(target, ":") && !strings.Contains(target, "[") {
		// check if it's a valid IP to avoid wrapping hostnames
		if ip := net.ParseIP(target); ip != nil && ip.To4() == nil {
			target = "[" + target + "]"
		}
	}

	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}

	transport := &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: flags.IgnoreUnsafeCert},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 在 Dial 之前解析 IP，排除 DNS 时间
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ip, err := resolveIP(host)
			if err != nil {
				return nil, err
			}
			return net.DialTimeout(network, net.JoinHostPort(ip, port), timeout)
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	start := time.Now()
	resp, err := client.Get(target)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return latency, nil
	}
	return latency, errors.New("http status not ok")
}

func NewPingTask(conn *ws.SafeConn, taskID uint, pingType, pingTarget string) {
	if taskID == 0 {
		log.Printf("Invalid task ID: %d", taskID)
		return
	}
	var err error = nil
	var latency int64
	pingResult := -1
	timeout := 3 * time.Second        // 默认超时时间
	const highLatencyThreshold = 1000 // ms 阈值

	measure := func() (int64, error) {
		switch pingType {
		case "icmp":
			return icmpPing(pingTarget, timeout)
		case "tcp":
			return tcpPing(pingTarget, timeout)
		case "http":
			return httpPing(pingTarget, timeout)
		default:
			return -1, errors.New("unsupported ping type")
		}
	}

	// 首次测量
	if latency, err = measure(); err == nil {
		// 若初次测量延迟偏高（> 1000ms），可能受冷启动或偶发握手抖动影响，进行复测以获取更准确的稳定值
		if latency > int64(highLatencyThreshold) {
			for i := 0; i < 2; i++ {
				if second, err2 := measure(); err2 == nil {
					if second < latency {
						latency = second
					}
					if latency <= int64(highLatencyThreshold) {
						break
					}
				}
			}
		}
	}

	if err != nil {
		log.Printf("Ping task %d failed: %v", taskID, err)
		pingResult = -1 // 如果有错误，设置结果为 -1
	} else {
		pingResult = int(latency)
	}
	finishedAt := time.Now()
	wsPayload := v2.BuildPingResultPayload(taskID, pingType, pingResult, finishedAt)
	// https://github.com/komari-monitor/komari/commit/eb87a4fc330b7d1c407fa4ff70177615a4f50a1f
	// -1 代表丢包，服务端计算
	//if pingResult == -1 {
	//	return
	//}
	if conn == nil {
		if err := postV2RPC(wsPayload); err != nil {
			log.Printf("Failed to upload ping result over POST: %v", err)
		}
		return
	}
	if err := conn.WriteJSON(wsPayload); err != nil {
		log.Printf("Failed to write JSON to WebSocket: %v", err)
	}

}

func postV2RPC(payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = postV2Payload(context.Background(), body, 30*time.Second, false)
	return err
}

// postV2Payload 压缩并 POST v2 RPC 载荷到面板，返回解析后的响应。
// requireResult 为 true 时，空响应体视为错误（报告/基础信息上传路径需要确认）。
func postV2Payload(ctx context.Context, body []byte, timeout time.Duration, requireResult bool) (*v2.Response, error) {
	endpoint := strings.TrimSuffix(flags.Endpoint, "/") + "/api/clients/v2/rpc"
	compressed := false
	if !flags.DisableCompression {
		if gz, err := transport.GzipBytes(body); err == nil {
			body = gz
			compressed = true
		}
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	addAgentAuthorization(req)
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}
	client := dnsresolver.GetHTTPClientWithPreference(timeout, flags.PreferIPVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := readBoundedBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &httpStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(respBody)}
	}
	if len(bytes.TrimSpace(respBody)) == 0 {
		if requireResult {
			return nil, errors.New("empty v2 rpc response")
		}
		return nil, nil
	}
	return parseV2Response(respBody)
}
