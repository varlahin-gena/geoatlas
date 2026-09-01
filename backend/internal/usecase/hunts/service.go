package hunts

import (
	"context"
	"errors"
	"strings"
	"time"

	"geoatlas/internal/mapsearch"
	"geoatlas/internal/model"
)

// Service — CRUD saved hunts + scheduled runs.
type Service struct {
	store    Store
	runner   MapRunner
	reporter AnomalyReporter
	heavy    HeavyGate
	timeout  time.Duration
}

// HeavyGate — общий слот тяжёлых задач.
type HeavyGate interface {
	TryAcquire() bool
	Release()
}

func New(store Store, runner MapRunner, reporter AnomalyReporter, heavy HeavyGate, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &Service{store: store, runner: runner, reporter: reporter, heavy: heavy, timeout: timeout}
}

func (s *Service) List(username string) ([]Hunt, error) {
	if s == nil || s.store == nil {
		return nil, ErrUnavailable
	}
	return s.store.List(strings.TrimSpace(username))
}

func (s *Service) ListAll() ([]HuntWithAuthor, error) {
	if s == nil || s.store == nil {
		return nil, ErrUnavailable
	}
	return s.store.ListAll()
}

func (s *Service) Create(username string, in Hunt) (Hunt, error) {
	if s == nil || s.store == nil {
		return Hunt{}, ErrUnavailable
	}
	out, err := s.normalizeHunt(in)
	if err != nil {
		return Hunt{}, err
	}
	return s.store.Create(strings.TrimSpace(username), out)
}

func (s *Service) Update(username, id string, in Hunt) (Hunt, error) {
	if s == nil || s.store == nil {
		return Hunt{}, ErrUnavailable
	}
	out, err := s.normalizeHunt(in)
	if err != nil {
		return Hunt{}, err
	}
	return s.store.Update(strings.TrimSpace(username), strings.TrimSpace(id), out)
}

func (s *Service) Delete(username, id string) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	return s.store.Delete(strings.TrimSpace(username), strings.TrimSpace(id))
}

func (s *Service) RunNow(ctx context.Context, username, id string) (Hunt, RunResult, error) {
	if s == nil || s.store == nil || s.runner == nil {
		return Hunt{}, RunResult{}, ErrUnavailable
	}
	list, err := s.store.List(strings.TrimSpace(username))
	if err != nil {
		return Hunt{}, RunResult{}, err
	}
	var hunt *Hunt
	for i := range list {
		if list[i].ID == id {
			hunt = &list[i]
			break
		}
	}
	if hunt == nil {
		return Hunt{}, RunResult{}, ErrNotFound
	}
	run, updated, err := s.execute(ctx, *hunt, time.Now().UTC(), false)
	if err != nil {
		return Hunt{}, run, err
	}
	if err := s.store.SaveRun(username, id, updated); err != nil {
		return updated, run, err
	}
	return updated, run, nil
}

func (s *Service) TickScheduled(ctx context.Context, now time.Time) {
	if s == nil || s.store == nil || s.runner == nil {
		return
	}
	all, err := s.store.ListAll()
	if err != nil {
		return
	}
	for _, item := range all {
		if !shouldRunScheduled(item.Hunt, now) {
			continue
		}
		run, updated, err := s.execute(ctx, item.Hunt, now, true)
		if err != nil {
			continue
		}
		_ = s.store.SaveRun(item.Username, item.ID, updated)
		if run.Breach && s.reporter != nil {
			_ = s.reporter.ReportHuntBreach(ctx, updated, run, now)
		}
	}
}

func shouldRunScheduled(h Hunt, now time.Time) bool {
	sch := NormalizeSchedule(h.Schedule)
	if !sch.Enabled {
		return false
	}
	if h.LastRunAt == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, h.LastRunAt)
	if err != nil {
		return true
	}
	return now.Sub(last.UTC()) >= time.Duration(sch.IntervalMin)*time.Minute
}

func (s *Service) execute(ctx context.Context, hunt Hunt, now time.Time, scheduled bool) (RunResult, Hunt, error) {
	run := RunResult{At: now.UTC().Format(time.RFC3339)}
	if s.heavy != nil {
		if !s.heavy.TryAcquire() {
			run.Skipped = "heavy_busy"
			return run, hunt, errors.New("heavy task slot busy")
		}
		defer s.heavy.Release()
	}
	tr := mapStateToTimeRange(hunt.Map, now)
	cost := mapsearchAssess(hunt.Map, tr)
	run.QueryCost = string(cost.Tier)
	if cost.Tier == "heavy" && scheduled {
		run.Skipped = "query_too_heavy"
		return run, hunt, nil
	}
	tctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	snap, err := s.runner.RunMap(tctx, hunt.Map, s.timeout)
	if err != nil {
		run.Skipped = "query_failed"
		return run, hunt, err
	}
	run.EdgeCount = snap.EdgeCount
	run.RawPairs = snap.RawPairs
	if snap.Skipped != "" {
		run.Skipped = snap.Skipped
	}
	prev := previousEdgeCount(hunt.Runs)
	run.PrevEdges = prev
	sch := NormalizeSchedule(hunt.Schedule)
	run.Breach = evaluateBreach(run.EdgeCount, prev, sch)
	if prev > 0 {
		run.Ratio = float64(run.EdgeCount) / float64(prev)
	}
	hunt.Runs = append(trimRuns(hunt.Runs), run)
	hunt.LastRunAt = run.At
	hunt.UpdatedAt = run.At
	if run.Breach && s.reporter != nil && !scheduled {
		_ = s.reporter.ReportHuntBreach(ctx, hunt, run, now)
	}
	return run, hunt, nil
}

func mapsearchAssess(st MapState, tr model.TimeRange) mapsearch.MapQueryCost {
	return mapsearch.AssessMapQueryCost(mapCostInput(st, tr))
}

// TimeRangeForState — exported for adapters.
func TimeRangeForState(st MapState, now time.Time) model.TimeRange {
	return mapStateToTimeRange(st, now)
}

// CostInputForState — exported for adapters.
func CostInputForState(st MapState, now time.Time) mapsearch.MapQueryCostInput {
	return mapCostInput(st, mapStateToTimeRange(st, now))
}

func previousEdgeCount(runs []RunResult) int {
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].Skipped == "" && runs[i].EdgeCount > 0 {
			return runs[i].EdgeCount
		}
	}
	return 0
}

func evaluateBreach(edges, prev int, sch Schedule) bool {
	if sch.EdgeThreshold > 0 && edges >= sch.EdgeThreshold {
		return true
	}
	if sch.EdgeRatio >= 1 && prev > 0 && float64(edges) >= float64(prev)*sch.EdgeRatio {
		return true
	}
	return false
}

func (s *Service) normalizeHunt(in Hunt) (Hunt, error) {
	if err := ValidateName(in.Name); err != nil {
		return Hunt{}, err
	}
	if err := ValidateMapState(in.Map); err != nil {
		return Hunt{}, err
	}
	sch, err := ValidateSchedule(in.Schedule)
	if err != nil {
		return Hunt{}, err
	}
	notes := strings.TrimSpace(in.Notes)
	if len([]rune(notes)) > MaxNotesLen {
		return Hunt{}, ErrInvalidName
	}
	out := in
	out.Name = strings.TrimSpace(in.Name)
	out.Notes = notes
	out.Schedule = sch
	if out.Map.GroupBy == "" {
		out.Map.GroupBy = "city"
	}
	if out.Map.Filter == "" {
		out.Map.Filter = "all"
	}
	if out.Map.Period == "" {
		out.Map.Period = "1d"
	}
	if out.Map.Limit <= 0 {
		out.Map.Limit = 5000
	}
	return out, nil
}
