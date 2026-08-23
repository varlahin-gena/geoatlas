package retention

import (
	"context"
	"errors"
	"fmt"
	"time"

	"geoatlas/internal/apperr"
)

const (
	MinDays = 1
	MaxDays = 730

	DefaultTrafficLogsDays   = 30
	DefaultEdgesDays         = 30
	DefaultParseErrorsDays   = 7
	DefaultSystemMetricsDays = 7
)

var (
	ErrInvalidDays = apperr.InvalidInput("invalid retention days")
)

// Defaults — значения из EnsureBaseSchema / Ensure*.
func Defaults() Settings {
	return Settings{
		TrafficLogsDays:   DefaultTrafficLogsDays,
		EdgesDays:         DefaultEdgesDays,
		ParseErrorsDays:   DefaultParseErrorsDays,
		SystemMetricsDays: DefaultSystemMetricsDays,
	}
}

// Service — GET/PUT retention + Apply при bootstrap.
type Service struct {
	store   Store
	applier TTLApplier
}

func New(store Store, applier TTLApplier) *Service {
	return &Service{store: store, applier: applier}
}

func (s *Service) Get() (Settings, error) {
	if s == nil || s.store == nil {
		return Defaults(), nil
	}
	out, err := s.store.Load()
	if err != nil {
		return Settings{}, err
	}
	return normalizeOrDefaults(out), nil
}

func (s *Service) Update(ctx context.Context, in Settings) (Settings, error) {
	if s == nil || s.store == nil || s.applier == nil {
		return Settings{}, errors.New("retention service not configured")
	}
	out, err := validate(in)
	if err != nil {
		return Settings{}, err
	}
	out.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.store.Save(out); err != nil {
		return Settings{}, err
	}
	if err := s.applyAll(ctx, out); err != nil {
		return Settings{}, err
	}
	return out, nil
}

// ApplyFromStore применяет сохранённые (или дефолтные) TTL к ClickHouse.
func (s *Service) ApplyFromStore(ctx context.Context) error {
	if s == nil || s.applier == nil {
		return nil
	}
	settings, err := s.Get()
	if err != nil {
		return err
	}
	return s.applyAll(ctx, settings)
}

func (s *Service) applyAll(ctx context.Context, st Settings) error {
	if err := s.applier.ApplyTrafficLogs(ctx, st.TrafficLogsDays); err != nil {
		return fmt.Errorf("traffic_logs: %w", err)
	}
	if err := s.applier.ApplyEdges(ctx, st.EdgesDays); err != nil {
		return fmt.Errorf("edges: %w", err)
	}
	if err := s.applier.ApplyParseErrors(ctx, st.ParseErrorsDays); err != nil {
		return fmt.Errorf("parse_errors: %w", err)
	}
	if err := s.applier.ApplySystemMetrics(ctx, st.SystemMetricsDays); err != nil {
		return fmt.Errorf("system_metrics: %w", err)
	}
	return nil
}

func validate(in Settings) (Settings, error) {
	check := func(name string, v int) error {
		if v < MinDays || v > MaxDays {
			return fmt.Errorf("%w: %s must be %d..%d (got %d)", ErrInvalidDays, name, MinDays, MaxDays, v)
		}
		return nil
	}
	if err := check("traffic_logs_days", in.TrafficLogsDays); err != nil {
		return Settings{}, err
	}
	if err := check("edges_days", in.EdgesDays); err != nil {
		return Settings{}, err
	}
	if err := check("parse_errors_days", in.ParseErrorsDays); err != nil {
		return Settings{}, err
	}
	if err := check("system_metrics_days", in.SystemMetricsDays); err != nil {
		return Settings{}, err
	}
	return Settings{
		TrafficLogsDays:   in.TrafficLogsDays,
		EdgesDays:         in.EdgesDays,
		ParseErrorsDays:   in.ParseErrorsDays,
		SystemMetricsDays: in.SystemMetricsDays,
	}, nil
}

func normalizeOrDefaults(s Settings) Settings {
	d := Defaults()
	if s.TrafficLogsDays < MinDays || s.TrafficLogsDays > MaxDays {
		s.TrafficLogsDays = d.TrafficLogsDays
	}
	if s.EdgesDays < MinDays || s.EdgesDays > MaxDays {
		s.EdgesDays = d.EdgesDays
	}
	if s.ParseErrorsDays < MinDays || s.ParseErrorsDays > MaxDays {
		s.ParseErrorsDays = d.ParseErrorsDays
	}
	if s.SystemMetricsDays < MinDays || s.SystemMetricsDays > MaxDays {
		s.SystemMetricsDays = d.SystemMetricsDays
	}
	return s
}

func IsClientError(err error) bool {
	return errors.Is(err, ErrInvalidDays) || apperr.IsClient(err)
}
