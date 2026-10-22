package jsonrpc

import (
	"context"
	"testing"

	"github.com/lyhbdw/komari-lite/pkg/rpc"
)

func TestEffectiveRetentionRPCIsRegisteredReadOnlyForAdmins(t *testing.T) {
	meta := rpc.Call(1, "rpc.help", map[string]any{"method": "admin:getEffectiveRetention"})
	if meta.Error != nil {
		t.Fatalf("effective retention RPC must be registered: %+v", meta.Error)
	}
	req := &rpc.JsonRpcRequest{Version: rpc.RPC_VERSION, ID: 1, Method: "admin:getEffectiveRetention"}
	anonymous := Dispatch(context.Background(), nil, req)
	if anonymous.Error == nil || anonymous.Error.Code != rpc.PermissionDenied {
		t.Fatalf("anonymous retention inspection must be denied: %+v", anonymous)
	}
	admin := Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, req)
	if admin.Error == nil || admin.Error.Code != rpc.Unavailable {
		t.Fatalf("uninitialized store must report unavailable without touching storage: %+v", admin)
	}
}
