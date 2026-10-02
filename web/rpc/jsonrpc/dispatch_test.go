package jsonrpc

import (
	"testing"

	"github.com/Tumb1er1376/komari-monitor-lite/pkg/rpc"
)

func TestRpcHelpUsesQualifiedMethodName(t *testing.T) {
	resp := rpc.Call(1, "rpc.help", map[string]any{"method": "public:getMe"})
	if resp.Error != nil {
		t.Fatalf("rpc.help returned error: %+v", resp.Error)
	}
	meta, ok := resp.Result.(*rpc.MethodMeta)
	if !ok || meta.Name != "public:getMe" {
		t.Fatalf("unexpected method metadata: %#v", resp.Result)
	}
}

func TestDatabaseMaintenanceRPCMethodsAreUnavailable(t *testing.T) {
	for _, method := range []string{"admin:getDatabaseSize", "admin:vacuumDatabase", "admin:dbTables"} {
		if resp := rpc.Call(1, "rpc.help", map[string]any{"method": method}); resp.Error == nil {
			t.Fatalf("database maintenance RPC %q must remain unavailable", method)
		}
	}
}

func TestLiteDisabledCapabilitiesAreNotRegisteredInRPC2(t *testing.T) {
	for _, method := range []string{
		"admin:terminal",
		"admin:plugin",
		"admin:restore",
		"admin:backupRestore",
		"admin:clipboard",
		"admin:oidc",
		"client:terminal",
		"client:file",
	} {
		if resp := rpc.Call(1, method, nil); resp.Error == nil || resp.Error.Code != rpc.MethodNotFound {
			t.Fatalf("Lite-disabled RPC method %q must not be registered: %+v", method, resp)
		}
	}
}
