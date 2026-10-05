package jsonrpc

import (
	"strings"
	"testing"

	"github.com/lyhbdw/komari-monitor-lite/pkg/rpc"
)

func TestPublicAgentIdentityDoesNotExposeToken(t *testing.T) {
	meta := &rpc.ContextMeta{Principal: rpc.NewAgentPrincipal("client-uuid"), ClientUUID: "client-uuid", ClientToken: "secret-token"}
	got := map[string]any{"username": "client", "logged_in": true, "uuid": meta.ClientUUID}
	if got["uuid"] != "client-uuid" {
		t.Fatalf("agent identity uuid = %v", got["uuid"])
	}
	if strings.Contains(got["uuid"].(string), "secret-token") {
		t.Fatalf("agent identity exposed token: %#v", got)
	}
}
