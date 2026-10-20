package dnsresolver

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPClientTransportConfigAndCache(t *testing.T) {
	originalUnsafe := flags.IgnoreUnsafeCert
	httpClientMu.Lock()
	originalClients := httpClients
	httpClients = make(map[httpClientKey]*http.Client)
	httpClientMu.Unlock()
	t.Cleanup(func() {
		httpClientMu.Lock()
		for _, client := range httpClients {
			client.CloseIdleConnections()
		}
		httpClients = originalClients
		httpClientMu.Unlock()
		flags.IgnoreUnsafeCert = originalUnsafe
	})
	flags.IgnoreUnsafeCert = false
	client := getHTTPClient(0, "4")
	if client != getHTTPClient(-time.Second, "4") || client != GetHTTPClientWithPreference(30*time.Second, "4") {
		t.Fatal("normalized equivalent requests must reuse cached client")
	}
	if client.Timeout != 30*time.Second {
		t.Fatalf("timeout = %v", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T", client.Transport)
	}
	if !transport.ForceAttemptHTTP2 || transport.TLSNextProto != nil {
		t.Fatal("HTTP/2 must be explicitly enabled without disabling protocol upgrades")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS verification must be enabled by default")
	}
	if transport.Proxy == nil || transport.DialContext == nil {
		t.Fatal("missing proxy/custom DNS dialer")
	}
	if transport.MaxIdleConns != 32 || transport.MaxIdleConnsPerHost != 4 || transport.MaxConnsPerHost != 8 ||
		transport.IdleConnTimeout != 90*time.Second || transport.TLSHandshakeTimeout != 10*time.Second || transport.ExpectContinueTimeout != time.Second {
		t.Fatalf("transport limits/timeouts changed: %+v", transport)
	}
	short := GetHTTPClientWithPreference(time.Second, "4")
	if short == client || short.Timeout != time.Second {
		t.Fatal("timeout must distinguish cached clients")
	}
	if GetHTTPClientWithPreference(30*time.Second, "6") == client {
		t.Fatal("IP preference must distinguish cached clients")
	}
	if GetHTTPClientWithPreference(30*time.Second, "invalid") != GetHTTPClientWithPreference(30*time.Second, "") {
		t.Fatal("invalid IP preference must normalize to automatic")
	}
	flags.IgnoreUnsafeCert = true
	unsafeClient := GetHTTPClientWithPreference(30*time.Second, "4")
	if unsafeClient == client || !unsafeClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS flag must distinguish cached clients and configure transport")
	}
	flags.IgnoreUnsafeCert = false
	if GetHTTPClientWithPreference(30*time.Second, "4") != client {
		t.Fatal("original TLS configuration must reuse its client")
	}
}
