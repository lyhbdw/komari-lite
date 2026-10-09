package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// writeTimeout 是单次 WS 写操作的超时。gorilla/websocket 默认无限等待，
// 网络半死（TCP 未断但拥塞）时写会永久阻塞，卡死整个上报循环。
const writeTimeout = 15 * time.Second

type SafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{
		conn: conn,
		mu:   sync.Mutex{},
	}
}

func (sc *SafeConn) WriteMessage(messageType int, data []byte) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if err := sc.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return sc.conn.WriteMessage(messageType, data)
}

func (sc *SafeConn) WriteJSON(v interface{}) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if err := sc.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return sc.conn.WriteJSON(v)
}

func (sc *SafeConn) Close() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.Close()
}
func (sc *SafeConn) ReadMessage() (int, []byte, error) {
	return sc.conn.ReadMessage()
}

func (sc *SafeConn) SetReadDeadline(t time.Time) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.SetReadDeadline(t)
}

func (sc *SafeConn) SetPongHandler(h func(string) error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.conn.SetPongHandler(h)
}
