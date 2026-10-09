package server

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pkg_flags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
)

func TestReadBoundedBodyTruncatesLargeSuccessAndErrorResponses(t *testing.T) {
	body := bytes.Repeat([]byte("x"), maxAgentResponseBytes+1)
	got, err := readBoundedBody(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("readBoundedBody() error = %v", err)
	}
	if len(got) != maxAgentResponseBytes {
		t.Fatalf("readBoundedBody() length = %d, want %d", len(got), maxAgentResponseBytes)
	}

	statusErr := &httpStatusError{StatusCode: 500, Status: "500 Internal Server Error", Body: string(got)}
	if len(statusErr.Body) > maxAgentResponseBytes {
		t.Fatalf("error body length = %d, want <= %d", len(statusErr.Body), maxAgentResponseBytes)
	}
}

func TestAddV2AckEventIDDeduplicates(t *testing.T) {
	v2AckMu.Lock()
	v2AckEventIDs = nil
	v2AckMu.Unlock()
	t.Cleanup(func() {
		v2AckMu.Lock()
		v2AckEventIDs = nil
		v2AckMu.Unlock()
	})

	addV2AckEventID("event-1")
	addV2AckEventID("event-1")
	addV2AckEventID("event-2")
	got := snapshotV2AckEventIDs()
	if len(got) != 2 || got[0] != "event-1" || got[1] != "event-2" {
		t.Fatalf("ACK IDs = %#v, want [event-1 event-2]", got)
	}
}

func TestRequestEventIDPreservesJSONRPCID(t *testing.T) {
	payload := v2.NewRequest("event-42", v2.MethodAgentPing, map[string]string{"ping_type": "tcp"})
	var request v2.Request
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if got := requestEventID(request.ID); got != "event-42" {
		t.Fatalf("requestEventID() = %q, want event-42", got)
	}
}

func TestMarkMigrationReadyCreatesMarkerOnce(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ready")
	oldFlags := flags
	flags = &pkg_flags.Config{MigrationReadyFile: marker}
	t.Cleanup(func() { flags = oldFlags })

	markMigrationReady()
	first, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if string(first) != "ready\n" {
		t.Fatalf("marker = %q, want ready", first)
	}
	if err := os.WriteFile(marker, []byte("preserve\n"), 0600); err != nil {
		t.Fatalf("rewrite marker: %v", err)
	}
	markMigrationReady()
	second, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker after second call: %v", err)
	}
	if string(second) != "preserve\n" {
		t.Fatalf("existing marker was overwritten: %q", second)
	}
}
