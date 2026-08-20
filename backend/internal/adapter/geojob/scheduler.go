package geojob

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type jobKind int

const (
	jobEnrich jobKind = iota
	jobReloadAndEnrich
	jobMaintenanceBackfill
)

// DefaultLookbackDays — окно auto-backfill, если lookback не задан.
const DefaultLookbackDays = 7

// Scheduler сериализует reload GeoIP, agg backfill и geo enrich lookup.
// Параллельные Schedule* не запускают несколько INSERT/DELETE сразу:
// предыдущая работа отменяется по context, следующая ждёт workMu.
type Scheduler struct {
	geo          GeoIndex
	store        Store
	lookbackDays int

	mu     sync.Mutex
	cancel context.CancelFunc
	gen    uint64
	wg     sync.WaitGroup
	workMu sync.Mutex
}

// New создаёт scheduler. lookbackDays ограничивает EnrichLogsMissingGeo
// (0 = весь объём; <0 → DefaultLookbackDays).
func New(geo GeoIndex, store Store, lookbackDays int) *Scheduler {
	if lookbackDays < 0 {
		lookbackDays = DefaultLookbackDays
	}
	return &Scheduler{geo: geo, store: store, lookbackDays: lookbackDays}
}

// ScheduleReloadAndEnrich ставит в очередь reload + EnrichLogsMissingGeo.
func (s *Scheduler) ScheduleReloadAndEnrich(parent context.Context, timeout time.Duration) {
	s.schedule(parent, timeout, jobReloadAndEnrich)
}

// ScheduleEnrichOnly — только geo enrich (например после Ensure* на старте).
func (s *Scheduler) ScheduleEnrichOnly(parent context.Context, timeout time.Duration) {
	s.schedule(parent, timeout, jobEnrich)
}

// ScheduleMaintenanceBackfill — edges/geo-edges backfill + geo enrich.
// Используется POST /api/system/maintenance/backfill и SKIP_STARTUP_BACKFILL.
func (s *Scheduler) ScheduleMaintenanceBackfill(parent context.Context, timeout time.Duration) {
	s.schedule(parent, timeout, jobMaintenanceBackfill)
}

func (s *Scheduler) schedule(parent context.Context, timeout time.Duration, kind jobKind) {
	if s == nil || s.store == nil {
		return
	}
	if kind != jobMaintenanceBackfill && s.geo == nil {
		return
	}
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	if kind == jobMaintenanceBackfill && timeout < time.Hour {
		timeout = 6 * time.Hour
	}

	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.gen++
	gen := s.gen
	ctx, cancel := detachTimeout(parent, timeout)
	s.cancel = cancel
	s.wg.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()
		s.run(ctx, gen, kind)
	}()
}

// detachTimeout намеренно отвязывает работу от parent (тот же приём, что в ingest).
func detachTimeout(_ context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func (s *Scheduler) run(ctx context.Context, gen uint64, kind jobKind) {
	s.workMu.Lock()
	defer s.workMu.Unlock()

	if ctx.Err() != nil {
		return
	}
	s.mu.Lock()
	current := s.gen
	s.mu.Unlock()
	if gen != current {
		return
	}

	switch kind {
	case jobMaintenanceBackfill:
		s.runMaintenance(ctx)
	case jobReloadAndEnrich:
		s.runReload(ctx)
		if ctx.Err() != nil {
			return
		}
		s.runEnrich(ctx)
	default:
		s.runEnrich(ctx)
	}
}

func (s *Scheduler) runMaintenance(ctx context.Context) {
	slog.Info("maintenance: edges/geo backfill started")
	if err := s.store.BackfillEdgesAgg(ctx); err != nil {
		if ctx.Err() != nil {
			slog.Info("maintenance: edges backfill canceled")
			return
		}
		slog.Error("maintenance: edges backfill failed", "err", err)
		return
	}
	if ctx.Err() != nil {
		return
	}
	if err := s.store.BackfillGeoEdgesAgg(ctx); err != nil {
		if ctx.Err() != nil {
			slog.Info("maintenance: geo-edges backfill canceled")
			return
		}
		slog.Error("maintenance: geo-edges backfill failed", "err", err)
		return
	}
	slog.Info("maintenance: edges/geo backfill done")
	if s.geo != nil && s.geo.RangeCount() > 0 {
		s.runEnrich(ctx)
	}
}

func (s *Scheduler) runReload(ctx context.Context) {
	if s.geo == nil {
		return
	}
	if err := s.geo.Reload(ctx); err != nil {
		if ctx.Err() != nil {
			slog.Info("geo job: reload canceled")
			return
		}
		slog.Error("geo job: index reload failed", "err", err)
		return
	}
}

func (s *Scheduler) runEnrich(ctx context.Context) {
	if s.geo == nil || s.store == nil {
		return
	}
	if ctx.Err() != nil {
		return
	}

	n, err := s.store.EnrichLogsMissingGeo(ctx, s.geo, s.lookbackDays)
	if err != nil {
		if ctx.Err() != nil {
			slog.Info("geo job: enrich canceled")
			return
		}
		slog.Error("geo job: backfill failed", "err", err)
		return
	}
	if n > 0 {
		slog.Info("geo job: enrich lookup ready", "ips", n, "lookback_days", s.lookbackDays)
		// Skip edges rebuild here: on small CH hosts day INSERT+JOIN OOMs (~1.2 GiB)
		// and flips edges_agg to error. Map reads already overlay nm_geo_enrich_ip.
		slog.Info("geo job: skip edges lookback rebuild (map uses enrich overlay)")
	}
}

// Shutdown отменяет текущую задачу и ждёт завершения (или ctx).
func (s *Scheduler) Shutdown(ctx context.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.Warn("geo job: shutdown wait timeout")
	}
}
