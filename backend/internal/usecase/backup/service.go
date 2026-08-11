package backup

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnavailable  = errors.New("backup service unavailable")
	ErrBusy         = errors.New("backup already running")
	ErrDisabled     = errors.New("backups disabled")
	ErrNotFound     = errors.New("backup not found")
	ErrNotAttached  = errors.New("backup is not attached")
	ErrDeleteActive = errors.New("cannot delete attached backup; detach first")
)

// Entry — один полный бэкап на disk backups.
type Entry struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	HasAuth   bool      `json:"has_auth"`
	Attached  bool      `json:"attached"`
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
	OK           bool    `json:"ok"`
	Enabled      bool    `json:"enabled"`
	DirReady     bool    `json:"dir_ready"`
	Keep         int     `json:"keep"`
	IncludeEdges bool    `json:"include_edges"`
	IncludeAuth  bool    `json:"include_auth"`
	Attached     string  `json:"attached,omitempty"`
	Backups      []Entry `json:"backups"`
	Status       Status  `json:"status"`
	Hint         string  `json:"hint,omitempty"`
}

// Runner — native BACKUP / RESTORE / DROP в ClickHouse.
type Runner interface {
	BackupTables(ctx context.Context, name string, tables []string) error
	RestoreTables(ctx context.Context, name string, tables []string) error
	RestoreTablesAs(ctx context.Context, name string, pairs [][2]string) error
	DropTables(ctx context.Context, tables []string) error
	TruncateTables(ctx context.Context, tables []string) error
	TableExists(ctx context.Context, name string) (bool, error)
}

// Store — каталог бэкапов на томе.
type Store interface {
	DirReady() bool
	List() ([]Entry, error)
	Exists(name string) bool
	WriteAuthTarball(name, dataDir string) error
	Prune(keep int) error
	Delete(name string) error
	Attached() (string, error)
	SetAttached(name string) error
}

// Options — из config.
type Options struct {
	Enabled      bool
	Dir          string
	DataDir      string
	Keep         int
	IncludeEdges bool
	IncludeAuth  bool
}

// Service — list + async create / attach / detach.
type Service struct {
	opts   Options
	runner Runner
	store  Store
	job    *Job
}

func New(opts Options, runner Runner, store Store) *Service {
	if opts.Keep < 1 {
		opts.Keep = 7
	}
	return &Service{
		opts:   opts,
		runner: runner,
		store:  store,
		job:    NewJob(),
	}
}

func (s *Service) Catalog() (Catalog, error) {
	if s == nil {
		return Catalog{}, ErrUnavailable
	}
	cat := Catalog{
		OK:           true,
		Enabled:      s.opts.Enabled,
		DirReady:     s.store != nil && s.store.DirReady(),
		Keep:         s.opts.Keep,
		IncludeEdges: s.opts.IncludeEdges,
		IncludeAuth:  s.opts.IncludeAuth,
		Backups:      []Entry{},
		Status:       s.job.Status(),
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
	} else {
		cat.Hint = "«Подключить» копирует данные бэкапа в nm_bak_* (live не меняется). На карте можно смотреть Live или Бэкап. Auth — снимок /app/data (*.auth.tgz), не трафик."
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

// ScheduleCreate ставит полный бэкап в очередь (не блокирует HTTP).
func (s *Service) ScheduleCreate(parent context.Context) error {
	if err := s.readyForJob(); err != nil {
		return err
	}
	if !s.opts.Enabled {
		return ErrDisabled
	}
	if !s.job.TryStart() {
		return ErrBusy
	}
	ctx, cancel := detachContext(parent, 2*time.Hour)
	go func() {
		defer cancel()
		s.runCreate(ctx)
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

func (s *Service) runCreate(ctx context.Context) {
	defer s.job.Finish()

	name := "nm-" + time.Now().UTC().Format("20060102T150405Z")
	s.job.SetRunning(name, "backup started")

	tables, err := s.resolveTables(ctx, s.opts.IncludeEdges)
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

	if s.opts.IncludeAuth && s.opts.DataDir != "" {
		s.job.SetRunning(name, "auth tarball…")
		if err := s.store.WriteAuthTarball(name, s.opts.DataDir); err != nil {
			s.job.SetError(name, "auth tarball: "+err.Error())
			return
		}
	}

	if err := s.store.Prune(s.opts.Keep); err != nil {
		s.job.SetError(name, "prune: "+err.Error())
		return
	}
	s.job.SetOK(name, "done")
}

func (s *Service) runAttach(ctx context.Context, name string) {
	defer s.job.Finish()
	s.job.SetRunning(name, "RESTORE в nm_bak_*…")

	// Импорт здесь нельзя (цикл) — пары задаём явно, зеркало query.MapShadowPairs.
	pairs := [][2]string{
		{"traffic_logs", "nm_bak_traffic_logs"},
		{"traffic_edges_city_daily", "nm_bak_traffic_edges_city_daily"},
		{"traffic_edges_country_daily", "nm_bak_traffic_edges_country_daily"},
	}
	if err := s.runner.RestoreTablesAs(ctx, name, pairs); err != nil {
		_ = s.store.SetAttached("")
		// ctx мог быть уже отменён — cleanup без cancel, но с тем же деревом значений.
		dropCtx, dropCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		_ = s.runner.DropTables(dropCtx, []string{
			"nm_bak_traffic_logs",
			"nm_bak_traffic_edges_city_daily",
			"nm_bak_traffic_edges_country_daily",
		})
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

	shadows := []string{
		"nm_bak_traffic_logs",
		"nm_bak_traffic_edges_city_daily",
		"nm_bak_traffic_edges_country_daily",
	}
	if err := s.runner.DropTables(ctx, shadows); err != nil {
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
	}
	if includeEdges {
		base = append(base,
			"traffic_edges_daily",
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
