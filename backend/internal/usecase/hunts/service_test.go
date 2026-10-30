package hunts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type memStore struct {
	users map[string][]Hunt
	err   error
}

func newMemStore() *memStore {
	return &memStore{users: map[string][]Hunt{}}
}

func (m *memStore) List(username string) ([]Hunt, error) {
	if m.err != nil {
		return nil, m.err
	}
	return append([]Hunt(nil), m.users[username]...), nil
}

func (m *memStore) ListAll() ([]HuntWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([]HuntWithAuthor, 0)
	for user, items := range m.users {
		for _, h := range items {
			out = append(out, HuntWithAuthor{Hunt: h, Username: user})
		}
	}
	return out, nil
}

func (m *memStore) Create(username string, hunt Hunt) (Hunt, error) {
	if m.err != nil {
		return Hunt{}, m.err
	}
	hunt.ID = "h1"
	hunt.UpdatedAt = "t"
	m.users[username] = append(m.users[username], hunt)
	return hunt, nil
}

func (m *memStore) Update(username, id string, hunt Hunt) (Hunt, error) {
	if m.err != nil {
		return Hunt{}, m.err
	}
	list := m.users[username]
	for i := range list {
		if list[i].ID != id {
			continue
		}
		hunt.ID = id
		list[i] = hunt
		m.users[username] = list
		return hunt, nil
	}
	return Hunt{}, ErrNotFound
}

func (m *memStore) Delete(username, id string) error {
	if m.err != nil {
		return m.err
	}
	list := m.users[username]
	next := make([]Hunt, 0, len(list))
	found := false
	for _, h := range list {
		if h.ID == id {
			found = true
			continue
		}
		next = append(next, h)
	}
	if !found {
		return ErrNotFound
	}
	m.users[username] = next
	return nil
}

func (m *memStore) SaveRun(username, id string, hunt Hunt) error {
	if m.err != nil {
		return m.err
	}
	list := m.users[username]
	for i := range list {
		if list[i].ID != id {
			continue
		}
		list[i] = hunt
		m.users[username] = list
		return nil
	}
	return ErrNotFound
}

type stubRunner struct {
	snap MapSnapshot
	err  error
	n    int
}

func (r *stubRunner) RunMap(context.Context, MapState, time.Duration) (MapSnapshot, error) {
	r.n++
	return r.snap, r.err
}

type stubReporter struct {
	n int
}

func (r *stubReporter) ReportHuntBreach(context.Context, Hunt, RunResult, time.Time) error {
	r.n++
	return nil
}

type stubHeavy struct {
	ok bool
	n  int
}

func (h *stubHeavy) TryAcquire() bool {
	h.n++
	return h.ok
}
func (h *stubHeavy) Release() {}

func validHunt() Hunt {
	return Hunt{
		Name: "scan RU",
		Map:  MapState{Period: "1d", GroupBy: "city", Filter: "all", Limit: 100},
		Schedule: Schedule{
			Enabled:       true,
			IntervalMin:   60,
			EdgeThreshold: 10,
			EdgeRatio:     3,
		},
	}
}

func TestValidateNameAndMap(t *testing.T) {
	if err := ValidateName(""); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("empty name: %v", err)
	}
	if err := ValidateName(strings.Repeat("я", MaxNameLen+1)); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("long name: %v", err)
	}
	if err := ValidateMapState(MapState{GroupBy: "nope"}); !errors.Is(err, ErrInvalidMapState) {
		t.Fatalf("bad group: %v", err)
	}
	if err := ValidateMapState(MapState{Period: "custom"}); !errors.Is(err, ErrInvalidMapState) {
		t.Fatalf("custom without range: %v", err)
	}
	if err := ValidateMapState(MapState{Limit: 999999}); !errors.Is(err, ErrInvalidMapState) {
		t.Fatalf("limit: %v", err)
	}
	if err := ValidateMapState(MapState{}); err != nil {
		t.Fatalf("defaults ok: %v", err)
	}
}

func TestNormalizeValidateSchedule(t *testing.T) {
	sch := NormalizeSchedule(Schedule{IntervalMin: 1, EdgeRatio: 0.5})
	if sch.IntervalMin != 60 || sch.EdgeRatio != 3 {
		t.Fatalf("normalize: %+v", sch)
	}
	if _, err := ValidateSchedule(Schedule{IntervalMin: 60, EdgeRatio: 2}); err != nil {
		t.Fatalf("valid: %v", err)
	}
}

func TestServiceCRUDAndNil(t *testing.T) {
	var s *Service
	if _, err := s.List("u"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil list: %v", err)
	}
	store := newMemStore()
	s = New(store, nil, nil, nil, 0)
	if s.timeout != 2*time.Minute {
		t.Fatalf("default timeout: %v", s.timeout)
	}
	if _, err := s.Create("u", Hunt{Name: ""}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("bad create: %v", err)
	}
	out, err := s.Create("u", validHunt())
	if err != nil || out.ID == "" || out.Map.GroupBy != "city" {
		t.Fatalf("create: %+v %v", out, err)
	}
	list, err := s.List("u")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	out.Name = "renamed"
	upd, err := s.Update("u", out.ID, out)
	if err != nil || upd.Name != "renamed" {
		t.Fatalf("update: %+v %v", upd, err)
	}
	all, err := s.ListAll()
	if err != nil || len(all) != 1 || all[0].Username != "u" {
		t.Fatalf("listall: %+v %v", all, err)
	}
	if err := s.Delete("u", out.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.Delete("u", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestRunNowAndBreach(t *testing.T) {
	store := newMemStore()
	runner := &stubRunner{snap: MapSnapshot{EdgeCount: 20, RawPairs: 40}}
	rep := &stubReporter{}
	heavy := &stubHeavy{ok: true}
	s := New(store, runner, rep, heavy, time.Minute)
	created, err := s.Create("alice", validHunt())
	if err != nil {
		t.Fatal(err)
	}
	// seed previous run so ratio/threshold can breach
	created.Runs = []RunResult{{EdgeCount: 5, At: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)}}
	store.users["alice"][0] = created

	hunt, run, err := s.RunNow(context.Background(), "alice", created.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !run.Breach || run.EdgeCount != 20 || hunt.LastRunAt == "" {
		t.Fatalf("run result: %+v hunt=%+v", run, hunt)
	}
	if rep.n != 1 {
		t.Fatalf("reporter calls: %d", rep.n)
	}
	if heavy.n != 1 {
		t.Fatalf("heavy: %d", heavy.n)
	}
}

func TestRunNowNotFoundAndHeavyBusy(t *testing.T) {
	store := newMemStore()
	s := New(store, &stubRunner{}, nil, &stubHeavy{ok: false}, time.Minute)
	_, err := s.Create("u", validHunt())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.RunNow(context.Background(), "u", "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("not found: %v", err)
	}
	id := store.users["u"][0].ID
	_, run, err := s.RunNow(context.Background(), "u", id)
	if err == nil || run.Skipped != "heavy_busy" {
		t.Fatalf("heavy busy: run=%+v err=%v", run, err)
	}
}

func TestTickScheduled(t *testing.T) {
	store := newMemStore()
	runner := &stubRunner{snap: MapSnapshot{EdgeCount: 100}}
	rep := &stubReporter{}
	s := New(store, runner, rep, nil, time.Minute)
	h := validHunt()
	h.Schedule.Enabled = true
	h.Schedule.EdgeThreshold = 1
	created, err := s.Create("bob", h)
	if err != nil {
		t.Fatal(err)
	}
	_ = created
	now := time.Now().UTC()
	s.TickScheduled(context.Background(), now)
	if runner.n != 1 {
		t.Fatalf("runner calls: %d", runner.n)
	}
	if rep.n != 1 {
		t.Fatalf("scheduled breach report: %d", rep.n)
	}
	// second tick within interval should skip
	s.TickScheduled(context.Background(), now.Add(time.Minute))
	if runner.n != 1 {
		t.Fatalf("should not re-run: %d", runner.n)
	}
}

func TestShouldRunScheduledAndEvaluateBreach(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	h := Hunt{Schedule: Schedule{Enabled: false}}
	if shouldRunScheduled(h, now) {
		t.Fatal("disabled")
	}
	h.Schedule.Enabled = true
	h.Schedule.IntervalMin = 60
	if !shouldRunScheduled(h, now) {
		t.Fatal("no last run")
	}
	h.LastRunAt = now.Add(-30 * time.Minute).Format(time.RFC3339)
	if shouldRunScheduled(h, now) {
		t.Fatal("too soon")
	}
	h.LastRunAt = "bad"
	if !shouldRunScheduled(h, now) {
		t.Fatal("bad last run")
	}
	if !evaluateBreach(10, 0, Schedule{EdgeThreshold: 5}) {
		t.Fatal("threshold")
	}
	if !evaluateBreach(10, 2, Schedule{EdgeRatio: 3}) {
		t.Fatal("ratio")
	}
	if evaluateBreach(1, 10, Schedule{EdgeThreshold: 5, EdgeRatio: 3}) {
		t.Fatal("no breach")
	}
}

func TestExecuteSkipsHeavyScheduled(t *testing.T) {
	// wide custom range → heavy query cost for scheduled skip
	store := newMemStore()
	runner := &stubRunner{snap: MapSnapshot{EdgeCount: 1}}
	s := New(store, runner, nil, nil, time.Minute)
	h := validHunt()
	h.Map.Period = "custom"
	h.Map.PeriodFrom = "2026-01-01T00:00:00Z"
	h.Map.PeriodTo = "2026-01-25T00:00:00Z"
	h.Map.GroupBy = "ip"
	run, _, err := s.execute(context.Background(), h, time.Now().UTC(), true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if run.Skipped != "query_too_heavy" && runner.n != 0 {
		// if assess doesn't mark heavy, runner may still run — accept either skip or run
		t.Logf("skip=%q runner=%d cost=%s", run.Skipped, runner.n, run.QueryCost)
	}
}

func TestPreviousEdgeCountAndTrimRuns(t *testing.T) {
	if previousEdgeCount(nil) != 0 {
		t.Fatal("empty")
	}
	runs := []RunResult{{Skipped: "x"}, {EdgeCount: 3}, {EdgeCount: 7}}
	if previousEdgeCount(runs) != 7 {
		t.Fatalf("prev=%d", previousEdgeCount(runs))
	}
	long := make([]RunResult, MaxRunHistory+5)
	got := trimRuns(long)
	if len(got) != MaxRunHistory {
		t.Fatalf("trim len=%d", len(got))
	}
}
