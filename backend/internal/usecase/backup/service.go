package backup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/apperr"
)

var (
	ErrUnavailable  = errors.New("backup service unavailable")
	ErrBusy         = apperr.Conflict("backup already running")
	ErrDisabled     = errors.New("backups disabled")
	ErrNotFound     = apperr.NotFound("backup not found")
	ErrNotAttached  = apperr.InvalidInput("backup is not attached")
	ErrDeleteActive = apperr.Conflict("cannot delete attached backup; detach first")
)

// Entry — один полный бэкап на disk backups.
type Entry struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	HasAuth   bool      `json:"has_auth"`
	Attached  bool      `json:"attached"`
	// Source: manual | schedule | "" (старые бэкапы без маркера).
	Source string `json:"source,omitempty"`
}

const (
	SourceManual   = "manual"
	SourceSchedule = "schedule"
)

// NormalizeSource — только известные значения.
func NormalizeSource(s string) string {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case SourceManual:
		return SourceManual
	case SourceSchedule:
		return SourceSchedule
	default:
		return ""
	}
}

// Status — состояние фоновой задачи.
type Status struct {
	State     string    `json:"state"` // idle|running|ok|error
	Message   string    `json:"message"`
	Name      string    `json:"name,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Catalog — список + настройки для UI.
type Catalog struct {
	OK           bool      `json:"ok"`
	Enabled      bool      `json:"enabled"`
	DirReady     bool      `json:"dir_ready"`
	Keep         int       `json:"keep"`
	IncludeEdges bool      `json:"include_edges"`
	IncludeAuth  bool      `json:"include_auth"`
	Attached     string    `json:"attached,omitempty"`
	Schedule     Schedule  `json:"schedule"`
	NextRunAt    string    `json:"next_run_at,omitempty"`
	Backups      []Entry   `json:"backups"`
	Status       Status    `json:"status"`
	Hint         string    `json:"hint,omitempty"`
}

// Runner — native BACKUP / RESTORE / DROP в ClickHouse.
type Runner interface {
	BackupTables(ctx context.Context, name string, tables []string) error
	RestoreMapShadow(ctx context.Context, name string) error
	DropMapShadow(ctx context.Context) error
	TableExists(ctx context.Context, name string) (bool, error)
}

// Store — каталог бэкапов на томе.
type Store interface {
	DirReady() bool
	List() ([]Entry, error)
	Exists(name string) bool
	WriteAuthTarball(name, dataDir string) error
	WriteSource(name, source string) error
	Prune(keep int) error
	Delete(name string) error
	Attached() (string, error)
	SetAttached(name string) error
}

// Options — из config (env defaults / kill-switch).
type Options struct {
	Enabled      bool
	Dir          string
	DataDir      string
	Keep         int
	IncludeEdges bool
	IncludeAuth  bool
}

// Service — list + async create / attach / detach + schedule.
type Service struct {
	opts     Options
	runner   Runner
	store    Store
	schedule ScheduleStore
	job      *Job

	mu           sync.Mutex
	lastFireDate string // in-memory dedupe на случай гонки тика
}

func New(opts Options, runner Runner, store Store, schedule ScheduleStore) *Service {
	if opts.Keep < 1 {
		opts.Keep = 7
	}
	return &Service{
		opts:     opts,
		runner:   runner,
		store:    store,
		schedule: schedule,
		job:      NewJob(),
	}
}

func (s *Service) Catalog() (Catalog, error) {
	if s == nil {
		return Catalog{}, ErrUnavailable
	}
	sch, _ := s.GetSchedule()
	cat := Catalog{
		OK:           true,
		Enabled:      s.opts.Enabled,
		DirReady:     s.store != nil && s.store.DirReady(),
		Keep:         sch.Keep,
		IncludeEdges: sch.IncludeEdges,
		IncludeAuth:  sch.IncludeAuth,
		Schedule:     sch,
		Backups:      []Entry{},
		Status:       s.job.Status(),
	}
	if sch.Enabled && s.opts.Enabled && cat.DirReady {
		cat.NextRunAt = NextRunAt(sch, time.Now().UTC()).Format(time.RFC3339)
	}
	if !cat.DirReady {
		cat.Hint = "Том clickhouse-backups не смонтирован в backend (BACKUP_DIR). Перезапустите compose."
		return cat, nil
	}
	attached, err := s.store.Attached()
	if err != nil {
		return cat, err
	}
	cat.Attached = attached
	if !cat.Enabled {
		cat.Hint = "Создание бэкапов отключено (BACKUP_ENABLED=0)."
	}
	entries, err := s.store.List()
	if err != nil {
		return cat, err
	}
	for i := range entries {
		entries[i].Attached = entries[i].Name == attached
	}
	cat.Backups = entries
	if attached != "" {
		cat.Hint = "Подключён " + attached + ": данные в shadow-таблицах nm_bak_*. Live и ingest не трогаются. На карте переключите источник на «Бэкап»."
	} else if sch.Enabled && cat.Enabled {
		cat.Hint = "Автобэкап включён. Внешний cron scripts/backup-clickhouse.sh не обязателен."
	} else {
		cat.Hint = "«Подключить» копирует данные бэкапа в nm_bak_* (live не меняется). Расписание — в блоке ниже."
	}
	return cat, nil
}

func (s *Service) Status() Status {
	if s == nil || s.job == nil {
		return Status{State: "idle", Message: "unavailable"}
	}
	return s.job.Status()
}

// AttachedName — имя подключённого бэкапа (пусто если нет).
func (s *Service) AttachedName() string {
	if s == nil || s.store == nil {
		return ""
	}
	name, err := s.store.Attached()
	if err != nil {
		return ""
	}
	return name
}

func (s *Service) GetSchedule() (Schedule, error) {
	if s == nil {
		return Schedule{}, ErrUnavailable
	}
	if s.schedule == nil {
		return DefaultsSchedule(s.opts), nil
	}
	out, err := s.schedule.Load()
	if err != nil {
		return Schedule{}, err
	}
	normalized, err := ValidateSchedule(out)
	if err != nil {
		seed := DefaultsSchedule(s.opts)
		seed.LastRunAt = out.LastRunAt
		seed.LastRunDate = out.LastRunDate
		s.dropStaleScheduleLastRun(&seed)
		return seed, nil
	}
	normalized.LastRunAt = out.LastRunAt
	normalized.LastRunDate = out.LastRunDate
	s.dropStaleScheduleLastRun(&normalized)
	return normalized, nil
}

func (s *Service) UpdateSchedule(in Schedule) (Schedule, error) {
	if s == nil || s.schedule == nil {
		return Schedule{}, ErrUnavailable
	}
	out, err := ValidateSchedule(in)
	if err != nil {
		return Schedule{}, err
	}
	prev, _ := s.GetSchedule()
	out.LastRunAt = prev.LastRunAt
	out.LastRunDate = prev.LastRunDate
	// Смена слота — разрешить новый прогон сегодня (иначе «Последний авто» блокирует догон).
	if prev.Hour != out.Hour || prev.Minute != out.Minute || prev.Timezone != out.Timezone {
		out.LastRunDate = ""
		s.clearLastFireDate()
	}
	out.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.schedule.Save(out); err != nil {
		return Schedule{}, err
	}
	return out, nil
}

// TickAutoCreate — вызов из backupjob; запускает бэкап если пора.
func (s *Service) TickAutoCreate(parent context.Context, now time.Time) {
	if s == nil || !s.opts.Enabled {
		return
	}
	sch, err := s.GetSchedule()
	if err != nil || !sch.Enabled {
		return
	}
	fire, dateKey := ShouldFire(sch, now)
	if !fire {
		return
	}
	s.mu.Lock()
	if s.lastFireDate == dateKey {
		s.mu.Unlock()
		return
	}
	s.lastFireDate = dateKey
	s.mu.Unlock()

	if err := s.ScheduleCreate(parent, SourceSchedule); err != nil {
		// ErrBusy / недоступность — снимем dedupe, чтобы тикер догнал позже сегодня.
		s.clearLastFireDate()
		return
	}
	// last_run_* пишем только после успешного runCreate (см. persistScheduleRun).
}

func (s *Service) clearLastFireDate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.lastFireDate = ""
	s.mu.Unlock()
}

// dropStaleScheduleLastRun — last_run без бэкапа source=schedule в этот день (старый баг:
// last_run писали при постановке в очередь). Сбрасываем, чтобы догон сработал.
func (s *Service) dropStaleScheduleLastRun(sch *Schedule) {
	if s == nil || s.store == nil || sch == nil || sch.LastRunDate == "" {
		return
	}
	list, err := s.store.List()
	if err != nil {
		return
	}
	loc, err := time.LoadLocation(sch.Timezone)
	if err != nil {
		loc = time.UTC
	}
	for _, e := range list {
		if e.Source != SourceSchedule {
			continue
		}
		if e.CreatedAt.In(loc).Format("2006-01-02") == sch.LastRunDate {
			return
		}
	}
	sch.LastRunDate = ""
	sch.LastRunAt = ""
	if s.schedule != nil {
		_ = s.schedule.Save(*sch)
	}
	s.clearLastFireDate()
}

func (s *Service) persistScheduleRun(now time.Time) {
	if s == nil || s.schedule == nil {
		return
	}
	sch, err := s.GetSchedule()
	if err != nil {
		return
	}
	loc, err := time.LoadLocation(sch.Timezone)
	if err != nil {
		loc = time.UTC
	}
	sch.LastRunAt = now.UTC().Format(time.RFC3339)
	sch.LastRunDate = now.In(loc).Format("2006-01-02")
	_ = s.schedule.Save(sch)
}

// ScheduleCreate ставит полный бэкап в очередь (не блокирует HTTP).
// source: manual | schedule.
func (s *Service) ScheduleCreate(parent context.Context, source string) error {
	if err := s.readyForJob(); err != nil {
		return err
	}
	if !s.opts.Enabled {
		return ErrDisabled
	}
	if !s.job.TryStart() {
		return ErrBusy
	}
	src := NormalizeSource(source)
	if src == "" {
		src = SourceManual
	}
	ctx, cancel := detachContext(parent, 2*time.Hour)
	go func() {
		defer cancel()
		s.runCreate(ctx, src)
	}()
	return nil
}

// ScheduleAttach: RESTORE map-таблиц в nm_bak_* (live не трогаем).
func (s *Service) ScheduleAttach(parent context.Context, name string) error {
	if err := s.readyForJob(); err != nil {
		return err
	}
	if !s.store.Exists(name) {
		return ErrNotFound
	}
	if !s.job.TryStart() {
		return ErrBusy
	}
	ctx, cancel := detachContext(parent, 2*time.Hour)
	go func() {
		defer cancel()
		s.runAttach(ctx, name)
	}()
	return nil
}

// ScheduleDetach: DROP nm_bak_*; бэкап на диске и live остаются.
func (s *Service) ScheduleDetach(parent context.Context, name string) error {
	if err := s.readyForJob(); err != nil {
		return err
	}
	attached, err := s.store.Attached()
	if err != nil {
		return err
	}
	if attached == "" || attached != name {
		return ErrNotAttached
	}
	if !s.job.TryStart() {
		return ErrBusy
	}
	ctx, cancel := detachContext(parent, 30*time.Minute)
	go func() {
		defer cancel()
		s.runDetach(ctx, name)
	}()
	return nil
}

// DeleteBackup удаляет файлы бэкапа с тома (нельзя, если он подключён).
func (s *Service) DeleteBackup(name string) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	if !s.store.DirReady() {
		return fmt.Errorf("%w: backup dir not ready", ErrUnavailable)
	}
	if s.job != nil {
		st := s.job.Status()
		if st.State == "running" {
			return ErrBusy
		}
	}
	attached, err := s.store.Attached()
	if err != nil {
		return err
	}
	if attached != "" && attached == name {
		return ErrDeleteActive
	}
	if !s.store.Exists(name) {
		return ErrNotFound
	}
	return s.store.Delete(name)
}

func (s *Service) readyForJob() error {
	if s == nil || s.runner == nil || s.store == nil {
		return ErrUnavailable
	}
	if !s.store.DirReady() {
		return fmt.Errorf("%w: backup dir not ready", ErrUnavailable)
	}
	return nil
}

func detachContext(_ context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func (s *Service) effectivePolicy() (keep int, includeEdges, includeAuth bool) {
	sch, err := s.GetSchedule()
	if err != nil {
		return s.opts.Keep, s.opts.IncludeEdges, s.opts.IncludeAuth
	}
	return sch.Keep, sch.IncludeEdges, sch.IncludeAuth
}

func (s *Service) runCreate(ctx context.Context, source string) {
	defer s.job.Finish()
	succeeded := false
	defer func() {
		if source != SourceSchedule {
			return
		}
		if succeeded {
			s.persistScheduleRun(time.Now().UTC())
			return
		}
		s.clearLastFireDate()
	}()

	name := "nm-" + time.Now().UTC().Format("20060102T150405Z")
	if sch, err := s.GetSchedule(); err == nil {
		name = FormatBackupName(time.Now(), sch.Timezone)
	}
	s.job.SetRunning(name, "backup started")

	keep, includeEdges, includeAuth := s.effectivePolicy()

	tables, err := s.resolveTables(ctx, includeEdges)
	if err != nil {
		s.job.SetError(name, err.Error())
		return
	}
	if len(tables) == 0 {
		s.job.SetError(name, "no tables to back up")
		return
	}

	s.job.SetRunning(name, "clickhouse BACKUP…")
	if err := s.runner.BackupTables(ctx, name, tables); err != nil {
		if ctx.Err() != nil {
			s.job.SetError(name, "canceled")
			return
		}
		s.job.SetError(name, err.Error())
		return
	}

	if includeAuth && s.opts.DataDir != "" {
		s.job.SetRunning(name, "auth tarball…")
		if err := s.store.WriteAuthTarball(name, s.opts.DataDir); err != nil {
			s.job.SetError(name, "auth tarball: "+err.Error())
			return
		}
	}

	_ = s.store.WriteSource(name, source)
	// Бэкап на диске есть — день расписания считаем закрытым (даже если prune ниже упадёт).
	succeeded = true

	if err := s.store.Prune(keep); err != nil {
		s.job.SetError(name, "prune: "+err.Error())
		return
	}
	s.job.SetOK(name, "done")
}

func (s *Service) runAttach(ctx context.Context, name string) {
	defer s.job.Finish()
	s.job.SetRunning(name, "RESTORE в nm_bak_*…")

	if err := s.runner.RestoreMapShadow(ctx, name); err != nil {
		_ = s.store.SetAttached("")
		dropCtx, dropCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		_ = s.runner.DropMapShadow(dropCtx)
		dropCancel()
		if ctx.Err() != nil {
			s.job.SetError(name, "canceled")
			return
		}
		s.job.SetError(name, err.Error())
		return
	}

	if err := s.store.SetAttached(name); err != nil {
		s.job.SetError(name, "marker: "+err.Error())
		return
	}
	s.job.SetOK(name, "подключён — смотрите на карте источник «Бэкап»; live не изменён")
}

func (s *Service) runDetach(ctx context.Context, name string) {
	defer s.job.Finish()
	s.job.SetRunning(name, "DROP nm_bak_*…")

	if err := s.runner.DropMapShadow(ctx); err != nil {
		s.job.SetError(name, "drop: "+err.Error())
		return
	}
	if err := s.store.SetAttached(""); err != nil {
		s.job.SetError(name, "marker: "+err.Error())
		return
	}
	s.job.SetOK(name, "отключён — shadow удалены, live и бэкап на диске на месте")
}

func (s *Service) resolveTables(ctx context.Context, includeEdges bool) ([]string, error) {
	base := []string{
		"traffic_logs",
		"geo_ranges",
		"reputation_ranges",
		"parse_errors",
		"system_metrics",
		"anomaly_events",
		"anomaly_acks",
	}
	if includeEdges {
		base = append(base,
			"traffic_edges_daily",
			"traffic_edges_hourly",
			"traffic_edges_city_daily",
			"traffic_edges_country_daily",
		)
	}
	var out []string
	for _, t := range base {
		ok, err := s.runner.TableExists(ctx, t)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// Shutdown ждёт текущий job (best-effort).
func (s *Service) Shutdown(ctx context.Context) {
	if s == nil || s.job == nil {
		return
	}
	s.job.Wait(ctx)
}
