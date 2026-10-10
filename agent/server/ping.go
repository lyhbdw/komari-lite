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
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lyhbdw/komari-lite/agent/dnsresolver"
	"github.com/lyhbdw/komari-lite/agent/protocol/transport"
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
	"github.com/lyhbdw/komari-lite/agent/ws"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// HandshakeDeadline 对应 monitor-probe 的 HANDSHAKE_DEADLINE (900ms)。
// 故意设计在 Linux 内核初始 SYN 重传定时器 (1秒) 之内。
// Linux 初始 SYN 计时器约为 1 秒，若超时长于 1 秒，会把丢弃的 SYN 变成迟到的成功，
// 将重传定时器加上 RTT 当作延迟上报（往往表现为 1200ms、3200ms 等非真实链路延迟）。
// 严格控制在 900ms 内截断，可确保每次有效采样都对应首个 SYN 握手完成；超时的首包直接记为 -1 丢包。
const HandshakeDeadline = 900 * time.Millisecond

// MaxPingAddrs 单次探测尝试的最大候选 IP 数量（对应 monitor-probe 的 MAX_PING_ADDRS = 3）。
// 每个地址独立分配一个 HandshakeDeadline 计时，顺序尝试直至成功或全部失败。
const MaxPingAddrs = 3

// ErrDNSOverrun 表示 DNS 解析耗时超过 HandshakeDeadline。
// monitor-probe 规则：DNS 解析超时属于“未采集到样本”，若报告为 -1 会让慢解析在网络未丢包时误绘出虚假丢包。
// 因此解析超时不生成丢包样本，跳过当轮上报。
var ErrDNSOverrun = errors.New("name resolution overrun")

var dnsOverrunSaid sync.Map

// resolveCandidateIPs 解析域名获取最多 MaxPingAddrs 个候选 IP 地址，遵循 IPv4/IPv6 偏好配置。
// 当 DNS 解析超时（> HandshakeDeadline）时返回 ErrDNSOverrun。
func resolveCandidateIPs(target string, deadline time.Duration) ([]string, error) {
	// 如果已经是 IP 地址，直接返回
	if ip := net.ParseIP(target); ip != nil {
		return []string{target}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	ips, err := dnsresolver.ResolveHostWithPreference(ctx, target, flags.PreferIPVersion)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			return nil, ErrDNSOverrun
		}
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("failed to resolve target")
	}
	if len(ips) > MaxPingAddrs {
		ips = ips[:MaxPingAddrs]
	}
	return ips, nil
}

// resolveIP 保留对旧调用的兼容，返回候选首个 IP
func resolveIP(target string) (string, error) {
	ips, err := resolveCandidateIPs(target, 5*time.Second)
	if err != nil {
		return "", err
	}
	return ips[0], nil
}

func icmpPing(target string, timeout time.Duration) (int64, error) {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		host = target
	}
	host = strings.Trim(host, "[]")

	ips, err := resolveCandidateIPs(host, timeout)
	if err != nil {
		return -1, err
	}

	for _, ipStr := range ips {
		rtt, err := icmpPingSingle(ipStr, timeout)
		if err == nil {
			return rtt, nil
		}
	}
	return -1, errors.New("all addresses failed")
}

func icmpPingSingle(ipStr string, timeout time.Duration) (int64, error) {
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
		host = target
		port = "80"
	}
	host = strings.Trim(host, "[]")

	ips, err := resolveCandidateIPs(host, timeout)
	if err != nil {
		return -1, err
	}

	return tcpHandshake(ips, port, timeout)
}

// tcpHandshake 顺序尝试最多 MaxPingAddrs 个候选 IP，每个 IP 独立分配 timeout。
// 计时在每个地址拨号前重置，故障地址不累计到最终耗时中。
func tcpHandshake(ips []string, port string, timeout time.Duration) (int64, error) {
	if len(ips) > MaxPingAddrs {
		ips = ips[:MaxPingAddrs]
	}
	for _, ip := range ips {
		targetAddr := net.JoinHostPort(ip, port)
		start := time.Now()
		conn, err := net.DialTimeout("tcp", targetAddr, timeout)
		if err == nil {
			_ = conn.Close()
			return time.Since(start).Milliseconds(), nil
		}
	}
	return -1, errors.New("all addresses failed")
}

func httpPing(target string, timeout time.Duration) (int64, error) {
	if strings.Contains(target, ":") && !strings.Contains(target, "[") {
		if ip := net.ParseIP(target); ip != nil && ip.To4() == nil {
			target = "[" + target + "]"
		}
	}

	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}

	parsedURL, err := url.Parse(target)
	if err != nil {
		return -1, err
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		if strings.EqualFold(parsedURL.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}

	ips, err := resolveCandidateIPs(host, timeout)
	if err != nil {
		return -1, err
	}

	transport := &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: flags.IgnoreUnsafeCert},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			for _, ip := range ips {
				d := net.Dialer{Timeout: timeout}
				conn, dialErr := d.DialContext(ctx, network, net.JoinHostPort(ip, port))
				if dialErr == nil {
					return conn, nil
				}
			}
			return nil, errors.New("all addresses failed")
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Timeout:   timeout*time.Duration(len(ips)) + 500*time.Millisecond,
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
	timeout := HandshakeDeadline

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

	latency, err = measure()
	if err != nil {
		// monitor-probe 规则：DNS 解析超时属于样本未采集（sample not taken），
		// 不向服务端上报 -1 丢包，避免 slow resolver 造成虚假丢包。
		if errors.Is(err, ErrDNSOverrun) {
			if _, loaded := dnsOverrunSaid.LoadOrStore(pingTarget, true); !loaded {
				log.Printf("%s: name resolution runs past %v, so these rounds report no sample rather than a loss",
					pingTarget, HandshakeDeadline)
			}
			return
		}

		log.Printf("Ping task %d failed: %v", taskID, err)
		pingResult = -1 // 真正无法连通、握手超时或域名不可解析，上报 -1 丢包
	} else {
		pingResult = int(latency)
	}

	finishedAt := time.Now()
	wsPayload := v2.BuildPingResultPayload(taskID, pingType, pingResult, finishedAt)
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
