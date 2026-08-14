package query

import (
	"testing"

	"network_monitor/internal/model"
)

func TestPromoteHoursToGeoDays(t *testing.T) {
	got := promoteHoursToGeoDays(model.TimeRange{Mode: "hours", Amount: 24}, "city", true)
	if got.Mode != "days" || got.Amount != 1 {
		t.Fatalf("hours=24 city → got %+v, want days=1", got)
	}

	got = promoteHoursToGeoDays(model.TimeRange{Mode: "hours", Amount: 72}, "country", true)
	if got.Mode != "days" || got.Amount != 3 {
		t.Fatalf("hours=72 country → got %+v, want days=3", got)
	}

	got = promoteHoursToGeoDays(model.TimeRange{Mode: "hours", Amount: 6}, "city", true)
	if got.Mode != "hours" || got.Amount != 6 {
		t.Fatalf("hours=6 must stay hours, got %+v", got)
	}

	got = promoteHoursToGeoDays(model.TimeRange{Mode: "hours", Amount: 24}, "ip", true)
	if got.Mode != "days" || got.Amount != 1 {
		t.Fatalf("hours=24 ip → got %+v, want days=1", got)
	}

	got = promoteHoursToGeoDays(model.TimeRange{Mode: "hours", Amount: 24}, "city", false)
	if got.Mode != "hours" || got.Amount != 24 {
		t.Fatalf("when preferEdges=false must stay hours, got %+v", got)
	}

	got = promoteMinutesToHours(model.TimeRange{Mode: "minutes", Amount: 120}, true)
	if got.Mode != "hours" || got.Amount != 2 {
		t.Fatalf("minutes=120 → hours=2, got %+v", got)
	}
	got = promoteMinutesToHours(model.TimeRange{Mode: "minutes", Amount: 15}, true)
	if got.Mode != "minutes" || got.Amount != 15 {
		t.Fatalf("minutes=15 must stay, got %+v", got)
	}
}
