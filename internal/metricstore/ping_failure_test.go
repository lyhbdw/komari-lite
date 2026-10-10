package metricstore

import (
	"context"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/pkg/metric"
)

func TestPingWriterNormalizesAllFailureSentinels(t *testing.T) {
	s := useReportTestStore(t, nil)
	at := time.Now().UTC().Add(-time.Second)
	if err := WritePingRecord(context.Background(), models.PingRecord{Client: "failure", TaskId: 1, Time: at, Value: -2}); err != nil {
		t.Fatal(err)
	}
	points, err := s.Query(context.Background(), metric.Query{MetricName: MetricPingLatency, EntityID: "failure", Start: at.Add(-time.Second), End: at.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Value != -1 {
		t.Fatalf("negative failure values must use the canonical -1 sentinel: %+v", points)
	}
}
