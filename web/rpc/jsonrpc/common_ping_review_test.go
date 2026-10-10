package jsonrpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
)

func TestPingSummaryUnknownMinimumIsNotZero(t *testing.T) {
	preparePingSummaryStore(t)
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Minute)
	for i, value := range []int{20, 40, -1} {
		if err := metricstore.WritePingRecord(context.Background(), models.PingRecord{Client: "minimum", TaskId: 1, Time: base.Add(time.Duration(i) * time.Second), Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	stat := getPingStatsForNode("minimum", []models.PingTask{{Id: 1, Clients: models.StringArray{"minimum"}}})["1"]
	payload, err := json.Marshal(stat)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["min"] != nil {
		t.Fatalf("unrecoverable minimum must be null rather than false zero: %s", payload)
	}
}
