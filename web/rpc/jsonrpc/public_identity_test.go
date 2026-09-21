package jsonrpc

import (
	"strings"
	"testing"

	"github.com/komari-monitor/komari/pkg/rpc"
)

func TestPublicAgentIdentityDoesNotExposeToken(t *testing.T) {
	meta := &rpc.ContextMeta{Principal: rpc.NewAgentPrincipal("client-uuid"), ClientUUID: "client-uuid", ClientToken: "secret-token"}
	got := publicAgentIdentity(meta)
	if got["uuid"] != "client-uuid" {
		t.Fatalf("agent identity uuid = %v", got["uuid"])
	}
	if strings.Contains(got["uuid"].(string), "secret-token") {
		t.Fatalf("agent identity exposed token: %#v", got)
	}
}
