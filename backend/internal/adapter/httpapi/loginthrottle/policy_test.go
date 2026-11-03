package loginthrottle

import (
	"testing"
	"time"
)

func TestLimiterSetPolicy(t *testing.T) {
	lim := New(10, time.Minute, 5*time.Minute)
	lim.SetPolicy(5, 2*time.Minute, 10*time.Minute)
	p := lim.Policy()
	if p.MaxFails != 5 || p.WindowSec != 120 || p.LockoutSec != 600 {
		t.Fatalf("policy=%+v", p)
	}
}

func TestSnapshotFailuresIncludesLockedUntil(t *testing.T) {
	lim := New(2, time.Minute, time.Minute)
	ip, user := "10.0.0.1", "bob"
	if !lim.Allow(ip, user) {
		t.Fatal("allow")
	}
	lim.RecordFailure(ip, user)
	lim.RecordFailure(ip, user)
	if lim.Allow(ip, user) {
		t.Fatal("should lock")
	}
	out := lim.SnapshotFailures()
	if len(out) == 0 {
		t.Fatal("empty snapshot")
	}
	if !out[0].Locked || out[0].LockedUntil.IsZero() {
		t.Fatalf("want locked_until, got %+v", out[0])
	}
}
