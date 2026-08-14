package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recSchema struct {
	calls []string
}

func (s *recSchema) rec(name string) error {
	s.calls = append(s.calls, name)
	return nil
}

func (s *recSchema) EnsureTTLOnlyDropParts(context.Context) error {
	return s.rec("ttl")
}
func (s *recSchema) EnsureTrafficLogsIPv4(context.Context) error {
	return s.rec("ipv4")
}
func (s *recSchema) EnsureTrafficLogsSuccess(context.Context) error {
	return s.rec("success")
}
func (s *recSchema) EnsureEdgesAggSchema(context.Context) error {
	return s.rec("edges")
}
func (s *recSchema) EnsureGeoEdgesAggSchema(context.Context) error {
	return s.rec("geo_edges")
}
func (s *recSchema) EnsureHourlyEdgesAggSchema(context.Context) error {
	return s.rec("hourly")
}
func (s *recSchema) EnsureReputationRanges(context.Context) error {
	return s.rec("reputation")
}

type recBackfill struct{ calls []string }

func (b *recBackfill) rec(name string) error {
	b.calls = append(b.calls, name)
	return nil
}
func (b *recBackfill) BackfillEdgesAgg(context.Context) error {
	return b.rec("edges")
}
func (b *recBackfill) BackfillGeoEdgesAgg(context.Context) error {
	return b.rec("geo")
}
func (b *recBackfill) BackfillHourlyEdgesAgg(context.Context) error {
	return b.rec("hourly")
}

type recReady struct{ calls []string }

func (r *recReady) rec(name string) error {
	r.calls = append(r.calls, name)
	return nil
}
func (r *recReady) RefreshEdgesAggReady(context.Context) error {
	return r.rec("edges")
}
func (r *recReady) RefreshGeoEdgesAggReady(context.Context) error {
	return r.rec("geo")
}
func (r *recReady) RefreshHourlyEdgesAggReady(context.Context) error {
	return r.rec("hourly")
}

type recEnrich struct{ n int }

func (e *recEnrich) ScheduleEnrichOnly(context.Context, time.Duration) { e.n++ }

type recGeo struct{ n int }

func (g recGeo) RangeCount() int { return g.n }

type recRetention struct{ n int }

func (r *recRetention) ApplyFromStore(context.Context) error {
	r.n++
	return nil
}

func TestRunStartupSkipBackfillRefreshesReady(t *testing.T) {
	schema := &recSchema{}
	ready := &recReady{}
	backfill := &recBackfill{}
	RunStartup(context.Background(), Dependencies{
		Schema:   schema,
		Ready:    ready,
		Backfill: backfill,
	}, Options{SkipStartupBackfill: true, ReputationEnabled: true}, nil, nil)

	if len(backfill.calls) != 0 {
		t.Fatalf("backfill must not run when skipped, got %v", backfill.calls)
	}
	if len(ready.calls) != 3 {
		t.Fatalf("ready refreshes: got %v", ready.calls)
	}
	if len(schema.calls) < 6 {
		t.Fatalf("schema ensures: got %v", schema.calls)
	}
	foundRep := false
	for _, c := range schema.calls {
		if c == "reputation" {
			foundRep = true
		}
	}
	if !foundRep {
		t.Fatalf("reputation schema not ensured: %v", schema.calls)
	}
}

func TestRunStartupReputationOffSkipsReputationSchema(t *testing.T) {
	schema := &recSchema{}
	RunStartup(context.Background(), Dependencies{Schema: schema}, Options{
		SkipStartupBackfill: true,
		ReputationEnabled:   false,
	}, nil, nil)
	for _, c := range schema.calls {
		if c == "reputation" {
			t.Fatal("reputation schema must not run when module off")
		}
	}
}

func TestRunStartupBackfillThenGeoEnrich(t *testing.T) {
	backfill := &recBackfill{}
	enrich := &recEnrich{}
	RunStartup(context.Background(), Dependencies{
		Backfill: backfill,
		Enrich:   enrich,
		Geo:      recGeo{n: 12},
	}, Options{GeoBackfillLookbackDays: 7}, nil, nil)
	if len(backfill.calls) != 3 {
		t.Fatalf("backfill calls: %v", backfill.calls)
	}
	if enrich.n != 1 {
		t.Fatalf("geo enrich scheduled: %d", enrich.n)
	}
}

func TestRunStartupNoGeoSkipsEnrich(t *testing.T) {
	enrich := &recEnrich{}
	RunStartup(context.Background(), Dependencies{
		Enrich: enrich,
		Geo:    recGeo{n: 0},
	}, Options{}, nil, nil)
	if enrich.n != 0 {
		t.Fatal("enrich must not run with empty geo index")
	}
}

func TestRunStartupNilDepsNoPanic(t *testing.T) {
	RunStartup(context.Background(), Dependencies{}, Options{SkipStartupBackfill: true}, nil, nil)
}

func TestRunStartupAppliesRetention(t *testing.T) {
	ret := &recRetention{}
	RunStartup(context.Background(), Dependencies{Retention: ret}, Options{SkipStartupBackfill: true}, nil, nil)
	if ret.n != 1 {
		t.Fatalf("retention apply: %d", ret.n)
	}
}

func TestRunStartupHonorsTimeoutBeforeBackfill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	backfill := &recBackfill{}
	schema := &recSchema{calls: nil}
	// Schema still runs (uses bctx); after cancel, backfill must not.
	// Use already-canceled parent so WithTimeout inherits cancellation.
	RunStartup(ctx, Dependencies{Schema: schema, Backfill: backfill}, Options{
		Timeout: time.Hour,
	}, nil, nil)
	if len(backfill.calls) != 0 {
		t.Fatalf("backfill after canceled ctx: %v", backfill.calls)
	}
}

func TestRunStartupWarnsOnSchemaError(t *testing.T) {
	var warned []string
	schema := &errSchema{}
	RunStartup(context.Background(), Dependencies{Schema: schema}, Options{SkipStartupBackfill: true}, func(msg string, err error) {
		if err == nil {
			t.Fatal("warn without error")
		}
		warned = append(warned, msg)
	}, nil)
	if len(warned) == 0 {
		t.Fatal("expected schema warnings")
	}
}

type errSchema struct{}

func (errSchema) EnsureTTLOnlyDropParts(context.Context) error {
	return errors.New("ttl")
}
func (errSchema) EnsureTrafficLogsIPv4(context.Context) error {
	return errors.New("ipv4")
}
func (errSchema) EnsureTrafficLogsSuccess(context.Context) error { return nil }
func (errSchema) EnsureEdgesAggSchema(context.Context) error     { return nil }
func (errSchema) EnsureGeoEdgesAggSchema(context.Context) error  { return nil }
func (errSchema) EnsureHourlyEdgesAggSchema(context.Context) error {
	return nil
}
func (errSchema) EnsureReputationRanges(context.Context) error { return nil }
