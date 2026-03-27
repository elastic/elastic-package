package rally

import (
	"strings"
	"testing"
)

func TestParseRallyReportReturnsStats(t *testing.T) {
	stats, err := parseRallyReport(strings.NewReader("metric,task,value,unit\n"), "/tmp/rally", "")
	if err != nil {
		t.Fatalf("parseRallyReport returned error: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0] != (rallyStat{
		Metric: "metric",
		Task:   "task",
		Value:  "value",
		Unit:   "unit",
	}) {
		t.Fatalf("unexpected stat: %#v", stats[0])
	}
}
