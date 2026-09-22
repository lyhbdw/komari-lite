package agent

import (
	"testing"
	"time"

	"github.com/komari-monitor/komari/web/connection"
)

func setupPresenceTestState(t *testing.T) {
	t.Helper()
	mu.Lock()
	previousConnected := connectedClients
	previousV2 := v2Clients
	previousPresence := presenceOnly
	connectedClients = make(map[string]*connection.SafeConn)
	v2Clients = make(map[string]struct{})
	presenceOnly = make(map[string]struct {
		id     int64
		expire time.Time
	})
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		connectedClients = previousConnected
		v2Clients = previousV2
		presenceOnly = previousPresence
		mu.Unlock()
	})
}

func TestGetAllOnlineUUIDsGarbageCollectsExpiredPresence(t *testing.T) {
	setupPresenceTestState(t)

	KeepAlivePresence("live-post", 1, time.Minute)
	KeepAlivePresence("dead-post", 2, -time.Minute) // already expired

	online := GetAllOnlineUUIDs()
	found := map[string]bool{}
	for _, uuid := range online {
		found[uuid] = true
	}
	if !found["live-post"] {
		t.Fatal("live POST presence missing from online list")
	}
	if found["dead-post"] {
		t.Fatal("expired POST presence still reported online")
	}

	// The expired entry must have been removed from the map, not just skipped.
	mu.RLock()
	_, stillTracked := presenceOnly["dead-post"]
	mu.RUnlock()
	if stillTracked {
		t.Fatal("expired presence entry was not garbage-collected from the map")
	}
}

func TestDeleteClientConditionallyPreservesActivePostPresence(t *testing.T) {
	setupPresenceTestState(t)

	conn := &connection.SafeConn{}
	mu.Lock()
	connectedClients["mixed-agent"] = conn
	mu.Unlock()
	MarkV2Client("mixed-agent")
	// The same agent also has a live POST reporting session.
	KeepAlivePresence("mixed-agent", 42, time.Minute)

	DeleteClientConditionally("mixed-agent", conn)

	if !IsV2Client("mixed-agent") {
		t.Fatal("WS disconnect cleared the v2 marker while a POST session was still active")
	}

	// Once the POST presence expires, the existing offline path
	// (ClearV2ClientIfOffline) must be able to clear the marker.
	KeepAlivePresence("mixed-agent", 42, -time.Minute)
	if !ClearV2ClientIfOffline("mixed-agent") {
		t.Fatal("expected stale v2 marker to be clearable after POST presence expired")
	}
	if IsV2Client("mixed-agent") {
		t.Fatal("v2 marker survived after both WS and POST sessions ended")
	}
}

func TestDeleteClientConditionallyClearsMarkerWithoutPostPresence(t *testing.T) {
	setupPresenceTestState(t)

	conn := &connection.SafeConn{}
	mu.Lock()
	connectedClients["ws-only"] = conn
	mu.Unlock()
	MarkV2Client("ws-only")

	DeleteClientConditionally("ws-only", conn)

	if IsV2Client("ws-only") {
		t.Fatal("v2 marker survived WS disconnect for a pure-WS agent")
	}

	mu.RLock()
	_, stillConnected := connectedClients["ws-only"]
	mu.RUnlock()
	if stillConnected {
		t.Fatal("WS connection entry was not removed")
	}
}
