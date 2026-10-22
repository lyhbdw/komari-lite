package jsonrpc

import (
	"context"
	"testing"

	"github.com/lyhbdw/komari-lite/pkg/rpc"
)

// The memory snapshot endpoint is read-only and admin-only, and it must stay
// useful before the metric store opens: a fresh install has runtime stats but
// no store, and the response has to say so instead of failing.
func TestMemoryStatsRPCIsRegisteredReadOnlyForAdmins(t *testing.T) {
	meta := rpc.Call(1, "rpc.help", map[string]any{"method": "admin:getMemoryStats"})
	if meta.Error != nil {
		t.Fatalf("memory stats RPC must be registered: %+v", meta.Error)
	}
	if rpc.IsSensitive("admin:getMemoryStats") {
		t.Fatalf("a read-only stats endpoint must not require 2FA")
	}
	req := &rpc.JsonRpcRequest{Version: rpc.RPC_VERSION, ID: 1, Method: "admin:getMemoryStats"}
	anonymous := Dispatch(context.Background(), nil, req)
	if anonymous.Error == nil || anonymous.Error.Code != rpc.PermissionDenied {
		t.Fatalf("anonymous memory stats must be denied: %+v", anonymous)
	}
	admin := Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, req)
	if admin.Error != nil {
		t.Fatalf("admin memory stats must succeed: %+v", admin.Error)
	}
	result, ok := admin.Result.(map[string]any)
	if !ok {
		t.Fatalf("memory stats result = %#v, want a map", admin.Result)
	}
	if ready, exists := result["store_ready"]; !exists || ready != false {
		t.Fatalf("store_ready = %#v (exists=%v), want false when the store is closed", ready, exists)
	}
	if _, exists := result["go"]; !exists {
		t.Fatalf("result must always include runtime stats: %#v", result)
	}
}
