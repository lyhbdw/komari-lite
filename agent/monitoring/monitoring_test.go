package monitoring

import (
	"encoding/json"
	"testing"

	unit "github.com/lyhbdw/komari-lite/agent/monitoring/unit"
)

func TestGPUReportJSON(t *testing.T) {
	details := []unit.DetailedGPUInfo{
		{Name: "GPU A", MemoryTotal: 1000, MemoryUsed: 250, Utilization: 12.5, Temperature: 40},
		{Name: "GPU B", MemoryTotal: 2000, MemoryUsed: 500, Utilization: 37.5, Temperature: 50},
	}
	populated := buildGPUReport(details)
	if populated.Count != 2 || populated.AverageUsage != 25 {
		t.Fatalf("unexpected GPU count/average: %+v", populated)
	}
	if &populated.DetailedInfo[0] != &details[0] {
		t.Fatal("GPU report must reuse the detailed info slice")
	}
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"populated", populated, `{"count":2,"average_usage":25,"detailed_info":[{"name":"GPU A","memory_total":1000,"memory_used":250,"utilization":12.5,"temperature":40},{"name":"GPU B","memory_total":2000,"memory_used":500,"utilization":37.5,"temperature":50}]}`},
		{"nil", buildGPUReport(nil), `{"count":0,"average_usage":0,"detailed_info":null}`},
		{"empty", buildGPUReport([]unit.DetailedGPUInfo{}), `{"count":0,"average_usage":0,"detailed_info":[]}`},
		{"models only", gpuModelsReport{Models: []string{"GPU A"}}, `{"models":["GPU A"]}`},
		{"nil models", gpuModelsReport{}, `{"models":null}`},
		{"empty models", gpuModelsReport{Models: []string{}}, `{"models":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("JSON = %s; want %s", got, tc.want)
			}
		})
	}
}

func TestReportOmitsAbsentGPU(t *testing.T) {
	data, err := json.Marshal(report{})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["gpu"]; ok {
		t.Fatalf("absent GPU must be omitted: %s", data)
	}
}
