package aggstate

import "testing"

func TestPreferDailyEdgesAggOnlyWhenReady(t *testing.T) {
	t.Cleanup(func() {
		SetEdgesAggStatus(EdgesAggStatus{State: "idle", Message: "not started"})
	})

	cases := []struct {
		state string
		phase string
		want  bool
	}{
		{"idle", "", false},
		{"running", PhaseSchema, false},
		{"running", PhaseBackfill, false},
		{"error", PhaseSchema, false},
		{"ready", "", true},
	}
	for _, tc := range cases {
		SetEdgesAggStatus(EdgesAggStatus{State: tc.state, Phase: tc.phase, Message: "test"})
		if got := PreferDailyEdgesAgg(); got != tc.want {
			t.Fatalf("state=%q phase=%q: PreferDailyEdgesAgg=%v, want %v", tc.state, tc.phase, got, tc.want)
		}
	}
}

func TestPreferGeoEdgesAggFlag(t *testing.T) {
	t.Cleanup(func() { SetGeoEdgesAggReady(false) })

	SetGeoEdgesAggReady(false)
	if PreferGeoEdgesAgg() {
		t.Fatal("expected false before ready")
	}
	SetGeoEdgesAggReady(true)
	if !PreferGeoEdgesAgg() {
		t.Fatal("expected true after ready")
	}
}

func TestPreferHourlyEdgesAggFlag(t *testing.T) {
	t.Cleanup(func() { SetHourlyEdgesAggReady(false) })

	SetHourlyEdgesAggReady(false)
	if PreferHourlyEdgesAgg() {
		t.Fatal("expected false before ready")
	}
	SetHourlyEdgesAggReady(true)
	if !PreferHourlyEdgesAgg() {
		t.Fatal("expected true after ready")
	}
}

func TestIsolatedStoreDoesNotAffectDefault(t *testing.T) {
	t.Cleanup(func() {
		SetEdgesAggStatus(EdgesAggStatus{State: "idle", Message: "not started"})
		SetGeoEdgesAggReady(false)
		SetHourlyEdgesAggReady(false)
	})
	SetEdgesAggStatus(EdgesAggStatus{State: "idle", Message: "not started"})
	SetGeoEdgesAggReady(false)

	s := NewStore()
	s.SetEdgesAggStatus(EdgesAggStatus{State: "ready"})
	s.SetGeoEdgesAggReady(true)
	if PreferDailyEdgesAgg() || PreferGeoEdgesAgg() {
		t.Fatal("isolated Store must not flip Default")
	}
	if !s.PreferDailyEdgesAgg() || !s.PreferGeoEdgesAgg() {
		t.Fatal("isolated Store must report its own state")
	}
}
