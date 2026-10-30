package retention

import (
	"context"
	"errors"
	"testing"
)

func TestNilGetUpdateNormalizeApplyErrors(t *testing.T) {
	var nilS *Service
	got, err := nilS.Get()
	if err != nil || got.TrafficLogsDays != DefaultTrafficLogsDays {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := nilS.Update(context.Background(), Defaults()); err == nil {
		t.Fatal("nil update")
	}
	if err := nilS.ApplyFromStore(context.Background()); err != nil {
		t.Fatal(err)
	}

	store := &memStore{err: errors.New("load fail")}
	svc := New(store, &memApplier{})
	if _, err := svc.Get(); err == nil {
		t.Fatal("load err")
	}
	store.err = nil
	store.s = Settings{TrafficLogsDays: 9999, EdgesDays: -1, ParseErrorsDays: 0, SystemMetricsDays: 800}
	norm, err := svc.Get()
	if err != nil || norm.TrafficLogsDays != DefaultTrafficLogsDays || norm.EdgesDays != DefaultEdgesDays {
		t.Fatalf("%+v", norm)
	}

	store.err = errors.New("save fail")
	if _, err := svc.Update(context.Background(), Settings{
		TrafficLogsDays: 10, EdgesDays: 10, ParseErrorsDays: 2, SystemMetricsDays: 2,
	}); err == nil {
		t.Fatal("save err")
	}
	store.err = nil
	app := &memApplier{fail: "traffic"}
	svc = New(store, app)
	if _, err := svc.Update(context.Background(), Settings{
		TrafficLogsDays: 10, EdgesDays: 10, ParseErrorsDays: 2, SystemMetricsDays: 2,
	}); err == nil {
		t.Fatal("apply traffic")
	}
	app.fail = "edges"
	if _, err := svc.Update(context.Background(), Settings{
		TrafficLogsDays: 10, EdgesDays: 10, ParseErrorsDays: 2, SystemMetricsDays: 2,
	}); err == nil {
		t.Fatal("apply edges")
	}
	app.fail = ""
	app2 := &memApplier{}
	// fail parse/metrics via custom wrappers — reuse applyAll through ApplyFromStore with failing applier mid-way
	svc2 := New(&memStore{}, &failLaterApplier{failAt: 2})
	if err := svc2.ApplyFromStore(context.Background()); err == nil {
		t.Fatal("parse fail")
	}
	svc3 := New(&memStore{}, &failLaterApplier{failAt: 3})
	if err := svc3.ApplyFromStore(context.Background()); err == nil {
		t.Fatal("metrics fail")
	}
	_ = app2
	_, err = validate(Settings{TrafficLogsDays: 10, EdgesDays: 10, ParseErrorsDays: 0, SystemMetricsDays: 2})
	if err == nil {
		t.Fatal("validate parse")
	}
	_, err = validate(Settings{TrafficLogsDays: 10, EdgesDays: 10, ParseErrorsDays: 2, SystemMetricsDays: 0})
	if err == nil {
		t.Fatal("validate metrics")
	}
	_, err = validate(Settings{TrafficLogsDays: 10, EdgesDays: 0, ParseErrorsDays: 2, SystemMetricsDays: 2})
	if err == nil {
		t.Fatal("validate edges")
	}
}

type failLaterApplier struct {
	n, failAt int
}

func (f *failLaterApplier) ApplyTrafficLogs(context.Context, int) error {
	f.n++
	if f.n-1 == f.failAt {
		return errors.New("boom")
	}
	return nil
}
func (f *failLaterApplier) ApplyEdges(context.Context, int) error {
	f.n++
	if f.n-1 == f.failAt {
		return errors.New("boom")
	}
	return nil
}
func (f *failLaterApplier) ApplyParseErrors(context.Context, int) error {
	f.n++
	if f.n-1 == f.failAt {
		return errors.New("boom")
	}
	return nil
}
func (f *failLaterApplier) ApplySystemMetrics(context.Context, int) error {
	f.n++
	if f.n-1 == f.failAt {
		return errors.New("boom")
	}
	return nil
}
