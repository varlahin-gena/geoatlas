package aggstate

import (
	"context"
	"sync"
	"time"
)

// EdgesAggStatus — состояние Ensure*/backfill для API и логов.
// PreferDailyEdgesAgg() == true только при State == "ready".
type EdgesAggStatus struct {
	State     string    `json:"state"`           // idle | pending | running | ready | error
	Phase     string    `json:"phase,omitempty"` // schema | backfill (только при running)
	Message   string    `json:"message"`
	RawRows   uint64    `json:"raw_rows"`
	AggRows   uint64    `json:"agg_rows"`
	DaysTotal int       `json:"days_total"`
	DaysDone  int       `json:"days_done"`
	StartedAt time.Time `json:"started_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

const (
	PhaseSchema   = "schema"
	PhaseBackfill = "backfill"
)

// Store holds process-local aggregate readiness flags (injectable; Default for prod).
type Store struct {
	edgesMu     sync.RWMutex
	edgesStatus EdgesAggStatus

	geoMu    sync.RWMutex
	geoReady bool

	hourlyMu    sync.RWMutex
	hourlyReady bool
}

// Default — process-wide store used by package-level helpers.
var Default = NewStore()

func NewStore() *Store {
	return &Store{
		edgesStatus: EdgesAggStatus{State: "idle", Message: "not started"},
	}
}

type aggCtxKey struct{}

// WithAgg attaches s to ctx for Scan* readiness checks (nil s → unchanged ctx).
func WithAgg(ctx context.Context, s *Store) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil {
		return ctx
	}
	return context.WithValue(ctx, aggCtxKey{}, s)
}

// AggFromContext returns the Store from ctx, or Default. Never nil.
func AggFromContext(ctx context.Context) *Store {
	if ctx != nil {
		if s, ok := ctx.Value(aggCtxKey{}).(*Store); ok && s != nil {
			return s
		}
	}
	return Default
}

func (s *Store) SetEdgesAggStatus(st EdgesAggStatus) {
	st.UpdatedAt = time.Now().UTC()
	s.edgesMu.Lock()
	s.edgesStatus = st
	s.edgesMu.Unlock()
}

func (s *Store) GetEdgesAggStatus() EdgesAggStatus {
	s.edgesMu.RLock()
	defer s.edgesMu.RUnlock()
	return s.edgesStatus
}

// PreferDailyEdgesAgg — true только после успешного полного backfill.
func (s *Store) PreferDailyEdgesAgg() bool {
	return s.GetEdgesAggStatus().State == "ready"
}

// PreferGeoEdgesAgg — true после успешного BackfillGeoEdgesAgg (city+country+continent).
func (s *Store) PreferGeoEdgesAgg() bool {
	s.geoMu.RLock()
	defer s.geoMu.RUnlock()
	return s.geoReady
}

func (s *Store) SetGeoEdgesAggReady(v bool) {
	s.geoMu.Lock()
	s.geoReady = v
	s.geoMu.Unlock()
}

// PreferHourlyEdgesAgg — true после успешного backfill hourly IP-агрегата.
func (s *Store) PreferHourlyEdgesAgg() bool {
	s.hourlyMu.RLock()
	defer s.hourlyMu.RUnlock()
	return s.hourlyReady
}

func (s *Store) SetHourlyEdgesAggReady(v bool) {
	s.hourlyMu.Lock()
	s.hourlyReady = v
	s.hourlyMu.Unlock()
}

// Package-level wrappers — совместимость с существующими call sites.

func SetEdgesAggStatus(st EdgesAggStatus) { Default.SetEdgesAggStatus(st) }
func GetEdgesAggStatus() EdgesAggStatus   { return Default.GetEdgesAggStatus() }
func PreferDailyEdgesAgg() bool           { return Default.PreferDailyEdgesAgg() }

func PreferGeoEdgesAgg() bool       { return Default.PreferGeoEdgesAgg() }
func SetGeoEdgesAggReady(v bool)    { Default.SetGeoEdgesAggReady(v) }
func PreferHourlyEdgesAgg() bool    { return Default.PreferHourlyEdgesAgg() }
func SetHourlyEdgesAggReady(v bool) { Default.SetHourlyEdgesAggReady(v) }
