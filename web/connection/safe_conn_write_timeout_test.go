package connection

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func wsSinkServer(t *testing.T) *httptest.Server {
	t.Helper()
	var upgrader websocket.Upgrader
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Never read: the server-side receive buffer fills up, so client
		// writes eventually block once TCP buffers are exhausted.
		select {}
	}))
}

func TestSafeConnWriteTimeoutUnblocksWriter(t *testing.T) {
	server := wsSinkServer(t)
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	rawConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rawConn.Close()

	// Shrink the client-side send buffer so the write stalls quickly.
	if tcp, ok := rawConn.UnderlyingConn().(*net.TCPConn); ok {
		_ = tcp.SetWriteBuffer(1024)
	}

	conn := NewSafeConn(rawConn)
	original := writeTimeout
	writeTimeout = 500 * time.Millisecond
	t.Cleanup(func() { writeTimeout = original })

	payload := strings.Repeat("x", 64*1024)
	deadline := time.Now().Add(5 * time.Second)
	var writeErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			if writeErr = conn.WriteMessage(websocket.TextMessage, []byte(payload)); writeErr != nil {
				return
			}
		}
	}()

	select {
	case <-done:
		if writeErr == nil {
			t.Fatal("expected a write error after the deadline, got nil")
		}
	case <-time.After(time.Until(deadline)):
		t.Fatal("WriteMessage blocked past the write timeout; slow consumer wedges the connection")
	}

	// Close must still complete even after a timed-out write.
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		_ = conn.Close()
	}()
	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked after a write timeout")
	}
}
