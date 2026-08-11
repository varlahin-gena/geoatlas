package backup

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnavailable = errors.New("backup service unavailable")
	ErrBusy        = errors.New("backup already running")
	ErrDisabled    = errors.New("backups disabled")
)

// Entry — один полный бэкап на disk backups.
type Entry struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	HasAuth   bool      `json:"has_auth"`
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
	OK           bool   `json:"ok"`
	Enabled      bool   `json:"enabled"`
	DirReady     bool   `json:"dir_ready"`
	Keep         int    `json:"keep"`
	IncludeEdges bool   `json:"include_edges"`
	IncludeAuth  bool   `json:"include_auth"`
	Backups      []Entry `json:"backups"`
	Status       Status `json:"status"`
	Hint         string `json:"hint,omitempty"`
}

// Runner выполняет native BACKUP в ClickHouse.
type Runner interface {
	BackupTables(ctx context.Context, name string, tables []string) error
	TableExists(ctx context.Context, name string) (bool, error)
}

// Store — каталог бэкапов на томе (list / auth tarball / prune).
type Store interface {
	DirReady() bool
	List() ([]Entry, error)
	WriteAuthTarball(name, dataDir string) error
	Prune(keep int) error
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

// Service — list + async create.
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
	if !cat.Enabled {
		cat.Hint = "Создание бэкапов отключено (BACKUP_ENABLED=0)."
	}
	entries, err := s.store.List()
	if err != nil {
		return cat, err
	}
	cat.Backups = entries
	cat.Hint = "Restore: ./scripts/restore-clickhouse.sh <name> на хосте appliance."
	return cat, nil
}

func (s *Service) Status() Status {
	if s == nil || s.job == nil {
		return Status{State: "idle", Message: "unavailable"}
	}
	return s.job.Status()
}

// ScheduleCreate ставит полный бэкап в очередь (не блокирует HTTP).
func (s *Service) ScheduleCreate(parent context.Context) error {
	if s == nil || s.runner == nil || s.store == nil {
		return ErrUnavailable
	}
	if !s.opts.Enabled {
		return ErrDisabled
	}
	if !s.store.DirReady() {
		return fmt.Errorf("%w: backup dir not ready", ErrUnavailable)
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

func detachContext(_ context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func (s *Service) runCreate(ctx context.Context) {
	defer s.job.Finish()

	name := "nm-" + time.Now().UTC().Format("20060102T150405Z")
	s.job.SetRunning(name, "backup started")

	tables, err := s.resolveTables(ctx)
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

func (s *Service) resolveTables(ctx context.Context) ([]string, error) {
	base := []string{
		"traffic_logs",
		"geo_ranges",
		"reputation_ranges",
		"parse_errors",
		"system_metrics",
	}
	if s.opts.IncludeEdges {
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
