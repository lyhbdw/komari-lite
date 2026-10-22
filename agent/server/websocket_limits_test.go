package server

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	pkgflags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
)

func TestConnectedWebSocketBoundsMessages(t *testing.T) {
	// Exercise the actual dial path, fragmentation and negotiated compression.
	const limit = 64 << 10
	for _, tc := range []struct {
		name       string
		size       int
		compressed bool
		wantLimit  bool
	}{
		{name: "exact", size: limit},
		{name: "oversized-fragmented", size: limit + 1, wantLimit: true},
		{name: "compressed-exact", size: limit, compressed: true},
		{name: "compressed-expanded-overflow", size: limit + 1, compressed: true, wantLimit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := bytes.Repeat([]byte("x"), tc.size)
			upgrader := websocket.Upgrader{EnableCompression: tc.compressed, WriteBufferSize: 1024}
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					t.Errorf("upgrade: %v", err)
					return
				}
				defer conn.Close()
				writer, err := conn.NextWriter(websocket.TextMessage)
				if err != nil {
					t.Errorf("writer: %v", err)
					return
				}
				if _, err := writer.Write(payload); err != nil {
					t.Errorf("write: %v", err)
					return
				}
				if err := writer.Close(); err != nil {
					t.Errorf("close writer: %v", err)
				}
				_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, _, _ = conn.ReadMessage()
			}))
			defer controller.Close()
			oldFlags := flags
			flags = &pkgflags.Config{DisableCompression: !tc.compressed}
			defer func() { flags = oldFlags }()
			conn, err := connectWebSocket("ws" + strings.TrimPrefix(controller.URL, "http"))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, got, err := conn.ReadMessage()
			if tc.wantLimit {
				if !errors.Is(err, websocket.ErrReadLimit) {
					t.Fatalf("overflow: received %d bytes, err=%v; want read-limit rejection", len(got), err)
				}
				if len(got) != 0 {
					t.Fatalf("overflow exposed %d partial bytes", len(got))
				}
			} else if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("exact boundary: received %d bytes, err=%v", len(got), err)
			}
		})
	}
}
