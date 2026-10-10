package jsonrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
)

func TestQueryBudgetWebSocketReceivesDeadline(t *testing.T) {
	method := "common:" + t.Name()
	if err := rpc.Register(method, func(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
		deadline, ok := ctx.Deadline()
		return ok && time.Until(deadline) <= 11*time.Second, nil
	}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/api/rpc2", OnRpcRequest)
	server := httptest.NewServer(router)
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/rpc2", http.Header{"Origin": []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(&rpc.JsonRpcRequest{Version: rpc.RPC_VERSION, ID: 1, Method: method}); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var resp rpc.JsonRpcResponse
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil || resp.Result != true {
		t.Fatalf("WS handler received no bounded context: %+v", resp)
	}
}
