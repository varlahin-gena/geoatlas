package backup

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

type memSchedule struct {
	mu  sync.Mutex
	sch Schedule
	err error
}

func (m *memSchedule) Load() (Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return Schedule{}, m.err
	}
	return m.sch, nil
}

func (m *memSchedule) Save(s Schedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sch = s
	return nil
}

type memStore struct {
	mu        sync.Mutex
	ready     bool
	entries   []Entry
	attached  string
	exists    map[string]bool
	failList  error
	failAtt   error
	failDel   error
	failAuth  error
	failPrune error
	sources   map[string]string
}

func newMemStore() *memStore {
	return &memStore{
		ready:   true,
		exists:  map[string]bool{},
		sources: map[string]string{},
	}
}

func (m *memStore) DirReady() bool { return m.ready }
func (m *memStore) List() ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failList != nil {
		return nil, m.failList
	}
	out := append([]Entry(nil), m.entries...)
	return out, nil
}
func (m *memStore) Exists(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.exists[name]
}
func (m *memStore) WriteAuthTarball(name, _ string) error {
	if m.failAuth != nil {
		return m.failAuth
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exists[name] = true
	return nil
}
func (m *memStore) WriteSource(name, source string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[name] = source
	m.exists[name] = true
	m.entries = append(m.entries, Entry{Name: name, Source: source, CreatedAt: time.Now().UTC()})
	return nil
}
func (m *memStore) Prune(int) error { return m.failPrune }
func (m *memStore) Delete(name string) error {
	if m.failDel != nil {
		return m.failDel
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.exists, name)
	next := m.entries[:0]
	for _, e := range m.entries {
		if e.Name != name {
			next = append(next, e)
		}
	}
	m.entries = next
	return nil
}
func (m *memStore) Attached() (string, error) {
	if m.failAtt != nil {
		return "", m.failAtt
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attached, nil
}
func (m *memStore) SetAttached(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attached = name
	return nil
}

type memRunner struct {
	tables      map[string]bool
	failBackup  error
	failRestore error
	failDrop    error
	failExists  error
	backupN     int
	restoreN    int
	dropN       int
}

func (r *memRunner) BackupTables(context.Context, string, []string) error {
	r.backupN++
	return r.failBackup
}
func (r *memRunner) RestoreMapShadow(context.Context, string) error {
	r.restoreN++
	return r.failRestore
}
func (r *memRunner) DropMapShadow(context.Context) error {
	r.dropN++
	return r.failDrop
}
func (r *memRunner) TableExists(_ context.Context, name string) (bool, error) {
	if r.failExists != nil {
		return false, r.failExists
	}
	if r.tables == nil {
		return true, nil
	}
	return r.tables[name], nil
}

type memHeavy struct {
	fail error
	n    int
}

func (h *memHeavy) Acquire(context.Context) error {
	h.n++
	return h.fail
}
func (h *memHeavy) Release() {}

type auditSink struct {
	mu    sync.Mutex
	dr    []usecaseaudit.DREvent
	audit []usecaseaudit.AuditEvent
}

func (a *auditSink) WriteDR(_ context.Context, e usecaseaudit.DREvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dr = append(a.dr, e)
	return nil
}
func (a *auditSink) WriteAudit(_ context.Context, e usecaseaudit.AuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.audit = append(a.audit, e)
	return nil
}
func (a *auditSink) ListDR(context.Context, usecaseaudit.DRQuery) ([]usecaseaudit.DREvent, error) {
	return nil, nil
}
func (a *auditSink) ListAudit(context.Context, usecaseaudit.AuditQuery) ([]usecaseaudit.AuditEvent, error) {
	return nil, nil
}

func waitJobIdle(t *testing.T, s *Service, timeout time.Duration) Status {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st := s.Status()
		if st.State != "running" {
			return st
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job still running: %+v", s.Status())
	return Status{}
}

func TestNormalizeSourceAndDefaults(t *testing.T) {
	if NormalizeSource("Manual") != SourceManual || NormalizeSource("x") != "" {
		t.Fatal("normalize")
	}
	d := DefaultsSchedule(Options{Keep: 0, IncludeEdges: true})
	if d.Keep != 7 || d.Enabled || !d.IncludeEdges {
		t.Fatalf("%+v", d)
	}
}

func TestCatalogHints(t *testing.T) {
	var nilSvc *Service
	if _, err := nilSvc.Catalog(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil: %v", err)
	}
	store := newMemStore()
	store.ready = false
	s := New(Options{Enabled: true, Keep: 3}, &memRunner{}, store, nil)
	cat, err := s.Catalog()
	if err != nil || cat.Hint == "" || cat.DirReady {
		t.Fatalf("dir not ready: %+v %v", cat, err)
	}
	store.ready = true
	s = New(Options{Enabled: false, Keep: 3}, &memRunner{}, store, nil)
	cat, err = s.Catalog()
	if err != nil || cat.Enabled {
		t.Fatalf("disabled: %+v %v", cat, err)
	}
	if cat.Hint == "" {
		t.Fatal("want disabled hint")
	}
	store.exists["ga-1"] = true
	store.entries = []Entry{{Name: "ga-1", CreatedAt: time.Now().UTC()}}
	store.attached = "ga-1"
	s = New(Options{Enabled: true}, &memRunner{}, store, &memSchedule{sch: DefaultsSchedule(Options{Keep: 7})})
	cat, err = s.Catalog()
	if err != nil || cat.Attached != "ga-1" || len(cat.Backups) != 1 || !cat.Backups[0].Attached {
		t.Fatalf("attached catalog: %+v %v", cat, err)
	}
}

func TestUpdateScheduleAndGetSchedule(t *testing.T) {
	schStore := &memSchedule{sch: DefaultsSchedule(Options{Keep: 7})}
	s := New(Options{Enabled: true}, &memRunner{}, newMemStore(), schStore)
	sink := &auditSink{}
	s.SetLogService(usecaseaudit.New(sink))

	out, err := s.UpdateSchedule(context.Background(), Schedule{
		Enabled: true, Hour: 4, Minute: 15, Timezone: "UTC", Keep: 5,
	}, "admin")
	if err != nil || !out.Enabled || out.Hour != 4 {
		t.Fatalf("update: %+v %v", out, err)
	}
	got, err := s.GetSchedule()
	if err != nil || got.Hour != 4 || got.Keep != 5 {
		t.Fatalf("get: %+v %v", got, err)
	}
	if _, err := s.UpdateSchedule(context.Background(), Schedule{Hour: 99, Minute: 0, Timezone: "UTC", Keep: 7}, "a"); err == nil {
		t.Fatal("want invalid")
	}
}

func TestDeleteBackupGuards(t *testing.T) {
	store := newMemStore()
	store.exists["ga-1"] = true
	store.attached = "ga-1"
	s := New(Options{Enabled: true}, &memRunner{}, store, nil)
	if err := s.DeleteBackup(context.Background(), "ga-1", "u"); !errors.Is(err, ErrDeleteActive) {
		t.Fatalf("attached: %v", err)
	}
	store.attached = ""
	if err := s.DeleteBackup(context.Background(), "missing", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	store.exists["ga-2"] = true
	store.entries = []Entry{{Name: "ga-2"}}
	if err := s.DeleteBackup(context.Background(), "ga-2", "u"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if store.exists["ga-2"] {
		t.Fatal("still exists")
	}
}

func TestScheduleCreateSuccess(t *testing.T) {
	store := newMemStore()
	runner := &memRunner{tables: map[string]bool{"traffic_logs": true, "geo_ranges": true}}
	sch := &memSchedule{sch: Schedule{Enabled: true, Hour: 2, Minute: 0, Timezone: "UTC", Keep: 7}}
	s := New(Options{Enabled: true, DataDir: t.TempDir(), IncludeAuth: true}, runner, store, sch)
	s.opts.IncludeAuth = true
	heavy := &memHeavy{}
	s.SetHeavySlot(heavy)
	sink := &auditSink{}
	s.SetLogService(usecaseaudit.New(sink))

	if err := s.ScheduleCreate(context.Background(), SourceManual, "admin"); err != nil {
		t.Fatal(err)
	}
	st := waitJobIdle(t, s, 3*time.Second)
	if st.State != "ok" {
		t.Fatalf("status=%+v backupN=%d", st, runner.backupN)
	}
	if runner.backupN != 1 || heavy.n != 1 {
		t.Fatalf("backupN=%d heavy=%d", runner.backupN, heavy.n)
	}
	if err := s.ScheduleCreate(context.Background(), SourceManual, "admin"); err != nil {
		// may succeed after finish
		_ = err
	}
}

func TestScheduleCreateDisabledBusyUnavailable(t *testing.T) {
	s := New(Options{Enabled: false}, &memRunner{}, newMemStore(), nil)
	if err := s.ScheduleCreate(context.Background(), SourceManual, "u"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled: %v", err)
	}
	var nilSvc *Service
	if err := nilSvc.ScheduleCreate(context.Background(), SourceManual, "u"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil: %v", err)
	}
	store := newMemStore()
	store.ready = false
	s = New(Options{Enabled: true}, &memRunner{}, store, nil)
	if err := s.ScheduleCreate(context.Background(), SourceManual, "u"); err == nil {
		t.Fatal("dir not ready")
	}
}

func TestScheduleCreateBusy(t *testing.T) {
	store := newMemStore()
	runner := &memRunner{tables: map[string]bool{"traffic_logs": true}}
	s := New(Options{Enabled: true}, runner, store, nil)
	// hold job busy
	if !s.job.TryStart() {
		t.Fatal("start")
	}
	if err := s.ScheduleCreate(context.Background(), SourceManual, "u"); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy: %v", err)
	}
	s.job.Finish()
}

func TestRunCreateFailures(t *testing.T) {
	store := newMemStore()
	runner := &memRunner{failExists: errors.New("ch down")}
	s := New(Options{Enabled: true}, runner, store, nil)
	if err := s.ScheduleCreate(context.Background(), SourceManual, "u"); err != nil {
		t.Fatal(err)
	}
	st := waitJobIdle(t, s, 3*time.Second)
	if st.State != "error" {
		t.Fatalf("want error: %+v", st)
	}

	runner = &memRunner{tables: map[string]bool{}}
	s = New(Options{Enabled: true}, runner, store, nil)
	if err := s.ScheduleCreate(context.Background(), SourceManual, "u"); err != nil {
		t.Fatal(err)
	}
	st = waitJobIdle(t, s, 3*time.Second)
	if st.State != "error" || st.Message == "" {
		t.Fatalf("no tables: %+v", st)
	}

	runner = &memRunner{tables: map[string]bool{"traffic_logs": true}, failBackup: errors.New("boom")}
	s = New(Options{Enabled: true}, runner, store, nil)
	if err := s.ScheduleCreate(context.Background(), SourceManual, "u"); err != nil {
		t.Fatal(err)
	}
	st = waitJobIdle(t, s, 3*time.Second)
	if st.State != "error" {
		t.Fatalf("backup fail: %+v", st)
	}
}

func TestAttachDetach(t *testing.T) {
	store := newMemStore()
	store.exists["ga-1"] = true
	runner := &memRunner{}
	s := New(Options{Enabled: true}, runner, store, nil)

	if err := s.ScheduleAttach(context.Background(), "missing", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err := s.ScheduleAttach(context.Background(), "ga-1", "u"); err != nil {
		t.Fatal(err)
	}
	st := waitJobIdle(t, s, 3*time.Second)
	if st.State != "ok" || store.attached != "ga-1" || runner.restoreN != 1 {
		t.Fatalf("attach: %+v att=%q restore=%d", st, store.attached, runner.restoreN)
	}

	if err := s.ScheduleDetach(context.Background(), "other", "u"); !errors.Is(err, ErrNotAttached) {
		t.Fatalf("not attached name: %v", err)
	}
	if err := s.ScheduleDetach(context.Background(), "ga-1", "u"); err != nil {
		t.Fatal(err)
	}
	st = waitJobIdle(t, s, 3*time.Second)
	if st.State != "ok" || store.attached != "" || runner.dropN != 1 {
		t.Fatalf("detach: %+v att=%q drop=%d", st, store.attached, runner.dropN)
	}
}

func TestAttachRestoreFails(t *testing.T) {
	store := newMemStore()
	store.exists["ga-1"] = true
	runner := &memRunner{failRestore: errors.New("restore boom")}
	s := New(Options{Enabled: true}, runner, store, nil)
	if err := s.ScheduleAttach(context.Background(), "ga-1", "u"); err != nil {
		t.Fatal(err)
	}
	st := waitJobIdle(t, s, 3*time.Second)
	if st.State != "error" || runner.dropN != 1 {
		t.Fatalf("want cleanup drop: %+v drop=%d", st, runner.dropN)
	}
}

func TestTickAutoCreate(t *testing.T) {
	store := newMemStore()
	runner := &memRunner{tables: map[string]bool{"traffic_logs": true}}
	loc := time.UTC
	now := time.Date(2026, 8, 11, 3, 0, 0, 0, loc)
	sch := &memSchedule{sch: Schedule{
		Enabled: true, Hour: 2, Minute: 0, Timezone: "UTC", Keep: 7,
	}}
	s := New(Options{Enabled: true}, runner, store, sch)
	s.TickAutoCreate(context.Background(), now)
	st := waitJobIdle(t, s, 3*time.Second)
	if st.State != "ok" {
		t.Fatalf("auto: %+v", st)
	}
	// second tick same day should dedupe in-memory
	s.TickAutoCreate(context.Background(), now.Add(time.Minute))
	if runner.backupN != 1 {
		t.Fatalf("dedupe backupN=%d", runner.backupN)
	}
}

func TestAttachedNameAndStatusNil(t *testing.T) {
	var s *Service
	if s.AttachedName() != "" || s.Status().State != "idle" {
		t.Fatal("nil helpers")
	}
	store := newMemStore()
	store.attached = "x"
	s = New(Options{}, &memRunner{}, store, nil)
	if s.AttachedName() != "x" {
		t.Fatal(s.AttachedName())
	}
	s.Shutdown(context.Background())
}

func TestDropStaleScheduleLastRun(t *testing.T) {
	store := newMemStore()
	schStore := &memSchedule{sch: Schedule{
		Enabled: true, Hour: 2, Minute: 0, Timezone: "UTC", Keep: 7,
		LastRunDate: "2026-08-11", LastRunAt: "t",
	}}
	s := New(Options{Enabled: true}, &memRunner{}, store, schStore)
	got, err := s.GetSchedule()
	if err != nil {
		t.Fatal(err)
	}
	if got.LastRunDate != "" {
		t.Fatalf("stale last run should drop: %+v", got)
	}
}

func TestSafeActor(t *testing.T) {
	if safeActor("") != "system" || safeActor("  a  ") != "a" {
		t.Fatal("actor")
	}
}
