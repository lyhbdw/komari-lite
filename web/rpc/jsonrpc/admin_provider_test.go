package jsonrpc

import (
	"strings"
	"testing"
)

func TestRedactProviderAdditionRemovesSecrets(t *testing.T) {
	input := `{"bot_token":"secret","password":"pw","chat_id":"123","nested":{"api_key":"key"}}`
	got := redactProviderAddition(input)
	for _, secret := range []string{"secret", "pw", `"key":"key"`} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted provider addition contains secret %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "chat_id") {
		t.Fatalf("non-sensitive provider setting was removed: %s", got)
	}
}
