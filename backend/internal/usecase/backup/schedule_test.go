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

func TestFormatBackupNameMoscow(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	// 07:21 UTC = 10:21 MSK
	now := time.Date(2026, 8, 11, 7, 21, 19, 0, time.UTC)
	got := FormatBackupName(now, "Europe/Moscow")
	want := "nm-20260811T102119+0300"
	if got != want {
		t.Fatalf("got %q want %q (local %s)", got, want, now.In(loc))
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
	// Догон: после слота всё ещё можно, пока нет успешного last_run_date.
	later := time.Date(2026, 8, 11, 10, 0, 0, 0, loc)
	fire, _ = ShouldFire(sch, later)
	if !fire {
		t.Fatal("should catch up after slot")
	}
	before := time.Date(2026, 8, 11, 2, 29, 0, 0, loc)
	fire, _ = ShouldFire(sch, before)
	if fire {
		t.Fatal("must not fire before slot")
	}
	sch.LastRunDate = "2026-08-11"
	fire, _ = ShouldFire(sch, later)
	if fire {
		t.Fatal("should not fire twice same day after success")
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
	if !next.Equal(after.UTC()) {
		t.Fatalf("overdue got %s want %s", next, after.UTC())
	}
	sch.LastRunDate = "2026-08-11"
	next = NextRunAt(sch, after)
	want = time.Date(2026, 8, 12, 3, 0, 0, 0, loc).UTC()
	if !next.Equal(want) {
		t.Fatalf("after success got %s want %s", next, want)
	}
}
