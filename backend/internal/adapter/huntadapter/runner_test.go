package huntadapter

import (
	"context"
	"testing"
	"time"

	"geoatlas/internal/usecase/hunts"
)

func TestHeavyGate(t *testing.T) {
	g := HeavyGate{}
	if !g.TryAcquire() {
		t.Fatal("nil try defaults true")
	}
	g.Release()

	n := 0
	g = HeavyGate{
		Try: func() bool { n++; return false },
		Rel: func() { n += 10 },
	}
	if g.TryAcquire() {
		t.Fatal("want false")
	}
	g.Release()
	if n != 11 {
		t.Fatalf("n=%d", n)
	}
}

func TestMapRunnerNilEvents(t *testing.T) {
	r := MapRunner{}
	_, err := r.RunMap(context.Background(), hunts.MapState{Period: "1d"}, time.Second)
	if err == nil {
		t.Fatal("want unavailable")
	}
}
