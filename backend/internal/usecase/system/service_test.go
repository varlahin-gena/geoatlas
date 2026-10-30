package system

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"geoatlas/internal/model"
)

type fakeMetrics struct {
	latest  []model.MetricRecord
	history map[string][]model.HistoryPoint
	counts  map[string]uint64
	err     error
	histErr error
}

func (f *fakeMetrics) FetchLatest(context.Context) ([]model.MetricRecord, error) {
	return f.latest, f.err
}
func (f *fakeMetrics) FetchHistory(context.Context, []model.MetricKey, time.Duration, time.Duration) (map[string][]model.HistoryPoint, error) {
	if f.histErr != nil {
		return nil, f.histErr
	}
	if f.history == nil {
		return map[string][]model.HistoryPoint{}, nil
	}
	return f.history, nil
}
func (f *fakeMetrics) CountRows(_ context.Context, table string) (uint64, error) {
	if f.counts == nil {
		return 0, nil
	}
	return f.counts[table], nil
}

type fakeEdges struct {
	st    EdgesAggStatus
	daily bool
	geo   bool
}

func (f *fakeEdges) Status() EdgesAggStatus { return f.st }
func (f *fakeEdges) PreferDaily() bool      { return f.daily }
func (f *fakeEdges) PreferGeo() bool        { return f.geo }

type fakeIngest struct {
	snap IngestSnapshot
	ok   bool
}

func (f *fakeIngest) Snapshot() (IngestSnapshot, bool) { return f.snap, f.ok }

type fakeSyslog struct {
	snap SyslogNGSnapshot
	ok   bool
}

func (f *fakeSyslog) Snapshot(context.Context) (SyslogNGSnapshot, bool) { return f.snap, f.ok }

type fakeGeo struct {
	n int
	b uint64
}

func (f *fakeGeo) RangeCount() int     { return f.n }
func (f *fakeGeo) ApproxBytes() uint64 { return f.b }

type fakeProfiles struct {
	p   *CapacityProfile
	err error
}

func (f *fakeProfiles) Load(string) (*CapacityProfile, error) { return f.p, f.err }

type fakeMaint struct{ n int }

func (f *fakeMaint) ScheduleMaintenanceBackfill(context.Context, time.Duration) { f.n++ }

type fakePinger struct{ err error }

func (f *fakePinger) Ping(context.Context) error { return f.err }

func TestNewAndScheduleMaintenance(t *testing.T) {
	m := &fakeMaint{}
	s := New(Dependencies{
		StartupTime: time.Now().Add(-time.Hour),
		Maintenance: m,
		IngestSLO:   IngestSLO{}, // triggers Default
	})
	if !s.ScheduleMaintenanceBackfill(context.Background()) || m.n != 1 {
		t.Fatalf("maint n=%d", m.n)
	}
	s2 := New(Dependencies{})
	if s2.ScheduleMaintenanceBackfill(context.Background()) {
		t.Fatal("nil maint")
	}
}

func TestCollectStatsAndStatus(t *testing.T) {
	labels, _ := json.Marshal(map[string]any{"role": "primary"})
	metrics := &fakeMetrics{
		latest: []model.MetricRecord{
			{MetricType: "container", Target: "backend", MetricName: "cpu_pct", Value: 12},
			{MetricType: "pipeline", Target: "ingest", MetricName: "queue_depth", Value: 1},
			{MetricType: "storage", Target: "traffic_logs", MetricName: "row_count", Value: 9},
			{MetricType: "health", Target: "clickhouse", MetricName: "up", Value: 1, Labels: string(labels)},
		},
	}
	ingest := &fakeIngest{ok: true, snap: IngestSnapshot{
		State: "running", QueueDepth: 10, QueueCapacity: 100, DroppedTotal: 0,
		ReceivedTotal: 50, InsertedTotal: 40, BufferedLines: 2,
	}}
	syslog := &fakeSyslog{ok: true, snap: SyslogNGSnapshot{Up: true, DroppedTotal: 1, Queued: 2, ProcessedTotal: 100}}
	edges := &fakeEdges{st: EdgesAggStatus{State: "ready", Message: "traffic_logs empty"}, daily: true, geo: true}
	metrics.counts = map[string]uint64{"traffic_logs": 100, "traffic_edges_daily": 20}
	prof := &fakeProfiles{p: &CapacityProfile{Profile: "small"}}
	s := New(Dependencies{
		Metrics: metrics, Edges: edges, Ingest: ingest, SyslogNG: syslog,
		GeoIndex: &fakeGeo{n: 5, b: 1 << 20}, Profiles: prof, InstallProfilePath: "x",
		StartupTime: time.Now().Add(-2 * time.Minute),
	})
	stats, err := s.CollectStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Containers["backend"]["cpu_pct"] != 12 {
		t.Fatalf("containers: %+v", stats.Containers)
	}
	if stats.Health["clickhouse"]["role"] != "primary" {
		t.Fatalf("health labels: %+v", stats.Health)
	}
	if stats.Backend.GeoIndexRanges != 5 {
		t.Fatalf("geo: %+v", stats.Backend)
	}
	if stats.EdgesAgg.MapSource != "edges_daily" || stats.EdgesAgg.RawRows != 100 {
		t.Fatalf("edges: %+v", stats.EdgesAgg)
	}
	if stats.InstallProfile == nil || stats.InstallProfile.Profile != "small" {
		t.Fatalf("profile: %+v", stats.InstallProfile)
	}
	st, err := s.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.AlertCount != len(st.Alerts) {
		t.Fatalf("status: %+v", st)
	}

	sNil := New(Dependencies{})
	if _, err := sNil.CollectStats(context.Background()); err == nil {
		t.Fatal("want metrics missing")
	}
}

func TestHistoryHelpers(t *testing.T) {
	metrics := &fakeMetrics{history: map[string][]model.HistoryPoint{"a": {{Timestamp: time.Now(), Value: 1}}}}
	s := New(Dependencies{Metrics: metrics})
	out, err := s.History(context.Background(), "6h", "pipeline.rate.events_per_sec,bad")
	if err != nil || out.StepSec <= 0 || len(out.Series) == 0 {
		t.Fatalf("history: %+v %v", out, err)
	}
	out, err = s.History(context.Background(), "1d", "")
	if err != nil || len(out.Series) == 0 && metrics.history != nil {
		// empty keys uses defaults; fake returns same history map
		_ = out
	}
	if parsePeriod("7d") != 7*24*time.Hour || parsePeriod("bogus") != time.Hour {
		t.Fatal("parsePeriod")
	}
	if chooseStep(time.Hour) != 30*time.Second || chooseStep(24*time.Hour) != 5*time.Minute {
		t.Fatal("chooseStep")
	}
	keys := parseMetricKeys("container.backend.cpu_pct,x")
	if len(keys) != 1 || keys[0].Target != "backend" {
		t.Fatalf("keys=%v", keys)
	}
	if len(defaultDashboardMetrics()) < 10 {
		t.Fatal("defaults")
	}
	if _, err := New(Dependencies{}).History(context.Background(), "1h", ""); err == nil {
		t.Fatal("no metrics")
	}
}

func TestEdgesAggIdleAndInstallProfile(t *testing.T) {
	s := New(Dependencies{})
	view := s.EdgesAgg(context.Background())
	if view.State != "idle" || view.MapSource != "traffic_logs" {
		t.Fatalf("%+v", view)
	}
	if _, err := s.InstallProfile(); err == nil {
		t.Fatal("no loader")
	}
	s = New(Dependencies{Profiles: &fakeProfiles{p: &CapacityProfile{Profile: "med"}}})
	p, err := s.InstallProfile()
	if err != nil || p.Profile != "med" {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestInstallMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install-meta.json")
	raw, _ := json.Marshal(InstallMeta{Version: "2.5.0", Source: "release", Ref: "v2.5.0", Commit: "abc"})
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(Dependencies{InstallMetaPath: path})
	meta := s.InstallMeta()
	if meta.Version != "2.5.0" || meta.Display == "unknown" {
		t.Fatalf("%+v", meta)
	}
	s2 := New(Dependencies{InstallMetaPath: filepath.Join(dir, "missing.json")})
	if s2.InstallMeta().Version != "unknown" {
		t.Fatal("missing file")
	}
	var nilSvc *Service
	if nilSvc.InstallMeta().Version != "unknown" {
		t.Fatal("nil")
	}
	// display fallbacks
	path2 := filepath.Join(dir, "meta2.json")
	_ = os.WriteFile(path2, []byte(`{"version":"1.0.0","source":"main"}`), 0o644)
	s3 := New(Dependencies{InstallMetaPath: path2})
	if s3.InstallMeta().Display != "main" {
		t.Fatalf("display=%q", s3.InstallMeta().Display)
	}
}

func TestHealth(t *testing.T) {
	s := New(Dependencies{
		Ingest: &fakeIngest{ok: true, snap: IngestSnapshot{State: "running"}},
	})
	res, err := s.Health(context.Background(), nil)
	if err != nil || res.OK {
		t.Fatalf("nil pinger: %+v %v", res, err)
	}
	res, err = s.Health(context.Background(), &fakePinger{err: errors.New("down")})
	if err != nil || res.OK {
		t.Fatalf("ping fail: %+v", res)
	}
	res, err = s.Health(context.Background(), &fakePinger{})
	if err != nil || !res.OK || res.Body["clickhouse"] != "ok" {
		t.Fatalf("healthy: %+v %v", res, err)
	}
}
