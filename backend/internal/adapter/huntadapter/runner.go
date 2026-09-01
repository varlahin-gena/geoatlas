package huntadapter

import (
	"context"
	"time"

	"geoatlas/internal/mapsearch"
	usecaseevents "geoatlas/internal/usecase/events"
	"geoatlas/internal/usecase/hunts"
)

// MapRunner — hunts.MapRunner через events UC.
type MapRunner struct {
	Events *usecaseevents.Service
}

func (r MapRunner) RunMap(ctx context.Context, st hunts.MapState, timeout time.Duration) (hunts.MapSnapshot, error) {
	if r.Events == nil {
		return hunts.MapSnapshot{}, hunts.ErrUnavailable
	}
	now := time.Now().UTC()
	cost := mapsearch.AssessMapQueryCost(hunts.CostInputForState(st, now))
	limit := hunts.MapLimit(st)
	if applied, capped := mapsearch.EffectiveMapLimit(limit, cost); capped && applied > 0 {
		limit = applied
	}
	in := usecaseevents.GetMapInput{
		TimeRange:  hunts.TimeRangeForState(st, now),
		Limit:      limit,
		GroupBy:    st.GroupBy,
		Filter:     st.Filter,
		Country:    st.Country,
		Query:      st.Query,
		DataSource: st.DataSource,
		Timeout:    timeout,
	}
	res, err := r.Events.GetMap(ctx, in)
	if err != nil {
		return hunts.MapSnapshot{}, err
	}
	return hunts.MapSnapshot{
		EdgeCount: len(res.Lines),
		RawPairs:  res.RawPairs,
		QueryCost: cost.Tier,
	}, nil
}

// HeavyGate adapts heavytask.Limiter.
type HeavyGate struct {
	Try func() bool
	Rel func()
}

func (g HeavyGate) TryAcquire() bool {
	if g.Try == nil {
		return true
	}
	return g.Try()
}

func (g HeavyGate) Release() {
	if g.Rel != nil {
		g.Rel()
	}
}
