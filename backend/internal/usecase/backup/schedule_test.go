package backup

import (
	"testing"
	"time"
)

func TestValidateScheduleOK(t *testing.T) {
	out, err := ValidateSchedule(Schedule{
		Enabled: true, Hour: 2, Minute: 30, Timezone: "Europe/Moscow", Keep: 7,
		IncludeEdges: true, IncludeAuth: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Timezone != "Europe/Moscow" || out.Hour != 2 || out.Keep != 7 {
		t.Fatalf("unexpected %+v", out)
	}
}

func TestValidateScheduleRejects(t *testing.T) {
	cases := []Schedule{
		{Hour: 24, Minute: 0, Timezone: "UTC", Keep: 7},
		{Hour: 0, Minute: 60, Timezone: "UTC", Keep: 7},
		{Hour: 0, Minute: 0, Timezone: "Not/AZone", Keep: 7},
		{Hour: 0, Minute: 0, Timezone: "UTC", Keep: 0},
		{Hour: 0, Minute: 0, Timezone: "UTC", Keep: 100},
	}
	for _, c := range cases {
		if _, err := ValidateSchedule(c); err == nil {
			t.Fatalf("expected error for %+v", c)
		}
	}
}

func TestShouldFire(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	sch := Schedule{Enabled: true, Hour: 2, Minute: 30, Timezone: "Europe/Moscow", Keep: 7}
	now := time.Date(2026, 8, 11, 2, 30, 15, 0, loc)
	fire, dateKey := ShouldFire(sch, now)
	if !fire || dateKey != "2026-08-11" {
		t.Fatalf("fire=%v date=%q", fire, dateKey)
	}
	sch.LastRunDate = "2026-08-11"
	fire, _ = ShouldFire(sch, now)
	if fire {
		t.Fatal("should not fire twice same day")
	}
	sch.LastRunDate = ""
	sch.Enabled = false
	fire, _ = ShouldFire(sch, now)
	if fire {
		t.Fatal("disabled must not fire")
	}
}

func TestNextRunAt(t *testing.T) {
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal(err)
	}
	sch := Schedule{Enabled: true, Hour: 3, Minute: 0, Timezone: "UTC", Keep: 7}
	before := time.Date(2026, 8, 11, 2, 0, 0, 0, loc)
	next := NextRunAt(sch, before)
	want := time.Date(2026, 8, 11, 3, 0, 0, 0, loc).UTC()
	if !next.Equal(want) {
		t.Fatalf("got %s want %s", next, want)
	}
	after := time.Date(2026, 8, 11, 3, 1, 0, 0, loc)
	next = NextRunAt(sch, after)
	want = time.Date(2026, 8, 12, 3, 0, 0, 0, loc).UTC()
	if !next.Equal(want) {
		t.Fatalf("after slot got %s want %s", next, want)
	}
}
