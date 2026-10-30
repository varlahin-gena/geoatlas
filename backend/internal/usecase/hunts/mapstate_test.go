package hunts

import (
	"testing"
	"time"
)

func TestMapStateToTimeRangePeriods(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	tr := mapStateToTimeRange(MapState{Period: "15m"}, now)
	if tr.Mode != "minutes" || tr.Amount != 15 {
		t.Fatalf("15m: %+v", tr)
	}
	tr = mapStateToTimeRange(MapState{Period: "3h"}, now)
	if tr.Mode != "hours" || tr.Amount != 3 {
		t.Fatalf("3h: %+v", tr)
	}
	tr = mapStateToTimeRange(MapState{Period: "2d"}, now)
	if tr.Mode != "days" || tr.Amount != 2 {
		t.Fatalf("2d: %+v", tr)
	}
	tr = mapStateToTimeRange(MapState{}, now)
	if tr.Mode != "days" || tr.Amount != 1 {
		t.Fatalf("default: %+v", tr)
	}
}

func TestMapStateCustomRange(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	tr := mapStateToTimeRange(MapState{
		Period:     "custom",
		PeriodFrom: "2026-05-01T00:00:00Z",
		PeriodTo:   "2026-05-02T00:00:00Z",
	}, now)
	if tr.Mode != "absolute" || tr.From.IsZero() || tr.To.Before(tr.From) {
		t.Fatalf("custom: %+v", tr)
	}
	// inverted + clamp to 30d
	tr = mapStateToTimeRange(MapState{
		Period:     "custom",
		PeriodFrom: "2026-01-01",
		PeriodTo:   "2025-12-01",
	}, now)
	if tr.To.Sub(tr.From) > 30*24*time.Hour+time.Hour {
		t.Fatalf("clamp: from=%v to=%v", tr.From, tr.To)
	}
}

func TestParseTimeAndHelpers(t *testing.T) {
	if _, err := parseTime(""); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if _, err := parseTime("2026-01-02T15:04:05Z"); err != nil {
		t.Fatalf("rfc: %v", err)
	}
	if _, err := parseTime("not-a-date"); err == nil {
		t.Fatal("want parse error")
	}
	if normalizeGroupBy("IP") != "ip" || normalizeGroupBy("x") != "city" {
		t.Fatal("groupby")
	}
	if normalizeFilter("Blocked") != "blocked" || normalizeFilter("") != "all" {
		t.Fatal("filter")
	}
	if MapLimit(MapState{}) != 5000 || MapLimit(MapState{Limit: 10}) != 10 {
		t.Fatal("limit")
	}
	_ = TimeRangeForState(MapState{Period: "1h"}, time.Now().UTC())
	_ = CostInputForState(MapState{Period: "1d", GroupBy: "country", Query: "x"}, time.Now().UTC())
}
