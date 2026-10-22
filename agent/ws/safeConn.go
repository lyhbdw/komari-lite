package ws

import (
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// writeTimeout 是单次 WS 写操作的超时。gorilla/websocket 默认无限等待，
// 网络半死（TCP 未断但拥塞）时写会永久阻塞，卡死整个上报循环。
const writeTimeout = 15 * time.Second

type SafeConn struct {
	conn      *websocket.Conn
	mu        sync.Mutex
	readLimit int64
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
	if sc.readLimit <= 0 {
		return sc.conn.ReadMessage()
	}
	messageType, reader, err := sc.conn.NextReader()
	if err != nil {
		return messageType, nil, err
	}
	// Gorilla's limit counts compressed frame bytes. Bound the expanded
	// message as well, before allocating or decoding the complete payload.
	data, err := io.ReadAll(io.LimitReader(reader, sc.readLimit+1))
	if err != nil {
		return messageType, nil, err
	}
	if int64(len(data)) > sc.readLimit {
		_ = sc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseMessageTooBig, "message too big"), time.Now().Add(time.Second))
		_ = sc.conn.Close()
		return messageType, nil, websocket.ErrReadLimit
	}
	return messageType, data, nil
}

// SetReadLimit must be called before starting the connection's reader.
func (sc *SafeConn) SetReadLimit(limit int64) {
	sc.readLimit = limit
	sc.conn.SetReadLimit(limit)
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
