package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lyhbdw/komari-lite/cmd/flags"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
	"github.com/lyhbdw/komari-lite/pkg/metric"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
	agentRuntime "github.com/lyhbdw/komari-lite/web/agent"
)

func setupRawV2Ingestion(t *testing.T) {
	t.Helper()
	oldType, oldFile := flags.DatabaseType, flags.DatabaseFile
	flags.DatabaseType, flags.DatabaseFile = "sqlite", "file:v2_rpc_raw?mode=memory&cache=shared"
	dbcore.GetDBInstance()
	oldDSN, _ := config.GetAs[string]("metric_db_dsn")
	if err := config.Set("metric_db_dsn", filepath.Join(t.TempDir(), "metrics.db")); err != nil {
		t.Fatal(err)
	}
	if err := metricstore.InitializeStore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricstore.CloseStoreContext(ctx); err != nil {
			t.Errorf("close test store: %v", err)
		}
		if err := config.Set("metric_db_dsn", oldDSN); err != nil {
			t.Errorf("restore metrics DSN: %v", err)
		}
		flags.DatabaseType, flags.DatabaseFile = oldType, oldFile
	})
}

func cleanupRawV2Client(t *testing.T, uuid string) {
	t.Helper()
	t.Cleanup(func() {
		postPresenceMu.Lock()
		if entry := postPresenceStates[uuid]; entry != nil {
			entry.timer.Stop()
			delete(postPresenceStates, uuid)
		}
		postPresenceMu.Unlock()
		agentRuntime.DeleteConnectedClients(uuid)
		agentRuntime.DeleteLatestReport(uuid)
	})
}

func TestV2RPCIngestsExactIntegersOverHTTPAndWebSocket(t *testing.T) {
	setupRawV2Ingestion(t)
	const exact = 9007199254740993
	report := fmt.Sprintf(`{"jsonrpc":"2.0","method":"agent.report","id":%d,"params":{"report":{"uuid":"untrusted","ram":{"total":%d},"disk":{"total":%d},"network":{"totalUp":%d,"totalDown":%d},"uptime":%d,"gpu":{"count":1,"detailed_info":[{"memory_total":%d}]}}}}`, exact, exact, exact, exact, exact, exact, exact)
	ping := fmt.Sprintf(`{"jsonrpc":"2.0","method":"agent.pingResult","id":"ping","params":{"task_id":%d,"value":-1,"finished_at":"2026-01-01T00:00:00Z"}}`, exact)
	for _, transport := range []string{"http", "gzip", "websocket"} {
		t.Run(transport, func(t *testing.T) {
			uuid := "raw-v2-" + transport
			cleanupRawV2Client(t, uuid)
			controller := httptest.NewServer(newV2RPCTestRouter(uuid))
			defer controller.Close()
			var exchange func(string) []byte
			if transport == "websocket" {
				conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(controller.URL, "http")+"/rpc", http.Header{"Origin": []string{controller.URL}})
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				defer conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
				exchange = func(payload string) []byte {
					t.Helper()
					if err := conn.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
						t.Fatal(err)
					}
					conn.SetReadDeadline(time.Now().Add(3 * time.Second))
					_, body, err := conn.ReadMessage()
					if err != nil {
						t.Fatal(err)
					}
					return body
				}
			} else {
				exchange = func(payload string) []byte {
					t.Helper()
					body := []byte(payload)
					if transport == "gzip" {
						var compressed bytes.Buffer
						w := gzip.NewWriter(&compressed)
						if _, err := w.Write(body); err != nil {
							t.Fatal(err)
						}
						if err := w.Close(); err != nil {
							t.Fatal(err)
						}
						body = compressed.Bytes()
					}
					req, err := http.NewRequest(http.MethodPost, controller.URL+"/rpc", bytes.NewReader(body))
					if err != nil {
						t.Fatal(err)
					}
					if transport == "gzip" {
						req.Header.Set("Content-Encoding", "gzip")
					}
					resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
					if err != nil {
						t.Fatal(err)
					}
					defer resp.Body.Close()
					data, err := io.ReadAll(resp.Body)
					if err != nil {
						t.Fatal(err)
					}
					if resp.StatusCode != http.StatusOK {
						t.Fatalf("HTTP %d: %s", resp.StatusCode, data)
					}
					return data
				}
			}
			response := exchange(report)
			var envelope struct {
				ID     json.RawMessage `json:"id"`
				Error  *v2.RPCError    `json:"error"`
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(response, &envelope); err != nil || envelope.Error != nil || string(envelope.ID) != "9007199254740993" {
				t.Fatalf("report response = %s, %v", response, err)
			}
			got := agentRuntime.GetLatestReport()[uuid]
			if got == nil {
				t.Fatal("actual ingest path did not record report")
			}
			if got.UUID != uuid || got.Ram.Total != exact || got.Disk.Total != exact || got.Network.TotalUp != exact || got.Network.TotalDown != exact || got.Uptime != exact || got.GPU == nil || got.GPU.DetailedInfo[0].MemoryTotal != exact {
				t.Fatalf("ingested integers changed: %+v", got)
			}
			if transport == "websocket" {
				postPresenceMu.Lock()
				entry := postPresenceStates[uuid]
				postPresenceMu.Unlock()
				if entry != nil {
					t.Fatal("WebSocket report created POST presence")
				}
			}
			response = exchange(ping)
			if err := json.Unmarshal(response, &envelope); err != nil || envelope.Error != nil {
				t.Fatalf("ping response = %s, %v", response, err)
			}
			points, err := metricstore.GetStore().Query(context.Background(), metric.Query{MetricName: metricstore.MetricPingLatency, EntityID: uuid, Start: time.Now().Add(-time.Minute), End: time.Now().Add(time.Second)})
			if err != nil || len(points) != 1 || points[0].Tags["task_id"] != "9007199254740993" || points[0].Value != -1 {
				t.Fatalf("ingested ping = %+v, %v", points, err)
			}
		})
	}
}
